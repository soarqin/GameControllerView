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

	"github.com/soar/gamecontrollerview/internal/gamepad"
	"github.com/soar/gamecontrollerview/internal/hub"
	"github.com/soar/gamecontrollerview/internal/server"
	"github.com/soar/gamecontrollerview/internal/web"
)

func main() {
	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create gamepad reader
	reader := gamepad.NewReader()

	// Set up shutdown handling (console Ctrl+C or system tray, depending on build mode).
	// onSDLInit must be called after SDL initializes (SDL may override console handlers).
	extraShutdownCh, onSDLInit := setupShutdown(reader)
	reader.SetOnSDLInitCallback(onSDLInit)

	// Handle OS signals (Ctrl+C on Unix, SIGTERM everywhere)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Create and start hub
	h := hub.NewHub()
	go h.Run()

	// Create broadcaster
	broadcaster := hub.NewBroadcaster(h, reader.Changes())
	go broadcaster.Run()

	// Determine the directory containing this executable.
	// Used for locating the external overlays/ directory next to the binary.
	appExeDir := "."
	if exe, err := os.Executable(); err == nil {
		appExeDir = filepath.Dir(exe)
	} else {
		log.Printf("Warning: could not determine executable path: %v", err)
	}

	// Create and start HTTP server
	srv := server.New(h, broadcaster, reader, web.FrontendFS(), appExeDir, ":8080")
	serverErrCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	log.Println("GameControllerView started: http://localhost:8080")

	// Run reader in a goroutine (SDL must run with LockOSThread internally)
	readerDone := make(chan struct{})
	go func() {
		reader.Run(ctx)
		close(readerDone)
	}()

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

	log.Println("GameControllerView stopped")
}
