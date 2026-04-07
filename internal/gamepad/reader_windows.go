//go:build windows

package gamepad

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/soar/inputview/internal/rawinput"
)

const (
	triggerMax = 255.0 // XINPUT trigger range: 0-255
)

// HID usage page / usage IDs for gamepad registration (mirrors rawinput constants).
const (
	hidUsagePageGeneric = 0x01
	hidUsageIDJoystick  = 0x04
	hidUsageIDGamepad   = 0x05
)

// Run initialises XInput, registers HID callbacks, and runs the polling loop
// until ctx is cancelled. XInput is thread-safe and does not require LockOSThread.
func (r *Reader) Run(ctx context.Context) {
	xinputAvailable := true
	if err := procXInputGetState.Find(); err != nil {
		slog.Warn("XInput unavailable, running HID-only mode", "error", err)
		xinputAvailable = false
	}

	if xinputAvailable {
		slog.Info("XInput initialised")
		// Initial scan for already-connected XInput controllers.
		for i := uint32(0); i < xinputMaxControllers; i++ {
			var state xinputState
			if xiGetStateEx(i, &state) == errorSuccess {
				r.connectXInput(i)
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			r.mu.Lock()
			r.joysticks = make(map[joystickKey]*joystickInfo)
			r.joystickOrder = nil
			r.hasActive = false
			r.state = GamepadState{}
			r.mu.Unlock()
			close(r.changes)
			return
		default:
		}

		if xinputAvailable {
			r.pollAllXInput()
		}
		time.Sleep(r.pollDelay)
	}
}

// SetRawInputReader registers HID gamepad callbacks on the provided rawinput.Reader
// so that non-XInput gamepads (PS4/PS5/Switch Pro/generic HID) are captured through
// the same HWND_MESSAGE window. Must be called before kmReader.Run().
func (r *Reader) SetRawInputReader(kmReader *rawinput.Reader) {
	kmReader.RegisterHIDCallback(
		hidUsagePageGeneric, hidUsageIDJoystick,
		r.handleHIDInput, r.handleHIDDeviceChange,
	)
	kmReader.RegisterHIDCallback(
		hidUsagePageGeneric, hidUsageIDGamepad,
		r.handleHIDInput, r.handleHIDDeviceChange,
	)
}

// ---------------------------------------------------------------------------
// XInput path
// ---------------------------------------------------------------------------

// pollAllXInput scans all XInput slots for connect/disconnect/state changes.
func (r *Reader) pollAllXInput() {
	for i := uint32(0); i < xinputMaxControllers; i++ {
		var state xinputState
		ret := xiGetStateEx(i, &state)
		key := xinputKey(i)

		r.mu.RLock()
		_, wasConnected := r.joysticks[key]
		r.mu.RUnlock()

		switch {
		case ret == errorSuccess && !wasConnected:
			r.connectXInput(i)
		case ret != errorSuccess && wasConnected:
			r.disconnectJoystick(key)
		case ret == errorSuccess && wasConnected:
			r.updateXInputState(i, &state)
		}
	}
}

// connectXInput handles a newly detected XInput controller at slot i.
func (r *Reader) connectXInput(userIndex uint32) {
	vid, pid, hasPID := xiGetCapabilitiesEx(userIndex)
	mapping := xboxMapping
	vidPID := ""
	if hasPID {
		mapping = GetMapping(vid, pid)
		vidPID = fmt.Sprintf("VID_%04X&PID_%04X", vid, pid)
		// Record the VID/PID so that duplicate HID registrations for the same
		// physical device can be suppressed (e.g. Switch Pro via Steam XInput).
		r.mu.Lock()
		r.xinputVIDPIDs[deviceKey{VendorID: vid, ProductID: pid}]++
		r.mu.Unlock()
	}
	name := buildControllerName(mapping.Name, vidPID)

	info := &joystickInfo{
		mapping:    mapping,
		name:       name,
		vidPID:     vidPID,
		sourceType: "xinput",
		xinputSlot: userIndex,
		devKey:     deviceKey{VendorID: vid, ProductID: pid},
	}
	key := xinputKey(userIndex)
	r.registerJoystick(key, info)
}

// updateXInputState reads the current XInput state for the active controller.
func (r *Reader) updateXInputState(userIndex uint32, state *xinputState) {
	key := xinputKey(userIndex)
	r.mu.RLock()
	isActive := r.hasActive && r.activeKey == key
	info := r.joysticks[key]
	r.mu.RUnlock()

	if !isActive || info == nil {
		return
	}

	newState := convertXInputState(state, info, r.deadzone)
	newState.PlayerIndex = r.GetPlayerIndex()

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
// dz is the deadzone threshold to apply to analog inputs.
func convertXInputState(xs *xinputState, info *joystickInfo, dz float64) GamepadState {
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
	state.Triggers.LT.Value = applyDeadzone(ltRaw, dz)
	state.Triggers.RT.Value = applyDeadzone(rtRaw, dz)

	// Sticks: int16 → float64 (-1.0 to 1.0)
	lx := applyDeadzone(normalizeAxis(gp.ThumbLX), dz)
	ly := applyDeadzone(normalizeAxis(gp.ThumbLY), dz)
	rx := applyDeadzone(normalizeAxis(gp.ThumbRX), dz)
	ry := applyDeadzone(normalizeAxis(gp.ThumbRY), dz)

	// XInput Y axes are positive-up. The frontend canvas rendering inverts Y
	// (knobY = s.y - position.y * maxTravel), so we pass the raw value unchanged.
	state.Sticks.Left.Position.X = lx
	state.Sticks.Left.Position.Y = ly
	state.Sticks.Right.Position.X = rx
	state.Sticks.Right.Position.Y = ry

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

	_ = mapping // Name is used above; axis remapping not needed for XInput
	return state
}

// ---------------------------------------------------------------------------
// HID path (callbacks from rawinput message loop)
// ---------------------------------------------------------------------------

// handleHIDInput is called from the rawinput message loop goroutine for each
// WM_INPUT event from a registered HID gamepad device.
func (r *Reader) handleHIDInput(hDevice uintptr, rawData []byte, reportSize uint32) {
	r.mu.Lock()

	// Suppress residual WM_INPUT events that arrive after a GIDC_REMOVAL
	// notification.  Without this guard the fallback registration path below
	// would re-add the device to the joystick list even though it has already
	// been disconnected.
	if _, wasDisconnected := r.disconnectedHIDs[hDevice]; wasDisconnected {
		r.mu.Unlock()
		return
	}

	dev := r.getOrInitHIDDevice(hDevice)
	if dev == nil || dev.isXInput || dev.isInvalid {
		r.mu.Unlock()
		return
	}

	// Ensure the device appears in the joystick list (handles cases where
	// WM_INPUT_DEVICE_CHANGE was not received, e.g. device already connected at startup).
	key := hidKey(hDevice)
	if _, exists := r.joysticks[key]; !exists {
		info := &joystickInfo{
			mapping:    dev.mapping,
			name:       dev.name,
			vidPID:     fmt.Sprintf("VID_%04X&PID_%04X", dev.vendorID, dev.productID),
			sourceType: "hid",
			hDevice:    hDevice,
			devKey:     deviceKey{VendorID: dev.vendorID, ProductID: dev.productID},
		}
		r.mu.Unlock()
		r.registerJoystick(key, info)
		r.mu.Lock()
	}

	isActive := r.hasActive && r.activeKey == key
	r.mu.Unlock()

	if !isActive {
		return
	}

	// If multiple HID reports are batched in a single WM_INPUT message
	// (dwCount > 1), use only the last report (most recent data).
	if reportSize > 0 && uint32(len(rawData)) > reportSize {
		rawData = rawData[uint32(len(rawData))-reportSize:]
	}

	newState, ok := parseHIDReport(dev, rawData, r.deadzone)
	if !ok {
		return // incompatible report ID (non-input report); skip
	}
	newState.PlayerIndex = r.GetPlayerIndex()

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

// handleHIDDeviceChange is called from the rawinput message loop goroutine when
// a HID gamepad is connected or disconnected.
func (r *Reader) handleHIDDeviceChange(added bool, hDevice uintptr) {
	if added {
		r.mu.Lock()
		// Device (re-)connected: remove from the disconnected-handle blocklist
		// so that handleHIDInput can re-register it on the next WM_INPUT event.
		delete(r.disconnectedHIDs, hDevice)
		dev := r.getOrInitHIDDevice(hDevice)
		if dev == nil || dev.isXInput || dev.isInvalid {
			r.mu.Unlock()
			return
		}
		key := hidKey(hDevice)
		info := &joystickInfo{
			mapping:    dev.mapping,
			name:       dev.name,
			vidPID:     fmt.Sprintf("VID_%04X&PID_%04X", dev.vendorID, dev.productID),
			sourceType: "hid",
			hDevice:    hDevice,
			devKey:     deviceKey{VendorID: dev.vendorID, ProductID: dev.productID},
		}
		r.mu.Unlock()
		r.registerJoystick(key, info)
	} else {
		key := hidKey(hDevice)
		r.mu.Lock()
		delete(r.hidDevices, hDevice)
		// Record as disconnected so that residual WM_INPUT events arriving after
		// this GIDC_REMOVAL notification do not trigger a spurious re-registration.
		r.disconnectedHIDs[hDevice] = struct{}{}
		r.mu.Unlock()
		r.disconnectJoystick(key)
	}
}

// getOrInitHIDDevice returns the cached hidDeviceInfo for hDevice, initialising
// it on first call. Caller must hold r.mu.
func (r *Reader) getOrInitHIDDevice(hDevice uintptr) *hidDeviceInfo {
	if dev, ok := r.hidDevices[hDevice]; ok {
		return dev
	}
	// Init outside the lock to avoid holding it during potentially slow API calls.
	r.mu.Unlock()
	dev := initHIDDevice(hDevice)
	r.mu.Lock()
	if existing, ok := r.hidDevices[hDevice]; ok {
		return existing
	}
	if dev != nil {
		// Suppress HID devices whose VID/PID matches a currently connected
		// XInput device. XInput emulation software (Steam, BetterJoy) creates
		// a virtual XInput device for controllers like Switch Pro, but the
		// raw HID interface remains visible and lacks the "IG_" marker that
		// isXInputDevice() checks. Without this guard, both XInput and HID
		// paths would read the same physical controller, and the different
		// interpretations cause button states to alternate ("jump").
		if !dev.isXInput && !dev.isInvalid {
			dk := deviceKey{VendorID: dev.vendorID, ProductID: dev.productID}
			if r.xinputVIDPIDs[dk] > 0 {
				slog.Info("hidinput: suppressing HID device (XInput duplicate)", "device", dev.name)
				dev.isXInput = true
			}
		}
		r.hidDevices[hDevice] = dev
	}
	return dev
}

// ---------------------------------------------------------------------------
// Shared connect / disconnect logic
// ---------------------------------------------------------------------------

// registerJoystick adds a joystick to the tracking lists and sets it as active
// if no controller is currently active. Thread-safe.
func (r *Reader) registerJoystick(key joystickKey, info *joystickInfo) {
	r.mu.Lock()
	r.joysticks[key] = info

	// Append to order list if not already present.
	found := false
	for _, k := range r.joystickOrder {
		if k == key {
			found = true
			break
		}
	}
	if !found {
		r.joystickOrder = append(r.joystickOrder, key)
	}

	playerIndex := r.getPlayerIndexLocked(key)

	// Set as active if no active controller yet (check under same lock).
	becameActive := false
	slog.Info("gamepad connected", "player", playerIndex, "name", info.name, "source", info.sourceType, "mapping", info.mapping.Name)

	// Set as active if no active controller yet.
	if !r.hasActive {
		r.activeKey = key
		r.hasActive = true
		r.state.Connected = true
		r.state.Name = info.name
		r.state.ControllerType = info.mapping.Name
		r.state.PlayerIndex = playerIndex
		becameActive = true
	}
	r.mu.Unlock()

	if becameActive {
		slog.Info("active controller set", "player", playerIndex, "name", info.name)
		r.emitState()
	}
}

// disconnectJoystick removes a joystick from the tracking lists and handles
// active controller promotion if necessary. Thread-safe.
func (r *Reader) disconnectJoystick(key joystickKey) {
	r.mu.Lock()
	info, ok := r.joysticks[key]
	if !ok {
		r.mu.Unlock()
		return
	}
	playerIndex := r.getPlayerIndexLocked(key)

	// Decrement XInput VID/PID tracking count so that HID devices with the
	// same VID/PID can be registered again after the XInput device is gone.
	if info.sourceType == "xinput" && info.devKey != (deviceKey{}) {
		if cnt := r.xinputVIDPIDs[info.devKey]; cnt <= 1 {
			delete(r.xinputVIDPIDs, info.devKey)
		} else {
			r.xinputVIDPIDs[info.devKey] = cnt - 1
		}
	}

	delete(r.joysticks, key)
	newOrder := make([]joystickKey, 0, len(r.joystickOrder))
	for _, k := range r.joystickOrder {
		if k != key {
			newOrder = append(newOrder, k)
		}
	}
	r.joystickOrder = newOrder

	wasActive := r.hasActive && r.activeKey == key
	if !wasActive {
		if info.sourceType == "hid" && info.hDevice != 0 {
			delete(r.hidDevices, info.hDevice)
		}
		r.mu.Unlock()
		slog.Info("gamepad disconnected", "player", playerIndex, "name", info.name, "source", info.sourceType)
		return
	}

	// Active controller disconnected: promote the next available one.
	r.hasActive = false
	if len(r.joystickOrder) == 0 {
		if info.sourceType == "hid" && info.hDevice != 0 {
			delete(r.hidDevices, info.hDevice)
		}
		r.state = GamepadState{}
		r.mu.Unlock()
		slog.Info("gamepad disconnected", "player", playerIndex, "name", info.name, "source", info.sourceType)
		r.emitState()
		return
	}

	nextKey := r.joystickOrder[0]
	nextInfo := r.joysticks[nextKey]
	nextPlayer := r.getPlayerIndexLocked(nextKey)
	r.activeKey = nextKey
	r.hasActive = true
	r.state.Connected = true
	r.state.Name = nextInfo.name
	r.state.ControllerType = nextInfo.mapping.Name
	r.state.PlayerIndex = nextPlayer
	if info.sourceType == "hid" && info.hDevice != 0 {
		delete(r.hidDevices, info.hDevice)
	}
	r.mu.Unlock()

	slog.Info("gamepad disconnected", "player", playerIndex, "name", info.name, "source", info.sourceType)
	slog.Info("active controller promoted", "player", nextPlayer, "name", nextInfo.name)
	r.emitState()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildControllerName constructs a human-readable controller name.
func buildControllerName(mappingName, vidPID string) string {
	if vidPID != "" {
		return fmt.Sprintf("%s (%s)", mappingName, vidPID)
	}
	return mappingName
}
