//go:build release

package main

import (
	"runtime"

	"github.com/soar/gamecontrollerview/internal/gamepad"
	"github.com/soar/gamecontrollerview/internal/tray"
)

// guiMode is true in release builds (-tags release).
const guiMode = true

// setupShutdown sets up GUI-mode shutdown handling via system tray (Windows).
// Returns a channel closed when the user requests exit from the tray.
// The extraShutdownCh is nil on non-Windows platforms (only OS signals are used).
func setupShutdown(reader *gamepad.Reader) (extraShutdownCh <-chan struct{}, onSDLInit func()) {
	if runtime.GOOS == "windows" {
		ch := make(chan struct{})
		go func() {
			t := tray.New(func() {
				close(ch)
			})
			t.Run(tray.GetIcon())
		}()
		return ch, func() {}
	}
	return nil, func() {}
}
