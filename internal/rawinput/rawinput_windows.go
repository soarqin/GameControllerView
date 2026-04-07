//go:build windows

// Package rawinput provides a Windows Raw Input global keyboard and mouse reader.
// It registers a hidden HWND_MESSAGE window and uses RIDEV_INPUTSINK so that input
// is received even when the window is not in the foreground.
//
// The package also supports registering callbacks for HID devices (e.g. gamepads)
// via RegisterHIDCallback, allowing other packages to receive raw HID input events
// through the same HWND_MESSAGE window without creating an additional OS thread.
package rawinput

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/soar/inputview/internal/input"
)

// pollInterval is the rate at which accumulated state is emitted to the changes channel.
const pollInterval = 16 * time.Millisecond // ~60 Hz

// defaultMouseSensitivity is the divisor applied to raw pixel deltas to produce a
// normalised [-1, 1] value. Users can override via Reader.SetMouseSensitivity().
const defaultMouseSensitivity float32 = 500.0

// Windows API constants
const (
	// Window class / style
	csOwnDC   = 0x0020
	wsExNoact = 0x08000000 // WS_EX_NOACTIVATE — never steal focus

	// Raw Input usage page / usage IDs (HID Generic Desktop)
	usagePageGeneric = 0x01
	usageMouse       = 0x02
	usageKeyboard    = 0x06

	// RegisterRawInputDevices flags
	ridevInputsink = 0x00000100 // receive events even when not in foreground
	ridevDevnotify = 0x00002000 // receive WM_INPUT_DEVICE_CHANGE notifications
	ridevRemove    = 0x00000001 // unregister a device
	ridevNolegacy  = 0x00000030 // disable legacy messages (keyboard/mouse only)

	// GetRawInputData command
	ridInput = 0x10000003

	// GetRawInputDeviceInfo commands
	ridiDevicename    = 0x20000007
	ridiDeviceinfo    = 0x2000000b
	ridiPreparseddata = 0x20000005

	// RAWINPUT type codes (from winuser.h)
	rimTypeMouse    = 0
	rimTypeKeyboard = 1
	rimTypeHID      = 2 // generic HID device (gamepad, joystick, etc.)

	// RAWKEYBOARD Flags bits
	riKeyBreak = 0x01 // key released (make = 0x00)
	riKeyE0    = 0x02 // E0 extended prefix

	// RAWMOUSE usButtonFlags — wheel
	riMouseWheelDelta   = 0x0400
	wheelDeltaThreshold = 120 // standard Windows WHEEL_DELTA

	// WM_INPUT message identifier
	wmInput = 0x00FF

	// WM_INPUT_DEVICE_CHANGE message identifier (sent when RIDEV_DEVNOTIFY is set)
	wmInputDeviceChange = 0x00FE

	// wParam values for WM_INPUT_DEVICE_CHANGE
	giahDeviceArrival = 1 // device arrived
	giahDeviceRemoval = 2 // device removed

	// WM_DESTROY message
	wmDestroy = 0x0002

	// SetWindowLongPtr index for window procedure
	gwlpWndproc = -4
)

// Windows API bindings (user32.dll)
var (
	modUser32 = syscall.NewLazyDLL("user32.dll")

	procRegisterClassExW        = modUser32.NewProc("RegisterClassExW")
	procCreateWindowExW         = modUser32.NewProc("CreateWindowExW")
	procDestroyWindow           = modUser32.NewProc("DestroyWindow")
	procDefWindowProcW          = modUser32.NewProc("DefWindowProcW")
	procRegisterRawInputDevices = modUser32.NewProc("RegisterRawInputDevices")
	procGetRawInputData         = modUser32.NewProc("GetRawInputData")
	procGetRawInputDeviceInfoW  = modUser32.NewProc("GetRawInputDeviceInfoW")
	procGetMessage              = modUser32.NewProc("GetMessageW")
	procTranslateMessage        = modUser32.NewProc("TranslateMessage")
	procDispatchMessage         = modUser32.NewProc("DispatchMessageW")
	procPostMessage             = modUser32.NewProc("PostMessageW")
	procSetWindowLongPtrW       = modUser32.NewProc("SetWindowLongPtrW")
)

