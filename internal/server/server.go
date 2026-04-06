package server

import (
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/soar/inputview/internal/gamepad"
	"github.com/soar/inputview/internal/hub"
)

type healthResponse struct {
	Status        string            `json:"status"`
	Version       string            `json:"version"`
	UptimeSeconds int64             `json:"uptime_seconds"`
	Listeners     map[string]string `json:"listeners"`
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

type Server struct {
	hub         *hub.Hub
	broadcaster *hub.Broadcaster
	reader      *gamepad.Reader
	sensSetter  hub.MouseSensitivitySetter
	frontendFS  fs.FS
	gzipCache   map[string][]byte
	exeDir      string
	overlayDir  string
	keyboardDir string
	addr        string
	httpServer  *http.Server
	startTime   time.Time
}

func New(h *hub.Hub, b *hub.Broadcaster, r *gamepad.Reader, sensSetter hub.MouseSensitivitySetter, frontendFS fs.FS, gzipCache map[string][]byte, exeDir string, overlayDir string, keyboardDir string, addr string) *Server {
	return &Server{
		hub:         h,
		broadcaster: b,
		reader:      r,
		sensSetter:  sensSetter,
		frontendFS:  frontendFS,
		gzipCache:   gzipCache,
		exeDir:      exeDir,
		overlayDir:  overlayDir,
		keyboardDir: keyboardDir,
		addr:        addr,
		startTime:   time.Now(),
	}
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := healthResponse{
			Status:        "ok",
			Version:       "0.3.0",
			UptimeSeconds: int64(time.Since(s.startTime).Seconds()),
			Listeners:     map[string]string{"addr": s.addr},
		}
		json.NewEncoder(w).Encode(resp)
	})

	// WebSocket endpoint
	mux.HandleFunc("/ws", handleWebSocket(s.hub, s.broadcaster, s.reader, s.sensSetter))

	// External overlays directory (next to the executable): /overlays/
	// This takes priority over the embedded overlays so users can override or add configs.
	overlaysDir := filepath.Join(s.exeDir, s.overlayDir)
	if info, err := os.Stat(overlaysDir); err == nil && info.IsDir() {
		slog.Info("serving external overlays", "dir", overlaysDir)
		mux.Handle("/overlays/", http.StripPrefix("/overlays/", http.FileServer(http.Dir(overlaysDir))))
	}

	// External keyboards directory (next to the executable): /keyboards/
	keyboardsDir := filepath.Join(s.exeDir, s.keyboardDir)
	if info, err := os.Stat(keyboardsDir); err == nil && info.IsDir() {
		slog.Debug("serving external keyboards", "dir", keyboardsDir)
		mux.Handle("/keyboards/", http.StripPrefix("/keyboards/", http.FileServer(http.Dir(keyboardsDir))))
	}

	// Static files (frontend) with gzip-aware serving.
	mux.Handle("/", newGzipFileServer(s.frontendFS, s.gzipCache))

	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: loggingMiddleware(mux),
	}

	slog.Info("HTTP server listening", "addr", s.addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		slog.Info("shutting down HTTP server")
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

// loggingMiddleware logs HTTP requests to stderr via slog, excluding /ws and /health paths.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip logging for /ws and /health
		if r.URL.Path == "/ws" || r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration", time.Since(start).String(),
			"ip", r.RemoteAddr,
		)
	})
}
