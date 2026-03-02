//go:build !windows

// Package console provides cross-platform console detection and signal handling.
// On non-Windows platforms, this package provides stub implementations.
package console

// IsRunningFromConsole returns true on non-Windows platforms as they always run in console mode.
func IsRunningFromConsole() bool {
	return true
}

// SetupConsoleHandler returns a no-op function on non-Windows platforms.
// Go's standard os.Interrupt signal handling works fine on Unix-like systems.
func SetupConsoleHandler(shutdownChan chan struct{}) func() {
	return func() {}
}
