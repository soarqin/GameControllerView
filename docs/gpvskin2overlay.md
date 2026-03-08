# gpvskin2overlay — GPV Skin Converter

`gpvskin2overlay` converts [GamepadViewer](https://gamepadviewer.com/) CSS skins into [Input Overlay](https://github.com/univrsal/input-overlay) format (JSON config + PNG texture atlas) for use with GameControllerView.

## Prerequisites

SVG rasterization requires one of:
- **rsvg-convert** (from `librsvg`) — recommended, faster
- **inkscape** — alternative

Either tool must be in your system PATH (or specified via `-svg-tool`).

## Build

```bash
go build -o gpvskin2overlay.exe ./cmd/gpvskin2overlay
```

## Usage

```bash
# Convert a built-in skin (fetches CSS from gamepadviewer.com)
gpvskin2overlay -skin xbox -out overlays/gpv-xbox
gpvskin2overlay -skin ds4 -variant white -out overlays/gpv-ds4-white

# Convert from a custom CSS URL (must specify the matching -skin class)
gpvskin2overlay -css https://example.com/xbox/style.css -skin xbox -out overlays/my-xbox

# Convert from a local CSS file
gpvskin2overlay -css ./my-skin.css -skin xbox -out overlays/my-skin

# List all supported skin names
gpvskin2overlay -list

# Scale up for higher-DPI output (e.g. 2x)
gpvskin2overlay -skin xbox -scale 2.0 -out overlays/gpv-xbox-2x

# Specify SVG tool explicitly
gpvskin2overlay -skin xbox -svg-tool /usr/bin/rsvg-convert -out overlays/gpv-xbox
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-skin` | *(required)* | Skin class name (see table below) |
| `-css` | `https://gamepadviewer.com/style.css` | CSS source: HTTP/HTTPS URL or local file path |
| `-variant` | — | Optional skin variant (e.g. `white`) |
| `-out` | `overlays/<skin>[-<variant>]` | Output directory |
| `-svg-tool` | *(auto-detect)* | Path to `rsvg-convert` or `inkscape` |
| `-scale` | `1.0` | SVG rasterization scale factor |
| `-list` | — | List all supported skin names and exit |

## Supported Skins

| `-skin` | GPV CSS class | Variants |
|---------|---------------|----------|
| `xbox` | `.xbox` | `white` |
| `xbox-old` | `.xbox-old` | — |
| `ps` | `.ps` | `white` |
| `ds4` | `.ds4` | `white` |
| `nes` | `.nes` | — |
| `gc` | `.gc` | — |
| `n64` | `.n64` | — |
| `fpp` | `.fpp` | — |
| `fight-stick` | `.fight-stick` | — |

**Note**: The `-skin` flag must match the CSS class used in your CSS file. If you are converting a custom CSS that uses `.xbox` selectors, use `-skin xbox`.

## Output

The tool writes two files to the output directory:

```
overlays/gpv-xbox/
├── gpv-xbox.json    # Input Overlay config
└── gpv-xbox.png     # Texture atlas (sprite sheet)
```

Place the output directory next to `GameControllerView.exe` under `overlays/`, then open:

```
http://localhost:8080/?overlay=gpv-xbox
```

## Conversion Pipeline

1. Load CSS (HTTP URL or local file)
2. Resolve each element's position and image URL from CSS selectors
3. Download image assets; rasterize SVG/SVGZ → PNG via external tool
4. Crop individual sprites from source images
5. Pack all sprites into one texture atlas PNG (Input Overlay layout conventions)
6. Write `<name>.json` + `<name>.png` to the output directory

## Known Limitations

- **Guide/Home/PS button**: GamepadViewer cannot detect the guide button via browser APIs, so it is not mapped in any skin.
- **Custom CSS**: Must match an existing skin class (e.g. a CSS file using `.xbox` selectors requires `-skin xbox`).
- **SVGZ**: Some servers return plain SVG data with a `.svgz` extension. The tool handles this transparently by checking the gzip magic bytes before decompression.
