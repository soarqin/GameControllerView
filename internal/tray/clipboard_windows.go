//go:build windows

package tray

import (
	"log/slog"
	"syscall"
	"unsafe"
)

var (
	modUser32   = syscall.NewLazyDLL("user32.dll")
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")

	procOpenClipboard    = modUser32.NewProc("OpenClipboard")
	procEmptyClipboard   = modUser32.NewProc("EmptyClipboard")
	procSetClipboardData = modUser32.NewProc("SetClipboardData")
	procCloseClipboard   = modUser32.NewProc("CloseClipboard")
	procGlobalAlloc      = modKernel32.NewProc("GlobalAlloc")
	procGlobalLock       = modKernel32.NewProc("GlobalLock")
	procGlobalUnlock     = modKernel32.NewProc("GlobalUnlock")
	procGlobalFree       = modKernel32.NewProc("GlobalFree")
)

const (
	cfUnicodeText = 13   // CF_UNICODETEXT
	gmemMoveable  = 0x02 // GMEM_MOVEABLE
)

// copyToClipboard writes text to the Windows clipboard using Win32 API calls.
func copyToClipboard(text string) {
	// Encode text as UTF-16 with null terminator.
	utf16, err := syscall.UTF16FromString(text)
	if err != nil {
		slog.Error("clipboard: UTF16 encoding failed", "error", err)
		return
	}
	byteLen := uintptr(len(utf16) * 2)

	// Allocate moveable global memory.
	hMem, _, err := procGlobalAlloc.Call(gmemMoveable, byteLen)
	if hMem == 0 {
		slog.Error("clipboard: GlobalAlloc failed", "error", err)
		return
	}

	// Track whether SetClipboardData took ownership of hMem.
	// If it did, we must NOT free the handle ourselves.
	ownershipTransferred := false
	defer func() {
		if !ownershipTransferred {
			procGlobalFree.Call(hMem)
		}
	}()

	// Lock and write UTF-16 data.
	ptr, _, err := procGlobalLock.Call(hMem)
	if ptr == 0 {
		slog.Error("clipboard: GlobalLock failed", "error", err)
		return
	}
	// nolint: gosec — ptr is a system-allocated handle from GlobalLock, not GC-managed memory.
	dst := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(utf16)) //nolint:govet
	copy(dst, utf16)
	procGlobalUnlock.Call(hMem)

	// Open clipboard, empty it, set data, close.
	ret, _, err := procOpenClipboard.Call(0)
	if ret == 0 {
		slog.Error("clipboard: OpenClipboard failed", "error", err)
		return
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()
	ret, _, err = procSetClipboardData.Call(cfUnicodeText, hMem)
	if ret == 0 {
		slog.Error("clipboard: SetClipboardData failed", "error", err)
		return
	}
	// SetClipboardData succeeded — the system now owns hMem.
	ownershipTransferred = true
}
