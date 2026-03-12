//go:build windows

package tray

import (
	"log"
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
		log.Printf("Clipboard: UTF16 encoding failed: %v", err)
		return
	}
	byteLen := uintptr(len(utf16) * 2)

	// Allocate moveable global memory.
	hMem, _, err := procGlobalAlloc.Call(gmemMoveable, byteLen)
	if hMem == 0 {
		log.Printf("Clipboard: GlobalAlloc failed: %v", err)
		return
	}

	// Lock and write UTF-16 data.
	ptr, _, err := procGlobalLock.Call(hMem)
	if ptr == 0 {
		log.Printf("Clipboard: GlobalLock failed: %v", err)
		return
	}
	dst := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(utf16))
	copy(dst, utf16)
	procGlobalUnlock.Call(hMem)

	// Open clipboard, empty it, set data, close.
	ret, _, err := procOpenClipboard.Call(0)
	if ret == 0 {
		log.Printf("Clipboard: OpenClipboard failed: %v", err)
		return
	}
	procEmptyClipboard.Call()
	ret, _, err = procSetClipboardData.Call(cfUnicodeText, hMem)
	if ret == 0 {
		log.Printf("Clipboard: SetClipboardData failed: %v", err)
	}
	procCloseClipboard.Call()
}
