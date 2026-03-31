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
	case strings.Contains(lower, "switch pro") || strings.Contains(lower, "pro controller") ||
		strings.Contains(lower, "nintendo switch"):
		return "switch_pro"
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

// ---------------------------------------------------------------------------
// Nintendo Switch Pro Controller — direct raw report parsing.
//
// The Switch Pro USB HID descriptor defines a fake standard HID layout for
// report ID 0x30: it declares buttons at bytes 1-2 and four 16-bit axes at
// bytes 3-10. However, the actual 0x30 data follows Nintendo's proprietary
// protocol:
//   - Byte 1:  Timer counter (increments every frame!)
//   - Byte 2:  Battery / connection info
//   - Bytes 3-5: Button state (3 bytes, bit-mapped)
//   - Bytes 6-8: Left stick  (12-bit X, 12-bit Y, packed)
//   - Bytes 9-11: Right stick (12-bit X, 12-bit Y, packed)
//
// Using HidP_* functions with the fake descriptor causes the timer byte to
// be interpreted as button state and button bytes as axis values, producing
// random button toggling and axis jumping every single frame.
//
// This parser bypasses HidP_* entirely and reads the known byte layout directly.
// Reference: https://github.com/dekuNukem/Nintendo_Switch_Reverse_Engineering
// ---------------------------------------------------------------------------

const (
	nintendoVendorID = uint16(0x057e)

	// Switch Pro HID report IDs.
	switchProReportFull   = 0x30 // Standard full input (60Hz push, buttons + sticks + IMU)
	switchProReportSimple = 0x3F // Simple HID mode (event-driven, basic buttons + sticks)
)

// isNintendoController returns true if the VID belongs to Nintendo.
// All Nintendo controllers (Pro Controller, Joy-Con, SNES, N64) share the
// same proprietary 0x30 report format whose byte layout does NOT match the
// USB HID descriptor.
func isNintendoController(vendorID uint16) bool {
	return vendorID == nintendoVendorID
}

// parseSwitchProReport parses a raw Switch Pro HID report by directly reading
// the known proprietary byte layout, bypassing HidP_* functions whose results
// are invalid because the HID descriptor lies about the data structure.
//
// Handles report ID 0x30 (full mode, 60Hz) and 0x3F (simple HID mode).
// Returns (state, false) for unknown or too-short reports.
func parseSwitchProReport(name string, rawData []byte, dz float64) (GamepadState, bool) {
	state := GamepadState{
		Connected:      true,
		ControllerType: "switch_pro",
		Name:           name,
	}

	if len(rawData) == 0 {
		return state, false
	}

	switch rawData[0] {
	case switchProReportFull:
		return parseSwitchProFull(name, rawData, dz)
	case switchProReportSimple:
		return parseSwitchProSimple(name, rawData, dz)
	default:
		return state, false
	}
}

// parseSwitchProFull parses a 0x30 full-mode report (also works for 0x21/0x31
// which share the same bytes 1-12 layout).
//
// Byte layout (after report ID byte 0):
//
//	Byte 1:    Timer
//	Byte 2:    Battery (high nibble) + connection (low nibble)
//	Byte 3:    Right buttons   — Y(0x01) X(0x02) B(0x04) A(0x08) SR(0x10) SL(0x20) R(0x40) ZR(0x80)
//	Byte 4:    Shared buttons  — Minus(0x01) Plus(0x02) RS(0x04) LS(0x08) Home(0x10) Capture(0x20)
//	Byte 5:    Left buttons    — Down(0x01) Up(0x02) Right(0x04) Left(0x08) SR(0x10) SL(0x20) L(0x40) ZL(0x80)
//	Bytes 6-8: Left stick      — X = [6] | ([7]&0x0F)<<8; Y = [7]>>4 | [8]<<4
//	Bytes 9-11: Right stick    — same packing
func parseSwitchProFull(name string, rawData []byte, dz float64) (GamepadState, bool) {
	if len(rawData) < 12 {
		return GamepadState{Connected: true, ControllerType: "switch_pro", Name: name}, false
	}

	state := GamepadState{
		Connected:      true,
		ControllerType: "switch_pro",
		Name:           name,
	}

	// Byte 3: Right-side buttons
	b3 := rawData[3]
	state.Buttons.Y = b3&0x01 != 0
	state.Buttons.X = b3&0x02 != 0
	state.Buttons.B = b3&0x04 != 0
	state.Buttons.A = b3&0x08 != 0
	// bits 4-5: SR/SL (Joy-Con only, ignored for Pro Controller)
	state.Buttons.RB = b3&0x40 != 0 // R
	if b3&0x80 != 0 {               // ZR (digital)
		state.Triggers.RT.Value = 1.0
	}

	// Byte 4: Shared buttons
	b4 := rawData[4]
	state.Buttons.Back = b4&0x01 != 0  // Minus
	state.Buttons.Start = b4&0x02 != 0 // Plus
	state.Sticks.Right.Pressed = b4&0x04 != 0
	state.Sticks.Left.Pressed = b4&0x08 != 0
	state.Buttons.Guide = b4&0x10 != 0   // Home
	state.Buttons.Capture = b4&0x20 != 0 // Capture

	// Byte 5: Left-side buttons
	b5 := rawData[5]
	state.Dpad.Down = b5&0x01 != 0
	state.Dpad.Up = b5&0x02 != 0
	state.Dpad.Right = b5&0x04 != 0
	state.Dpad.Left = b5&0x08 != 0
	// bits 4-5: SR/SL (Joy-Con only)
	state.Buttons.LB = b5&0x40 != 0 // L
	if b5&0x80 != 0 {               // ZL (digital)
		state.Triggers.LT.Value = 1.0
	}

	// Left stick: 12-bit packed across 3 bytes
	lx := uint16(rawData[6]) | (uint16(rawData[7]&0x0F) << 8)
	ly := (uint16(rawData[7]) >> 4) | (uint16(rawData[8]) << 4)

	// Right stick: same packing
	rx := uint16(rawData[9]) | (uint16(rawData[10]&0x0F) << 8)
	ry := (uint16(rawData[10]) >> 4) | (uint16(rawData[11]) << 4)

	// Normalize 12-bit (0-4095, centre ~2048) to [-1.0, 1.0].
	// Nintendo 0x30 proprietary format: Y increases upward (positive-up),
	// already matching XInput convention — no negation needed.
	state.Sticks.Left.Position.X = applyDeadzone(normalize12bit(lx), dz)
	state.Sticks.Left.Position.Y = applyDeadzone(normalize12bit(ly), dz)
	state.Sticks.Right.Position.X = applyDeadzone(normalize12bit(rx), dz)
	state.Sticks.Right.Position.Y = applyDeadzone(normalize12bit(ry), dz)

	return state, true
}

