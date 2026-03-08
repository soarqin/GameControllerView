package gpvskin

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG decoder
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ImageCache downloads and optionally rasterizes images, caching results by URL.
type ImageCache struct {
	// CacheDir is a directory for storing downloaded/rasterized files.
	CacheDir string
	// SVGTool is the path to rsvg-convert or inkscape. Empty = auto-detect.
	SVGTool string
	// Scale is the rasterization scale factor (default 1.0).
	Scale float64

	cached map[string]image.Image
}

// NewImageCache creates an ImageCache with a temp directory.
func NewImageCache(cacheDir, svgTool string, scale float64) (*ImageCache, error) {
	if cacheDir == "" {
		dir, err := os.MkdirTemp("", "gpvskin-*")
		if err != nil {
			return nil, fmt.Errorf("create image cache dir: %w", err)
		}
		cacheDir = dir
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir %s: %w", cacheDir, err)
	}
	if scale <= 0 {
		scale = 1.0
	}
	return &ImageCache{
		CacheDir: cacheDir,
		SVGTool:  svgTool,
		Scale:    scale,
		cached:   make(map[string]image.Image),
	}, nil
}

// Get returns the decoded image for the given URL or file path.
// SVG files are rasterized to PNG first. Results are cached in memory.
func (c *ImageCache) Get(rawURL string) (image.Image, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("empty image URL")
	}
	if img, ok := c.cached[rawURL]; ok {
		return img, nil
	}

	localPath, err := c.fetchRaw(rawURL)
	if err != nil {
		return nil, err
	}

	// Rasterize SVG if needed.
	if isSVG(localPath) {
		localPath, err = c.rasterizeSVG(localPath)
		if err != nil {
			return nil, err
		}
	}

	img, err := decodeImage(localPath)
	if err != nil {
		return nil, fmt.Errorf("decode image %s: %w", rawURL, err)
	}
	c.cached[rawURL] = img
	return img, nil
}

// fetchRaw downloads or copies a file to the cache directory and returns the local path.
func (c *ImageCache) fetchRaw(rawURL string) (string, error) {
	// Determine a stable cache filename from the URL.
	baseName := sanitizeFilename(rawURL)
	localPath := filepath.Join(c.CacheDir, baseName)

	if _, err := os.Stat(localPath); err == nil {
		return localPath, nil // already cached
	}

	if isURL(rawURL) {
		if err := downloadFile(rawURL, localPath); err != nil {
			return "", err
		}
	} else if strings.HasPrefix(rawURL, "file://") {
		fsPath := strings.TrimPrefix(rawURL, "file://")
		if err := copyFile(fsPath, localPath); err != nil {
			return "", fmt.Errorf("copy local file %s: %w", fsPath, err)
		}
	} else {
		// Plain filesystem path.
		if err := copyFile(rawURL, localPath); err != nil {
			return "", fmt.Errorf("copy file %s: %w", rawURL, err)
		}
	}
	return localPath, nil
}

// rasterizeSVG converts an SVG or SVGZ file to PNG using rsvg-convert or inkscape.
// Returns the path to the output PNG.
func (c *ImageCache) rasterizeSVG(svgPath string) (string, error) {
	// Decompress .svgz to plain .svg before rasterizing.
	if strings.ToLower(filepath.Ext(svgPath)) == ".svgz" {
		decompressed, err := decompressSVGZ(svgPath)
		if err != nil {
			return "", err
		}
		svgPath = decompressed
	}

	outPath := svgPath + ".png"
	if _, err := os.Stat(outPath); err == nil {
		return outPath, nil // already rasterized
	}

	tool := c.SVGTool
	if tool == "" {
		tool = detectSVGTool()
	}
	if tool == "" {
		return "", fmt.Errorf("no SVG rasterization tool found; install rsvg-convert or inkscape")
	}

	var cmd *exec.Cmd
	toolBase := strings.ToLower(filepath.Base(tool))
	switch {
	case strings.Contains(toolBase, "rsvg"):
		// rsvg-convert -z <scale> -f png -o out.png in.svg
		zoomStr := fmt.Sprintf("%.2f", c.Scale)
		cmd = exec.Command(tool, "-z", zoomStr, "-f", "png", "-o", outPath, svgPath)
	case strings.Contains(toolBase, "inkscape"):
		// inkscape --export-type=png --export-filename=out.png in.svg
		cmd = exec.Command(tool,
			"--export-type=png",
			fmt.Sprintf("--export-filename=%s", outPath),
			svgPath,
		)
	default:
		return "", fmt.Errorf("unknown SVG tool %q; expected rsvg-convert or inkscape", tool)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("SVG rasterization failed (%s): %w\n%s", tool, err, string(out))
	}
	return outPath, nil
}

// detectSVGTool searches PATH for rsvg-convert or inkscape.
func detectSVGTool() string {
	for _, name := range []string{"rsvg-convert", "rsvg-convert.exe", "inkscape", "inkscape.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

// isSVG reports whether a file is an SVG or SVGZ based on its extension.
func isSVG(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".svg" || ext == ".svgz"
}

// decompressSVGZ converts a .svgz file to a plain .svg file.
// It first tries gzip decompression; if the file is not actually gzip-compressed
// (some servers serve plain SVG with a .svgz extension), it copies the file as-is.
// Returns the path to the plain .svg file.
func decompressSVGZ(svgzPath string) (string, error) {
	outPath := strings.TrimSuffix(svgzPath, filepath.Ext(svgzPath)) + ".svg"
	if _, err := os.Stat(outPath); err == nil {
		return outPath, nil // already decompressed
	}

	data, err := os.ReadFile(svgzPath)
	if err != nil {
		return "", fmt.Errorf("read svgz %s: %w", svgzPath, err)
	}

	var svgData []byte
	// gzip magic number: 0x1f 0x8b
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return "", fmt.Errorf("gzip open %s: %w", svgzPath, err)
		}
		defer gz.Close()
		svgData, err = io.ReadAll(gz)
		if err != nil {
			return "", fmt.Errorf("gzip decompress %s: %w", svgzPath, err)
		}
	} else {
		// Not actually gzip — server returned plain SVG with .svgz extension.
		svgData = data
	}

	if err := os.WriteFile(outPath, svgData, 0644); err != nil {
		return "", fmt.Errorf("write svg %s: %w", outPath, err)
	}
	return outPath, nil
}

// sanitizeFilename turns a URL into a safe filename.
func sanitizeFilename(rawURL string) string {
	// Replace everything that's not alphanumeric, dash, dot, or underscore.
	var sb strings.Builder
	for _, ch := range rawURL {
		switch {
		case ch >= 'a' && ch <= 'z',
			ch >= 'A' && ch <= 'Z',
			ch >= '0' && ch <= '9',
			ch == '-', ch == '_', ch == '.':
			sb.WriteRune(ch)
		default:
			sb.WriteRune('_')
		}
	}
	name := sb.String()
	// Trim leading underscores from protocol slashes.
	name = strings.TrimLeft(name, "_")
	if len(name) > 200 {
		name = name[len(name)-200:]
	}
	if name == "" {
		name = "image"
	}
	return name
}

// downloadFile downloads a URL to a local file path.
func downloadFile(url, dest string) error {
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file %s: %w", dest, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	return nil
}

// copyFile copies src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// decodeImage reads a PNG or JPEG from disk.
func decodeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// SavePNG writes an image to a PNG file.
func SavePNG(img image.Image, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create PNG %s: %w", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encode PNG %s: %w", path, err)
	}
	return nil
}
