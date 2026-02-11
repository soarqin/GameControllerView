// Package console provides cross-platform console detection and signal handling.
// On Windows, it provides utilities to detect if the program is running from a terminal
// or was double-clicked (GUI mode), and sets up reliable Ctrl+C handling that works
// even with libraries like SDL3 that use runtime.LockOSThread().
package console

import (
	"log"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// Windows API declarations
var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	user32                    = syscall.NewLazyDLL("user32.dll")
	procGetConsoleWindow      = kernel32.NewProc("GetConsoleWindow")
	procAllocConsole          = kernel32.NewProc("AllocConsole")
	procAttachConsole         = kernel32.NewProc("AttachConsole")
	procFreeConsole           = kernel32.NewProc("FreeConsole")
	procGetStdHandle          = kernel32.NewProc("GetStdHandle")
	procSetStdHandle          = kernel32.NewProc("SetStdHandle")
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First        = kernel32.NewProc("Process32First")
	procProcess32Next         = kernel32.NewProc("Process32Next")
	procOpenProcess           = kernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procSetConsoleCtrlHandler = kernel32.NewProc("SetConsoleCtrlHandler")
)

const (
	TH32CS_SNAPPROCESS         = 0x00000002
	PROCESS_QUERY_LIMITED_INFO = 0x1000
	MAX_PATH                   = 260
	CTRL_C_EVENT               = 0
	CTRL_BREAK_EVENT           = 1
	ATTACH_PARENT_PROCESS      = ^uint32(0) // 0xFFFFFFFF, attaches to parent process console
	STD_INPUT_HANDLE           = ^uint32(0) - 10 + 1 // 0xFFFFFFF6, -10
	STD_OUTPUT_HANDLE          = ^uint32(0) - 11 + 1 // 0xFFFFFFF5, -11
	STD_ERROR_HANDLE           = ^uint32(0) - 12 + 1 // 0xFFFFFFF4, -12
)

// PROCESSENTRY32 is the structure for Process32First/Next
type PROCESSENTRY32 struct {
	DwSize              uint32
	CntUsage            uint32
	Th32ProcessID       uint32
	Th32DefaultHeapID   uintptr
	Th32ModuleID         uint32
	CntThreads          uint32
	Th32ParentProcessID uint32
	PcPriClassBase      int32
	DwFlags             uint32
	SzExeFile           [MAX_PATH]uint16
}

// IsRunningFromConsole checks if the program is running from a terminal or in GUI mode.
// Returns true if running from a terminal (cmd/PowerShell), false if GUI mode (double-clicked).
//
// On Windows, this function handles console allocation intelligently:
// - If the program already has a console (console-mode build or launched from terminal),
//   it reuses the existing console.
// - If the program has no console (GUI-mode build) and was launched from a terminal,
//   it allocates a new console window and redirects stdout/stderr/stdin.
// - If the program was double-clicked (launched from explorer.exe), it returns false
//   to indicate GUI mode (no console needed). If a console was auto-created (console-mode build),
//   it frees the console to hide the window.
func IsRunningFromConsole() bool {
	if runtime.GOOS != "windows" {
		return true // Non-Windows platforms always have console
	}

	// Check if we already have a console window
	if hasConsoleWindow() {
		// Check if parent process is explorer.exe (double-click scenario)
		if isLaunchedFromExplorer() {
			// Console-mode build was double-clicked:
			// console was auto-created, free it to hide the window
			freeConsole()
			return false
		}
		// Already have a console from terminal, reuse it
		return true
	}

	// No console window exists
	// Check if parent process is explorer.exe (double-click scenario)
	if isLaunchedFromExplorer() {
		// GUI-mode build was double-clicked: no console needed
		return false
	}

	// GUI-mode build launched from terminal:
	// Attach to parent console and redirect std streams
	attachToParentConsole()
	return true
}

// hasConsoleWindow checks if the process has an attached console window.
func hasConsoleWindow() bool {
	hwnd, _, _ := procGetConsoleWindow.Call()
	return hwnd != 0
}

// attachToParentConsole allocates a new console and redirects std streams.
// This is used for GUI-mode builds that are launched from a terminal.
// Note: We use AllocConsole() instead of AttachConsole() because:
// - AttachConsole() causes input confusion since both parent and child share the console
// - AllocConsole() creates a separate console window with its own input/output
func attachToParentConsole() {
	// Allocate a new console window (independent from parent terminal)
	procAllocConsole.Call()

	// Redirect standard streams to the console
	redirectStdStreams()
}

// redirectStdStreams redirects stdout, stderr, and stdin to the attached/allocated console.
// This is necessary because Go's os.Stdout/stderr/stdin are initialized at startup
// and need to be updated after console allocation/attachment.
func redirectStdStreams() {
	// Get handles to the console std streams
	nStdout, _, _ := procGetStdHandle.Call(uintptr(STD_OUTPUT_HANDLE))
	nStderr, _, _ := procGetStdHandle.Call(uintptr(STD_ERROR_HANDLE))
	nStdin, _, _ := procGetStdHandle.Call(uintptr(STD_INPUT_HANDLE))

	if nStdout == 0 || nStderr == 0 {
		// Failed to get console handles
		return
	}

	// Create new os.File objects from the Windows handles
	// The mode parameter should match the expected behavior:
	// - For stdout/stderr: write-only (syscall.O_WRONLY)
	// - For stdin: read-only (syscall.O_RDONLY)
	os.Stdout = os.NewFile(uintptr(nStdout), "/dev/stdout")
	os.Stderr = os.NewFile(uintptr(nStderr), "/dev/stderr")
	if nStdin != 0 {
		os.Stdin = os.NewFile(uintptr(nStdin), "/dev/stdin")
	}

	// Also update log package's default output if it's already configured
	// (Note: this doesn't affect custom log writers)
	log.SetOutput(os.Stderr)
}

