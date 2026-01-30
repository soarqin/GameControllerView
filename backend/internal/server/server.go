package server

import (
	"io/fs"
	"log"
	"net/http"

	"github.com/soar/GameControllerView/backend/internal/hub"
)

type Server struct {
	hub         *hub.Hub
	broadcaster *hub.Broadcaster
	frontendFS  fs.FS
	addr        string
}

func New(h *hub.Hub, b *hub.Broadcaster, frontendFS fs.FS, addr string) *Server {
	return &Server{
		hub:         h,
		broadcaster: b,
		frontendFS:  frontendFS,
		addr:        addr,
	}
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()

	// WebSocket endpoint
	mux.HandleFunc("/ws", handleWebSocket(s.hub, s.broadcaster))

	// Static files (frontend)
	fileServer := http.FileServer(http.FS(s.frontendFS))
	mux.Handle("/", fileServer)

	log.Printf("HTTP server listening on %s", s.addr)
	return http.ListenAndServe(s.addr, mux)
}
