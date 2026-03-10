//go:build windows

package gamepad

import (
	"fmt"
	"log"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// ---------------------------------------------------------------------------
// hid.dll bindings
// ---------------------------------------------------------------------------

var (
	modHid = syscall.NewLazyDLL("hid.dll")

	procHidPGetCaps       = modHid.NewProc("HidP_GetCaps")
	procHidPGetValueCaps  = modHid.NewProc("HidP_GetValueCaps")
	procHidPGetButtonCaps = modHid.NewProc("HidP_GetButtonCaps")
	procHidPGetUsages     = modHid.NewProc("HidP_GetUsages")
	procHidPGetUsageValue = modHid.NewProc("HidP_GetUsageValue")
)

// GetRawInputDeviceInfoW is also used in rawinput package; we bind it separately here
// to keep the gamepad package self-contained.
var (
	modUser32HID           = syscall.NewLazyDLL("user32.dll")
	procGetRawInputDevInfo = modUser32HID.NewProc("GetRawInputDeviceInfoW")
)

// HID report type for input reports.
const hidpInput = 0

// HIDP_STATUS_SUCCESS is the success return value from HidP_* functions.
const hidpStatusSuccess = 0x00110000

// HID Usage Page / Usage constants
const (
	usagePageGenericDesktop = 0x01
	usageJoystick           = 0x04
	usageGamepad            = 0x05

	// HID Generic Desktop axis usages
	hidUsageX      = 0x30
	hidUsageY      = 0x31
	hidUsageZ      = 0x32
	hidUsageRx     = 0x33
	hidUsageRy     = 0x34
	hidUsageRz     = 0x35
	hidUsageSlider = 0x36
	hidUsageDial   = 0x37
	hidUsageHat    = 0x39

	// HID Button usage page
	usagePageButton = 0x09
)

// GetRawInputDeviceInfo commands (duplicated from rawinput package; avoid import cycle)
const (
	ridiDevNameHID = 0x20000007
	ridiDevInfoHID = 0x2000000b
	ridiPreparsed  = 0x20000005
)

// ---------------------------------------------------------------------------
// Windows SDK struct layouts
// ---------------------------------------------------------------------------

// hidpCaps mirrors HIDP_CAPS from hidpi.h.
type hidpCaps struct {
	Usage                     uint16
	UsagePage                 uint16
	InputReportByteLength     uint16
	OutputReportByteLength    uint16
	FeatureReportByteLength   uint16
	Reserved                  [17]uint16
	NumberLinkCollectionNodes uint16
	NumberInputButtonCaps     uint16
	NumberInputValueCaps      uint16
	NumberInputDataIndices    uint16
	NumberOutputButtonCaps    uint16
	NumberOutputValueCaps     uint16
	NumberOutputDataIndices   uint16
	NumberFeatureButtonCaps   uint16
	NumberFeatureValueCaps    uint16
	NumberFeatureDataIndices  uint16
}

// hidpValueCaps mirrors HIDP_VALUE_CAPS from hidpi.h.
// The IsRange / IsStringRange / IsDesignatorRange / IsAbsolute fields are bools
// stored as BOOLEAN (1 byte). The union that follows depends on IsRange.
// We always use the Range variant (IsRange=true) for axis caps.
type hidpValueCaps struct {
	UsagePage           uint16
	ReportID            uint8
	IsAlias             uint8 // BOOLEAN
	BitField            uint16
	LinkCollection      uint16
	LinkUsage           uint16
	LinkUsagePage       uint16
	IsRange             uint8  // BOOLEAN
	IsStringRange       uint8  // BOOLEAN
	IsDesignatorRange   uint8  // BOOLEAN
	IsAbsolute          uint8  // BOOLEAN
	SupportedActivities uint16 // padding / reserved in older SDK
	Reserved            [5]uint16
	// Union: Range (IsRange=1) or NotRange (IsRange=0).
	// We always read as Range.
	UsageMin      uint16
	UsageMax      uint16
	StringMin     uint16
	StringMax     uint16
	DesignatorMin uint16
	DesignatorMax uint16
	DataIndexMin  uint16
	DataIndexMax  uint16
	// Value fields
	LogicalMin  int32
	LogicalMax  int32
	PhysicalMin int32
	PhysicalMax int32
	// Bit precision
	UnitsExp    uint32
	Units       uint32
	BitSize     uint16
	ReportCount uint16
}

// hidpButtonCaps mirrors HIDP_BUTTON_CAPS from hidpi.h (Range variant).
type hidpButtonCaps struct {
	UsagePage           uint16
	ReportID            uint8
	IsAlias             uint8
	BitField            uint16
	LinkCollection      uint16
	LinkUsage           uint16
	LinkUsagePage       uint16
	IsRange             uint8
	IsStringRange       uint8
	IsDesignatorRange   uint8
	IsAbsolute          uint8
	SupportedActivities uint16
	Reserved            [5]uint16
	UsageMin            uint16
	UsageMax            uint16
	StringMin           uint16
	StringMax           uint16
	DesignatorMin       uint16
	DesignatorMax       uint16
	DataIndexMin        uint16
	DataIndexMax        uint16
}

// ridDeviceInfoHID mirrors the HID portion of RID_DEVICE_INFO.
// Full RID_DEVICE_INFO layout:
//
//	DWORD cbSize   (offset 0)
//	DWORD dwType   (offset 4)
//	union (offset 8):
//	  RID_DEVICE_INFO_HID:
//	    DWORD dwVendorId    (offset 8)
//	    DWORD dwProductId   (offset 12)
//	    DWORD dwVersionNumber (offset 16)
//	    USAGE usUsagePage   (offset 20)
//	    USAGE usUsage       (offset 22)
type ridDeviceInfoBuf [32]byte

// ---------------------------------------------------------------------------
// hidDeviceInfo — per-device cached data
// ---------------------------------------------------------------------------

// hidDeviceInfo holds all parsed capability info for a connected HID gamepad.
// It is created once per device on first WM_INPUT or device arrival, then cached
// in Reader.hidDevices. Only accessed from the reader's goroutines under r.mu.
type hidDeviceInfo struct {
	hDevice   uintptr
	vendorID  uint16
	productID uint16
	mapping   *DeviceMapping
	name      string
	isXInput  bool // filtered out: device is an XInput virtual HID
	isInvalid bool // failed to initialise; skip future events

	preparsedData []byte // raw PHIDP_PREPARSED_DATA blob

	caps       hidpCaps
	valueCaps  []hidpValueCaps
	buttonCaps []hidpButtonCaps

	// Resolved axis map: HID usage → semantic target
	axisMap map[uint16]string

	// Button count (max usage in button caps range)
	buttonCount uint16
}

// ---------------------------------------------------------------------------
// isXInputDevice checks if a device path contains "IG_" which marks XInput
// virtual HID devices. These are handled by XInput; we skip them in HID.
// ---------------------------------------------------------------------------

func isXInputDevice(hDevice uintptr) bool {
	// First call: get required buffer size.
	var size uint32
	procGetRawInputDevInfo.Call(
		hDevice,
		ridiDevNameHID,
		0,
		uintptr(unsafe.Pointer(&size)),
	)
	if size == 0 {
		return false
	}

	// Second call: get the device name (UTF-16 string).
	buf := make([]uint16, size)
	ret, _, _ := procGetRawInputDevInfo.Call(
		hDevice,
		ridiDevNameHID,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == ^uintptr(0) {
		return false
	}

	// Convert to Go string and check for "IG_" marker.
	name := string(utf16.Decode(buf))
	return strings.Contains(strings.ToUpper(name), "IG_")
}

// ---------------------------------------------------------------------------
// initHIDDevice queries all capabilities for a device handle and builds the
// hidDeviceInfo cache entry. Returns nil if the device should be skipped.
// ---------------------------------------------------------------------------

func initHIDDevice(hDevice uintptr) *hidDeviceInfo {
	dev := &hidDeviceInfo{hDevice: hDevice}

	// Check for XInput virtual device.
	if isXInputDevice(hDevice) {
		dev.isXInput = true
		return dev
	}

	// Get VID/PID from RID_DEVICE_INFO.
	var infoBuf ridDeviceInfoBuf
	infoSize := uint32(len(infoBuf))
	ret, _, _ := procGetRawInputDevInfo.Call(
		hDevice,
		ridiDevInfoHID,
		uintptr(unsafe.Pointer(&infoBuf[0])),
		uintptr(unsafe.Pointer(&infoSize)),
	)
	if ret == ^uintptr(0) {
		log.Printf("hidinput: GetRawInputDeviceInfo(RIDI_DEVICEINFO) failed for hDevice=0x%x", hDevice)
		dev.isInvalid = true
		return dev
	}

	// Parse RID_DEVICE_INFO_HID fields (see layout comment above).
	dwType := *(*uint32)(unsafe.Pointer(&infoBuf[4]))
	if dwType != 2 { // not a HID device
		dev.isInvalid = true
		return dev
	}
	dev.vendorID = *(*uint16)(unsafe.Pointer(&infoBuf[8]))   // dwVendorId low word
	dev.productID = *(*uint16)(unsafe.Pointer(&infoBuf[12])) // dwProductId low word

	// Resolve device mapping via VID/PID table.
	dev.mapping = GetMapping(dev.vendorID, dev.productID)
	dev.name = fmt.Sprintf("%s (VID_%04X&PID_%04X)", dev.mapping.Name, dev.vendorID, dev.productID)

	// Get preparsed data (needed for HidP_* calls).
	var prepSize uint32
	procGetRawInputDevInfo.Call(
		hDevice,
		ridiPreparsed,
		0,
		uintptr(unsafe.Pointer(&prepSize)),
	)
	if prepSize == 0 {
		log.Printf("hidinput: no preparsed data for %s", dev.name)
		dev.isInvalid = true
		return dev
	}
	// Preparsed data must be aligned to at least 8 bytes.
	// make([]byte) aligns to 8 bytes on all supported Go runtime architectures.
	dev.preparsedData = make([]byte, prepSize)
	ret, _, _ = procGetRawInputDevInfo.Call(
		hDevice,
		ridiPreparsed,
		uintptr(unsafe.Pointer(&dev.preparsedData[0])),
		uintptr(unsafe.Pointer(&prepSize)),
	)
	if ret == ^uintptr(0) {
		log.Printf("hidinput: GetRawInputDeviceInfo(RIDI_PREPARSEDDATA) failed for %s", dev.name)
		dev.isInvalid = true
		return dev
	}

	ppd := uintptr(unsafe.Pointer(&dev.preparsedData[0]))

	// Get top-level collection capabilities.
	status, _, _ := procHidPGetCaps.Call(ppd, uintptr(unsafe.Pointer(&dev.caps)))
	if status != hidpStatusSuccess {
		log.Printf("hidinput: HidP_GetCaps failed (0x%08x) for %s", status, dev.name)
		dev.isInvalid = true
		return dev
	}

	// Get value caps (axes, triggers, hat switch).
	if dev.caps.NumberInputValueCaps > 0 {
		dev.valueCaps = make([]hidpValueCaps, dev.caps.NumberInputValueCaps)
		vcLen := uint16(dev.caps.NumberInputValueCaps)
		procHidPGetValueCaps.Call(
			hidpInput,
			uintptr(unsafe.Pointer(&dev.valueCaps[0])),
			uintptr(unsafe.Pointer(&vcLen)),
			ppd,
		)
		dev.valueCaps = dev.valueCaps[:vcLen]
	}

	// Get button caps.
	if dev.caps.NumberInputButtonCaps > 0 {
		dev.buttonCaps = make([]hidpButtonCaps, dev.caps.NumberInputButtonCaps)
		bcLen := uint16(dev.caps.NumberInputButtonCaps)
		procHidPGetButtonCaps.Call(
			hidpInput,
			uintptr(unsafe.Pointer(&dev.buttonCaps[0])),
			uintptr(unsafe.Pointer(&bcLen)),
			ppd,
		)
		dev.buttonCaps = dev.buttonCaps[:bcLen]
	}

	// Determine button count from button caps.
	for _, bc := range dev.buttonCaps {
		if bc.UsagePage == usagePageButton {
			max := bc.UsageMax
			if max > dev.buttonCount {
				dev.buttonCount = max
			}
		}
	}

	// Build axis map: HID usage → semantic target.
	dev.axisMap = buildAxisMap(dev.mapping)

	log.Printf("hidinput: initialised %s — axes=%d buttons=%d",
		dev.name, len(dev.valueCaps), dev.buttonCount)

	return dev
}

// ---------------------------------------------------------------------------
// buildAxisMap constructs the HID usage → semantic axis target mapping.
// If the DeviceMapping provides an explicit HIDAxes table it takes priority;
// otherwise a generic default based on common HID axis assignments is used.
// ---------------------------------------------------------------------------

// defaultHIDAxes maps the most common HID Generic Desktop axis usages to semantic targets.
// This covers the majority of gamepads that follow the standard HID usage table.
// Controllers with unusual layouts should have explicit HIDAxes in their DeviceMapping.
var defaultHIDAxes = map[uint16]string{
	hidUsageX:  "left_x",
	hidUsageY:  "left_y",
	hidUsageZ:  "rt", // Z is typically the right trigger (or combined on some devices)
	hidUsageRx: "right_x",
	hidUsageRy: "right_y",
	hidUsageRz: "lt", // Rz is typically the left trigger (or right stick Z on some)
}

func buildAxisMap(mapping *DeviceMapping) map[uint16]string {
	if mapping != nil && len(mapping.HIDAxes) > 0 {
		// Use device-specific mapping.
		m := make(map[uint16]string, len(mapping.HIDAxes))
		for k, v := range mapping.HIDAxes {
			m[k] = v
		}
		return m
	}
	// Fall back to generic defaults.
	m := make(map[uint16]string, len(defaultHIDAxes))
	for k, v := range defaultHIDAxes {
		m[k] = v
	}
	return m
}

// ---------------------------------------------------------------------------
// parseHIDReport converts a raw HID input report into a GamepadState.
// ---------------------------------------------------------------------------

// hatToDirections maps the standard HID hat switch values (0-7 = N/NE/E/SE/S/SW/W/NW)
// to DpadState booleans. Values >= 8 mean centred.
var hatDirTable = [8][4]bool{
	// Up    Down   Left   Right
	{true, false, false, false}, // 0: N
	{true, false, false, true},  // 1: NE
	{false, false, false, true}, // 2: E
	{false, true, false, true},  // 3: SE
	{false, true, false, false}, // 4: S
	{false, true, true, false},  // 5: SW
	{false, false, true, false}, // 6: W
	{true, false, true, false},  // 7: NW
}

// defaultButtonOrder is used when a DeviceMapping has no explicit HIDButtons.
// It matches the most common button ordering for HID gamepads (PlayStation / generic HID).
var defaultButtonOrder = []string{
	"x", "a", "b", "y", // buttons 1-4  (PS: Square/Cross/Circle/Triangle)
	"lb", "rb", // buttons 5-6  (L1/R1)
	"lt", "rt", // buttons 7-8  (L2/R2 as digital — rare, mostly analog)
	"back", "start", // buttons 9-10 (Share/Options / Select/Start)
	"ls", "rs", // buttons 11-12 (L3/R3)
	"guide", "touchpad", // buttons 13-14 (PS/Home, Touchpad)
}

func parseHIDReport(dev *hidDeviceInfo, rawData []byte) GamepadState {
	state := GamepadState{
		Connected:      true,
		ControllerType: dev.mapping.Name,
		Name:           dev.name,
	}

	ppd := uintptr(unsafe.Pointer(&dev.preparsedData[0]))
	reportPtr := uintptr(unsafe.Pointer(&rawData[0]))
	reportLen := uint32(len(rawData))

	// --- Axes / values ---
	for i := range dev.valueCaps {
		vc := &dev.valueCaps[i]

		// Determine the usage code to look up in the axis map.
		usage := vc.UsageMin // for non-range caps this equals the single usage

		if vc.UsagePage == usagePageGenericDesktop && usage == hidUsageHat {
			// Hat switch → D-pad
			var value uint32
			procHidPGetUsageValue.Call(
				hidpInput,
				uintptr(vc.UsagePage),
				0, // link collection
				uintptr(usage),
				uintptr(unsafe.Pointer(&value)),
				ppd,
				reportPtr,
				uintptr(reportLen),
			)
			idx := int(value) - int(vc.LogicalMin)
			if idx >= 0 && idx < 8 {
				dirs := hatDirTable[idx]
				state.Dpad.Up = dirs[0]
				state.Dpad.Down = dirs[1]
				state.Dpad.Left = dirs[2]
				state.Dpad.Right = dirs[3]
			}
			continue
		}

		target, ok := dev.axisMap[usage]
		if !ok {
			continue
		}

		var value uint32
		status, _, _ := procHidPGetUsageValue.Call(
			hidpInput,
			uintptr(vc.UsagePage),
			0,
			uintptr(usage),
			uintptr(unsafe.Pointer(&value)),
			ppd,
			reportPtr,
			uintptr(reportLen),
		)
		if status != hidpStatusSuccess {
			continue
		}

		// Normalise: logical range [LogicalMin, LogicalMax] → target range.
		lMin := vc.LogicalMin
		lMax := vc.LogicalMax

		// Some controllers report LogicalMax < LogicalMin due to sign-extension
		// from a smaller type. Detect and correct using BitSize.
		if lMax < lMin && vc.BitSize > 0 && vc.BitSize < 32 {
			lMax = (1 << vc.BitSize) - 1
			lMin = 0
		}

		isTrigger := target == "lt" || target == "rt"
		normalized := normalizeHIDAxis(value, lMin, lMax, isTrigger)
		normalized = applyDeadzone(normalized, deadzone)

		switch target {
		case "left_x":
			state.Sticks.Left.Position.X = normalized
		case "left_y":
			// HID Y axes are typically positive-down; frontend also positive-down.
			// No inversion needed here (unlike XInput which is positive-up).
			state.Sticks.Left.Position.Y = normalized
		case "right_x":
			state.Sticks.Right.Position.X = normalized
		case "right_y":
			state.Sticks.Right.Position.Y = normalized
		case "lt":
			state.Triggers.LT.Value = normalized
		case "rt":
			state.Triggers.RT.Value = normalized
		}
	}

	// --- Buttons ---
	if len(dev.buttonCaps) == 0 {
		return state
	}

	// Collect all pressed button usages from the button usage page.
	maxButtons := uint32(dev.buttonCount)
	if maxButtons == 0 {
		maxButtons = 32
	}
	usageList := make([]uint16, maxButtons)
	usageLen := uint32(maxButtons)

	for _, bc := range dev.buttonCaps {
		if bc.UsagePage != usagePageButton {
			continue
		}
		uLen := usageLen
		procHidPGetUsages.Call(
			hidpInput,
			uintptr(usagePageButton),
			0,
			uintptr(unsafe.Pointer(&usageList[0])),
			uintptr(unsafe.Pointer(&uLen)),
			ppd,
			reportPtr,
			uintptr(reportLen),
		)
		usageLen = uLen
		break
	}

	// Map pressed button usages to GamepadState fields.
	for i := uint32(0); i < usageLen; i++ {
		buttonUsage := usageList[i]
		if buttonUsage == 0 {
			break
		}
		target := resolveButtonTarget(dev.mapping, buttonUsage)
		if target == "" {
			continue
		}
		applyButton(&state, target)
	}

	return state
}

// normalizeHIDAxis converts a raw HID axis value to a normalised float64.
// For axis-style inputs (sticks) it produces [-1.0, 1.0].
// For trigger-style inputs it produces [0.0, 1.0].
func normalizeHIDAxis(raw uint32, logMin, logMax int32, isTrigger bool) float64 {
	if logMax <= logMin {
		return 0
	}
	// Interpret raw as signed if LogicalMin < 0
	var fRaw float64
	if logMin < 0 {
		// Sign-extend based on the actual bit field range.
		fRaw = float64(int32(raw))
	} else {
		fRaw = float64(raw)
	}

	span := float64(logMax - logMin)
	if isTrigger {
		// Map [logMin, logMax] → [0, 1]
		v := (fRaw - float64(logMin)) / span
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		return v
	}

	// Map [logMin, logMax] → [-1, 1]
	mid := float64(logMin) + span/2.0
	v := (fRaw - mid) / (span / 2.0)
	if v < -1 {
		v = -1
	}
	if v > 1 {
		v = 1
	}
	return v
}

// resolveButtonTarget returns the semantic button name for a 1-based HID button usage.
// Uses the DeviceMapping's HIDButtons table if available, otherwise falls back to
// defaultButtonOrder.
func resolveButtonTarget(mapping *DeviceMapping, buttonUsage uint16) string {
	if mapping != nil && len(mapping.HIDButtons) > 0 {
		return mapping.HIDButtons[buttonUsage]
	}
	idx := int(buttonUsage) - 1
	if idx >= 0 && idx < len(defaultButtonOrder) {
		return defaultButtonOrder[idx]
	}
	return ""
}

// applyButton sets the appropriate boolean field on state for the given target name.
func applyButton(state *GamepadState, target string) {
	switch target {
	case "a":
		state.Buttons.A = true
	case "b":
		state.Buttons.B = true
	case "x":
		state.Buttons.X = true
	case "y":
		state.Buttons.Y = true
	case "lb":
		state.Buttons.LB = true
	case "rb":
		state.Buttons.RB = true
	case "back":
		state.Buttons.Back = true
	case "start":
		state.Buttons.Start = true
	case "guide":
		state.Buttons.Guide = true
	case "touchpad":
		state.Buttons.Touchpad = true
	case "capture":
		state.Buttons.Capture = true
	case "ls":
		state.Sticks.Left.Pressed = true
	case "rs":
		state.Sticks.Right.Pressed = true
	}
}
