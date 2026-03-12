//go:build release

package main

import (
	"path/filepath"
	"runtime"

	"github.com/soar/inputview/internal/overlay"
	"github.com/soar/inputview/internal/tray"
)

// guiMode is true in release builds (-tags release).
const guiMode = true

// setupShutdown sets up GUI-mode shutdown handling via system tray (Windows).
// Returns a channel closed when the user requests exit from the tray menu.
// Returns nil on non-Windows platforms (only OS signals are used).
func setupShutdown(exeDir string) <-chan struct{} {
	if runtime.GOOS == "windows" {
		overlays := overlay.ScanDir(filepath.Join(exeDir, "overlays"))
		ch := make(chan struct{})
		go func() {
			t := tray.New(func() {
				close(ch)
			}, overlays, ":8080")
			t.Run(tray.GetIcon())
		}()
		return ch
	}
	return nil
}
