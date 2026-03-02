package tray

import (
	"log"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"

	"fyne.io/systray"
)

// ShutdownFunc is called when "Exit" is clicked
type ShutdownFunc func()

// Tray manages the system tray icon and menu
type Tray struct {
	shutdownFunc ShutdownFunc
	once         sync.Once
	shuttingDown atomic.Bool
	menuOpen     *systray.MenuItem
	menuExit     *systray.MenuItem
}

// New creates a new Tray instance
func New(shutdownFn ShutdownFunc) *Tray {
	return &Tray{
		shutdownFunc: shutdownFn,
	}
}

// Run initializes and runs the system tray (blocks until Quit())
func (t *Tray) Run(iconData []byte) {
	systray.Run(func() {
		t.onReady(iconData)
	}, func() {
		t.onExit()
	})
}

// onReady is called when the tray is ready
func (t *Tray) onReady(iconData []byte) {
	if iconData != nil {
		systray.SetIcon(iconData)
	}
	systray.SetTitle("GameControllerView")
	systray.SetTooltip("GameControllerView - http://localhost:8080")

	t.menuOpen = systray.AddMenuItem("Open Browser", "Open web interface")
	t.menuExit = systray.AddMenuItem("Exit", "Quit application")

	// Handle menu clicks in separate goroutines to prevent blocking
	go t.handleMenuClicks()

	log.Println("System tray initialized")
}

// handleMenuClicks processes menu item clicks without blocking
func (t *Tray) handleMenuClicks() {
	for {
		select {
		case <-t.menuOpen.ClickedCh:
			if !t.shuttingDown.Load() {
				t.openBrowser()
			}
		case <-t.menuExit.ClickedCh:
			if t.shuttingDown.CompareAndSwap(false, true) {
				t.once.Do(t.shutdownFunc)
				// Small delay to ensure shutdown function completes
				systray.Quit()
				return
			}
		}
	}
}

// onExit is called when the tray is exiting
func (t *Tray) onExit() {
	t.shuttingDown.Store(true)
	log.Println("System tray exiting")
}

// openBrowser opens the default web browser
func (t *Tray) openBrowser() {
	// Prevent multiple browser launches during shutdown
	if t.shuttingDown.Load() {
		return
	}

	url := "http://localhost:8080"
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to open browser: %v", err)
	}
}
