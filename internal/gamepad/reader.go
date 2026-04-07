package gamepad

import (
	"bytes"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"time"
)

// sdlMappingsMu protects globalSDLMappings from concurrent read/write.
var sdlMappingsMu sync.RWMutex

// globalSDLMappings holds SDL gamecontrollerdb entries loaded at startup.
// Keyed by (VendorID, ProductID). Protected by sdlMappingsMu.
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
		slog.Warn("sdldb: failed to parse embedded DB", "error", err)
		merged = make(map[deviceKey]*SDLMapping)
	}

	// Overlay with external file if present.
	if externalPath != "" {
		if _, statErr := os.Stat(externalPath); statErr == nil {
			ext, extErr := LoadSDLMappingsFromFile(externalPath, platform)
			if extErr != nil {
				slog.Warn("sdldb: failed to load external db", "path", externalPath, "error", extErr)
			} else {
				for k, v := range ext {
					merged[k] = v
				}
				slog.Info("sdldb: merged external db", "path", externalPath, "total", len(merged))
			}
		}
	}

	sdlMappingsMu.Lock()
	globalSDLMappings = merged
	sdlMappingsMu.Unlock()
}

// lookupSDLMapping returns the SDL mapping for a device's VID/PID, or nil if
// no mapping was loaded or none matches.
func lookupSDLMapping(vendorID, productID uint16) *SDLMapping {
	sdlMappingsMu.RLock()
	m := globalSDLMappings
	sdlMappingsMu.RUnlock()
	if m == nil {
		return nil
	}
	return m[deviceKey{VendorID: vendorID, ProductID: productID}]
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

	// deadzone is the analog stick deadzone threshold (0.0-1.0).
	deadzone float64

	// pollDelay is the interval between XInput polling cycles.
	pollDelay time.Duration

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

	// xinputVIDPIDs tracks VID/PID pairs of currently connected XInput devices.
	// Used to suppress duplicate HID registrations when XInput emulation software
	// (e.g. Steam, BetterJoy) creates a virtual XInput device for the same
	// physical controller — the raw HID interface lacks the "IG_" marker in its
	// device name, so isXInputDevice() cannot filter it.
	// Only accessed under r.mu.
	xinputVIDPIDs map[deviceKey]int
}

// joystickInfo holds per-device metadata for a connected controller.
type joystickInfo struct {
	mapping    *DeviceMapping
	name       string
	vidPID     string    // "VID_XXXX&PID_XXXX" for logging; empty if unavailable
	sourceType string    // "xinput" or "hid"
	xinputSlot uint32    // XInput slot (0-3); only valid when sourceType=="xinput"
	hDevice    uintptr   // HID device handle; only valid when sourceType=="hid"
	devKey     deviceKey // VID/PID pair; zero if unavailable
}

// NewReader creates a new Reader with default deadzone and poll rate.
func NewReader() *Reader {
	return &Reader{
		joysticks:        make(map[joystickKey]*joystickInfo),
		hidDevices:       make(map[uintptr]*hidDeviceInfo),
		disconnectedHIDs: make(map[uintptr]struct{}),
		xinputVIDPIDs:    make(map[deviceKey]int),
		changes:          make(chan GamepadState, 64),
		deadzone:         0.05,
		pollDelay:        16 * time.Millisecond,
	}
}

// SetDeadzone sets the analog stick deadzone threshold (0.0-1.0).
// Values outside [0, 1] are clamped.
func (r *Reader) SetDeadzone(dz float64) {
	if dz < 0 {
		dz = 0
	} else if dz > 1 {
		dz = 1
	}
	r.deadzone = dz
}

// SetPollDelay sets the interval between XInput polling cycles.
func (r *Reader) SetPollDelay(d time.Duration) { r.pollDelay = d }

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
