package alsa

import "fmt"

type cardNotFound struct{ cardName string }

func (cnf *cardNotFound) Error() string {
	return fmt.Sprintf("Card %q not found", cnf.cardName)
}

type DeviceNotFound struct{ deviceName string }

func (cnf *DeviceNotFound) Error() string {
	return fmt.Sprintf("Device %q not found", cnf.deviceName)
}

type deviceNotPlayable struct{ deviceName string }

func (d *deviceNotPlayable) Error() string {
	return fmt.Sprint("unable to play audio on device %q", d)
}
