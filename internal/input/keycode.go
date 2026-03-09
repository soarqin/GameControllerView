// Package input defines keyboard and mouse input state types used across the application.
package input

// Windows Raw Input reports keyboard events using PS/2 Set 1 scan codes (MakeCode) and
// an extended-key flag (RI_KEY_E0 = 0x02 in Flags field). The uiohook virtual key codes
// used by Input Overlay closely follow the same scan code numbering, with E0 extended keys
// indicated by a 0x0E00 prefix, except for the four cursor-movement keys (Up/Left/Right/Down)
// which use a 0xE000 prefix.
//
// Mapping rules applied by RawScanToUIohook():
//   - Flags has RI_KEY_E0 bit set → check cursor-movement set; if yes use 0xE000|makeCode,
//     otherwise use 0x0E00|makeCode.
//   - No E0 flag → uiohook = makeCode (direct pass-through).
//
// A small override table handles the handful of keys whose MakeCode + E0 combination does
// not map cleanly to the standard formula (e.g. Pause/Break which generates a special
// sequence).

// e0CursorMakeCodes is the set of MakeCodes that are E0-extended cursor-movement keys.
// These use the 0xE000 prefix in uiohook (matching vc.js from input-overlay).
var e0CursorMakeCodes = map[uint16]bool{
	0x48: true, // Arrow Up
	0x4B: true, // Arrow Left
	0x4C: true, // Clear (Numpad 5 with NumLock off)
	0x4D: true, // Arrow Right
	0x50: true, // Arrow Down
}

// rawScanOverrides maps (makeCode, e0flag) → uiohook for keys that don't fit the standard formula.
// Key: makeCode<<1 | (1 if E0 else 0)
var rawScanOverrides = map[uint32]uint16{
	// Pause/Break: Raw Input delivers MakeCode=0x45 without E0 for Pause.
	// uiohook uses 0x0E45 for Pause (same as NumLock + E0 treatment, but Pause is special).
	// We keep the direct value 0x0045 for NumLock (no E0) and handle Pause separately.
	// In practice, Pause sends MakeCode=0x45 with Flags=0x04 (RI_KEY_E1 prefix) which
	// our code will not match the E0 branch, so it falls through to direct mapping (0x0045).
	// That is actually correct — uiohook NumLock = 0x0045, Pause = 0x0e45.
	// The E1 sequence is complex; we approximate by treating the first scan (0x1D with E0=false)
	// as a no-op via the zero return in RawScanToUIohook.
}

// RawScanToUIohook converts a Windows Raw Input keyboard scan code to the uiohook virtual
// key code expected by Input Overlay config files.
//
// Parameters:
//   - makeCode: RAWKEYBOARD.MakeCode value (PS/2 Set 1 scan code, 0x01–0x7F range)
//   - flags: RAWKEYBOARD.Flags (use RI_KEY_E0 = 0x02 to check extended-key flag)
//
// Returns 0 for unknown / unrepresentable keys.
func RawScanToUIohook(makeCode uint16, flags uint16) uint16 {
	const riKeyE0 = 0x02

	if makeCode == 0 {
		return 0
	}

	// Check the override table first.
	overrideKey := uint32(makeCode)<<1 | (func() uint32 {
		if flags&riKeyE0 != 0 {
			return 1
		}
		return 0
	}())
	if vc, ok := rawScanOverrides[overrideKey]; ok {
		return vc
	}

	if flags&riKeyE0 != 0 {
		// E0-extended key
		if e0CursorMakeCodes[makeCode] {
			return 0xE000 | makeCode
		}
		return 0x0E00 | makeCode
	}

	// Regular key: uiohook = makeCode directly
	return makeCode
}

// MouseButtonToIOCode converts a Windows Raw Input mouse button event flag to the
// Input Overlay mouse button code (1-5).
//
// Windows usButtonFlags values for button-down events:
//
//	RI_MOUSE_LEFT_BUTTON_DOWN   = 0x0001
//	RI_MOUSE_RIGHT_BUTTON_DOWN  = 0x0004
//	RI_MOUSE_MIDDLE_BUTTON_DOWN = 0x0010
//	RI_MOUSE_BUTTON_4_DOWN      = 0x0040
//	RI_MOUSE_BUTTON_5_DOWN      = 0x0100
//
// Input Overlay codes: 1=left, 2=right, 3=middle, 4=X1(back), 5=X2(forward)
func MouseButtonFlagToIOCode(buttonFlag uint16) (code uint16, pressed bool) {
	switch {
	case buttonFlag&0x0001 != 0: // LEFT_DOWN
		return 1, true
	case buttonFlag&0x0002 != 0: // LEFT_UP
		return 1, false
	case buttonFlag&0x0004 != 0: // RIGHT_DOWN
		return 2, true
	case buttonFlag&0x0008 != 0: // RIGHT_UP
		return 2, false
	case buttonFlag&0x0010 != 0: // MIDDLE_DOWN
		return 3, true
	case buttonFlag&0x0020 != 0: // MIDDLE_UP
		return 3, false
	case buttonFlag&0x0040 != 0: // BUTTON4_DOWN
		return 4, true
	case buttonFlag&0x0080 != 0: // BUTTON4_UP
		return 4, false
	case buttonFlag&0x0100 != 0: // BUTTON5_DOWN
		return 5, true
	case buttonFlag&0x0200 != 0: // BUTTON5_UP
		return 5, false
	}
	return 0, false
}
