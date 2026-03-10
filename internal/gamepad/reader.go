package gamepad

import (
	"bytes"
	"log"
	"os"
	"runtime"
	"sync"
)

// globalSDLMappings holds SDL gamecontrollerdb entries loaded at startup.
// Keyed by (VendorID, ProductID). Set by LoadSDLDB(); nil if not loaded.
var globalSDLMappings map[deviceKey]*SDLMapping

// sdlPlatformName maps runtime.GOOS values to the platform strings used in
// gamecontrollerdb.txt entries.
func sdlPlatformName() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	case "darwin":
		return "Mac OS X"
	default:
		return "Windows"
	}
}

// LoadSDLDB initialises the global SDL mapping table.
//
// It always loads the embedded gamecontrollerdb.txt as a base. If externalPath
// is non-empty and the file exists, its entries are merged on top (external
// entries take priority), allowing users to provide an updated database next to
// the executable without recompiling.
//
// Call before any gamepads connect (ideally before Reader.Run). Safe to call
// from any goroutine before Run().
func LoadSDLDB(externalPath string) {
	platform := sdlPlatformName()

	// Load embedded DB as the base.
	merged, err := LoadSDLMappingsFromReader(bytes.NewReader(embeddedGameControllerDB), platform)
	if err != nil {
		log.Printf("sdldb: failed to parse embedded DB: %v", err)
		merged = make(map[deviceKey]*SDLMapping)
	}

	// Overlay with external file if present.
	if externalPath != "" {
		if _, statErr := os.Stat(externalPath); statErr == nil {
			ext, extErr := LoadSDLMappingsFromFile(externalPath, platform)
			if extErr != nil {
				log.Printf("sdldb: failed to load %q: %v", externalPath, extErr)
			} else {
				for k, v := range ext {
					merged[k] = v
				}
				log.Printf("sdldb: merged external %q (%d entries total)", externalPath, len(merged))
			}
		}
	}

	globalSDLMappings = merged
}

// lookupSDLMapping returns the SDL mapping for a device's VID/PID, or nil if
// no mapping was loaded or none matches.
func lookupSDLMapping(vendorID, productID uint16) *SDLMapping {
	if globalSDLMappings == nil {
		return nil
	}
	return globalSDLMappings[deviceKey{VendorID: vendorID, ProductID: productID}]
}

// joystickKey is a unified key for both XInput slots (0-3) and HID device handles.
// XInput slots use values 0-3 directly.
// HID device handles are stored as-is (kernel handles are always >= 4 and aligned,
// so they never collide with XInput slot values 0-3).
type joystickKey = uint64

// xinputKey converts an XInput slot index (0-3) to a joystickKey.
func xinputKey(slot uint32) joystickKey { return joystickKey(slot) }

// hidKey converts a HID device handle to a joystickKey.
func hidKey(hDevice uintptr) joystickKey { return joystickKey(hDevice) }

// Reader reads gamepad input and emits state changes on a channel.
// On Windows, it uses XInput for Xbox-compatible controllers and Raw Input HID
// for all other HID gamepads (PS4/PS5, Switch Pro, generic). On other platforms
// it is a no-op stub pending future implementation.
type Reader struct {
	state         GamepadState
	prevState     GamepadState
	joysticks     map[joystickKey]*joystickInfo // key: xinputKey(slot) or hidKey(hDevice)
	activeKey     joystickKey                   // key of the active controller
	hasActive     bool
	joystickOrder []joystickKey // connection order
	changes       chan GamepadState
	mu            sync.RWMutex

	// hidDevices caches per-device HID capability info.
	// Only accessed under r.mu.
	hidDevices map[uintptr]*hidDeviceInfo

	// disconnectedHIDs records HID device handles that have been explicitly
	// disconnected via WM_INPUT_DEVICE_CHANGE (GIDC_REMOVAL). Residual WM_INPUT
	// events that arrive after disconnection are suppressed using this set,
	// preventing the fallback path in handleHIDInput from re-registering a
	// device that has already been removed.
	// Only accessed under r.mu.
	disconnectedHIDs map[uintptr]struct{}
}

// joystickInfo holds per-device metadata for a connected controller.
type joystickInfo struct {
	mapping    *DeviceMapping
	name       string
	vidPID     string  // "VID_XXXX&PID_XXXX" for logging; empty if unavailable
	sourceType string  // "xinput" or "hid"
	xinputSlot uint32  // XInput slot (0-3); only valid when sourceType=="xinput"
	hDevice    uintptr // HID device handle; only valid when sourceType=="hid"
}

// NewReader creates a new Reader.
func NewReader() *Reader {
	return &Reader{
		joysticks:        make(map[joystickKey]*joystickInfo),
		hidDevices:       make(map[uintptr]*hidDeviceInfo),
		disconnectedHIDs: make(map[uintptr]struct{}),
		changes:          make(chan GamepadState, 64),
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
	for i, key := range r.joystickOrder {
		if key == r.activeKey {
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
	newKey := r.joystickOrder[playerIndex-1]
	info := r.joysticks[newKey]
	if info == nil {
		r.mu.Unlock()
		return false
	}

	r.activeKey = newKey
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

// getPlayerIndexLocked returns the 1-based player index for a key.
// Caller must hold r.mu (at least read lock).
func (r *Reader) getPlayerIndexLocked(key joystickKey) int {
	for i, k := range r.joystickOrder {
		if k == key {
			return i + 1
		}
	}
	return 0
}