// wndClassExW is the WNDCLASSEXW structure used to register a window class.
type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

// rawInputDevice is the RAWINPUTDEVICE structure.
type rawInputDevice struct {
	usUsagePage uint16
	usUsage     uint16
	dwFlags     uint32
	hwndTarget  uintptr
}

// rawInputHeader is the RAWINPUTHEADER structure.
type rawInputHeader struct {
	dwType  uint32
	dwSize  uint32
	hDevice uintptr
	wParam  uintptr
}

// rawKeyboard is the RAWKEYBOARD structure.
type rawKeyboard struct {
	MakeCode         uint16
	Flags            uint16
	Reserved         uint16
	VKey             uint16
	Message          uint32
	ExtraInformation uint32
}

// rawMouse is the RAWMOUSE structure.
type rawMouse struct {
	usFlags            uint16
	_                  uint16 // padding for union alignment
	usButtonFlags      uint16
	usButtonData       uint16
	ulRawButtons       uint32
	lLastX             int32
	lLastY             int32
	ulExtraInformation uint32
}

// msg is the MSG structure for the Windows message loop.
type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      [2]int32
}

// HIDInputCallback is called from the message loop goroutine whenever a WM_INPUT
// event arrives for a registered HID device (usage page / usage pair).
// hDevice is the raw input device handle from RAWINPUTHEADER.hDevice.
// rawData is the raw HID report bytes (RAWHID.bRawData, dwSizeHid bytes per report).
// reportSize is the size in bytes of a single HID report.
type HIDInputCallback func(hDevice uintptr, rawData []byte, reportSize uint32)

// HIDDeviceChangeCallback is called from the message loop goroutine when a HID
// device matching a registered usage page/usage is added or removed.
// added is true for arrival, false for removal.
type HIDDeviceChangeCallback func(added bool, hDevice uintptr)

// hidRegistration stores a single HID callback registration.
type hidRegistration struct {
	usagePage uint16
	usage     uint16
	inputCb   HIDInputCallback
	changeCb  HIDDeviceChangeCallback
}

// hidDeviceUsage caches the usage page/usage of a known HID device handle.
type hidDeviceUsage struct {
	usagePage uint16
	usage     uint16
}

// Reader collects keyboard and mouse input via Windows Raw Input API.
// It creates an invisible HWND_MESSAGE window and runs a Windows message loop on a
// dedicated OS-locked goroutine. State is emitted to Changes() at ~60 Hz.
//
// Additional HID device types (e.g. gamepads) can be registered via RegisterHIDCallback;
// their raw input events are routed to the provided callback on the same message loop thread.
type Reader struct {
	mu           sync.Mutex
	keys         map[uint16]bool // uiohook scancode → currently held
	mouseButtons map[uint16]bool // IO button code (1-5) → currently held
	rawMouseDX   int32           // accumulated raw pixel delta X
	rawMouseDY   int32           // accumulated raw pixel delta Y
	wheelUp      bool
	wheelDown    bool
	// pendingKeysDown/Up record every key event within the current tick so that
	// a press+release within 16ms is not invisible to the emitter.
	pendingKeysDown []uint16
	pendingKeysUp   []uint16
	// pendingButtonsDown/Up record every mouse button event within the current tick.
	pendingButtonsDown []uint16
	pendingButtonsUp   []uint16
	sensitivity        float32
	changes            chan input.KeyMouseState
	hwnd               uintptr // set by message loop goroutine
	hwndReady          chan struct{}
	stopPostHwnd       uintptr // HWND to post WM_DESTROY to for clean shutdown

	// HID callback registrations. Protected by hidMu.
	// Written before Run() or via RegisterHIDCallback (which waits for hwndReady).
	// Read only from the message loop goroutine (no lock needed there).
	hidMu        sync.RWMutex
	hidCallbacks []hidRegistration

	// Cache of hDevice → usage page/usage, to avoid GetRawInputDeviceInfoW on
	// every WM_INPUT. Only accessed from the message loop goroutine (no lock needed).
	hidDeviceCache map[uintptr]hidDeviceUsage
}

