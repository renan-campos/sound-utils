package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/yobert/alsa"
)

func usage() string {
	return fmt.Sprintf(`%s "Card Name" "Device Name"
	Plays a two second A4 sine wave specified devices on the card.
	If no device is specified, the first playable device on the card is used.
`, os.Args[0])
}

func stderr(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", a...)
}

func main() {
	if len(os.Args) < 2 {
		stderr("Insufficient number of arguments")
		stderr(usage())
		os.Exit(1)
	}

	cards, err := alsa.OpenCards()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer alsa.CloseCards(cards)

	card, err := findCard(cards, os.Args[1])
	if err != nil {
		stderr(err.Error())
		os.Exit(1)
	}
	fmt.Println(card, "found!")
	// Shouldn't I close all of the cards that aren't being used?

	devices, err := card.Devices()
	if err != nil {
		stderr(errors.Wrap(err, "Failed to get card devices").Error())
		os.Exit(1)
	}
	var deviceName string
	if len(os.Args) > 2 {
		deviceName = os.Args[2]
	}
	device, err := findPlayableDevice(devices, deviceName)
	if err != nil {
		stderr(errors.Wrap(err, "Failed to determine playable device").Error())
		os.Exit(1)
	}
	fmt.Println("  ", device, "found!")

	if err := beepDevice(device); err != nil {
		stderr(errors.Wrap(err, "failed to play audio on device").Error())
		os.Exit(1)
	}
}

type cardNotFound struct{ cardName string }

func (cnf *cardNotFound) Error() string {
	return fmt.Sprintf("Card %q not found", cnf.cardName)
}

func findCard(cards []*alsa.Card, name string) (*alsa.Card, error) {
	for _, card := range cards {
		if card.Title == name {
			return card, nil
		}
	}
	return nil, &cardNotFound{cardName: name}
}

type deviceNotFound struct{ deviceName string }

func (cnf *deviceNotFound) Error() string {
	return fmt.Sprintf("device %q not found", cnf.deviceName)
}
func findPlayableDevice(devices []*alsa.Device, deviceName string) (*alsa.Device, error) {
	for _, device := range devices {
		if device.Type != alsa.PCM || !device.Play {
			continue
		}
		if device.Title == deviceName {
			return device, nil
		}
	}
	return nil, &deviceNotFound{deviceName: deviceName}
}

type deviceNotPlayable struct{ deviceName string }

func (d *deviceNotPlayable) Error() string {
	return fmt.Sprint("unable to play audio on device %q", d)
}

func beepDevice(device *alsa.Device) error {
	var err error

	if device.Type != alsa.PCM || !device.Play {
		return &deviceNotPlayable{deviceName: device.Title}
	}

	if err = device.Open(); err != nil {
		return err
	}

	// Cleanup device when done or force cleanup after 3 seconds.
	wg := sync.WaitGroup{}
	wg.Add(1)
	defer wg.Wait()
	childCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(201*time.Second))
	defer cancel()
	go func(ctx context.Context) {
		defer device.Close()
		<-ctx.Done()
		fmt.Println("Closing device.")
		wg.Done()
	}(childCtx)

	// Note:
	// When playing a wav file:
	// The number of channels should be what the file specifies.
	channels, err := device.NegotiateChannels(1, 2)
	if err != nil {
		return err
	}

	// Note:
	// When playing a wav file:
	// The sample rate should be that or higher than what the file specifieds.
	// The sample rate should be greater than or equal to what the file specifies.
	rate, err := device.NegotiateRate(44100)
	if err != nil {
		return err
	}

	// Note:
	// When playing a wav file:
	// The format should be what the wav format will be.
	// In the case of wav, the codec library will have int.
	// But the ratio between sample rate and bytes per second
	// of the file I was reading was 1 byte per sample.
	// This means that the data format will be S8_LE (assuming little endian)
	// If this is the case, the data should be set to it or higher,
	// and the buffer data needs to adapt to what it was set to.
	format, err := device.NegotiateFormat(alsa.S16_LE, alsa.S32_LE)
	if err != nil {
		return err
	}

	// A 50ms period is a sensible value to test low-ish latency.
	// We adjust the buffer so it's of minimal size (period * 2) since it appear ALSA won't
	// start playback until the buffer has been filled to a certain degree and the automatic
	// buffer size can be quite large.
	// Some devices only accept even periods while others want powers of 2.
	wantPeriodSize := 2048 // 46ms @ 44100Hz

	periodSize, err := device.NegotiatePeriodSize(wantPeriodSize)
	if err != nil {
		return err
	}

	bufferSize, err := device.NegotiateBufferSize(wantPeriodSize * 2)
	if err != nil {
		return err
	}

	if err = device.Prepare(); err != nil {
		return err
	}

	fmt.Printf("Negotiated parameters: %d channels, %d hz, %v, %d period size, %d buffer size\n",
		channels, rate, format, periodSize, bufferSize)

	// Play 2 seconds of beep.
	duration := 2 * time.Second
	t := time.NewTimer(duration)
	for t := 0.; t < duration.Seconds(); {
		var buf bytes.Buffer

		for i := 0; i < periodSize; i++ {
			v := math.Sin(t * 2 * math.Pi * 440) // A4
			v *= 0.1                             // make a little quieter

			switch format {
			case alsa.S16_LE:
				sample := int16(v * math.MaxInt16)

				for c := 0; c < channels; c++ {
					binary.Write(&buf, binary.LittleEndian, sample)
				}

			case alsa.S32_LE:
				sample := int32(v * math.MaxInt32)

				for c := 0; c < channels; c++ {
					binary.Write(&buf, binary.LittleEndian, sample)
				}

			default:
				return fmt.Errorf("Unhandled sample format: %v", format)
			}

			t += 1 / float64(rate)
		}

		if err := device.Write(buf.Bytes(), periodSize); err != nil {
			return err
		}
	}
	// Wait for playback to complete.
	<-t.C
	fmt.Printf("Playback should be complete now.\n")
	time.Sleep(1 * time.Second) // To allow a human to compare real playback end with supposed.

	return nil
}
