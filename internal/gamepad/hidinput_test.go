package gamepad

import (
	"math"
	"testing"
)

// TestNormalizeHIDAxis verifies normalizeHIDAxis across stick, trigger, edge, and clamp cases.
func TestNormalizeHIDAxis(t *testing.T) {
	tests := []struct {
		name      string
		raw       uint32
		logMin    int32
		logMax    int32
		isTrigger bool
		want      float64
	}{
		// Stick (isTrigger=false): maps [logMin,logMax] → [-1,1]
		{"stick min", 0, 0, 255, false, -1.0},
		{"stick center", 128, 0, 255, false, 0.0},
		{"stick max", 255, 0, 255, false, 1.0},

		// Trigger (isTrigger=true): maps [logMin,logMax] → [0,1]
		{"trigger min", 0, 0, 255, true, 0.0},
		{"trigger max", 255, 0, 255, true, 1.0},

		// Edge: logMin == logMax → guard fires, return 0 (no panic)
		{"equal logMin logMax", 128, 0, 0, false, 0.0},

		// Edge: logMax < logMin → guard fires, return 0 (no panic)
		{"logMax less than logMin", 128, 10, 5, false, 0.0},

		// Clamp: raw beyond logMax should be clamped to 1.0 for trigger
		{"trigger clamp high", 300, 0, 255, true, 1.0},
	}

	const tolerance = 0.02
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeHIDAxis(tt.raw, tt.logMin, tt.logMax, tt.isTrigger)
			if math.Abs(got-tt.want) > tolerance {
				t.Errorf("normalizeHIDAxis(%d, %d, %d, %v) = %v, want %v (tolerance ±%v)",
					tt.raw, tt.logMin, tt.logMax, tt.isTrigger, got, tt.want, tolerance)
			}
		})
	}
}

// TestHatDirTable verifies the cardinal direction entries and the table length.
func TestHatDirTable(t *testing.T) {
	if len(hatDirTable) != 8 {
		t.Fatalf("len(hatDirTable) = %d, want 8", len(hatDirTable))
	}

	// Each entry is [Up, Down, Left, Right]
	cardinals := []struct {
		index uint
		dir   string
		want  [4]bool
	}{
		{0, "N (up only)", [4]bool{true, false, false, false}},
		{2, "E (right only)", [4]bool{false, false, false, true}},
		{4, "S (down only)", [4]bool{false, true, false, false}},
		{6, "W (left only)", [4]bool{false, false, true, false}},
	}
	for _, tt := range cardinals {
		t.Run(tt.dir, func(t *testing.T) {
			got := hatDirTable[tt.index]
			if got != tt.want {
				t.Errorf("hatDirTable[%d] = %v, want %v (up,down,left,right)",
					tt.index, got, tt.want)
			}
		})
	}
}

// TestResolveButtonTarget verifies lookup against defaultButtonOrder and out-of-range behaviour.
func TestResolveButtonTarget(t *testing.T) {
	tests := []struct {
		name        string
		mapping     *DeviceMapping
		buttonUsage uint16
		want        string
	}{
		// defaultButtonOrder[0] == "x"
		{"nil mapping button 1 → x", nil, 1, "x"},
		// defaultButtonOrder[1] == "a"
		{"nil mapping button 2 → a", nil, 2, "a"},
		// usage 999 is far beyond the slice, must return ""
		{"nil mapping out of range → empty", nil, 999, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveButtonTarget(tt.mapping, tt.buttonUsage)
			if got != tt.want {
				t.Errorf("resolveButtonTarget(%v, %d) = %q, want %q",
					tt.mapping, tt.buttonUsage, got, tt.want)
			}
		})
	}
}