// New creates a new Raw Input Reader with default settings.
func New() *Reader {
	return &Reader{
		keys:           make(map[uint16]bool),
		mouseButtons:   make(map[uint16]bool),
		sensitivity:    defaultMouseSensitivity,
		changes:        make(chan input.KeyMouseState, 64),
		hwndReady:      make(chan struct{}),
		hidDeviceCache: make(map[uintptr]hidDeviceUsage),
	}
}

// SetMouseSensitivity sets the raw-pixel divisor for mouse movement normalisation.
// Larger values make the movement indicator less sensitive. Default: 500.
func (r *Reader) SetMouseSensitivity(s float32) {
	if s <= 0 {
		s = defaultMouseSensitivity
	}
	r.mu.Lock()
	r.sensitivity = s
	r.mu.Unlock()
}

// Changes returns the read-only channel that receives KeyMouseState snapshots at ~60 Hz.
func (r *Reader) Changes() <-chan input.KeyMouseState {
	return r.changes
}

// RegisterHIDCallback registers a callback to receive raw HID input events for the
// specified usage page and usage (e.g. usagePage=0x01, usage=0x04 for Joystick;
// usage=0x05 for Gamepad). Both inputCb and changeCb may be nil if not needed.
//
// If the message window is already running, the new HID device type is registered
// immediately. Otherwise it will be registered when Run() starts.
// Safe to call from any goroutine.
func (r *Reader) RegisterHIDCallback(usagePage, usage uint16, inputCb HIDInputCallback, changeCb HIDDeviceChangeCallback) {
	r.hidMu.Lock()
	r.hidCallbacks = append(r.hidCallbacks, hidRegistration{
		usagePage: usagePage,
		usage:     usage,
		inputCb:   inputCb,
		changeCb:  changeCb,
	})
	r.hidMu.Unlock()

	// If the window is already running, register the device immediately.
	// We do a non-blocking check first so this path is fast when called before Run().
	select {
	case <-r.hwndReady:
		// Window exists — register this usage right now.
		// r.hwnd is written exactly once before hwndReady is closed, so the
		// read here is safe (close provides a happens-before guarantee).
		hwnd := r.hwnd
		if hwnd != 0 {
			dev := rawInputDevice{
				usUsagePage: usagePage,
				usUsage:     usage,
				dwFlags:     ridevInputsink | ridevDevnotify,
				hwndTarget:  hwnd,
			}
			procRegisterRawInputDevices.Call(
				uintptr(unsafe.Pointer(&dev)),
				1,
				uintptr(unsafe.Sizeof(rawInputDevice{})),
			)
		}
	default:
		// Window not yet created; Run() will register all pending callbacks at startup.
	}
}

// Run starts the Raw Input message loop and the periodic state emitter.
// It blocks until ctx is cancelled. Must be called in a goroutine.
func (r *Reader) Run(ctx context.Context) {
	// Lock this goroutine to its OS thread.
	// The Windows message loop must run on the same thread that created the window.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hwnd, err := r.createMessageWindow()
	if err != nil {
		slog.Error("rawinput: failed to create message window", "error", err)
		close(r.hwndReady)
		close(r.changes)
		return
	}
	r.hwnd = hwnd
	close(r.hwndReady) // signal that HWND is valid

	if err := r.registerDevices(hwnd); err != nil {
		slog.Warn("rawinput: failed to register raw input devices", "error", err)
		// Continue anyway — partial functionality is better than none
	} else {
		slog.Info("rawinput: registered keyboard+mouse", "hwnd", fmt.Sprintf("0x%x", hwnd))
	}

	// Register any HID callbacks that were added before Run() started.
	r.hidMu.RLock()
	callbacks := make([]hidRegistration, len(r.hidCallbacks))
	copy(callbacks, r.hidCallbacks)
	r.hidMu.RUnlock()

	for _, cb := range callbacks {
		dev := rawInputDevice{
			usUsagePage: cb.usagePage,
			usUsage:     cb.usage,
			dwFlags:     ridevInputsink | ridevDevnotify,
			hwndTarget:  hwnd,
		}
		ret, _, regErr := procRegisterRawInputDevices.Call(
			uintptr(unsafe.Pointer(&dev)),
			1,
			uintptr(unsafe.Sizeof(rawInputDevice{})),
		)
		if ret == 0 {
			slog.Warn("rawinput: failed to register HID device", "usagePage", fmt.Sprintf("0x%02x", cb.usagePage), "usage", fmt.Sprintf("0x%02x", cb.usage), "error", regErr)
		} else {
			slog.Info("rawinput: registered HID device", "usagePage", fmt.Sprintf("0x%02x", cb.usagePage), "usage", fmt.Sprintf("0x%02x", cb.usage))
		}
	}

	// Start the periodic state emitter in a separate goroutine.
	go r.runEmitter(ctx)

	// Windows message loop
	var m msg
	for {
		// Non-blocking check for context cancellation before each GetMessage.
		select {
		case <-ctx.Done():
			procDestroyWindow.Call(hwnd)
			return
		default:
		}

		// GetMessage blocks until a message is available.
		ret, _, _ := procGetMessage.Call(
			uintptr(unsafe.Pointer(&m)),
			hwnd,
			0,
			0,
		)
		// GetMessage returns 0 on WM_QUIT, -1 on error.
		if ret == 0 || ret == ^uintptr(0) {
			return
		}

		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))

		if m.message == wmDestroy {
			return
		}
	}
}

