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

// GetRawInputDeviceInfoW is also used in rawinput package; we bind it separately
// here to keep the gamepad package self-contained.
var (
	modUser32HID           = syscall.NewLazyDLL("user32.dll")
	procGetRawInputDevInfo = modUser32HID.NewProc("GetRawInputDeviceInfoW")
)

// HID report type for input reports.
const hidpInput = 0

// HIDP_STATUS_SUCCESS is the success return value from HidP_* functions.
const hidpStatusSuccess = 0x00110000

// Windows-only HID usage IDs for gamepad registration (joystick / gamepad).
// usagePageGenericDesktop, hidUsage*, etc. are in hidinput_shared.go.
const (
	usageJoystick = 0x04
	usageGamepad  = 0x05
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

// hidpValueCaps mirrors HIDP_VALUE_CAPS from hidpi.h exactly.
//
// SDK field order (hidpi.h):
//
//	USAGE   UsagePage;          // offset  0 (2)
//	UCHAR   ReportID;           // offset  2 (1)
//	BOOLEAN IsAlias;            // offset  3 (1)
//	USHORT  BitField;           // offset  4 (2)
//	USHORT  LinkCollection;     // offset  6 (2)
//	USAGE   LinkUsage;          // offset  8 (2)
//	USAGE   LinkUsagePage;      // offset 10 (2)
//	BOOLEAN IsRange;            // offset 12 (1)
//	BOOLEAN IsStringRange;      // offset 13 (1)
//	BOOLEAN IsDesignatorRange;  // offset 14 (1)
//	BOOLEAN IsAbsolute;         // offset 15 (1)
//	BOOLEAN HasNull;            // offset 16 (1)
//	UCHAR   Reserved;           // offset 17 (1)
//	USHORT  BitSize;            // offset 18 (2)
//	USHORT  ReportCount;        // offset 20 (2)
//	USHORT  Reserved2[5];       // offset 22 (10) → end 32
//	ULONG   UnitsExp;           // offset 32 (4)
//	ULONG   Units;              // offset 36 (4)
//	LONG    LogicalMin;         // offset 40 (4)
//	LONG    LogicalMax;         // offset 44 (4)
//	LONG    PhysicalMin;        // offset 48 (4)
//	LONG    PhysicalMax;        // offset 52 (4)
//	union { Range / NotRange }  // offset 56 (16)
//	Total size: 72 bytes
type hidpValueCaps struct {
	UsagePage         uint16
	ReportID          uint8
	IsAlias           uint8 // BOOLEAN
	BitField          uint16
	LinkCollection    uint16
	LinkUsage         uint16
	LinkUsagePage     uint16
	IsRange           uint8 // BOOLEAN
	IsStringRange     uint8 // BOOLEAN
	IsDesignatorRange uint8 // BOOLEAN
	IsAbsolute        uint8 // BOOLEAN
	HasNull           uint8 // BOOLEAN
	Reserved          uint8
	BitSize           uint16
	ReportCount       uint16
	Reserved2         [5]uint16
	UnitsExp          uint32
	Units             uint32
	LogicalMin        int32
	LogicalMax        int32
	PhysicalMin       int32
	PhysicalMax       int32
	// Union (Range variant — we always read as Range):
	UsageMin      uint16
	UsageMax      uint16
	StringMin     uint16
	StringMax     uint16
	DesignatorMin uint16
	DesignatorMax uint16
	DataIndexMin  uint16
	DataIndexMax  uint16
}

// hidpButtonCaps mirrors HIDP_BUTTON_CAPS from hidpi.h exactly.
//
// SDK field order (hidpi.h):
//
//	USAGE   UsagePage;          // offset  0 (2)
//	UCHAR   ReportID;           // offset  2 (1)
//	BOOLEAN IsAlias;            // offset  3 (1)
//	USHORT  BitField;           // offset  4 (2)
//	USHORT  LinkCollection;     // offset  6 (2)
//	USAGE   LinkUsage;          // offset  8 (2)
//	USAGE   LinkUsagePage;      // offset 10 (2)
//	BOOLEAN IsRange;            // offset 12 (1)
//	BOOLEAN IsStringRange;      // offset 13 (1)
//	BOOLEAN IsDesignatorRange;  // offset 14 (1)
//	BOOLEAN IsAbsolute;         // offset 15 (1)
//	USHORT  ReportCount;        // offset 16 (2)
//	USHORT  Reserved2;          // offset 18 (2)
//	ULONG   Reserved[9];        // offset 20 (36) → end 56
//	union { Range / NotRange }  // offset 56 (16)
//	Total size: 72 bytes
type hidpButtonCaps struct {
	UsagePage         uint16
	ReportID          uint8
	IsAlias           uint8 // BOOLEAN
	BitField          uint16
	LinkCollection    uint16
	LinkUsage         uint16
	LinkUsagePage     uint16
	IsRange           uint8 // BOOLEAN
	IsStringRange     uint8 // BOOLEAN
	IsDesignatorRange uint8 // BOOLEAN
	IsAbsolute        uint8 // BOOLEAN
	ReportCount       uint16
	Reserved2         uint16
	Reserved          [9]uint32
	// Union (Range variant):
	UsageMin      uint16
	UsageMax      uint16
	StringMin     uint16
	StringMax     uint16
	DesignatorMin uint16
	DesignatorMax uint16
	DataIndexMin  uint16
	DataIndexMax  uint16
}

// ridDeviceInfoBuf is a raw buffer for RID_DEVICE_INFO.
//
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
	sdlMap    *SDLMapping // SDL gamecontrollerdb mapping (may be nil)
	name      string
	isXInput  bool // filtered out: device is an XInput virtual HID
	isInvalid bool // failed to initialise; skip future events

	preparsedData []byte // raw PHIDP_PREPARSED_DATA blob

	caps       hidpCaps
	valueCaps  []hidpValueCaps
	buttonCaps []hidpButtonCaps

	// axisOrder is the ordered list of value-cap entries (one per usage),
	// expanded from ranges, hat switch excluded.
	// SDL axis index N → axisOrder[N].
	axisOrder []hidAxisEntry

	// Resolved axis map: HID usage → semantic target (used when sdlMap is nil)
	axisMap map[uint16]string

	// Button count (max usage in button caps range)
	buttonCount uint16
}

