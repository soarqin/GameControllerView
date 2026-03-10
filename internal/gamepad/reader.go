package gamepad

import (
	"sync"
)

// Reader reads gamepad input and emits state changes on a channel.
// On Windows, it uses the XInput API (xinput1_4.dll / xinput1_3.dll).
// On other platforms, it is a no-op stub pending future implementation.
type Reader struct {
	state         GamepadState
	prevState     GamepadState
	joysticks     map[uint32]*joystickInfo // key: XInput user index (0-3)
	activeIndex   uint32                   // XInput user index of the active controller
	hasActive     bool
	joystickOrder []uint32 // connection order (XInput user indices)
	changes       chan GamepadState
	mu            sync.RWMutex
}

// joystickInfo holds per-slot metadata for a connected XInput controller.
type joystickInfo struct {
	mapping *DeviceMapping
	name    string
	vidPID  string // "VID_XXXX&PID_XXXX" for logging; empty if unavailable
	index   uint32 // XInput user index (0-3)
}

// NewReader creates a new Reader.
func NewReader() *Reader {
	return &Reader{
		joysticks: make(map[uint32]*joystickInfo),
		changes:   make(chan GamepadState, 64),
	}
}

// Changes returns the channel on which state changes are emitted.
func (r *Reader) Changes() <-chan GamepadState {
	return r.changes
}

// GetPlayerIndex returns the 1-based player index of the currently active controller.
func (r *Reader) GetPlayerIndex() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for i, idx := range r.joystickOrder {
		if idx == r.activeIndex {
			return i + 1
		}
	}
	return 0
}

// SetActiveByPlayerIndex sets the active controller by 1-based player index.
// Returns true if successful, false if the index is out of range.
func (r *Reader) SetActiveByPlayerIndex(playerIndex int) bool {
	r.mu.Lock()
	if playerIndex < 1 || playerIndex > len(r.joystickOrder) {
		r.mu.Unlock()
		return false
	}
	newIndex := r.joystickOrder[playerIndex-1]
	info := r.joysticks[newIndex]
	if info == nil {
		r.mu.Unlock()
		return false
	}

	r.activeIndex = newIndex
	r.hasActive = true
	r.state.Connected = true
	r.state.Name = info.name
	r.state.ControllerType = info.mapping.Name
	r.state.PlayerIndex = playerIndex
	r.mu.Unlock()

	r.emitState()
	return true
}

// emitState sends the current state snapshot to the changes channel (non-blocking).
func (r *Reader) emitState() {
	r.mu.RLock()
	s := r.state
	r.mu.RUnlock()

	select {
	case r.changes <- s:
	default:
		// Drop if channel is full to avoid blocking the polling goroutine.
	}
}
