//go:build !windows

package gamepad

// hidDeviceInfo is a stub type on non-Windows platforms.
// The HID input path is Windows-only; this keeps the compiler happy
// when shared code references hidDeviceInfo fields.
type hidDeviceInfo struct {
	hDevice     uintptr
	isXInput    bool
	isInvalid   bool
	mapping     *DeviceMapping
	sdlMap      *SDLMapping // SDL gamecontrollerdb mapping (may be nil)
	axisOrder   []hidAxisEntry
	axisMap     map[uint16]string
	buttonCount uint16
	name        string
}
