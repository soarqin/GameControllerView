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

	"github.com/soar/GameControllerView/backend/internal/gamepad"
	"github.com/soar/GameControllerView/backend/internal/hub"
	"github.com/soar/GameControllerView/backend/internal/server"
	"github.com/soar/GameControllerView/backend/internal/tray"
)

// Cross-platform signal handling: use os.Interrupt on all platforms
// On Windows: os.Interrupt is sent when Ctrl+C is pressed
// On Unix: os.Interrupt is equivalent to syscall.SIGINT
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

func main() {
	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to wait for reader completion
	readerDone := make(chan struct{})

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, shutdownSignals...)

	// Create gamepad reader
	reader := gamepad.NewReader()

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

	// Initialize system tray on Windows only
	if runtime.GOOS == "windows" {
		go func() {
			t := tray.New(func() {
				close(shutdownRequested)
			})
			t.Run(tray.GetIcon())
		}()
	} else {
		log.Println("Press Ctrl+C to exit")
	}

	// Run reader in goroutine (but SDL main thread handling is inside)
	// Note: reader.Run() must be called from a goroutine with LockOSThread
	// The signal handling will cancel the context, causing reader.Run() to return
	go func() {
		reader.Run(ctx)
		close(readerDone)
	}()

	// Wait for shutdown signal, tray request, or server error
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
