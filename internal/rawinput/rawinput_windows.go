//go:build windows

// Package rawinput provides a Windows Raw Input global keyboard and mouse reader.
// It registers a hidden HWND_MESSAGE window and uses RIDEV_INPUTSINK so that input
// is received even when the window is not in the foreground.
package rawinput

import (
	"context"
	"log"
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

	// GetRawInputData command
	ridInput = 0x10000003

	// RAWINPUT type codes (from winuser.h)
	rimTypeMouse    = 0
	rimTypeKeyboard = 1

	// RAWKEYBOARD Flags bits
	riKeyBreak = 0x01 // key released (make = 0x00)
	riKeyE0    = 0x02 // E0 extended prefix

	// RAWMOUSE usButtonFlags — wheel
	riMouseWheelDelta   = 0x0400
	wheelDeltaThreshold = 120 // standard Windows WHEEL_DELTA

	// WM_INPUT message identifier
	wmInput = 0x00FF

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

// rawInput is the RAWINPUT structure (keyboard + mouse variant; we size for both).
type rawInput struct {
	header rawInputHeader
	// The data union is at most sizeof(RAWMOUSE)=24 or sizeof(RAWKEYBOARD)=16 bytes.
	// We allocate 32 bytes of padding to cover both.
	data [32]byte
}

// rawInputMouse returns a pointer to the RAWMOUSE data within a rawInput.
func (r *rawInput) rawInputMouse() *rawMouse {
	return (*rawMouse)(unsafe.Pointer(&r.data[0]))
}

// rawInputKeyboard returns a pointer to the RAWKEYBOARD data within a rawInput.
func (r *rawInput) rawInputKeyboard() *rawKeyboard {
	return (*rawKeyboard)(unsafe.Pointer(&r.data[0]))
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

// Reader collects keyboard and mouse input via Windows Raw Input API.
// It creates an invisible HWND_MESSAGE window and runs a Windows message loop on a
// dedicated OS-locked goroutine. State is emitted to Changes() at ~60 Hz.
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
}

// New creates a new Raw Input Reader with default settings.
func New() *Reader {
	return &Reader{
		keys:         make(map[uint16]bool),
		mouseButtons: make(map[uint16]bool),
		sensitivity:  defaultMouseSensitivity,
		changes:      make(chan input.KeyMouseState, 64),
		hwndReady:    make(chan struct{}),
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

// Run starts the Raw Input message loop and the periodic state emitter.
// It blocks until ctx is cancelled. Must be called in a goroutine.
func (r *Reader) Run(ctx context.Context) {
	// Lock this goroutine to its OS thread.
	// The Windows message loop must run on the same thread that created the window.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hwnd, err := r.createMessageWindow()
	if err != nil {
		log.Printf("rawinput: failed to create message window: %v", err)
		close(r.hwndReady)
		close(r.changes)
		return
	}
	r.hwnd = hwnd
	close(r.hwndReady) // signal that HWND is valid

	if err := r.registerDevices(hwnd); err != nil {
		log.Printf("rawinput: failed to register raw input devices: %v", err)
		// Continue anyway — partial functionality is better than none
	} else {
		log.Printf("rawinput: registered keyboard+mouse with RIDEV_INPUTSINK on hwnd=0x%x", hwnd)
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
// It handles WM_INPUT messages; all other messages are forwarded to DefWindowProc.
func (r *Reader) wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	if msg != wmInput {
		ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return ret
	}
	// Determine required buffer size
	var size uint32
	procGetRawInputData.Call(
		lParam,
		uintptr(ridInput),
		0,
		uintptr(unsafe.Pointer(&size)),
		uintptr(unsafe.Sizeof(rawInputHeader{})),
	)
	if size == 0 || size > 1024 {
		ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return ret
	}

	// Allocate buffer and read data
	buf := make([]byte, size)
	written, _, _ := procGetRawInputData.Call(
		lParam,
		uintptr(ridInput),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
		uintptr(unsafe.Sizeof(rawInputHeader{})),
	)
	if written == 0 || written == ^uintptr(0) {
		ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return ret
	}

	ri := (*rawInput)(unsafe.Pointer(&buf[0]))
	switch ri.header.dwType {
	case rimTypeKeyboard:
		r.handleKeyboard(ri.rawInputKeyboard())
	case rimTypeMouse:
		r.handleMouse(ri.rawInputMouse())
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return ret
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
