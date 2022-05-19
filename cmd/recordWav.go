// record some audio and save it as a WAV file
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/renan-campos/sound-utils/pkg/alsa"
	. "github.com/renan-campos/sound-utils/pkg/logging"
)

func main() {
	var (
		channels     int
		rate         int
		duration_str string
		file         string
	)

	flag.IntVar(&channels, "channels", 2, "Channels (1 for mono, 2 for stereo)")
	flag.IntVar(&rate, "rate", 44100, "Frame rate (Hz)")
	flag.StringVar(&duration_str, "duration", "5s", "Recording duration")
	flag.StringVar(&file, "file", "out.wave", "Output file")
	flag.Parse()

	os.Environ()
	cardName := os.Getenv("ALSA_CARDNAME")
	deviceName := os.Getenv("ALSA_DEVICENAME")

	duration, err := time.ParseDuration(duration_str)
	if err != nil {
		fmt.Println("Cannot parse duration:", err)
		os.Exit(1)
	}

	card, err := alsa.FindCard(cardName)
	defer alsa.CloseCard(card)
	if err != nil {
		Stderr(errors.Wrap(err, "Failed to find card").Error())
		os.Exit(1)
	}
	fmt.Println(card, "found!")

	device, err := alsa.FindRecordableDevice(card, deviceName)
	if err != nil {
		Stderr(errors.Wrap(err, "Failed to determine recordable device").Error())
		os.Exit(1)
	}
	fmt.Println("  ", device, "found!")

	fmt.Printf("Recording device: %v\n", device)

	recording, err := alsa.RecordWav(device, duration, channels, rate)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = alsa.SaveWav(recording, file)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// success!
	return
}
