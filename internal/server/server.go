package server

import (
	"context"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/soar/inputview/internal/gamepad"
	"github.com/soar/inputview/internal/hub"
)

type Server struct {
	hub         *hub.Hub
	broadcaster *hub.Broadcaster
	reader      *gamepad.Reader
	frontendFS  fs.FS
	gzipCache   map[string][]byte
	exeDir      string
	addr        string
	httpServer  *http.Server
}

func New(h *hub.Hub, b *hub.Broadcaster, r *gamepad.Reader, frontendFS fs.FS, gzipCache map[string][]byte, exeDir string, addr string) *Server {
	return &Server{
		hub:         h,
		broadcaster: b,
		reader:      r,
		frontendFS:  frontendFS,
		gzipCache:   gzipCache,
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

	// Static files (frontend) with gzip-aware serving.
	mux.Handle("/", newGzipFileServer(s.frontendFS, s.gzipCache))

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

// newGzipFileServer returns an http.Handler that serves files from fsys.
// For files present in gzipCache, it sends pre-compressed gzip bytes directly
// when the client advertises "gzip" in its Accept-Encoding header, avoiding
// per-request compression overhead. Other requests fall back to the standard
// http.FileServer (which serves the already-minified content from fsys).
func newGzipFileServer(fsys fs.FS, gzipCache map[string][]byte) http.Handler {
	fallback := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only handle GET/HEAD; let fallback deal with anything else.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			fallback.ServeHTTP(w, r)
			return
		}

		// Resolve the URL path to a cache key (strip leading slash, clean).
		urlPath := r.URL.Path
		if urlPath == "/" || urlPath == "" {
			urlPath = "/index.html"
		}
		key := path.Clean(strings.TrimPrefix(urlPath, "/"))

		// Check whether pre-compressed content is available.
		gz, hasGz := gzipCache[key]

		// Only use the gzip cache when the client supports it.
		if !hasGz || !acceptsGzip(r) {
			fallback.ServeHTTP(w, r)
			return
		}

		// Determine Content-Type from the original extension.
		ext := path.Ext(key)
		ct := mime.TypeByExtension(ext)
		if ct == "" {
			ct = "application/octet-stream"
		}

		h := w.Header()
		h.Set("Content-Type", ct)
		h.Set("Content-Encoding", "gzip")
		h.Set("Vary", "Accept-Encoding")
		// Prevent browsers from caching stale gzip responses when the server restarts.
		h.Set("Cache-Control", "no-cache")

		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(gz)
	})
}

// acceptsGzip reports whether the request's Accept-Encoding header includes "gzip".
func acceptsGzip(r *http.Request) bool {
	ae := r.Header.Get("Accept-Encoding")
	for _, part := range strings.Split(ae, ",") {
		token := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if strings.EqualFold(token, "gzip") {
			return true
		}
	}
	return false
}
