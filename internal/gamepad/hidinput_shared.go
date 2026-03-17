package gamepad

// hidinput_shared.go — platform-agnostic HID constants, types, and logic.
//
// Everything here compiles on all platforms. The Windows-specific DLL bindings,
// Windows SDK struct mirrors, and syscall-based parsing functions live in
// hidinput_windows.go. Non-Windows stubs live in hidinput_other.go.

import "strings"

// ---------------------------------------------------------------------------
// HID Usage Page / Usage constants (from USB HID Usage Tables spec)
// ---------------------------------------------------------------------------

const (
	usagePageGenericDesktop = uint16(0x01)
	usagePageButton         = uint16(0x09)

	// HID Generic Desktop axis usages (HID Usage Tables §4)
	hidUsageX      = uint16(0x30)
	hidUsageY      = uint16(0x31)
	hidUsageZ      = uint16(0x32)
	hidUsageRx     = uint16(0x33)
	hidUsageRy     = uint16(0x34)
	hidUsageRz     = uint16(0x35)
	hidUsageSlider = uint16(0x36)
	hidUsageDial   = uint16(0x37)
	hidUsageHat    = uint16(0x39)
)

// ---------------------------------------------------------------------------
// hidAxisEntry — one entry in a device's ordered axis list.
// SDL axis indices correspond to position in this list (hat switch excluded).
// ---------------------------------------------------------------------------

type hidAxisEntry struct {
	usagePage  uint16
	usage      uint16
	logicalMin int32
	logicalMax int32
	bitSize    uint16
}

// ---------------------------------------------------------------------------
// Lookup tables and defaults
// ---------------------------------------------------------------------------

// hatDirTable maps standard HID hat switch values (0-7 = N/NE/E/SE/S/SW/W/NW)
// to [Up, Down, Left, Right] booleans. Values >= 8 mean centred.
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

// defaultHIDAxes maps the most common HID Generic Desktop axis usages to
// semantic targets. Covers the majority of gamepads that follow the standard
// HID usage table. Controllers with unusual layouts get explicit HIDAxes in
// their DeviceMapping.
var defaultHIDAxes = map[uint16]string{
	hidUsageX:  "left_x",
	hidUsageY:  "left_y",
	hidUsageZ:  "rt", // Z is typically the right trigger (or combined on some)
	hidUsageRx: "right_x",
	hidUsageRy: "right_y",
	hidUsageRz: "lt", // Rz is typically the left trigger (or right stick Z)
}

// defaultButtonOrder is used when a DeviceMapping has no explicit HIDButtons.
// Matches the most common button ordering for HID gamepads (PlayStation / generic).
var defaultButtonOrder = []string{
	"x", "a", "b", "y", // buttons 1-4  (PS: Square/Cross/Circle/Triangle)
	"lb", "rb", // buttons 5-6  (L1/R1)
	"lt", "rt", // buttons 7-8  (L2/R2 as digital — mostly analog)
	"back", "start", // buttons 9-10 (Share/Options / Select/Start)
	"ls", "rs", // buttons 11-12 (L3/R3)
	"guide", "touchpad", // buttons 13-14 (PS/Home, Touchpad)
}

// dpadAxisThreshold is the normalised axis value (in [-1,1]) above which a
// half-axis dpad binding is considered "pressed".
const dpadAxisThreshold = 0.5

// ---------------------------------------------------------------------------
// buildAxisMap — construct HID usage → semantic axis target mapping.
// ---------------------------------------------------------------------------

func buildAxisMap(mapping *DeviceMapping) map[uint16]string {
	if mapping != nil && len(mapping.HIDAxes) > 0 {
		m := make(map[uint16]string, len(mapping.HIDAxes))
		for k, v := range mapping.HIDAxes {
			m[k] = v
		}
		return m
	}
	m := make(map[uint16]string, len(defaultHIDAxes))
	for k, v := range defaultHIDAxes {
		m[k] = v
	}
	return m
}

// ---------------------------------------------------------------------------
// normalizeHIDAxis — map raw unsigned HID value to normalised float64.
// ---------------------------------------------------------------------------

// normalizeHIDAxis converts a raw HID axis value to a normalised float64.
// For axis-style inputs (sticks) it produces [-1.0, 1.0].
// For trigger-style inputs it produces [0.0, 1.0].
func normalizeHIDAxis(raw uint32, logMin, logMax int32, isTrigger bool) float64 {
	if logMax <= logMin {
		return 0
	}
	var fRaw float64
	if logMin < 0 {
		// Sign-extend: LogicalMin < 0 means the field is signed.
		fRaw = float64(int32(raw))
	} else {
		fRaw = float64(raw)
	}

	span := float64(logMax - logMin)
	if isTrigger {
		v := (fRaw - float64(logMin)) / span
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		return v
	}

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

// ---------------------------------------------------------------------------
// resolveButtonTarget / applyButton — button → GamepadState field mapping.
// ---------------------------------------------------------------------------

// resolveButtonTarget returns the semantic button name for a 1-based HID button
// usage. Uses the DeviceMapping's HIDButtons table if available, otherwise falls
// back to defaultButtonOrder.
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

// ---------------------------------------------------------------------------
// SDL-specific helpers (no OS dependency)
// ---------------------------------------------------------------------------

// sdlNameToControllerType maps well-known SDL controller name substrings to the
// ControllerType identifiers used by the frontend (must match configMap in app.js).
// Falls back to "Xbox" (generic layout) for unrecognised names.
func sdlNameToControllerType(sdlName string) string {
	lower := strings.ToLower(sdlName)
	switch {
	case strings.Contains(lower, "dualsense") || strings.Contains(lower, "ps5") ||
		strings.Contains(lower, "dualshock") || strings.Contains(lower, "ps4") ||
		strings.Contains(lower, "ps3") || strings.Contains(lower, "ps2") ||
		strings.Contains(lower, "ps1"):
		return "PlayStation"
	default:
		return "Xbox"
	}
}

// applyAxisToState sets the appropriate GamepadState field for a semantic axis name.
// HID Y axes are positive-downward; we negate them here to match the XInput
// convention (positive-upward). The frontend renderer inverts Y again when drawing
// the stick knob, so the net result is correct in both directions.
func applyAxisToState(state *GamepadState, target string, v float64) {
	switch target {
	case "left_x":
		state.Sticks.Left.Position.X = v
	case "left_y":
		state.Sticks.Left.Position.Y = -v
	case "right_x":
		state.Sticks.Right.Position.X = v
	case "right_y":
		state.Sticks.Right.Position.Y = -v
	case "lt":
		state.Triggers.LT.Value = v
	case "rt":
		state.Triggers.RT.Value = v
	}
}

// applySDLButtonTarget maps SDL semantic button/dpad targets to GamepadState.
func applySDLButtonTarget(state *GamepadState, target string) {
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
	case "lt":
		state.Triggers.LT.Value = 1.0
	case "rt":
		state.Triggers.RT.Value = 1.0
	case "back":
		state.Buttons.Back = true
	case "start":
		state.Buttons.Start = true
	case "guide":
		state.Buttons.Guide = true
	case "ls":
		state.Sticks.Left.Pressed = true
	case "rs":
		state.Sticks.Right.Pressed = true
	case "touchpad":
		state.Buttons.Touchpad = true
	case "capture":
		state.Buttons.Capture = true
	case "dpup":
		state.Dpad.Up = true
	case "dpdown":
		state.Dpad.Down = true
	case "dpleft":
		state.Dpad.Left = true
	case "dpright":
		state.Dpad.Right = true
	}
}
