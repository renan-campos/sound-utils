package main

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/renan-campos/sound-utils/pkg/alsa"
	. "github.com/renan-campos/sound-utils/pkg/logging"
)

func main() {
	if len(os.Args) < 2 {
		Stderr("Card name expected")
		os.Exit(1)
	}

	cardName := os.Args[1]

	card, err := alsa.FindCard(cardName)
	defer alsa.CloseCard(card)
	if err != nil {
		Stderr(err.Error())
		os.Exit(1)
	}

	devices, err := card.Devices()
	if err != nil {
		Stderr(errors.Wrap(err, "Failed to get card devices").Error())
		os.Exit(1)
	}
	fmt.Println("===", card, "Device List ===")
	for _, device := range devices {
		fmt.Printf(`
%-15s:%d
%-15s:%s
%-15s:%v
%-15s:%v
%-15s:%v
`,
			"Device Number", device.Number,
			"Title", device.Title,
			"Play?", device.Play,
			"Record?", device.Record,
			"Path", device.Path,
		)
	}
}