// ---------------------------------------------------------------------------
// isXInputDevice — skip XInput virtual HID devices
// ---------------------------------------------------------------------------

func isXInputDevice(hDevice uintptr) bool {
	var size uint32
	procGetRawInputDevInfo.Call(hDevice, ridiDevNameHID, 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		return false
	}
	buf := make([]uint16, size)
	ret, _, _ := procGetRawInputDevInfo.Call(
		hDevice, ridiDevNameHID,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)),
	)
	if ret == ^uintptr(0) {
		return false
	}
	name := string(utf16.Decode(buf))
	return strings.Contains(strings.ToUpper(name), "IG_")
}

// ---------------------------------------------------------------------------
// initHIDDevice — build hidDeviceInfo cache entry for a device handle
// ---------------------------------------------------------------------------

func initHIDDevice(hDevice uintptr) *hidDeviceInfo {
	dev := &hidDeviceInfo{hDevice: hDevice}

	if isXInputDevice(hDevice) {
		dev.isXInput = true
		return dev
	}

	// Get VID/PID from RID_DEVICE_INFO.
	var infoBuf ridDeviceInfoBuf
	infoSize := uint32(len(infoBuf))
	ret, _, _ := procGetRawInputDevInfo.Call(
		hDevice, ridiDevInfoHID,
		uintptr(unsafe.Pointer(&infoBuf[0])), uintptr(unsafe.Pointer(&infoSize)),
	)
	if ret == ^uintptr(0) {
		log.Printf("hidinput: GetRawInputDeviceInfo(RIDI_DEVICEINFO) failed for hDevice=0x%x", hDevice)
		dev.isInvalid = true
		return dev
	}

	dwType := *(*uint32)(unsafe.Pointer(&infoBuf[4]))
	if dwType != 2 { // not a HID device
		dev.isInvalid = true
		return dev
	}
	dev.vendorID = *(*uint16)(unsafe.Pointer(&infoBuf[8]))
	dev.productID = *(*uint16)(unsafe.Pointer(&infoBuf[12]))

	dev.mapping = GetMapping(dev.vendorID, dev.productID)
	dev.name = fmt.Sprintf("%s (VID_%04X&PID_%04X)", dev.mapping.Name, dev.vendorID, dev.productID)

	// Get preparsed data (needed for HidP_* calls).
	var prepSize uint32
	procGetRawInputDevInfo.Call(hDevice, ridiPreparsed, 0, uintptr(unsafe.Pointer(&prepSize)))
	if prepSize == 0 {
		log.Printf("hidinput: no preparsed data for %s", dev.name)
		dev.isInvalid = true
		return dev
	}
	dev.preparsedData = make([]byte, prepSize)
	ret, _, _ = procGetRawInputDevInfo.Call(
		hDevice, ridiPreparsed,
		uintptr(unsafe.Pointer(&dev.preparsedData[0])), uintptr(unsafe.Pointer(&prepSize)),
	)
	if ret == ^uintptr(0) {
		log.Printf("hidinput: GetRawInputDeviceInfo(RIDI_PREPARSEDDATA) failed for %s", dev.name)
		dev.isInvalid = true
		return dev
	}

	ppd := uintptr(unsafe.Pointer(&dev.preparsedData[0]))

	status, _, _ := procHidPGetCaps.Call(ppd, uintptr(unsafe.Pointer(&dev.caps)))
	if status != hidpStatusSuccess {
		log.Printf("hidinput: HidP_GetCaps failed (0x%08x) for %s", status, dev.name)
		dev.isInvalid = true
		return dev
	}

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

	for _, bc := range dev.buttonCaps {
		if bc.UsagePage == usagePageButton {
			if bc.UsageMax > dev.buttonCount {
				dev.buttonCount = bc.UsageMax
			}
		}
	}

	// Build ordered axis list (SDL axis index → HID usage).
	dev.axisOrder = buildAxisOrder(dev.valueCaps)

	// Look up SDL mapping by VID/PID.
	dev.sdlMap = lookupSDLMapping(dev.vendorID, dev.productID)

	if dev.sdlMap != nil {
		dev.name = fmt.Sprintf("%s (VID_%04X&PID_%04X)", dev.sdlMap.Name, dev.vendorID, dev.productID)
		log.Printf("hidinput: initialised %s — sdlAxes=%d sdlButtons=%d hid_axes=%d buttons=%d [SDL DB]",
			dev.name, len(dev.sdlMap.Axes), len(dev.sdlMap.Buttons), len(dev.axisOrder), dev.buttonCount)
	} else {
		dev.axisMap = buildAxisMap(dev.mapping)
		log.Printf("hidinput: initialised %s — axes=%d buttons=%d",
			dev.name, len(dev.valueCaps), dev.buttonCount)
	}

	return dev
}

