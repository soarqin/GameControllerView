//go:build !release

package main

import (
	"log"
	"runtime"

	"github.com/soar/gamecontrollerview/internal/console"
	"github.com/soar/gamecontrollerview/internal/gamepad"
)

// guiMode is false in dev/console builds (default).
const guiMode = false

// setupShutdown sets up console-mode shutdown handling.
// Returns a channel closed on Ctrl+C (Windows) and a re-register callback for SDL init.
func setupShutdown(reader *gamepad.Reader) (extraShutdownCh <-chan struct{}, onSDLInit func()) {
	ch := make(chan struct{}, 1)
	registerHandler := console.SetupConsoleHandler(ch)

	if runtime.GOOS == "windows" {
		onSDLInit = registerHandler
	} else {
		onSDLInit = func() {}
	}

	if runtime.GOOS == "windows" {
		log.Println("Running in console mode. Press Ctrl+C or Ctrl+Break to exit.")
	} else {
		log.Println("Press Ctrl+C to exit.")
	}

	return ch, onSDLInit
}