// runEmitter periodically snapshots the accumulated input state and sends it to
// the changes channel. Runs in its own goroutine (not the OS-locked one).
func (r *Reader) runEmitter(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer func() {
		ticker.Stop()
		close(r.changes)
	}()

	var prevState input.KeyMouseState

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			curr := r.snapshot()
			delta := input.ComputeKeyMouseDelta(prevState, curr)
			if !delta.IsEmpty() {
				select {
				case r.changes <- curr:
					// Store curr as prevState but without pending events —
					// pending fields must not bleed into the next tick's delta computation.
					prevState = curr
					prevState.PendingKeysDown = nil
					prevState.PendingKeysUp = nil
					prevState.PendingButtonsDown = nil
					prevState.PendingButtonsUp = nil
				default:
					// Channel full: skip this tick
				}
			}
		}
	}
}

// snapshot takes a thread-safe snapshot of current input state and resets per-tick fields.
func (r *Reader) snapshot() input.KeyMouseState {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Copy keys map
	keys := make(map[uint16]bool, len(r.keys))
	for k, v := range r.keys {
		if v {
			keys[k] = true
		}
	}

	// Copy mouse buttons map
	buttons := make(map[uint16]bool, len(r.mouseButtons))
	for k, v := range r.mouseButtons {
		if v {
			buttons[k] = true
		}
	}

	// Normalise mouse movement
	sensitivity := r.sensitivity
	if sensitivity <= 0 {
		sensitivity = defaultMouseSensitivity
	}
	dx := clampF32(float32(r.rawMouseDX)/sensitivity, -1.0, 1.0)
	dy := clampF32(float32(r.rawMouseDY)/sensitivity, -1.0, 1.0)

	// Reset per-tick accumulators
	r.rawMouseDX = 0
	r.rawMouseDY = 0
	wu := r.wheelUp
	wd := r.wheelDown
	r.wheelUp = false
	r.wheelDown = false

	// Drain pending event lists
	pendingKeysDown := r.pendingKeysDown
	pendingKeysUp := r.pendingKeysUp
	pendingButtonsDown := r.pendingButtonsDown
	pendingButtonsUp := r.pendingButtonsUp
	r.pendingKeysDown = nil
	r.pendingKeysUp = nil
	r.pendingButtonsDown = nil
	r.pendingButtonsUp = nil

	return input.KeyMouseState{
		Keys:               keys,
		MouseButtons:       buttons,
		MouseDX:            dx,
		MouseDY:            dy,
		WheelUp:            wu,
		WheelDown:          wd,
		PendingKeysDown:    pendingKeysDown,
		PendingKeysUp:      pendingKeysUp,
		PendingButtonsDown: pendingButtonsDown,
		PendingButtonsUp:   pendingButtonsUp,
	}
}

