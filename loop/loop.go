/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2025 KarpelesLab. All Rights Reserved.
 */

package loop

import (
	"errors"
	"io"
	"os"
	"sync"

	"github.com/KarpelesLab/wireguard-loop-go/tun"
)

const DefaultMTU = 1420

type Device struct {
	events   chan tun.Event
	closed   chan struct{}
	mtu      int
	name     string
	incoming chan [][]byte
	mu       sync.Mutex
}

// CreateLoop creates a new loop device that echoes packets back
func CreateLoop(name string, mtu int) (tun.Device, error) {
	if mtu <= 0 {
		mtu = DefaultMTU
	}

	d := &Device{
		events:   make(chan tun.Event, 10),
		closed:   make(chan struct{}),
		mtu:      mtu,
		name:     name,
		incoming: make(chan [][]byte, 128000),
	}

	// Send EventUp to signal the device is ready
	d.events <- tun.EventUp

	return d, nil
}

func (d *Device) File() *os.File {
	// Loop device doesn't need a file descriptor
	return nil
}

func (d *Device) Read(bufs [][]byte, sizes []int, offset int) (n int, err error) {
	select {
	case <-d.closed:
		return 0, io.ErrClosedPipe
	case packets := <-d.incoming:
		// Copy received packets to the provided buffers
		n = len(packets)
		if n > len(bufs) {
			n = len(bufs)
		}
		for i := 0; i < n; i++ {
			if i >= len(sizes) {
				break
			}
			packet := packets[i]
			if len(packet) > len(bufs[i])-offset {
				return 0, errors.New("packet too large for buffer")
			}
			copy(bufs[i][offset:], packet)
			sizes[i] = len(packet)
		}
		return n, nil
	}
}

func (d *Device) Write(bufs [][]byte, offset int) (int, error) {
	select {
	case <-d.closed:
		return 0, io.ErrClosedPipe
	default:
		// Loop back: make a copy of the packets and send them back
		packets := make([][]byte, len(bufs))
		for i, buf := range bufs {
			if offset >= len(buf) {
				continue
			}
			packet := make([]byte, len(buf)-offset)
			copy(packet, buf[offset:])
			packets[i] = packet
		}

		// Blocking send - wait if queue is full
		select {
		case <-d.closed:
			return 0, io.ErrClosedPipe
		case d.incoming <- packets:
			return len(bufs), nil
		}
	}
}

func (d *Device) MTU() (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.mtu, nil
}

func (d *Device) Name() (string, error) {
	return d.name, nil
}

func (d *Device) Events() <-chan tun.Event {
	return d.events
}

func (d *Device) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	select {
	case <-d.closed:
		return nil
	default:
		close(d.closed)
		close(d.events)
		close(d.incoming)
	}
	return nil
}

func (d *Device) BatchSize() int {
	// Return a reasonable batch size for packet processing
	return 128
}