package server

import (
	"context"
	"io/fs"
	"log"
	"net/http"

	"github.com/soar/GameControllerView/backend/internal/gamepad"
	"github.com/soar/GameControllerView/backend/internal/hub"
)

type Server struct {
	hub         *hub.Hub
	broadcaster *hub.Broadcaster
	reader      *gamepad.Reader
	frontendFS  fs.FS
	addr        string
	httpServer  *http.Server
}

func New(h *hub.Hub, b *hub.Broadcaster, r *gamepad.Reader, frontendFS fs.FS, addr string) *Server {
	return &Server{
		hub:         h,
		broadcaster: b,
		reader:      r,
		frontendFS:  frontendFS,
		addr:        addr,
	}
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()

	// WebSocket endpoint
	mux.HandleFunc("/ws", handleWebSocket(s.hub, s.broadcaster, s.reader))

	// Static files (frontend)
	fileServer := http.FileServer(http.FS(s.frontendFS))
	mux.Handle("/", fileServer)

	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	log.Printf("HTTP server listening on %s", s.addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		log.Println("Shutting down HTTP server...")
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
