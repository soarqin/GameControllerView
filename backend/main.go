package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/soar/GameControllerView/backend/internal/gamepad"
	"github.com/soar/GameControllerView/backend/internal/hub"
	"github.com/soar/GameControllerView/backend/internal/server"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

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
	srv := server.New(h, broadcaster, frontendFS, ":8080")
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("HTTP server error: %v", err)
			cancel()
		}
	}()

	log.Println("GameControllerView started: http://localhost:8080")

	// Run SDL Joystick polling loop on main thread (blocking)
	reader.Run(ctx)
}
