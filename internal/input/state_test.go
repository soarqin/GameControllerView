package input

import (
	"testing"
)

// containsUint16 reports whether s contains v (order-independent).
func containsUint16(s []uint16, v uint16) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// TestComputeKeyMouseDelta covers key transitions, mouse button transitions,
// mouse movement, and the pending-event override path.
func TestComputeKeyMouseDelta(t *testing.T) {
	t.Run("both empty → delta is empty", func(t *testing.T) {
		var prev, curr KeyMouseState
		d := ComputeKeyMouseDelta(prev, curr)
		if !d.IsEmpty() {
			t.Errorf("expected empty delta, got %+v", d)
		}
	})

	t.Run("key added → appears in KeysDown", func(t *testing.T) {
		prev := KeyMouseState{}
		curr := KeyMouseState{Keys: map[uint16]bool{0x1E: true}}
		d := ComputeKeyMouseDelta(prev, curr)
		if !containsUint16(d.KeysDown, 0x1E) {
			t.Errorf("KeysDown = %v, expected to contain 0x1E", d.KeysDown)
		}
		if len(d.KeysUp) != 0 {
			t.Errorf("KeysUp = %v, expected empty", d.KeysUp)
		}
	})

	t.Run("key removed → appears in KeysUp", func(t *testing.T) {
		prev := KeyMouseState{Keys: map[uint16]bool{0x1E: true}}
		curr := KeyMouseState{}
		d := ComputeKeyMouseDelta(prev, curr)
		if !containsUint16(d.KeysUp, 0x1E) {
			t.Errorf("KeysUp = %v, expected to contain 0x1E", d.KeysUp)
		}
		if len(d.KeysDown) != 0 {
			t.Errorf("KeysDown = %v, expected empty", d.KeysDown)
		}
	})

	t.Run("mouse button added → appears in ButtonsDown", func(t *testing.T) {
		prev := KeyMouseState{}
		curr := KeyMouseState{MouseButtons: map[uint16]bool{1: true}}
		d := ComputeKeyMouseDelta(prev, curr)
		if !containsUint16(d.ButtonsDown, 1) {
			t.Errorf("ButtonsDown = %v, expected to contain 1", d.ButtonsDown)
		}
		if len(d.ButtonsUp) != 0 {
			t.Errorf("ButtonsUp = %v, expected empty", d.ButtonsUp)
		}
	})

	t.Run("mouse button removed → appears in ButtonsUp", func(t *testing.T) {
		prev := KeyMouseState{MouseButtons: map[uint16]bool{2: true}}
		curr := KeyMouseState{}
		d := ComputeKeyMouseDelta(prev, curr)
		if !containsUint16(d.ButtonsUp, 2) {
			t.Errorf("ButtonsUp = %v, expected to contain 2", d.ButtonsUp)
		}
		if len(d.ButtonsDown) != 0 {
			t.Errorf("ButtonsDown = %v, expected empty", d.ButtonsDown)
		}
	})

	t.Run("mouse movement → MouseMove is non-nil with correct values", func(t *testing.T) {
		var prev KeyMouseState
		curr := KeyMouseState{MouseDX: 0.5, MouseDY: -0.3}
		d := ComputeKeyMouseDelta(prev, curr)
		if d.MouseMove == nil {
			t.Fatal("expected MouseMove to be non-nil")
		}
		if d.MouseMove.X != 0.5 {
			t.Errorf("MouseMove.X = %v, want 0.5", d.MouseMove.X)
		}
		if d.MouseMove.Y != -0.3 {
			t.Errorf("MouseMove.Y = %v, want -0.3", d.MouseMove.Y)
		}
	})

	t.Run("no movement → MouseMove is nil", func(t *testing.T) {
		var prev, curr KeyMouseState
		d := ComputeKeyMouseDelta(prev, curr)
		if d.MouseMove != nil {
			t.Errorf("MouseMove = %v, want nil", d.MouseMove)
		}
	})

	t.Run("pending keys override state comparison", func(t *testing.T) {
		// PendingKeysDown set but Keys map is empty; pending path must be used.
		prev := KeyMouseState{}
		curr := KeyMouseState{
			PendingKeysDown: []uint16{0x10, 0x11},
		}
		d := ComputeKeyMouseDelta(prev, curr)
		if !containsUint16(d.KeysDown, 0x10) {
			t.Errorf("KeysDown = %v, expected to contain 0x10", d.KeysDown)
		}
		if !containsUint16(d.KeysDown, 0x11) {
			t.Errorf("KeysDown = %v, expected to contain 0x11", d.KeysDown)
		}
	})
}

// TestMouseButtonFlagToIOCode verifies both the returned code and the pressed flag
// for every documented Windows Raw Input button flag.
func TestMouseButtonFlagToIOCode(t *testing.T) {
	tests := []struct {
		name        string
		flag        uint16
		wantCode    uint16
		wantPressed bool
	}{
		{"LEFT_DOWN", 0x0001, 1, true},
		{"LEFT_UP", 0x0002, 1, false},
		{"RIGHT_DOWN", 0x0004, 2, true},
		{"RIGHT_UP", 0x0008, 2, false},
		{"MIDDLE_DOWN", 0x0010, 3, true},
		{"BUTTON4_DOWN (X1/back)", 0x0040, 4, true},
		{"BUTTON4_UP (X1/back)", 0x0080, 4, false},
		{"BUTTON5_DOWN (X2/forward)", 0x0100, 5, true},
		{"BUTTON5_UP (X2/forward)", 0x0200, 5, false},
		{"no match → (0,false)", 0x0000, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, pressed := MouseButtonFlagToIOCode(tt.flag)
			if code != tt.wantCode || pressed != tt.wantPressed {
				t.Errorf("MouseButtonFlagToIOCode(0x%04x) = (%d, %v), want (%d, %v)",
					tt.flag, code, pressed, tt.wantCode, tt.wantPressed)
			}
		})
	}
}
