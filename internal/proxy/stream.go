package proxy

import "errors"

type Device struct {
	id int

	inUse   bool
	clients []chan struct{}

	channel string
	program string
}

func (d *Device) GetStream(channel string, program string) error {
	if d.inUse {
		if channel != d.channel && program != d.program {
			return errors.New("device in use")
		}
	}

	d.inUse = true
	// todo
	go d.streamThread()

	return nil
}

func (d *Device) streamThread() {

}