// ---------------------------------------------------------------------------
// buildAxisOrder — ordered axis list from value caps (hat switch excluded)
// ---------------------------------------------------------------------------

// buildAxisOrder builds the ordered axis list from value caps (expanding usage
// ranges). Hat switch entries are excluded — SDL numbers axes and hats as
// separate arrays, so SDL axis index N corresponds to the N-th non-hat value
// cap usage.
func buildAxisOrder(valueCaps []hidpValueCaps) []hidAxisEntry {
	var entries []hidAxisEntry
	for i := range valueCaps {
		vc := &valueCaps[i]
		usageMin := vc.UsageMin
		usageMax := vc.UsageMax
		if vc.IsRange == 0 || usageMax < usageMin {
			usageMax = usageMin
		}
		for u := usageMin; u <= usageMax; u++ {
			if vc.UsagePage == usagePageGenericDesktop && u == hidUsageHat {
				continue // hat switch is not an SDL axis
			}
			entries = append(entries, hidAxisEntry{
				usagePage:  vc.UsagePage,
				usage:      u,
				logicalMin: vc.LogicalMin,
				logicalMax: vc.LogicalMax,
				bitSize:    vc.BitSize,
			})
		}
	}
	return entries
}

// ---------------------------------------------------------------------------
// parseHIDReport — top-level dispatcher
// ---------------------------------------------------------------------------

