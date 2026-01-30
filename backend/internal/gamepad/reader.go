package gamepad

import (
	"context"
	"log"
	"runtime"
	"sync"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

const (
	deadzone     = 0.05
	pollDelayNS  = 16_000_000 // ~60Hz
	hatUp    uint8 = 0x01
	hatRight uint8 = 0x02
	hatDown  uint8 = 0x04
	hatLeft  uint8 = 0x08
)

type joystickInfo struct {
	joystick *sdl.Joystick
	mapping  *DeviceMapping
	name     string
	id       sdl.JoystickID
}

// Reader reads gamepad input from SDL3 Joystick API and emits state changes.
type Reader struct {
	state     GamepadState
	prevState GamepadState
	joysticks map[sdl.JoystickID]*joystickInfo
	activeID  sdl.JoystickID // the first connected joystick
	hasActive bool
	changes   chan GamepadState
	mu        sync.RWMutex
}

func NewReader() *Reader {
	return &Reader{
		joysticks: make(map[sdl.JoystickID]*joystickInfo),
		changes:   make(chan GamepadState, 64),
	}
}

// Changes returns the channel on which state changes are sent.
func (r *Reader) Changes() <-chan GamepadState {
	return r.changes
}

// CurrentState returns a snapshot of the current gamepad state.
func (r *Reader) CurrentState() GamepadState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// Run initializes SDL and runs the main event+polling loop on the current thread.
// Must be called from the main goroutine with runtime.LockOSThread().
func (r *Reader) Run(ctx context.Context) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if !sdl.Init(sdl.InitJoystick) {
		log.Fatalf("SDL Init failed: %s", sdl.GetError())
	}
	defer sdl.Quit()

	log.Println("SDL3 Joystick subsystem initialized")

	// Check for already-connected joysticks
	ids := sdl.GetJoysticks()
	for _, id := range ids {
		r.openJoystick(id)
	}

	for {
		select {
		case <-ctx.Done():
			r.closeAll()
			return
		default:
		}

		r.processEvents()
		r.pollState()
		sdl.DelayNS(pollDelayNS)
	}
}

// Close cleans up all opened joysticks. Safe to call from any goroutine
// after Run has returned.
func (r *Reader) Close() {
	// Joysticks are closed in Run's cleanup via closeAll
}

func (r *Reader) processEvents() {
	var event sdl.Event
	for sdl.PollEvent(&event) {
		switch event.Type() {
		case sdl.EventJoystickAdded:
			devEvent := event.JDevice()
			r.openJoystick(devEvent.Which)

		case sdl.EventJoystickRemoved:
			devEvent := event.JDevice()
			r.removeJoystick(devEvent.Which)

		case sdl.EventJoystickButtonDown:
			be := event.JButton()
			log.Printf("[DEBUG] Button DOWN: index=%d joystick=%d", be.Button, be.Which)

		case sdl.EventJoystickButtonUp:
			be := event.JButton()
			log.Printf("[DEBUG] Button UP:   index=%d joystick=%d", be.Button, be.Which)

		case sdl.EventJoystickAxisMotion:
			ae := event.JAxis()
			if ae.Value > 8000 || ae.Value < -8000 {
				log.Printf("[DEBUG] Axis: index=%d value=%d joystick=%d", ae.Axis, ae.Value, ae.Which)
			}

		case sdl.EventJoystickHatMotion:
			he := event.JHat()
			log.Printf("[DEBUG] Hat: index=%d value=0x%02X joystick=%d", he.Hat, he.Value, he.Which)
		}
	}
}

func (r *Reader) openJoystick(instanceID sdl.JoystickID) {
	if _, exists := r.joysticks[instanceID]; exists {
		return
	}

	js := sdl.OpenJoystick(instanceID)
	if js == nil {
		log.Printf("Failed to open joystick %d: %s", instanceID, sdl.GetError())
		return
	}

	jsID := sdl.GetJoystickID(js)
	vendorID := sdl.GetJoystickVendor(js)
	productID := sdl.GetJoystickProduct(js)
	name := sdl.GetJoystickName(js)
	mapping := GetMapping(vendorID, productID)

	info := &joystickInfo{
		joystick: js,
		mapping:  mapping,
		name:     name,
		id:       jsID,
	}
	r.joysticks[jsID] = info

	numAxes := sdl.GetNumJoystickAxes(js)
	numButtons := sdl.GetNumJoystickButtons(js)
	numHats := sdl.GetNumJoystickHats(js)

	log.Printf("Joystick connected: %s (VID=%04X PID=%04X) mapping=%s axes=%d buttons=%d hats=%d",
		name, vendorID, productID, mapping.Name, numAxes, numButtons, numHats)

	// Use the first connected joystick as active
	if !r.hasActive {
		r.activeID = jsID
		r.hasActive = true
		log.Printf("Active joystick set: %s (ID=%d)", name, jsID)

		r.mu.Lock()
		r.state.Connected = true
		r.state.Name = name
		r.state.ControllerType = mapping.Name
		r.mu.Unlock()

		r.emitState()
	}
}

