package tray

import (
	"log/slog"
	"net/url"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"

	"fyne.io/systray"
	"github.com/soar/inputview/internal/overlay"
)

// ShutdownFunc is called when "Exit" is clicked
type ShutdownFunc func()

// overlayMenuItem pairs a menu item with the overlay URL path it represents.
type overlayMenuItem struct {
	item    *systray.MenuItem
	urlPath string
}

// Tray manages the system tray icon and menu
type Tray struct {
	shutdownFunc ShutdownFunc
	once         sync.Once
	shuttingDown atomic.Bool
	stopCh       chan struct{}
	overlays     []overlay.Entry
	addr         string

	// "Open Browser" parent + sub-items
	menuOpen        *systray.MenuItem
	menuOpenDefault *systray.MenuItem
	openItems       []overlayMenuItem

	// "Copy URL for Streaming" parent + sub-items
	menuCopy        *systray.MenuItem
	menuCopyDefault *systray.MenuItem
	copyItems       []overlayMenuItem

	menuExit *systray.MenuItem
}

// New creates a new Tray instance.
// overlays is the list of available overlay configs (may be empty).
// addr is the HTTP listen address, e.g. ":8080".
func New(shutdownFn ShutdownFunc, overlays []overlay.Entry, addr string) *Tray {
	return &Tray{
		shutdownFunc: shutdownFn,
		stopCh:       make(chan struct{}),
		overlays:     overlays,
		addr:         addr,
	}
}

// Run initializes and runs the system tray (blocks until Quit())
func (t *Tray) Run(iconData []byte) {
	runtime.LockOSThread()
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
	systray.SetTitle("InputView")
	systray.SetTooltip("InputView - http://localhost" + t.addr)

	// ── "Open Browser" parent ────────────────────────────────────────────────
	t.menuOpen = systray.AddMenuItem("Open Browser", "Open web interface in browser")
	t.menuOpenDefault = t.menuOpen.AddSubMenuItem("Default", "Open default page")
	for _, ov := range t.overlays {
		sub := t.menuOpen.AddSubMenuItem(ov.Name, "Open overlay: "+ov.Name)
		t.openItems = append(t.openItems, overlayMenuItem{item: sub, urlPath: ov.URLPath})
	}

	// ── "Copy URL for Streaming" parent ──────────────────────────────────────
	// Sub-items mirror "Open Browser" but the URLs include &simple=1 for use
	// in streaming software (transparent background, no UI chrome).
	t.menuCopy = systray.AddMenuItem("Copy URL for Streaming", "Copy URL with simple=1 to clipboard")
	t.menuCopyDefault = t.menuCopy.AddSubMenuItem("Default", "Copy default streaming URL")
	for _, ov := range t.overlays {
		sub := t.menuCopy.AddSubMenuItem(ov.Name, "Copy streaming URL for overlay: "+ov.Name)
		t.copyItems = append(t.copyItems, overlayMenuItem{item: sub, urlPath: ov.URLPath})
	}

	t.menuExit = systray.AddMenuItem("Exit", "Quit application")

	// Aggregate overlay sub-item clicks into single channels so that the main
	// select loop does not need a dynamic number of cases.
	openURLCh := make(chan string, 1)
	for _, ov := range t.openItems {
		go func(item *systray.MenuItem, urlPath string) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic in tray overlay open handler", "panic", r)
				}
			}()

			for {
				select {
				case <-item.ClickedCh:
					if !t.shuttingDown.Load() {
						select {
						case openURLCh <- t.buildURL(urlPath, false):
						default:
						}
					}
				case <-t.stopCh:
					return
				}
			}
		}(ov.item, ov.urlPath)
	}

	copyURLCh := make(chan string, 1)
	for _, ov := range t.copyItems {
		go func(item *systray.MenuItem, urlPath string) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic in tray overlay copy handler", "panic", r)
				}
			}()

			for {
				select {
				case <-item.ClickedCh:
					if !t.shuttingDown.Load() {
						select {
						case copyURLCh <- t.buildURL(urlPath, true):
						default:
						}
					}
				case <-t.stopCh:
					return
				}
			}
		}(ov.item, ov.urlPath)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic in tray menu click handler", "panic", r)
			}
		}()
		t.handleMenuClicks(openURLCh, copyURLCh)
	}()

	slog.Info("system tray initialized")
}

// handleMenuClicks processes menu item clicks without blocking
func (t *Tray) handleMenuClicks(openURLCh, copyURLCh <-chan string) {
	for {
		select {
		case <-t.stopCh:
			return

		// ── Open Browser ────────────────────────────────────────────────────
		case <-t.menuOpenDefault.ClickedCh:
			if !t.shuttingDown.Load() {
				// Run in a separate goroutine so that this select loop is never
				// blocked. On Windows, exec.Command(...).Start() can stall under
				// certain conditions (antivirus scanning, disk pressure), which
				// would prevent ClickedCh from being drained and cause all
				// subsequent menu clicks to be silently dropped by systray's
				// non-blocking send.
				go func() {
					defer func() {
						if r := recover(); r != nil {
							slog.Error("panic in openBrowserURL", "panic", r)
						}
					}()
					t.openBrowserURL(t.buildURL("", false))
				}()
			}
		case u := <-openURLCh:
			if !t.shuttingDown.Load() {
				go func() {
					defer func() {
						if r := recover(); r != nil {
							slog.Error("panic in openBrowserURL", "panic", r)
						}
					}()
					t.openBrowserURL(u)
				}()
			}

		// ── Copy URL for Streaming ───────────────────────────────────────────
		case <-t.menuCopyDefault.ClickedCh:
			if !t.shuttingDown.Load() {
				go func() {
					defer func() {
						if r := recover(); r != nil {
							slog.Error("panic in copyToClipboard", "panic", r)
						}
					}()
					copyToClipboard(t.buildURL("", true))
				}()
			}
		case u := <-copyURLCh:
			if !t.shuttingDown.Load() {
				go func() {
					defer func() {
						if r := recover(); r != nil {
							slog.Error("panic in copyToClipboard", "panic", r)
						}
					}()
					copyToClipboard(u)
				}()
			}

		// ── Exit ─────────────────────────────────────────────────────────────
		case <-t.menuExit.ClickedCh:
			if t.shuttingDown.CompareAndSwap(false, true) {
				t.once.Do(t.shutdownFunc)
				systray.Quit()
				return
			}
		}
	}
}

// onExit is called when the tray is exiting
func (t *Tray) onExit() {
	t.shuttingDown.Store(true)
	close(t.stopCh)
	slog.Info("system tray exiting")
}

// buildURL constructs the URL for the given overlay path.
// If urlPath is empty the base URL is returned.
// If streaming is true, simple=1 is appended (for use in streaming software).
func (t *Tray) buildURL(urlPath string, streaming bool) string {
	base := "http://localhost" + t.addr
	if urlPath == "" && !streaming {
		return base
	}

	params := ""
	if urlPath != "" {
		params += "overlay=" + url.QueryEscape(urlPath)
	}
	if streaming {
		if params != "" {
			params += "&"
		}
		params += "simple=1"
	}
	return base + "?" + params
}

// openBrowserURL opens the given URL in the default web browser.
func (t *Tray) openBrowserURL(targetURL string) {
	if t.shuttingDown.Load() {
		return
	}

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL)
	case "darwin":
		cmd = exec.Command("open", targetURL)
	default:
		cmd = exec.Command("xdg-open", targetURL)
	}

	if err := cmd.Start(); err != nil {
		slog.Warn("failed to open browser", "error", err)
	} else {
		go cmd.Wait()
	}
}
