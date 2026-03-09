// Command gpvskin2overlay converts a GamepadViewer CSS skin into Input Overlay
// format (JSON config + PNG texture atlas) that can be used with InputView.
//
// Usage:
//
//	gpvskin2overlay -skin xbox -out overlays/gpv-xbox
//	gpvskin2overlay -skin ds4 -variant white -out overlays/gpv-ds4-white
//	gpvskin2overlay -css https://gamepadviewer.com/style.css -skin xbox -out overlays/gpv-xbox
//	gpvskin2overlay -css ./my-custom.css -skin custom -out overlays/my-custom
//
// The -css flag defaults to https://gamepadviewer.com/style.css.
// The -out flag defaults to overlays/<skin>[-<variant>].
//
// Prerequisites: rsvg-convert or inkscape must be in PATH (for SVG rasterization).
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/soar/inputview/internal/gpvskin"
)

func main() {
	var (
		cssSource = flag.String("css", "https://gamepadviewer.com/style.css",
			"CSS source: HTTP/HTTPS URL or local file path")
		skin = flag.String("skin", "",
			"Skin class name: xbox, xbox-old, ps, ds4, nes, gc, n64, fpp, fight-stick, custom (required)")
		variant = flag.String("variant", "",
			"Optional skin variant (e.g. white)")
		out = flag.String("out", "",
			"Output directory (default: overlays/<skin>[-<variant>])")
		svgTool = flag.String("svg-tool", "",
			"Path to rsvg-convert or inkscape (auto-detected if empty)")
		scale = flag.Float64("scale", 1.0,
			"SVG rasterization scale factor")
		listSkins = flag.Bool("list", false,
			"List all supported skin names and exit")
	)
	flag.Parse()

	if *listSkins {
		fmt.Println("Supported skins:")
		for _, s := range gpvskin.AllSkins {
			line := "  " + s.CSSClass
			if len(s.Variants) > 0 {
				line += fmt.Sprintf("  (variants: %v)", s.Variants)
			}
			fmt.Println(line)
		}
		fmt.Println("  custom  (for external CSS files)")
		os.Exit(0)
	}

	if *skin == "" {
		fmt.Fprintln(os.Stderr, "error: -skin is required")
		flag.Usage()
		os.Exit(1)
	}

	// Determine output directory.
	outDir := *out
	if outDir == "" {
		outDir = "overlays/" + *skin
		if *variant != "" {
			outDir += "-" + *variant
		}
	}
	outDir = filepath.Clean(outDir)

	log.Printf("CSS source : %s", *cssSource)
	log.Printf("Skin       : %s", *skin)
	if *variant != "" {
		log.Printf("Variant    : %s", *variant)
	}
	log.Printf("Output dir : %s", outDir)

	if err := gpvskin.Convert(*cssSource, *skin, *variant, outDir, *svgTool, *scale); err != nil {
		log.Fatalf("Conversion failed: %v", err)
	}

	name := filepath.Base(outDir)
	log.Printf("Done: %s/%s.json + %s/%s.png", outDir, name, outDir, name)
	log.Printf("Use with: http://localhost:8080/?overlay=%s", name)
}
