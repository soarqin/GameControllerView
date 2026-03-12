//go:build !windows

package tray

import "log"

// copyToClipboard is a no-op stub on non-Windows platforms.
// The tray is only active on Windows in release builds, so this path
// is never reached at runtime.
func copyToClipboard(text string) {
	log.Printf("Clipboard: copyToClipboard not implemented on this platform (text: %s)", text)
}
