package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/pkg/errors"
	"github.com/yobert/alsa"
)

func usage() string {
	return fmt.Sprintf(`%s "Card Name" "Device Name" "Wav File"
	Plays a WAV file on the specified card and device
`, os.Args[0])
}

func stderr(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", a...)
}

func main() {
	if len(os.Args) < 3 {
		stderr("Insufficient number of arguments")
		stderr(usage())
		os.Exit(1)
	}

	cardName := os.Args[1]
	deviceName := os.Args[2]
	wavFileName := os.Args[3]

	cards, err := alsa.OpenCards()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer alsa.CloseCards(cards)

	card, err := findCard(cards, cardName)
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
	device, err := findPlayableDevice(devices, deviceName)
	if err != nil {
		stderr(errors.Wrap(err, "Failed to determine playable device").Error())
		os.Exit(1)
	}
	fmt.Println("  ", device, "found!")

	if err := playWav(device, wavFileName); err != nil {
		stderr(errors.Wrap(err, "failed to play wav file on device").Error())
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
	return fmt.Sprintf("Device %q not found", cnf.deviceName)
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

func playWav(device *alsa.Device, wavFileName string) error {
	var err error

	f, err := os.Open(wavFileName)
	if err != nil {
		stderr(errors.Wrapf(err, "failed to open %q", wavFileName).Error())
		os.Exit(1)
	}
	wavDecoder := wav.NewDecoder(f)
	if !wavDecoder.IsValidFile() {
		return fmt.Errorf("%q is not a valid wav file", wavFileName)
	}

	if err = device.Open(); err != nil {
		return err
	}

	dur, err := wavDecoder.Duration()
	if err != nil {
		return errors.Wrapf(err, "failed to determine duration of %q", wavFileName)
	}

	// Cleanup device when done or force cleanup 3 seconds after the duration of the wav file.
	wg := sync.WaitGroup{}
	wg.Add(1)
	defer wg.Wait()
	childCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(dur).Add(3*time.Second))
	defer cancel()
	go func(ctx context.Context) {
		defer device.Close()
		<-ctx.Done()
		fmt.Println("Closing device.")
		wg.Done()
	}(childCtx)

	wavFormat := wavDecoder.Format()
	// Note:
	// When playing a wav file:
	// The number of channels should be what the file specifies.
	channels, err := device.NegotiateChannels(wavFormat.NumChannels)
	if err != nil {
		return err
	}

	// Note:
	// When playing a wav file:
	// The sample rate should be that or higher than what the file specifieds.
	// The sample rate should be greater than or equal to what the file specifies.
	rate, err := device.NegotiateRate(wavFormat.SampleRate)
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
	format, err := device.NegotiateFormat(alsa.S32_LE)
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

	inbuf := audio.IntBuffer{
		Format: wavFormat,
		Data:   make([]int, periodSize*channels),
	}

	for !wavDecoder.EOF() {
		n, err := wavDecoder.PCMBuffer(&inbuf)
		if err != nil {
			return errors.Wrap(err, "failed to fill buffer with wav data")
		}
		if n == 0 {
			break
		}

		frames := bytes.Buffer{}
		for _, sample := range inbuf.Data {
			switch format {
			case alsa.S32_LE:
				err := binary.Write(&frames, binary.LittleEndian, int32(sample))
				if err != nil {
					fmt.Println(err)
				}
			default:
				return fmt.Errorf("Unhandled sample format: %v", format)
			}
		}

		if err := device.Write(frames.Bytes(), periodSize); err != nil {
			return err
		}
	}
	// Wait for playback to complete.
	fmt.Printf("Playback should be complete now.\n")

	return nil
}