// createMessageWindow registers a window class and creates an HWND_MESSAGE window.
// An HWND_MESSAGE window has no visible UI, receives no system messages like WM_PAINT,
// but can receive WM_INPUT.
func (r *Reader) createMessageWindow() (uintptr, error) {
	className, _ := syscall.UTF16PtrFromString("InputViewRawInput")
	wndProc := syscall.NewCallback(r.wndProc)

	cls := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		style:         csOwnDC,
		lpfnWndProc:   wndProc,
		lpszClassName: className,
	}

	atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&cls)))
	if atom == 0 {
		// Ignore ERROR_CLASS_ALREADY_EXISTS (1410) — happens on restart within same process lifetime
		if errno, ok := err.(syscall.Errno); !ok || errno != 1410 {
			return 0, err
		}
	}

	// HWND_MESSAGE = uintptr(-3) — parent for message-only windows
	const hwndMessage = ^uintptr(2) // = uintptr(-3) in two's complement

	hwnd, _, err := procCreateWindowExW.Call(
		uintptr(wsExNoact),                 // dwExStyle
		uintptr(unsafe.Pointer(className)), // lpClassName
		0,                                  // lpWindowName (nil)
		0,                                  // dwStyle
		0, 0, 0, 0,                         // x, y, nWidth, nHeight
		hwndMessage, // hWndParent = HWND_MESSAGE
		0,           // hMenu
		0,           // hInstance
		0,           // lpParam
	)
	if hwnd == 0 {
		return 0, err
	}
	return hwnd, nil
}

// registerDevices calls RegisterRawInputDevices for keyboard and mouse with RIDEV_INPUTSINK
// so we receive input regardless of which window has focus.
func (r *Reader) registerDevices(hwnd uintptr) error {
	devices := [2]rawInputDevice{
		{
			usUsagePage: usagePageGeneric,
			usUsage:     usageMouse,
			dwFlags:     ridevInputsink,
			hwndTarget:  hwnd,
		},
		{
			usUsagePage: usagePageGeneric,
			usUsage:     usageKeyboard,
			dwFlags:     ridevInputsink,
			hwndTarget:  hwnd,
		},
	}

	ret, _, err := procRegisterRawInputDevices.Call(
		uintptr(unsafe.Pointer(&devices[0])),
		2,
		uintptr(unsafe.Sizeof(rawInputDevice{})),
	)
	if ret == 0 {
		return err
	}
	return nil
}

// wndProc is the Windows window procedure callback for the message-only window.
// It handles WM_INPUT and WM_INPUT_DEVICE_CHANGE messages; all others go to DefWindowProc.
// Called exclusively on the OS-locked message loop goroutine.
func (r *Reader) wndProc(hwnd, msgID, wParam, lParam uintptr) uintptr {
	switch msgID {
	case wmInputDeviceChange:
		r.handleDeviceChange(wParam, lParam)
		ret, _, _ := procDefWindowProcW.Call(hwnd, msgID, wParam, lParam)
		return ret

	case wmInput:
		// Determine required buffer size.
		var size uint32
		procGetRawInputData.Call(
			lParam,
			uintptr(ridInput),
			0,
			uintptr(unsafe.Pointer(&size)),
			uintptr(unsafe.Sizeof(rawInputHeader{})),
		)
		if size == 0 || size > 4096 {
			ret, _, _ := procDefWindowProcW.Call(hwnd, msgID, wParam, lParam)
			return ret
		}

		// Allocate buffer and read data.
		buf := make([]byte, size)
		written, _, _ := procGetRawInputData.Call(
			lParam,
			uintptr(ridInput),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&size)),
			uintptr(unsafe.Sizeof(rawInputHeader{})),
		)
		if written == 0 || written == ^uintptr(0) {
			ret, _, _ := procDefWindowProcW.Call(hwnd, msgID, wParam, lParam)
			return ret
		}

		// The RAWINPUT header is at the start of the buffer.
		hdr := (*rawInputHeader)(unsafe.Pointer(&buf[0]))

		switch hdr.dwType {
		case rimTypeKeyboard:
			// RAWKEYBOARD immediately follows RAWINPUTHEADER.
			if len(buf) >= int(unsafe.Sizeof(rawInputHeader{}))+int(unsafe.Sizeof(rawKeyboard{})) {
				kb := (*rawKeyboard)(unsafe.Pointer(&buf[unsafe.Sizeof(rawInputHeader{})]))
				r.handleKeyboard(kb)
			}
		case rimTypeMouse:
			// RAWMOUSE immediately follows RAWINPUTHEADER.
			if len(buf) >= int(unsafe.Sizeof(rawInputHeader{}))+int(unsafe.Sizeof(rawMouse{})) {
				rm := (*rawMouse)(unsafe.Pointer(&buf[unsafe.Sizeof(rawInputHeader{})]))
				r.handleMouse(rm)
			}
		case rimTypeHID:
			// RAWHID struct: dwSizeHid (uint32) + dwCount (uint32) + bRawData (variable).
			// Total header size is sizeof(RAWINPUTHEADER) = 16/24 bytes depending on arch.
			headerSize := unsafe.Sizeof(rawInputHeader{})
			// RAWHID fields start after the header.
			if uintptr(len(buf)) < headerSize+8 {
				break
			}
			dwSizeHid := *(*uint32)(unsafe.Pointer(&buf[headerSize]))
			dwCount := *(*uint32)(unsafe.Pointer(&buf[headerSize+4]))
			if dwSizeHid == 0 || dwCount == 0 {
				break
			}
			dataOffset := headerSize + 8
			totalData := uintptr(dwSizeHid) * uintptr(dwCount)
			if uintptr(len(buf)) < dataOffset+totalData {
				break
			}
			rawHIDData := buf[dataOffset : dataOffset+totalData]
			r.routeHIDInput(hdr.hDevice, rawHIDData, dwSizeHid)
		}

		ret, _, _ := procDefWindowProcW.Call(hwnd, msgID, wParam, lParam)
		return ret

	default:
		ret, _, _ := procDefWindowProcW.Call(hwnd, msgID, wParam, lParam)
		return ret
	}
}

