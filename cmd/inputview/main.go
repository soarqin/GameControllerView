package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/soar/inputview/internal/config"
	"github.com/soar/inputview/internal/gamepad"
	"github.com/soar/inputview/internal/hub"
	"github.com/soar/inputview/internal/rawinput"
	"github.com/soar/inputview/internal/server"
	"github.com/soar/inputview/internal/web"
)

func main() {
	// Set up structured logging. Use TextHandler (not JSON) for desktop app.
	slogLevel := &slog.LevelVar{}
	slogLevel.Set(slog.LevelInfo)
	slogHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slogLevel})
	slog.SetDefault(slog.New(slogHandler))

	// Determine the directory containing this executable early (needed for config + SDL DB path).
	appExeDir := "."
	if exe, err := os.Executable(); err == nil {
		appExeDir = filepath.Dir(exe)
	} else {
		slog.Warn("could not determine executable path", "error", err)
	}

	// Load configuration (flags + optional inputview.toml). Must happen before
	// anything else so all settings are available to subsystem setup.
	cfg, err := config.Load(appExeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create gamepad reader and apply config settings.
	reader := gamepad.NewReader()
	reader.SetDeadzone(cfg.Deadzone)
	reader.SetPollDelay(time.Duration(cfg.PollRate) * time.Millisecond)

	// Load SDL GameControllerDB. The embedded database is always used as a base;
	// an external gamecontrollerdb.txt next to the executable (if present) is
	// merged on top so users can update mappings without recompiling.
	sdlDBPath := filepath.Join(appExeDir, cfg.SDLDBPath)
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
	kmReader.SetMouseSensitivity(float32(cfg.MouseSensitivity))

	// Register HID gamepad callbacks on the Raw Input window so that non-XInput
	// controllers (PS4/PS5/Switch Pro/generic HID) are captured alongside XInput.
	// Must be called before kmReader.Run().
	reader.SetRawInputReader(kmReader)

	// Create broadcaster (listens to both gamepad and keyboard/mouse channels)
	broadcaster := hub.NewBroadcaster(h, reader.Changes(), kmReader.Changes())
	go broadcaster.Run()

	// Create and start HTTP server
	srv := server.New(h, broadcaster, reader, kmReader, web.FrontendFS(), web.GzipCache(), appExeDir, cfg.OverlayDir, cfg.KeyboardDir, cfg.Addr)
	serverErrCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	slog.Info("InputView started", "addr", "http://localhost"+cfg.Addr)

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
			slog.Info("shutting down")
		case <-extraShutdownCh:
			slog.Info("shutdown requested")
		case err := <-serverErrCh:
			slog.Error("HTTP server error", "error", err)
		}
	} else {
		select {
		case <-sigCh:
			slog.Info("shutting down")
		case err := <-serverErrCh:
			slog.Error("HTTP server error", "error", err)
		}
	}
	cancel()

	// Wait for reader to finish
	<-readerDone

	// Graceful HTTP server shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	slog.Info("InputView stopped")
}
