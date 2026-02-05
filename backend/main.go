package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/soar/GameControllerView/backend/internal/console"
	"github.com/soar/GameControllerView/backend/internal/gamepad"
	"github.com/soar/GameControllerView/backend/internal/hub"
	"github.com/soar/GameControllerView/backend/internal/server"
	"github.com/soar/GameControllerView/backend/internal/tray"
)

// buildShutdownSignals constructs the signal list based on the platform.
// On Windows, os.Interrupt handles both Ctrl+C and Ctrl+Break.
func buildShutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}

var shutdownSignals = buildShutdownSignals()

func main() {
	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to wait for reader completion
	readerDone := make(chan struct{})

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, shutdownSignals...)

	// Windows-specific: Set up console control handler for reliable Ctrl+C handling
	// This is needed because SDL3 with LockOSThread() may interfere with Go's signal handling
	windowsCtrlCh := make(chan struct{}, 1)
	registerWindowsHandler := console.SetupConsoleHandler(windowsCtrlCh)

	// Create gamepad reader
	reader := gamepad.NewReader()

	// On Windows, set up a callback to re-register the console handler after SDL initialization
	// This is needed because SDL3 may override or disable our console handler during initialization
	if runtime.GOOS == "windows" {
		reader.SetOnSDLInitCallback(func() {
			registerWindowsHandler()
		})
	}

	// Create and start hub
	h := hub.NewHub()
	go h.Run()

	// Create broadcaster
	broadcaster := hub.NewBroadcaster(h, reader.Changes())
	go broadcaster.Run()

	// Create and start HTTP server
	frontendFS := getFrontendFS()
	srv := server.New(h, broadcaster, reader, frontendFS, ":8080")
	serverErrCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	log.Println("GameControllerView started: http://localhost:8080")

	// Channel for tray-triggered shutdown
	shutdownRequested := make(chan struct{})

	// Determine startup mode based on whether we have a console
	consoleMode := console.IsRunningFromConsole()

	// Initialize system tray only in GUI mode (no console attached)
	if runtime.GOOS == "windows" && !consoleMode {
		go func() {
			t := tray.New(func() {
				close(shutdownRequested)
			})
			t.Run(tray.GetIcon())
		}()
	} else {
		// Console mode: show exit instructions
		if runtime.GOOS == "windows" {
			log.Println("Running in console mode. Press Ctrl+C or Ctrl+Break to exit.")
		} else {
			log.Println("Press Ctrl+C to exit")
		}
	}

	// Run reader in goroutine (but SDL main thread handling is inside)
	// Note: reader.Run() must be called from a goroutine with LockOSThread
	// The signal handling will cancel the context, causing reader.Run() to return
	go func() {
		reader.Run(ctx)
		close(readerDone)
	}()

	// Wait for shutdown signal, tray request, server error, or Windows Ctrl+C
	select {
	case <-sigCh:
		log.Println("Shutting down...")
		cancel()
	case <-shutdownRequested:
		log.Println("Shutdown requested from tray")
		cancel()
	case err := <-serverErrCh:
		log.Printf("HTTP server error: %v", err)
		cancel()
	case <-windowsCtrlCh:
		log.Println("Ctrl+C detected via Windows console handler")
		cancel()
	}

	// Wait for reader to finish
	<-readerDone

	// Shutdown the HTTP server gracefully
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("GameControllerView stopped")
}
