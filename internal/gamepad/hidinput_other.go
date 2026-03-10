//go:build !windows

package gamepad

// hidDeviceInfo is a stub type on non-Windows platforms.
// The HID input path is Windows-only; this keeps the compiler happy.
type hidDeviceInfo struct {
	hDevice   uintptr
	isXInput  bool
	isInvalid bool
	mapping   *DeviceMapping
	name      string
}
