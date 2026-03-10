//go:build windows

package gamepad

import (
	"syscall"
	"unsafe"
)

// XInput DLL loading with fallback chain: xinput1_4 (Win8+) → xinput1_3 (Win7) → xinput9_1_0 (Vista)
var (
	modXInput                   *syscall.LazyDLL
	procXInputGetState          *syscall.LazyProc
	procXInputGetCapabilities   *syscall.LazyProc
	procXInputGetStateEx        *syscall.LazyProc // ordinal 100: includes Guide button
	procXInputGetCapabilitiesEx *syscall.LazyProc // ordinal 108: includes VID/PID (undocumented, Win8+)
)

func init() {
	modXInput = loadXInputDLL()
	procXInputGetState = modXInput.NewProc("XInputGetState")
	procXInputGetCapabilities = modXInput.NewProc("XInputGetCapabilities")
	// Ordinal 100: XInputGetStateEx — same as XInputGetState but Buttons includes Guide (0x0400).
	// Available since xinput1_3.dll; silently falls back if unavailable.
	procXInputGetStateEx = modXInput.NewProc("#100")
	// Ordinal 108: XInputGetCapabilitiesEx — extends XINPUT_CAPABILITIES with VID/PID.
	// Only present in xinput1_4.dll (Windows 8+); may not exist on older systems.
	procXInputGetCapabilitiesEx = modXInput.NewProc("#108")
}

// loadXInputDLL tries to load the best available XInput DLL.
func loadXInputDLL() *syscall.LazyDLL {
	for _, name := range []string{"xinput1_4.dll", "xinput1_3.dll", "xinput9_1_0.dll"} {
		dll := syscall.NewLazyDLL(name)
		if err := dll.Load(); err == nil {
			return dll
		}
	}
	// Return a lazy DLL that will fail loudly at first call.
	return syscall.NewLazyDLL("xinput1_4.dll")
}

// xinputMaxControllers is the maximum number of controllers XInput supports simultaneously.
const xinputMaxControllers = 4

// XInput error codes
const (
	errorSuccess            uint32 = 0
	errorDeviceNotConnected uint32 = 1167 // ERROR_DEVICE_NOT_CONNECTED
)

// XInput button bitmasks (XINPUT_GAMEPAD_* constants)
const (
	xiDpadUp        uint16 = 0x0001
	xiDpadDown      uint16 = 0x0002
	xiDpadLeft      uint16 = 0x0004
	xiDpadRight     uint16 = 0x0008
	xiStart         uint16 = 0x0010
	xiBack          uint16 = 0x0020
	xiLeftThumb     uint16 = 0x0040
	xiRightThumb    uint16 = 0x0080
	xiLeftShoulder  uint16 = 0x0100
	xiRightShoulder uint16 = 0x0200
	xiGuide         uint16 = 0x0400 // only via XInputGetStateEx (ordinal 100)
	xiA             uint16 = 0x1000
	xiB             uint16 = 0x2000
	xiX             uint16 = 0x4000
	xiY             uint16 = 0x8000
)

// xinputGamepad mirrors XINPUT_GAMEPAD.
type xinputGamepad struct {
	Buttons      uint16
	LeftTrigger  uint8
	RightTrigger uint8
	ThumbLX      int16
	ThumbLY      int16
	ThumbRX      int16
	ThumbRY      int16
}

// xinputState mirrors XINPUT_STATE.
type xinputState struct {
	PacketNumber uint32
	Gamepad      xinputGamepad
}

// xinputCapabilities mirrors XINPUT_CAPABILITIES.
type xinputCapabilities struct {
	Type      uint8
	SubType   uint8
	Flags     uint16
	Gamepad   xinputGamepad
	Vibration [4]byte // XINPUT_VIBRATION: two uint16 fields, not used here
}

// xinputCapabilitiesEx mirrors the undocumented XINPUT_CAPABILITIES_EX structure
// returned by XInputGetCapabilitiesEx (ordinal 108 in xinput1_4.dll, Windows 8+).
// Layout confirmed by reverse engineering and community documentation.
type xinputCapabilitiesEx struct {
	Capabilities  xinputCapabilities
	VendorID      uint16
	ProductID     uint16
	VersionNumber uint16
	Unknown1      uint16
	Unknown2      uint32
}

// xiGetStateEx calls XInputGetStateEx (ordinal 100) which populates the Guide button bit.
// Falls back to XInputGetState if ordinal 100 is unavailable.
func xiGetStateEx(userIndex uint32, state *xinputState) uint32 {
	// Try the extended version first (includes Guide button)
	if err := procXInputGetStateEx.Find(); err == nil {
		ret, _, _ := procXInputGetStateEx.Call(
			uintptr(userIndex),
			uintptr(unsafe.Pointer(state)),
		)
		return uint32(ret)
	}
	// Fall back to standard XInputGetState
	ret, _, _ := procXInputGetState.Call(
		uintptr(userIndex),
		uintptr(unsafe.Pointer(state)),
	)
	return uint32(ret)
}

// xiGetCapabilitiesEx calls the undocumented XInputGetCapabilitiesEx (ordinal 108)
// to obtain VID/PID. Returns (vendorID, productID, true) on success, or (0, 0, false)
// if the ordinal is unavailable (e.g., on Windows 7 with xinput1_3.dll).
func xiGetCapabilitiesEx(userIndex uint32) (vendorID, productID uint16, ok bool) {
	if err := procXInputGetCapabilitiesEx.Find(); err != nil {
		return 0, 0, false
	}
	var capsEx xinputCapabilitiesEx
	// Signature: DWORD XInputGetCapabilitiesEx(DWORD reserved, DWORD dwUserIndex, DWORD dwFlags, XINPUT_CAPABILITIES_EX* pCapabilities)
	// reserved must be 1.
	ret, _, _ := procXInputGetCapabilitiesEx.Call(
		1, // reserved, must be 1
		uintptr(userIndex),
		0, // dwFlags (0 = all devices)
		uintptr(unsafe.Pointer(&capsEx)),
	)
	if uint32(ret) != errorSuccess {
		return 0, 0, false
	}
	return capsEx.VendorID, capsEx.ProductID, true
}
