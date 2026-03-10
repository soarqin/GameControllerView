//go:build windows

package gamepad

import (
	"context"
	"fmt"
	"log"
	"time"
)

const (
	deadzone   = 0.05
	pollDelay  = 16 * time.Millisecond // ~60 Hz
	triggerMax = 255.0                 // XINPUT trigger range: 0-255
)

// Run initialises XInput and runs the polling loop until ctx is cancelled.
// XInput is thread-safe and does not require LockOSThread.
func (r *Reader) Run(ctx context.Context) {
	if err := procXInputGetState.Find(); err != nil {
		log.Fatalf("gamepad: XInput not available: %v", err)
	}
	log.Println("XInput initialised")

	// Initial scan for already-connected controllers
	for i := uint32(0); i < xinputMaxControllers; i++ {
		var state xinputState
		if xiGetStateEx(i, &state) == errorSuccess {
			r.connectController(i)
		}
	}

	for {
		select {
		case <-ctx.Done():
			r.mu.Lock()
			r.joysticks = make(map[uint32]*joystickInfo)
			r.joystickOrder = nil
			r.hasActive = false
			r.state = GamepadState{}
			r.mu.Unlock()
			close(r.changes)
			return
		default:
		}

		r.pollAll()
		time.Sleep(pollDelay)
	}
}

// pollAll scans all XInput slots for connect/disconnect/state changes.
func (r *Reader) pollAll() {
	for i := uint32(0); i < xinputMaxControllers; i++ {
		var state xinputState
		ret := xiGetStateEx(i, &state)

		r.mu.RLock()
		_, wasConnected := r.joysticks[i]
		r.mu.RUnlock()

		switch {
		case ret == errorSuccess && !wasConnected:
			r.connectController(i)

		case ret != errorSuccess && wasConnected:
			r.disconnectController(i)

		case ret == errorSuccess && wasConnected:
			r.updateState(i, &state)
		}
	}
}

// connectController handles a newly detected XInput controller at slot i.
func (r *Reader) connectController(userIndex uint32) {
	// Try to get VID/PID via the undocumented XInputGetCapabilitiesEx (ordinal 108).
	// Fall back to xboxMapping if unavailable (e.g., Windows 7 / xinput1_3.dll).
	vid, pid, hasPID := xiGetCapabilitiesEx(userIndex)
	mapping := xboxMapping // default
	vidPID := ""
	if hasPID {
		mapping = GetMapping(vid, pid)
		vidPID = fmt.Sprintf("VID_%04X&PID_%04X", vid, pid)
	}

	// Build a human-readable name.
	name := buildControllerName(mapping.Name, vidPID)

	info := &joystickInfo{
		mapping: mapping,
		name:    name,
		vidPID:  vidPID,
		index:   userIndex,
	}

	r.mu.Lock()
	r.joysticks[userIndex] = info
	// Insert into order list if not already present.
	found := false
	for _, idx := range r.joystickOrder {
		if idx == userIndex {
			found = true
			break
		}
	}
	if !found {
		r.joystickOrder = append(r.joystickOrder, userIndex)
	}
	playerIndex := 0
	for i, idx := range r.joystickOrder {
		if idx == userIndex {
			playerIndex = i + 1
			break
		}
	}
	r.mu.Unlock()

	log.Printf("Gamepad connected: Player %d - %s (slot=%d %s) mapping=%s",
		playerIndex, name, userIndex, vidPID, mapping.Name)

	// Set as active if no active controller yet.
	if !r.hasActive {
		r.mu.Lock()
		r.activeIndex = userIndex
		r.hasActive = true
		r.state.Connected = true
		r.state.Name = name
		r.state.ControllerType = mapping.Name
		r.state.PlayerIndex = playerIndex
		r.mu.Unlock()

		log.Printf("Active controller set to player %d: %s (slot=%d)", playerIndex, name, userIndex)
		r.emitState()
	}
}

// disconnectController handles removal of an XInput controller at slot i.
func (r *Reader) disconnectController(userIndex uint32) {
	r.mu.Lock()
	info, ok := r.joysticks[userIndex]
	if !ok {
		r.mu.Unlock()
		return
	}
	playerIndex := r.getPlayerIndexLocked(userIndex)
	log.Printf("Gamepad disconnected: Player %d - %s (slot=%d)", playerIndex, info.name, userIndex)

	delete(r.joysticks, userIndex)
	newOrder := make([]uint32, 0, len(r.joystickOrder))
	for _, idx := range r.joystickOrder {
		if idx != userIndex {
			newOrder = append(newOrder, idx)
		}
	}
	r.joystickOrder = newOrder

	wasActive := r.hasActive && r.activeIndex == userIndex
	r.mu.Unlock()

	if !wasActive {
		return
	}

	// Active controller disconnected: try to promote the next available one.
	r.mu.Lock()
	r.hasActive = false
	if len(r.joystickOrder) == 0 {
		r.state = GamepadState{}
		r.mu.Unlock()
		r.emitState()
		return
	}

	// Promote first remaining controller.
	nextIndex := r.joystickOrder[0]
	nextInfo := r.joysticks[nextIndex]
	nextPlayer := r.getPlayerIndexLocked(nextIndex)
	r.activeIndex = nextIndex
	r.hasActive = true
	r.state.Connected = true
	r.state.Name = nextInfo.name
	r.state.ControllerType = nextInfo.mapping.Name
	r.state.PlayerIndex = nextPlayer
	r.mu.Unlock()

	log.Printf("Active controller promoted to player %d: %s (slot=%d)", nextPlayer, nextInfo.name, nextIndex)
	r.emitState()
}