// handleDeviceChange processes WM_INPUT_DEVICE_CHANGE messages.
// wParam is GIDC_ARRIVAL (1) or GIDC_REMOVAL (2); lParam is the device handle.
func (r *Reader) handleDeviceChange(wParam, lParam uintptr) {
	hDevice := lParam
	added := wParam == giahDeviceArrival

	var usage hidDeviceUsage
	if !added {
		// For removal: read usage from cache before deleting it, so we can
		// route to the correct callback without re-querying the OS (which
		// would re-populate the cache for a stale handle and allow residual
		// WM_INPUT events to re-register the disconnected device).
		usage = r.hidDeviceCache[hDevice]
		delete(r.hidDeviceCache, hDevice)
		if usage.usagePage == 0 {
			return // device was never tracked; nothing to do
		}
	} else {
		// For arrival: look up (and cache) usage page/usage for this device.
		usage = r.resolveDeviceUsage(hDevice)
		if usage.usagePage == 0 {
			return // unknown device type
		}
	}

	// Route to matching callbacks.
	r.hidMu.RLock()
	callbacks := r.hidCallbacks
	r.hidMu.RUnlock()

	for _, cb := range callbacks {
		if cb.usagePage == usage.usagePage && cb.usage == usage.usage && cb.changeCb != nil {
			cb.changeCb(added, hDevice)
		}
	}
}

// routeHIDInput routes a raw HID report to registered callbacks.
// Called from wndProc on the message loop thread.
func (r *Reader) routeHIDInput(hDevice uintptr, rawData []byte, reportSize uint32) {
	usage := r.resolveDeviceUsage(hDevice)
	if usage.usagePage == 0 {
		return
	}

	r.hidMu.RLock()
	callbacks := r.hidCallbacks
	r.hidMu.RUnlock()

	for _, cb := range callbacks {
		if cb.usagePage == usage.usagePage && cb.usage == usage.usage && cb.inputCb != nil {
			cb.inputCb(hDevice, rawData, reportSize)
		}
	}
}

