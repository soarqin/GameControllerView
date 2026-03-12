//go:build !release

package main

import (
	"log"
	"runtime"

	"github.com/soar/inputview/internal/console"
)

// guiMode is false in dev/console builds (default).
const guiMode = false

// setupShutdown sets up console-mode shutdown handling.
// exeDir is passed for API symmetry with the release build; it is not used in
// dev/console mode.
// Returns a channel that is closed on Ctrl+C / Ctrl+Break (Windows).
func setupShutdown(exeDir string) <-chan struct{} {
	ch := make(chan struct{}, 1)
	console.SetupConsoleHandler(ch)

	if runtime.GOOS == "windows" {
		log.Println("Running in console mode. Press Ctrl+C or Ctrl+Break to exit.")
	} else {
		log.Println("Press Ctrl+C to exit.")
	}

	return ch
}