// isLaunchedFromExplorer checks if the parent process is explorer.exe
func isLaunchedFromExplorer() bool {
	currentPID := os.Getpid()

	// Get parent process ID using CreateToolhelp32Snapshot
	parentPID := getParentProcessID(currentPID)
	if parentPID == 0 {
		return false
	}

	// Get parent process image name
	parentName := getProcessImageName(parentPID)
	if parentName == "" {
		return false
	}

	// Check if parent is explorer.exe (case-insensitive)
	return isExplorerExe(parentName)
}

// getParentProcessID returns the parent process ID for a given process ID
func getParentProcessID(pid int) int {
	// Create snapshot of all processes
	handle, _, _ := procCreateToolhelp32Snapshot.Call(uintptr(TH32CS_SNAPPROCESS), 0)
	if handle == uintptr(syscall.InvalidHandle) {
		return 0
	}
	defer syscall.CloseHandle(syscall.Handle(handle))

	var entry PROCESSENTRY32
	entry.DwSize = uint32(unsafe.Sizeof(entry))

	// First process
	ret, _, _ := procProcess32First.Call(handle, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return 0
	}

	// Iterate through processes
	for {
		if int(entry.Th32ProcessID) == pid {
			return int(entry.Th32ParentProcessID)
		}

		ret, _, _ = procProcess32Next.Call(handle, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	return 0
}

// getProcessImageName returns the executable name for a given process ID
func getProcessImageName(pid int) string {
	// Open process with query permissions
	hProcess, _, _ := procOpenProcess.Call(uintptr(PROCESS_QUERY_LIMITED_INFO), 0, uintptr(pid))
	if hProcess == 0 {
		return ""
	}
	defer syscall.CloseHandle(syscall.Handle(hProcess))

	// Query full process image name
	var nameBuf [MAX_PATH]uint16
	size := uint32(MAX_PATH)
	ret, _, _ := procQueryFullProcessImageNameW.Call(hProcess, 0, uintptr(unsafe.Pointer(&nameBuf[0])), uintptr(unsafe.Pointer(&size)))
	if ret == 0 {
		return ""
	}

	// Convert to string and extract filename
	name := syscall.UTF16ToString(nameBuf[:size])
	return name
}

// isExplorerExe checks if the process name is explorer.exe (case-insensitive)
func isExplorerExe(path string) bool {
	// Extract filename from path
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '\\' || path[i] == '/' {
			path = path[i+1:]
			break
		}
	}

	// Case-insensitive comparison
	return strings.EqualFold(path, "explorer.exe")
}

func freeConsole() {
	procFreeConsole.Call()
}

// consoleHandlerState holds the state for Windows console control handler
type consoleHandlerState struct {
	closed       int32          // atomic: 0 = not closed, 1 = closed
	shutdownChan chan struct{}
	callbackFn   uintptr        // Stores the callback function pointer
}

// Global state for Windows console handler (accessible from callback)
var globalHandlerState *consoleHandlerState

// SetupConsoleHandler sets up a Windows console control handler for Ctrl+C.
// This is needed because Go's os.Interrupt signal handling may not work reliably
// when certain libraries (e.g., SDL3) are running with runtime.LockOSThread().
//
// Only applicable on Windows. On other platforms, it returns a no-op function.
//
// Parameters:
//   - shutdownChan: Channel that will be closed when Ctrl+C or Ctrl+Break is pressed.
//
// Returns:
//   - A function that can be called to re-register the handler after library initialization.
//     This is necessary because some libraries (like SDL3) override console handlers during init.
func SetupConsoleHandler(shutdownChan chan struct{}) func() {
	if runtime.GOOS != "windows" {
		return func() {}
	}

	// Allocate state on heap to ensure it's valid for the callback
	globalHandlerState = &consoleHandlerState{
		shutdownChan: shutdownChan,
	}

	// Create a callback function that Windows can call
	// Must be in a format that Windows API expects: BOOL WINAPI HandlerRoutine(DWORD dwCtrlType)
	globalHandlerState.callbackFn = syscall.NewCallback(func(ctrlType uint32) uintptr {
		if ctrlType == CTRL_C_EVENT || ctrlType == CTRL_BREAK_EVENT {
			// Use atomic operation to ensure we only close once
			if atomic.CompareAndSwapInt32(&globalHandlerState.closed, 0, 1) {
				close(globalHandlerState.shutdownChan)
			}
			return 1 // Return TRUE to indicate we handled the event
		}
		return 0 // Return FALSE to let the next handler handle it
	})

	// Function to register the handler
	registerHandler := func() {
		if globalHandlerState == nil {
			return
		}
		ret, _, _ := procSetConsoleCtrlHandler.Call(
			globalHandlerState.callbackFn,
			1, // TRUE = add handler
		)
		if ret == 0 {
			log.Printf("Warning: Failed to set Windows console control handler")
		}
	}

	// Initial registration
	registerHandler()

	// Return a function that can be called to re-register (e.g., after SDL init)
	return registerHandler
}