func parseHIDReport(dev *hidDeviceInfo, rawData []byte) GamepadState {
	controllerType := dev.mapping.Name
	if dev.sdlMap != nil {
		controllerType = sdlNameToControllerType(dev.sdlMap.Name)
	}
	state := GamepadState{
		Connected:      true,
		ControllerType: controllerType,
		Name:           dev.name,
	}

	ppd := uintptr(unsafe.Pointer(&dev.preparsedData[0]))
	reportPtr := uintptr(unsafe.Pointer(&rawData[0]))
	reportLen := uint32(len(rawData))

	parseHatSwitch(dev, &state, ppd, reportPtr, reportLen)
	pressedButtons := collectPressedButtons(dev, ppd, reportPtr, reportLen)

	if dev.sdlMap != nil {
		parseHIDReportSDL(dev, &state, ppd, reportPtr, reportLen, pressedButtons)
	} else {
		parseHIDReportLegacy(dev, &state, ppd, reportPtr, reportLen, pressedButtons)
	}

	return state
}

// ---------------------------------------------------------------------------
// parseHatSwitch — read hat switch and set state.Dpad
// ---------------------------------------------------------------------------

func parseHatSwitch(dev *hidDeviceInfo, state *GamepadState, ppd, reportPtr uintptr, reportLen uint32) {
	for i := range dev.valueCaps {
		vc := &dev.valueCaps[i]
		usageMin := vc.UsageMin
		usageMax := vc.UsageMax
		if vc.IsRange == 0 || usageMax < usageMin {
			usageMax = usageMin
		}
		for usage := usageMin; usage <= usageMax; usage++ {
			if vc.UsagePage != usagePageGenericDesktop || usage != hidUsageHat {
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
				return
			}
			idx := int(value) - int(vc.LogicalMin)
			if idx >= 0 && idx < 8 {
				dirs := hatDirTable[idx]
				state.Dpad.Up = dirs[0]
				state.Dpad.Down = dirs[1]
				state.Dpad.Left = dirs[2]
				state.Dpad.Right = dirs[3]
			}
			return
		}
	}
}

// ---------------------------------------------------------------------------
// collectPressedButtons — get 1-based HID button usages that are pressed
// ---------------------------------------------------------------------------

func collectPressedButtons(dev *hidDeviceInfo, ppd, reportPtr uintptr, reportLen uint32) []uint16 {
	if len(dev.buttonCaps) == 0 {
		return nil
	}
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
		if uLen < usageLen {
			return usageList[:uLen]
		}
		break
	}
	end := len(usageList)
	for end > 0 && usageList[end-1] == 0 {
		end--
	}
	return usageList[:end]
}

// ---------------------------------------------------------------------------
// parseHIDReportSDL — SDL gamecontrollerdb mapping path
// ---------------------------------------------------------------------------

