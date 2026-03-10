//go:build !windows

// Package rawinput provides a Windows Raw Input global keyboard and mouse reader.
// On non-Windows platforms this package provides a no-op stub that never emits events.
package rawinput

import (
	"context"

	"github.com/soar/inputview/internal/input"
)

// HIDInputCallback is the type for raw HID input event callbacks.
// On non-Windows platforms this type exists for API compatibility only.
type HIDInputCallback func(hDevice uintptr, rawData []byte, reportSize uint32)

// HIDDeviceChangeCallback is the type for HID device arrival/removal callbacks.
// On non-Windows platforms this type exists for API compatibility only.
type HIDDeviceChangeCallback func(added bool, hDevice uintptr)

// Reader is a no-op keyboard/mouse reader on non-Windows platforms.
type Reader struct {
	changes chan input.KeyMouseState
}

// New returns a stub Reader that never emits events.
func New() *Reader {
	return &Reader{
		changes: make(chan input.KeyMouseState),
	}
}

// SetMouseSensitivity is a no-op on non-Windows platforms.
func (r *Reader) SetMouseSensitivity(_ float32) {}

// Changes returns a channel that is never written to on non-Windows platforms.
func (r *Reader) Changes() <-chan input.KeyMouseState {
	return r.changes
}

// RegisterHIDCallback is a no-op on non-Windows platforms.
func (r *Reader) RegisterHIDCallback(_, _ uint16, _ HIDInputCallback, _ HIDDeviceChangeCallback) {}

// Run blocks until ctx is cancelled, then returns.
func (r *Reader) Run(ctx context.Context) {
	<-ctx.Done()
}
