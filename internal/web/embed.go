package web

import (
	"bytes"
	"compress/gzip"
	"embed"
	"io"
	"io/fs"
	"log"
	"path"
	"strings"
	"testing/fstest"
	"time"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
	mjson "github.com/tdewolff/minify/v2/json"
)

//go:embed all:frontend
var frontendFiles embed.FS

// minifiedFS is a pre-processed in-memory filesystem with minified content.
// gzipCache maps file paths (e.g. "app.js") to their pre-compressed gzip bytes.
var (
	minifiedFS fs.FS
	gzipCache  map[string][]byte
)

func init() {
	minifiedFS, gzipCache = buildMinifiedFS()
}

// FrontendFS returns the minified in-memory filesystem rooted at the frontend directory.
func FrontendFS() fs.FS {
	return minifiedFS
}

// GzipCache returns a map of file path → pre-gzip-compressed bytes for eligible files.
// Keys are slash-separated paths relative to the frontend root (e.g. "app.js", "configs/xbox.json").
func GzipCache() map[string][]byte {
	return gzipCache
}

// minifyMediaType maps file extensions to their minify media type.
// Files with extensions not in this map are copied verbatim.
var minifyMediaType = map[string]string{
	".js":   "application/javascript",
	".css":  "text/css",
	".html": "text/html",
	".json": "application/json",
}

// gzipMinSize is the minimum size (bytes) of minified content to include in gzip cache.
// Files smaller than this threshold are too small to benefit from compression overhead.
const gzipMinSize = 150

func buildMinifiedFS() (fs.FS, map[string][]byte) {
	m := minify.New()
	m.AddFunc("text/html", html.Minify)
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("application/javascript", js.Minify)
	m.AddFunc("application/json", mjson.Minify)

	// Use program startup time as the ModTime for all synthetic entries.
	modTime := time.Now()

	mapFS := make(fstest.MapFS)
	gz := make(map[string][]byte)

	// Walk the embedded frontend/ directory.
	err := fs.WalkDir(frontendFiles, "frontend", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Strip the "frontend/" prefix to get the key used in the served filesystem.
		key := strings.TrimPrefix(p, "frontend/")

		raw, err := frontendFiles.ReadFile(p)
		if err != nil {
			return err
		}

		ext := strings.ToLower(path.Ext(key))
		mediaType, shouldMinify := minifyMediaType[ext]

		var content []byte
		if shouldMinify {
			minified, merr := m.String(mediaType, string(raw))
			if merr != nil {
				log.Printf("web: minify %s: %v (using original)", key, merr)
				content = raw
			} else {
				content = []byte(minified)
			}
		} else {
			content = raw
		}

		mapFS[key] = &fstest.MapFile{
			Data:    content,
			Mode:    0444,
			ModTime: modTime,
		}

		// Pre-compress files that are large enough to benefit.
		if shouldMinify && len(content) >= gzipMinSize {
			var buf bytes.Buffer
			w, werr := gzip.NewWriterLevel(&buf, gzip.BestCompression)
			if werr != nil {
				log.Printf("web: gzip init %s: %v", key, werr)
			} else {
				if _, werr = io.Copy(w, bytes.NewReader(content)); werr == nil {
					werr = w.Close()
				}
				if werr != nil {
					log.Printf("web: gzip compress %s: %v", key, werr)
				} else {
					gz[key] = buf.Bytes()
				}
			}
		}

		return nil
	})

	if err != nil {
		// This should never happen with a valid embed.FS; panic so the bug is visible.
		panic("web: failed to build minified filesystem: " + err.Error())
	}

	// Log size summary for the most important files.
	logSizes(mapFS, gz)

	return mapFS, gz
}

// logSizes emits a one-time startup log showing original vs. minified vs. gzipped sizes.
func logSizes(mapFS fstest.MapFS, gz map[string][]byte) {
	type entry struct {
		key  string
		orig int
	}
	// Walk original embed to get raw sizes.
	origSizes := make(map[string]int)
	_ = fs.WalkDir(frontendFiles, "frontend", func(p string, d fs.DirEntry, _ error) error {
		if d.IsDir() {
			return nil
		}
		if raw, err := frontendFiles.ReadFile(p); err == nil {
			origSizes[strings.TrimPrefix(p, "frontend/")] = len(raw)
		}
		return nil
	})

	var totalOrig, totalMin, totalGz int
	for key, f := range mapFS {
		orig := origSizes[key]
		min := len(f.Data)
		gz := len(gz[key]) // 0 if not gzipped
		totalOrig += orig
		totalMin += min
		totalGz += gz
		if orig > 0 && orig != min {
			log.Printf("web: %-40s  orig=%5d  min=%5d  gz=%5d bytes", key, orig, min, gz)
		}
	}
	log.Printf("web: total frontend  orig=%d  min=%d  gz=%d bytes", totalOrig, totalMin, totalGz)
}
