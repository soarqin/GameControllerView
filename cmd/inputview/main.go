package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/soar/inputview/internal/gamepad"
	"github.com/soar/inputview/internal/hub"
	"github.com/soar/inputview/internal/rawinput"
	"github.com/soar/inputview/internal/server"
	"github.com/soar/inputview/internal/web"
)

func main() {
	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create gamepad reader
	reader := gamepad.NewReader()

	// Determine the directory containing this executable early (needed for SDL DB path).
	appExeDir := "."
	if exe, err := os.Executable(); err == nil {
		appExeDir = filepath.Dir(exe)
	} else {
		log.Printf("Warning: could not determine executable path: %v", err)
	}

	// Load SDL GameControllerDB. The embedded database is always used as a base;
	// an external gamecontrollerdb.txt next to the executable (if present) is
	// merged on top so users can update mappings without recompiling.
	sdlDBPath := filepath.Join(appExeDir, "gamecontrollerdb.txt")
	gamepad.LoadSDLDB(sdlDBPath)

	// Set up shutdown handling (console Ctrl+C or system tray, depending on build mode).
	extraShutdownCh := setupShutdown(appExeDir)

	// Handle OS signals (Ctrl+C on Unix, SIGTERM everywhere)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Create and start hub
	h := hub.NewHub()
	go h.Run()

	// Create Raw Input reader for keyboard and mouse (Windows: global capture via Raw Input API).
	// Also used as the shared HWND_MESSAGE window for HID gamepad input.
	kmReader := rawinput.New()

	// Register HID gamepad callbacks on the Raw Input window so that non-XInput
	// controllers (PS4/PS5/Switch Pro/generic HID) are captured alongside XInput.
	// Must be called before kmReader.Run().
	reader.SetRawInputReader(kmReader)

	// Create broadcaster (listens to both gamepad and keyboard/mouse channels)
	broadcaster := hub.NewBroadcaster(h, reader.Changes(), kmReader.Changes())
	go broadcaster.Run()

	// Create and start HTTP server
	srv := server.New(h, broadcaster, reader, web.FrontendFS(), appExeDir, ":8080")
	serverErrCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	log.Println("InputView started: http://localhost:8080")

	// Run gamepad reader (XInput polling loop, ~60 Hz)
	readerDone := make(chan struct{})
	go func() {
		reader.Run(ctx)
		close(readerDone)
	}()

	// Run keyboard/mouse Raw Input reader in a separate goroutine
	// (also uses LockOSThread internally on Windows for the message loop)
	go kmReader.Run(ctx)

	// Wait for any shutdown trigger
	if extraShutdownCh != nil {
		select {
		case <-sigCh:
			log.Println("Shutting down...")
		case <-extraShutdownCh:
			log.Println("Shutdown requested")
		case err := <-serverErrCh:
			log.Printf("HTTP server error: %v", err)
		}
	} else {
		select {
		case <-sigCh:
			log.Println("Shutting down...")
		case err := <-serverErrCh:
			log.Printf("HTTP server error: %v", err)
		}
	}
	cancel()

	// Wait for reader to finish
	<-readerDone

	// Graceful HTTP server shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("InputView stopped")
}