func (r *Reader) removeJoystick(instanceID sdl.JoystickID) {
	info, exists := r.joysticks[instanceID]
	if !exists {
		return
	}

	log.Printf("Joystick disconnected: %s", info.name)
	sdl.CloseJoystick(info.joystick)
	delete(r.joysticks, instanceID)

	if r.hasActive && r.activeID == instanceID {
		r.hasActive = false
		if len(r.joysticks) == 0 {
			r.mu.Lock()
			r.state = GamepadState{}
			r.mu.Unlock()
			r.emitState()
		} else {
			// Promote the next available joystick
			for id, js := range r.joysticks {
				if sdl.JoystickConnected(js.joystick) {
					r.activeID = id
					r.hasActive = true
					log.Printf("Active joystick switched to: %s (ID=%d)", js.name, id)

					r.mu.Lock()
					r.state.Connected = true
					r.state.Name = js.name
					r.state.ControllerType = js.mapping.Name
					r.mu.Unlock()

					r.emitState()
					break
				}
			}
		}
	}
}

func (r *Reader) closeAll() {
	for id, info := range r.joysticks {
		sdl.CloseJoystick(info.joystick)
		delete(r.joysticks, id)
	}
}

func (r *Reader) pollState() {
	if !r.hasActive {
		return
	}

	info, exists := r.joysticks[r.activeID]
	if !exists || !sdl.JoystickConnected(info.joystick) {
		return
	}

	js := info.joystick
	mapping := info.mapping
	state := GamepadState{
		Connected:      true,
		ControllerType: mapping.Name,
		Name:           info.name,
	}

	// Read axes
	for _, am := range mapping.Axes {
		raw := sdl.GetJoystickAxis(js, am.Index)
		if am.IsTrigger {
			val := NormalizeTrigger(raw, am.RawMin, am.RawMax)
			val = ApplyDeadzone(val, deadzone)
			switch am.Target {
			case "lt":
				state.Triggers.LT.Value = val
			case "rt":
				state.Triggers.RT.Value = val
			}
		} else {
			val := NormalizeAxis(raw)
			if am.Invert {
				val = -val
			}
			val = ApplyDeadzone(val, deadzone)
			switch am.Target {
			case "left_x":
				state.Sticks.Left.Position.X = val
			case "left_y":
				state.Sticks.Left.Position.Y = val
			case "right_x":
				state.Sticks.Right.Position.X = val
			case "right_y":
				state.Sticks.Right.Position.Y = val
			}
		}
	}

	// Read buttons
	numButtons := sdl.GetNumJoystickButtons(js)
	for _, bm := range mapping.Buttons {
		if bm.Index >= numButtons {
			continue
		}
		pressed := sdl.GetJoystickButton(js, bm.Index)
		switch bm.Target {
		case "a":
			state.Buttons.A = pressed
		case "b":
			state.Buttons.B = pressed
		case "x":
			state.Buttons.X = pressed
		case "y":
			state.Buttons.Y = pressed
		case "lb":
			state.Buttons.LB = pressed
		case "rb":
			state.Buttons.RB = pressed
		case "select":
			state.Buttons.Select = pressed
		case "start":
			state.Buttons.Start = pressed
		case "home":
			state.Buttons.Home = pressed
		case "l3":
			state.Sticks.Left.Pressed = pressed
		case "r3":
			state.Sticks.Right.Pressed = pressed
		}
	}

	// Read hat (D-pad)
	if mapping.HasHat && sdl.GetNumJoystickHats(js) > 0 {
		hat := sdl.GetJoystickHat(js, 0)
		state.Dpad.Up = hat&hatUp != 0
		state.Dpad.Right = hat&hatRight != 0
		state.Dpad.Down = hat&hatDown != 0
		state.Dpad.Left = hat&hatLeft != 0
	}

	// Compare with previous state and emit if changed
	r.mu.Lock()
	delta := ComputeDelta(r.prevState, state)
	if !delta.IsEmpty() {
		r.state = state
		r.prevState = state
		r.mu.Unlock()
		r.emitState()
	} else {
		r.mu.Unlock()
	}
}

func (r *Reader) emitState() {
	r.mu.RLock()
	s := r.state
	r.mu.RUnlock()

	select {
	case r.changes <- s:
	default:
		// Drop if channel is full to avoid blocking the SDL thread
	}
}