// parseSwitchProSimple parses a 0x3F simple-HID-mode report.
// This mode is event-driven (only sent on state change), uses 8-bit sticks,
// and a hat-switch byte for the D-pad.
//
// Byte layout (after report ID byte 0):
//
//	Byte 1:  Button byte 1 — same bit mapping as 0x30 byte 3 (right buttons)
//	Byte 2:  Button byte 2 — same bit mapping as 0x30 byte 4 (shared buttons)
//	Byte 3:  Hat switch (0=N,1=NE,2=E,3=SE,4=S,5=SW,6=W,7=NW,8=centre)
//	Bytes 4-5: Left stick  (uint16 LE: X, Y)  — 0-65535, centre ~32768
//	Bytes 6-7: Right stick (uint16 LE: X, Y)  — same range
func parseSwitchProSimple(name string, rawData []byte, dz float64) (GamepadState, bool) {
	if len(rawData) < 8 {
		return GamepadState{Connected: true, ControllerType: "switch_pro", Name: name}, false
	}

	state := GamepadState{
		Connected:      true,
		ControllerType: "switch_pro",
		Name:           name,
	}

	// Byte 1: Right buttons (same layout as 0x30 byte 3)
	b1 := rawData[1]
	state.Buttons.Y = b1&0x01 != 0
	state.Buttons.X = b1&0x02 != 0
	state.Buttons.B = b1&0x04 != 0
	state.Buttons.A = b1&0x08 != 0
	state.Buttons.RB = b1&0x40 != 0
	if b1&0x80 != 0 {
		state.Triggers.RT.Value = 1.0
	}

	// Byte 2: Shared buttons (same layout as 0x30 byte 4)
	b2 := rawData[2]
	state.Buttons.Back = b2&0x01 != 0
	state.Buttons.Start = b2&0x02 != 0
	state.Sticks.Right.Pressed = b2&0x04 != 0
	state.Sticks.Left.Pressed = b2&0x08 != 0
	state.Buttons.Guide = b2&0x10 != 0
	state.Buttons.Capture = b2&0x20 != 0

	// Byte 3: Hat switch (D-pad)
	hat := rawData[3]
	if hat < 8 {
		dirs := hatDirTable[hat]
		state.Dpad.Up = dirs[0]
		state.Dpad.Down = dirs[1]
		state.Dpad.Left = dirs[2]
		state.Dpad.Right = dirs[3]
	}

	// NOTE: 0x3F simple mode does NOT have left-side button byte — L and ZL
	// are encoded in byte 2 bits 6-7 for this mode according to some sources,
	// but the standard simple-HID format only provides 2 button bytes.
	// L/ZL may not be available in this mode.
	state.Buttons.LB = b2&0x40 != 0
	if b2&0x80 != 0 {
		state.Triggers.LT.Value = 1.0
	}

	// Sticks: uint16 LE, 0-65535 range, centre ~32768
	if len(rawData) >= 12 {
		lx := uint16(rawData[4]) | (uint16(rawData[5]) << 8)
		ly := uint16(rawData[6]) | (uint16(rawData[7]) << 8)
		rx := uint16(rawData[8]) | (uint16(rawData[9]) << 8)
		ry := uint16(rawData[10]) | (uint16(rawData[11]) << 8)

		state.Sticks.Left.Position.X = applyDeadzone(normalize16bit(lx), dz)
		state.Sticks.Left.Position.Y = applyDeadzone(-normalize16bit(ly), dz)
		state.Sticks.Right.Position.X = applyDeadzone(normalize16bit(rx), dz)
		state.Sticks.Right.Position.Y = applyDeadzone(-normalize16bit(ry), dz)
	}

	return state, true
}

// normalize12bit converts a 12-bit unsigned value (0-4095) to [-1.0, 1.0].
// Centre is at 2048. Used for Switch Pro 0x30 report packed stick data.
func normalize12bit(raw uint16) float64 {
	v := (float64(raw) - 2048.0) / 2048.0
	if v < -1.0 {
		v = -1.0
	}
	if v > 1.0 {
		v = 1.0
	}
	return v
}

// normalize16bit converts a 16-bit unsigned value (0-65535) to [-1.0, 1.0].
// Centre is at 32768. Used for Switch Pro 0x3F simple mode stick data.
func normalize16bit(raw uint16) float64 {
	v := (float64(raw) - 32768.0) / 32768.0
	if v < -1.0 {
		v = -1.0
	}
	if v > 1.0 {
		v = 1.0
	}
	return v
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
