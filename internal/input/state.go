// Package input defines keyboard and mouse input state types used across the application.
package input

// KeyMouseState represents the complete state of keyboard and mouse at a point in time.
type KeyMouseState struct {
	// Keys maps uiohook scancode to pressed state. Only currently-held keys are present.
	Keys map[uint16]bool `json:"keys"`
	// MouseButtons maps Input Overlay mouse button code (1-5) to pressed state.
	// 1=left, 2=right, 3=middle, 4=X1(back), 5=X2(forward)
	MouseButtons map[uint16]bool `json:"mouseButtons"`
	// MouseDX and MouseDY are normalized mouse movement deltas in the range [-1.0, 1.0].
	// Computed from raw pixel deltas divided by the configured sensitivity value.
	MouseDX float32 `json:"mouseDX"`
	MouseDY float32 `json:"mouseDY"`
	// WheelUp and WheelDown indicate whether a scroll event occurred during this tick.
	WheelUp   bool `json:"wheelUp"`
	WheelDown bool `json:"wheelDown"`

	// PendingKeysDown/Up capture key events that occurred within this tick, including
	// keys that were pressed and released before the snapshot was taken.
	// These are used by ComputeKeyMouseDelta instead of prev/curr comparison.
	PendingKeysDown []uint16 `json:"-"`
	PendingKeysUp   []uint16 `json:"-"`
	// PendingButtonsDown/Up capture mouse button events within this tick.
	PendingButtonsDown []uint16 `json:"-"`
	PendingButtonsUp   []uint16 `json:"-"`
}

// KeyMouseDelta contains only the fields that changed since the previous tick.
// Nil/zero-value fields indicate no change.
type KeyMouseDelta struct {
	// KeysDown contains scancodes of keys that became pressed this tick.
	KeysDown []uint16 `json:"keysDown,omitempty"`
	// KeysUp contains scancodes of keys that became released this tick.
	KeysUp []uint16 `json:"keysUp,omitempty"`
	// ButtonsDown contains mouse button codes that became pressed this tick.
	ButtonsDown []uint16 `json:"buttonsDown,omitempty"`
	// ButtonsUp contains mouse button codes that became released this tick.
	ButtonsUp []uint16 `json:"buttonsUp,omitempty"`
	// MouseMove is non-nil when there was mouse movement this tick.
	MouseMove *MouseMoveData `json:"mouseMove,omitempty"`
	// WheelUp / WheelDown indicate scroll direction this tick.
	WheelUp   bool `json:"wheelUp,omitempty"`
	WheelDown bool `json:"wheelDown,omitempty"`
}

// MouseMoveData carries the normalized mouse movement delta for a tick.
type MouseMoveData struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
}

// IsEmpty returns true if there are no changes in this delta.
func (d *KeyMouseDelta) IsEmpty() bool {
	return len(d.KeysDown) == 0 &&
		len(d.KeysUp) == 0 &&
		len(d.ButtonsDown) == 0 &&
		len(d.ButtonsUp) == 0 &&
		d.MouseMove == nil &&
		!d.WheelUp &&
		!d.WheelDown
}

// ComputeKeyMouseDelta calculates the delta between two KeyMouseState snapshots.
// prev is the previous state; curr is the new state.
//
// For keys and mouse buttons, curr.Pending* fields are used when available (non-nil),
// because a button press+release within a single 16ms tick would otherwise be invisible
// to a prev/curr state comparison. The Pending* slices record every event that occurred
// during the tick regardless of the final held state.
func ComputeKeyMouseDelta(prev, curr KeyMouseState) *KeyMouseDelta {
	d := &KeyMouseDelta{}

	// Keys
	if curr.PendingKeysDown != nil || curr.PendingKeysUp != nil {
		// Use event-driven pending lists (captures sub-tick press+release)
		d.KeysDown = curr.PendingKeysDown
		d.KeysUp = curr.PendingKeysUp
	} else {
		// Fallback: state comparison
		for code := range curr.Keys {
			if !prev.Keys[code] {
				d.KeysDown = append(d.KeysDown, code)
			}
		}
		for code := range prev.Keys {
			if !curr.Keys[code] {
				d.KeysUp = append(d.KeysUp, code)
			}
		}
	}

	// Mouse buttons
	if curr.PendingButtonsDown != nil || curr.PendingButtonsUp != nil {
		// Use event-driven pending lists
		d.ButtonsDown = curr.PendingButtonsDown
		d.ButtonsUp = curr.PendingButtonsUp
	} else {
		// Fallback: state comparison
		for code := range curr.MouseButtons {
			if !prev.MouseButtons[code] {
				d.ButtonsDown = append(d.ButtonsDown, code)
			}
		}
		for code := range prev.MouseButtons {
			if !curr.MouseButtons[code] {
				d.ButtonsUp = append(d.ButtonsUp, code)
			}
		}
	}

	// Mouse movement: send if non-zero
	if curr.MouseDX != 0 || curr.MouseDY != 0 {
		d.MouseMove = &MouseMoveData{X: curr.MouseDX, Y: curr.MouseDY}
	}

	d.WheelUp = curr.WheelUp
	d.WheelDown = curr.WheelDown

	return d
}