// resolveDeviceUsage returns the HID usage page and usage for a device handle,
// using a cache to avoid repeated GetRawInputDeviceInfoW calls.
// Only called from the message loop goroutine (no locking needed for the cache).
func (r *Reader) resolveDeviceUsage(hDevice uintptr) hidDeviceUsage {
	if u, ok := r.hidDeviceCache[hDevice]; ok {
		return u
	}

	// Query device info to determine usage page/usage.
	// RID_DEVICE_INFO layout: dwSize(4) + dwType(4) + union(varying)
	// For HID: dwType=2, then hid struct: dwVendorId(4)+dwProductId(4)+dwVersionNumber(4)+usUsagePage(2)+usUsage(2)
	// Total minimum size: 32 bytes (covers all variants on both 32/64-bit).
	var devInfo [32]byte
	devInfoSize := uint32(len(devInfo))
	ret, _, _ := procGetRawInputDeviceInfoW.Call(
		hDevice,
		ridiDeviceinfo,
		uintptr(unsafe.Pointer(&devInfo[0])),
		uintptr(unsafe.Pointer(&devInfoSize)),
	)
	if ret == ^uintptr(0) { // -1 = error
		u := hidDeviceUsage{}
		r.hidDeviceCache[hDevice] = u
		return u
	}

	// devInfo layout (RID_DEVICE_INFO):
	//   offset 0: DWORD cbSize
	//   offset 4: DWORD dwType  (0=mouse, 1=keyboard, 2=HID)
	//   offset 8: union { RID_DEVICE_INFO_MOUSE | RID_DEVICE_INFO_KEYBOARD | RID_DEVICE_INFO_HID }
	// RID_DEVICE_INFO_HID:
	//   offset 8:  DWORD dwVendorId
	//   offset 12: DWORD dwProductId
	//   offset 16: DWORD dwVersionNumber
	//   offset 20: USAGE usUsagePage
	//   offset 22: USAGE usUsage
	dwType := *(*uint32)(unsafe.Pointer(&devInfo[4]))
	if dwType != 2 { // not a HID device
		u := hidDeviceUsage{}
		r.hidDeviceCache[hDevice] = u
		return u
	}

	usagePage := *(*uint16)(unsafe.Pointer(&devInfo[20]))
	usage := *(*uint16)(unsafe.Pointer(&devInfo[22]))
	u := hidDeviceUsage{usagePage: usagePage, usage: usage}
	r.hidDeviceCache[hDevice] = u
	return u
}

// handleKeyboard processes a RAWKEYBOARD event.
func (r *Reader) handleKeyboard(kb *rawKeyboard) {
	vc := input.RawScanToUIohook(kb.MakeCode, kb.Flags)
	if vc == 0 {
		return
	}

	pressed := (kb.Flags & riKeyBreak) == 0

	r.mu.Lock()
	if pressed {
		r.keys[vc] = true
		r.pendingKeysDown = append(r.pendingKeysDown, vc)
	} else {
		delete(r.keys, vc)
		r.pendingKeysUp = append(r.pendingKeysUp, vc)
	}
	r.mu.Unlock()
}

// handleMouse processes a RAWMOUSE event.
func (r *Reader) handleMouse(rm *rawMouse) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Button events
	if rm.usButtonFlags != 0 {
		// Each bit pair in usButtonFlags represents a button down or up.
		// We iterate over each possible flag independently.
		for _, flag := range []uint16{
			0x0001, 0x0002, // left
			0x0004, 0x0008, // right
			0x0010, 0x0020, // middle
			0x0040, 0x0080, // X1
			0x0100, 0x0200, // X2
		} {
			if rm.usButtonFlags&flag == 0 {
				continue
			}
			code, down := input.MouseButtonFlagToIOCode(flag)
			if code == 0 {
				continue
			}
			if down {
				r.mouseButtons[code] = true
				r.pendingButtonsDown = append(r.pendingButtonsDown, code)
			} else {
				delete(r.mouseButtons, code)
				r.pendingButtonsUp = append(r.pendingButtonsUp, code)
			}
		}

		// Mouse wheel (encoded in usButtonFlags + usButtonData)
		if rm.usButtonFlags&riMouseWheelDelta != 0 {
			// usButtonData is a USHORT that holds a signed short wheel delta
			delta := int16(rm.usButtonData)
			if delta > 0 {
				r.wheelUp = true
			} else if delta < 0 {
				r.wheelDown = true
			}
		}
	}

	// Mouse movement (relative mode only; MOUSE_MOVE_ABSOLUTE is ignored here)
	const mouseMoveRelative = 0x00
	if rm.usFlags&0x01 == mouseMoveRelative {
		r.rawMouseDX += rm.lLastX
		r.rawMouseDY += rm.lLastY
	}
}

// clampF32 clamps v to [lo, hi].
func clampF32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