func parseHIDReportSDL(dev *hidDeviceInfo, state *GamepadState, ppd, reportPtr uintptr, reportLen uint32, pressedButtons []uint16) {
	sm := dev.sdlMap

	// --- Axes ---
	for _, ab := range sm.Axes {
		if ab.AxisIndex < 0 || ab.AxisIndex >= len(dev.axisOrder) {
			continue
		}
		ae := &dev.axisOrder[ab.AxisIndex]

		var rawVal uint32
		status, _, _ := procHidPGetUsageValue.Call(
			hidpInput,
			uintptr(ae.usagePage),
			0,
			uintptr(ae.usage),
			uintptr(unsafe.Pointer(&rawVal)),
			ppd,
			reportPtr,
			uintptr(reportLen),
		)
		if status != hidpStatusSuccess {
			continue
		}

		lMin := ae.logicalMin
		lMax := ae.logicalMax
		if lMax < lMin && ae.bitSize > 0 && ae.bitSize < 32 {
			lMax = (1 << ae.bitSize) - 1
			lMin = 0
		}

		isDpad := ab.Target == "dpup" || ab.Target == "dpdown" ||
			ab.Target == "dpleft" || ab.Target == "dpright"
		isTrigger := ab.Target == "lt" || ab.Target == "rt"

		var normalized float64
		if isDpad {
			normalized = normalizeHIDAxis(rawVal, lMin, lMax, false)
		} else {
			normalized = normalizeHIDAxis(rawVal, lMin, lMax, isTrigger)
		}

		if ab.HalfPos {
			if normalized < 0 {
				normalized = 0
			}
		} else if ab.HalfNeg {
			if normalized > 0 {
				normalized = 0
			}
			normalized = -normalized
		}

		if ab.Invert {
			normalized = -normalized
		}

		if isDpad {
			if normalized > dpadAxisThreshold {
				applySDLButtonTarget(state, ab.Target)
			}
			continue
		}

		normalized = applyDeadzone(normalized, deadzone)
		applyAxisToState(state, ab.Target, normalized)
	}

	// --- Half-axis from button (N64 C-stick pattern) ---
	pressedSet := make(map[int]bool, len(pressedButtons))
	for _, u := range pressedButtons {
		pressedSet[int(u)-1] = true // 1-based → 0-based
	}
	for _, ahb := range sm.AxisHalfButtons {
		if pressedSet[ahb.ButtonIdx] {
			applyAxisToState(state, ahb.Target, float64(ahb.Sign))
		}
	}

	// --- Buttons ---
	for _, bb := range sm.Buttons {
		if pressedSet[bb.ButtonIndex] {
			applySDLButtonTarget(state, bb.Target)
		}
	}

	// Hat dpad is already handled by parseHatSwitch.
}

// ---------------------------------------------------------------------------
// parseHIDReportLegacy — usage-code-based fallback path
// ---------------------------------------------------------------------------

func parseHIDReportLegacy(dev *hidDeviceInfo, state *GamepadState, ppd, reportPtr uintptr, reportLen uint32, pressedButtons []uint16) {
	for i := range dev.valueCaps {
		vc := &dev.valueCaps[i]
		usageMin := vc.UsageMin
		usageMax := vc.UsageMax
		if vc.IsRange == 0 || usageMax < usageMin {
			usageMax = usageMin
		}
		for usage := usageMin; usage <= usageMax; usage++ {
			if vc.UsagePage == usagePageGenericDesktop && usage == hidUsageHat {
				continue // already handled by parseHatSwitch
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
			lMin := vc.LogicalMin
			lMax := vc.LogicalMax
			if lMax < lMin && vc.BitSize > 0 && vc.BitSize < 32 {
				lMax = (1 << vc.BitSize) - 1
				lMin = 0
			}
			isTrigger := target == "lt" || target == "rt"
			normalized := normalizeHIDAxis(value, lMin, lMax, isTrigger)
			normalized = applyDeadzone(normalized, deadzone)
			applyAxisToState(state, target, normalized)
		}
	}

	for _, buttonUsage := range pressedButtons {
		if buttonUsage == 0 {
			break
		}
		target := resolveButtonTarget(dev.mapping, buttonUsage)
		if target == "" {
			continue
		}
		applyButton(state, target)
	}
}
