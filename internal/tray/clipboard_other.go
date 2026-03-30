//go:build !windows

package tray

import "log/slog"

// copyToClipboard is a no-op stub on non-Windows platforms.
// The tray is only active on Windows in release builds, so this path
// is never reached at runtime.
func copyToClipboard(text string) {
	slog.Warn("clipboard: copyToClipboard not implemented on this platform", "text", text)
}
