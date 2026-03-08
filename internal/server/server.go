package server

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/soar/gamecontrollerview/internal/gamepad"
	"github.com/soar/gamecontrollerview/internal/hub"
)

type Server struct {
	hub         *hub.Hub
	broadcaster *hub.Broadcaster
	reader      *gamepad.Reader
	frontendFS  fs.FS
	exeDir      string
	addr        string
	httpServer  *http.Server
}

func New(h *hub.Hub, b *hub.Broadcaster, r *gamepad.Reader, frontendFS fs.FS, exeDir string, addr string) *Server {
	return &Server{
		hub:         h,
		broadcaster: b,
		reader:      r,
		frontendFS:  frontendFS,
		exeDir:      exeDir,
		addr:        addr,
	}
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()

	// WebSocket endpoint
	mux.HandleFunc("/ws", handleWebSocket(s.hub, s.broadcaster, s.reader))

	// External overlays directory (next to the executable): /overlays/
	// This takes priority over the embedded overlays so users can override or add configs.
	overlaysDir := filepath.Join(s.exeDir, "overlays")
	if info, err := os.Stat(overlaysDir); err == nil && info.IsDir() {
		log.Printf("Serving external overlays from: %s", overlaysDir)
		mux.Handle("/overlays/", http.StripPrefix("/overlays/", http.FileServer(http.Dir(overlaysDir))))
	}

	// Static files (frontend, includes embedded overlays/ under frontend/overlays/)
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
