package alsa

import (
	"github.com/pkg/errors"
	"github.com/yobert/alsa"
)

func FindPlayableDevice(card *alsa.Card, deviceName string) (*alsa.Device, error) {
	devices, err := card.Devices()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get card devices")
	}
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

func FindRecordableDevice(card *alsa.Card, deviceName string) (*alsa.Device, error) {
	devices, err := card.Devices()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get card devices")
	}
	for _, device := range devices {
		if device.Type != alsa.PCM || !device.Record {
			continue
		}
		if device.Title == deviceName {
			return device, nil
		}
	}
	return nil, &deviceNotFound{deviceName: deviceName}
}
