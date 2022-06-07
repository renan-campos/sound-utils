package main

import (
	"fmt"
	"os"

	"github.com/renan-campos/sound-utils/pkg/logging"

	"github.com/pkg/errors"
	"github.com/renan-campos/sound-utils/pkg/alsa"
)

func usage() string {
	return fmt.Sprintf(`%s "Wav File"
	Plays a WAV file on the specified card and device
`, os.Args[0])
}

func main() {

	logging.DisplayDebug = true

	if len(os.Args) < 2 {
		logging.Stderr("Insufficient number of arguments")
		logging.Stderr(usage())
		os.Exit(1)
	}

	os.Environ()
	cardName := os.Getenv("ALSA_CARDNAME")
	deviceName := os.Getenv("ALSA_DEVICENAME")
	wavFileName := os.Args[1]

	card, err := alsa.FindCard(cardName)
	defer alsa.CloseCard(card)
	if err != nil {
		logging.Stderr(errors.Wrap(err, "Failed to find card").Error())
		os.Exit(1)
	}
	logging.Debugf("%s found.\n", card)

	device, err := alsa.FindPlayableDevice(card, deviceName)
	if err != nil {
		logging.Stderr(errors.Wrap(err, "Failed to determine playable device").Error())
		os.Exit(1)
	}
	logging.Debugf("%s found.\n", device)

	if err := alsa.PlayWav(device, wavFileName); err != nil {
		logging.Stderr(errors.Wrap(err, "failed to play wav file on device").Error())
		os.Exit(1)
	}
}