// updateState reads the current XInput state for the active controller and emits if changed.
func (r *Reader) updateState(userIndex uint32, state *xinputState) {
	r.mu.RLock()
	isActive := r.hasActive && r.activeIndex == userIndex
	info := r.joysticks[userIndex]
	r.mu.RUnlock()

	if !isActive || info == nil {
		return
	}

	newState := convertXInputState(state, info)

	r.mu.Lock()
	delta := ComputeDelta(r.prevState, newState)
	if !delta.IsEmpty() {
		r.state = newState
		r.prevState = newState
		r.mu.Unlock()
		r.emitState()
	} else {
		r.mu.Unlock()
	}
}

// convertXInputState converts a raw xinputState to a GamepadState.
func convertXInputState(xs *xinputState, info *joystickInfo) GamepadState {
	gp := xs.Gamepad
	mapping := info.mapping

	state := GamepadState{
		Connected:      true,
		ControllerType: mapping.Name,
		Name:           info.name,
	}

	// Triggers: uint8 (0-255) → float64 (0.0-1.0)
	ltRaw := float64(gp.LeftTrigger) / triggerMax
	rtRaw := float64(gp.RightTrigger) / triggerMax
	state.Triggers.LT.Value = applyDeadzone(ltRaw, deadzone)
	state.Triggers.RT.Value = applyDeadzone(rtRaw, deadzone)

	// Sticks: int16 → float64 (-1.0 to 1.0)
	lx := applyDeadzone(normalizeAxis(gp.ThumbLX), deadzone)
	ly := applyDeadzone(normalizeAxis(gp.ThumbLY), deadzone)
	rx := applyDeadzone(normalizeAxis(gp.ThumbRX), deadzone)
	ry := applyDeadzone(normalizeAxis(gp.ThumbRY), deadzone)

	// XInput Y axes are positive-up; frontend uses positive-down, so invert Y.
	state.Sticks.Left.Position.X = lx
	state.Sticks.Left.Position.Y = -ly
	state.Sticks.Right.Position.X = rx
	state.Sticks.Right.Position.Y = -ry

	// Buttons
	btn := gp.Buttons
	state.Buttons.A = btn&xiA != 0
	state.Buttons.B = btn&xiB != 0
	state.Buttons.X = btn&xiX != 0
	state.Buttons.Y = btn&xiY != 0
	state.Buttons.LB = btn&xiLeftShoulder != 0
	state.Buttons.RB = btn&xiRightShoulder != 0
	state.Buttons.Back = btn&xiBack != 0
	state.Buttons.Start = btn&xiStart != 0
	state.Buttons.Guide = btn&xiGuide != 0
	state.Sticks.Left.Pressed = btn&xiLeftThumb != 0
	state.Sticks.Right.Pressed = btn&xiRightThumb != 0

	// D-pad
	state.Dpad.Up = btn&xiDpadUp != 0
	state.Dpad.Down = btn&xiDpadDown != 0
	state.Dpad.Left = btn&xiDpadLeft != 0
	state.Dpad.Right = btn&xiDpadRight != 0

	// Override button/axis mapping from device mapping table if defined.
	// XInput provides a canonical layout, but mapping.Name is used for frontend
	// config selection (e.g., "playstation" shows PS button labels).
	// The actual input values are always read via XInput above; the mapping table
	// is only consulted for the controller type name (frontend visual).
	// No axis remapping needed for XInput — it always uses the canonical layout.
	_ = mapping

	return state
}

// getPlayerIndexLocked returns the 1-based player index for a slot.
// Caller must hold r.mu (at least read lock).
func (r *Reader) getPlayerIndexLocked(userIndex uint32) int {
	for i, idx := range r.joystickOrder {
		if idx == userIndex {
			return i + 1
		}
	}
	return 0
}

// buildControllerName constructs a human-readable controller name.
func buildControllerName(mappingName, vidPID string) string {
	if vidPID != "" {
		return fmt.Sprintf("%s (%s)", mappingName, vidPID)
	}
	return mappingName
}