// TestSDLNameToControllerType guards the case-sensitive contract between the
// HID SDL path (which calls this function) and the frontend's configNameForType()
// in internal/web/frontend/config.js — whose configMap keys are lowercase
// ("xbox", "playstation", "switch_pro"). Returning a value with mismatched
// case silently falls through to the xbox default, producing the symptom
// "PS4/PS5 controllers render with Xbox button layout".
func TestSDLNameToControllerType(t *testing.T) {
	tests := []struct {
		name    string
		sdlName string
		want    string
	}{
		// PlayStation family — must return lowercase "playstation"
		// (matches configMap key in internal/web/frontend/config.js).
		{"DualSense", "DualSense Wireless Controller", "playstation"},
		{"PS5 Controller", "PS5 Controller", "playstation"},
		{"PS5 Access", "PS5 Access Controller", "playstation"},
		{"PS4 Controller", "PS4 Controller", "playstation"},
		{"Sony DualShock 4", "Sony DualShock 4", "playstation"},
		{"Sony DualShock 4 V2", "Sony DualShock 4 V2", "playstation"},
		{"PS3 Controller", "PS3 Controller", "playstation"},

		// Switch Pro family — must return lowercase "switch_pro".
		{"Pro Controller", "Pro Controller", "switch_pro"},
		{"Switch Pro", "Switch Pro Controller", "switch_pro"},
		{"Nintendo Switch", "Nintendo Switch Pro Controller", "switch_pro"},

		// Default fallback — must return lowercase "xbox", not "Xbox".
		{"Xbox 360", "Microsoft Xbox 360 Wireless Controller", "xbox"},
		{"Generic", "Generic USB Joystick", "xbox"},
		{"Empty", "", "xbox"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sdlNameToControllerType(tt.sdlName)
			if got != tt.want {
				t.Errorf("sdlNameToControllerType(%q) = %q, want %q",
					tt.sdlName, got, tt.want)
			}
			// Defensive: any returned identifier must equal its lowercase form,
			// otherwise configNameForType() in config.js will silently fail.
			if got != "" && got != toLowerASCII(got) {
				t.Errorf("sdlNameToControllerType(%q) returned non-lowercase %q "+
					"— frontend configMap requires lowercase keys",
					tt.sdlName, got)
			}
		})
	}
}

// toLowerASCII lowercases ASCII letters; we avoid pulling in strings here since
// the test only needs a simple guard against accidental capitalisation.
func toLowerASCII(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// TestApplyButton verifies that applyButton sets the correct GamepadState field
// and that an empty/unknown target is a no-op (does not panic).
func TestApplyButton(t *testing.T) {
	t.Run("target a sets Buttons.A", func(t *testing.T) {
		var state GamepadState
		applyButton(&state, "a")
		if !state.Buttons.A {
			t.Error("applyButton(state, \"a\"): expected Buttons.A=true, got false")
		}
	})

	t.Run("target b sets Buttons.B", func(t *testing.T) {
		var state GamepadState
		applyButton(&state, "b")
		if !state.Buttons.B {
			t.Error("applyButton(state, \"b\"): expected Buttons.B=true, got false")
		}
	})

	t.Run("empty target does not panic and leaves state unchanged", func(t *testing.T) {
		var state GamepadState
		// Must not panic.
		applyButton(&state, "")
		zero := GamepadState{}
		if state.Buttons != zero.Buttons {
			t.Errorf("applyButton with empty target modified Buttons: %+v", state.Buttons)
		}
	})

	t.Run("unknown target does not panic and leaves state unchanged", func(t *testing.T) {
		var state GamepadState
		applyButton(&state, "nonexistent_button")
		zero := GamepadState{}
		if state.Buttons != zero.Buttons {
			t.Errorf("applyButton with unknown target modified Buttons: %+v", state.Buttons)
		}
	})
}

// TestApplyButtonTriggerGuard verifies that the digital trigger fallback in
// applyButton does NOT overwrite an analog trigger value already set by axis
// parsing. PlayStation L2/R2 fire both the analog axis and the digital button
// simultaneously; before the guard, fully-pressed triggers always reported 1.0
// regardless of analog input, and partially-pressed triggers snapped to 1.0
// once the digital bit fired. With the guard, the analog value is preserved.
func TestApplyButtonTriggerGuard(t *testing.T) {
	t.Run("lt: digital press preserves prior analog value", func(t *testing.T) {
		state := GamepadState{}
		state.Triggers.LT.Value = 0.5 // axis already populated this
		applyButton(&state, "lt")
		if state.Triggers.LT.Value != 0.5 {
			t.Errorf("LT.Value = %v, want 0.5 (analog should be preserved)", state.Triggers.LT.Value)
		}
	})

	t.Run("rt: digital press preserves prior analog value", func(t *testing.T) {
		state := GamepadState{}
		state.Triggers.RT.Value = 0.97
		applyButton(&state, "rt")
		if state.Triggers.RT.Value != 0.97 {
			t.Errorf("RT.Value = %v, want 0.97 (analog should be preserved)", state.Triggers.RT.Value)
		}
	})

	t.Run("lt: digital press on zero analog value sets 1.0", func(t *testing.T) {
		state := GamepadState{}
		// LT.Value is zero (no analog input or analog deadzoned to 0).
		applyButton(&state, "lt")
		if state.Triggers.LT.Value != 1.0 {
			t.Errorf("LT.Value = %v, want 1.0 (digital fallback)", state.Triggers.LT.Value)
		}
	})

	t.Run("rt: digital press on zero analog value sets 1.0", func(t *testing.T) {
		state := GamepadState{}
		applyButton(&state, "rt")
		if state.Triggers.RT.Value != 1.0 {
			t.Errorf("RT.Value = %v, want 1.0 (digital fallback)", state.Triggers.RT.Value)
		}
	})
}
