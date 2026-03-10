# InputView

Real-time gamepad, keyboard, and mouse input visualizer. Go backend reads input and pushes state to a browser frontend via WebSocket for live Canvas rendering.

## Features

- **Gamepad visualization** — Xbox, PlayStation, Switch Pro, and 550+ devices via VID/PID mapping
- **Keyboard & Mouse visualization** — Global capture via Windows Raw Input API; works when InputView is in the background or in an OBS browser source
- **Multi-controller support** — Multiple browser tabs, each showing a different controller
- **Input Overlay support** — Drop-in compatible with [Input Overlay](https://github.com/univrsal/input-overlay) `.json` + `.png` texture atlas presets; all 10 element types supported
- **Keyboard/mouse-only overlays** — Overlays that contain no gamepad elements render immediately without a controller connected
- **Simple / OBS mode** — Transparent background, no UI chrome (`?simple=1`)
- **System tray** — Windows GUI mode with tray icon and quit menu
- **Zero-config binary** — Single executable with embedded frontend assets

## Requirements

- **Windows** — gamepad input uses XInput (built into Windows 8+, `xinput1_4.dll`); no external DLL required
- Windows also required for keyboard/mouse capture (Raw Input API)

## Quick Start

```bash
# Run in dev/console mode (logs visible in terminal)
go run ./cmd/inputview

# Release build — no console window, system tray enabled (Windows)
./build.ps1     # Windows
./build.sh      # Linux/macOS

# Open browser
http://localhost:8080
```

## URL Parameters

| Parameter | Description | Default | Example |
|-----------|-------------|---------|---------|
| `p` | Gamepad number (1-based) | `1` | `?p=2` |
| `simple` | Transparent background, no UI | off | `?simple=1` |
| `alpha` | Gamepad body opacity (0.0–1.0) | `1.0` | `?alpha=0.5` |
| `overlay` | Input Overlay preset name | — | `?overlay=dualsense` |
| `mouse_sens` | Mouse movement sensitivity divisor (lower = more sensitive) | `500` | `?mouse_sens=300` |

### Examples

```
# Default — first connected gamepad
http://localhost:8080/

# Second connected gamepad
http://localhost:8080/?p=2

# Simple mode (transparent background, no UI — for OBS browser source)
http://localhost:8080/?simple=1

# Semi-transparent gamepad
http://localhost:8080/?alpha=0.5

# Input Overlay preset
http://localhost:8080/?overlay=dualsense

# Mouse overlay with increased sensitivity
http://localhost:8080/?overlay=mouse&mouse_sens=300&simple=1

# Combine parameters
http://localhost:8080/?overlay=dualsense&p=2&simple=1
```

### Multi-Controller

Open multiple browser tabs with different `p` values:

```
http://localhost:8080/?p=1
http://localhost:8080/?p=2
```

## Input Overlay Presets

Place preset directories next to the executable:

```
InputView.exe
overlays/
  dualsense/
    dualsense.json
    dualsense.png
  mouse/
    mouse.json
    mouse.png
```

Open `http://localhost:8080/?overlay=dualsense` to use a preset.

Presets that contain only keyboard/mouse element types (types 1, 3, 4, 9) automatically hide the controller status bar and render without waiting for a gamepad connection.

Presets from the [Input Overlay project](https://github.com/univrsal/input-overlay/tree/master/presets) are licensed under **GPL-2.0** and must not be bundled with InputView distributions. See [docs/third-party-licenses.md](docs/third-party-licenses.md).

See [docs/input-overlay-format.md](docs/input-overlay-format.md) for the full format specification.

## GPV Skin Converter

`cmd/gpvskin2overlay` converts [GamepadViewer](https://gamepadviewer.com/) CSS skins to Input Overlay format.

```bash
go build -o gpvskin2overlay.exe ./cmd/gpvskin2overlay
gpvskin2overlay -skin xbox -out overlays/gpv-xbox
# Then open: http://localhost:8080/?overlay=gpv-xbox
```

See [docs/gpvskin2overlay.md](docs/gpvskin2overlay.md) for full usage.

## Project Structure

```
cmd/
  inputview/          # Main binary entry point
  gpvskin2overlay/    # GPV skin converter CLI
internal/
  input/              # KeyMouseState model, scancode mapping (Raw Input → uiohook)
  rawinput/           # Windows Raw Input API reader (keyboard + mouse, global capture)
  gamepad/            # SDL3 joystick reader, VID/PID device mapping table (550+ entries)
  hub/                # WebSocket hub, broadcaster, client management
  server/             # HTTP server, WebSocket upgrade
  tray/               # Windows system tray integration
  gpvskin/            # GPV skin → Input Overlay conversion pipeline
  web/frontend/       # HTML/CSS/JS frontend + gamepad layout configs
overlays/             # External Input Overlay presets (not embedded in binary)
docs/                 # Format specs and guides
```

## Architecture

### Thread Model

```
goroutine: gamepad.Reader.Run(ctx)    ← SDL3 joystick polling (~60Hz)
                                           ↓ chan GamepadState
goroutine: rawinput.Reader.Run(ctx)   ← Windows Raw Input (keyboard + mouse, global)
                                           ↓ chan KeyMouseState (~60Hz)
goroutine: Broadcaster.Run()          ← Computes deltas, broadcasts to WebSocket clients
goroutine: Hub.Run()                  ← WebSocket client registration / unregistration
goroutine: HTTP Server                ← Static files + /ws WebSocket endpoint
```

### WebSocket Protocol

**Server → Client:**
| Type | When sent |
|------|-----------|
| `full` | On connect, every 5s, every 100 deltas |
| `delta` | On gamepad state change |
| `player_selected` | Confirms `select_player` request |
| `km_full` | On `subscribe_km` (current keyboard/mouse snapshot) |
| `km_delta` | On keyboard/mouse state change |

**Client → Server:**
| Type | Purpose |
|------|---------|
| `select_player` | Switch to a different gamepad |
| `subscribe_km` | Subscribe to keyboard/mouse events (sent automatically when overlay contains km elements) |

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/lxzan/gws` | WebSocket server |
| `github.com/klauspost/compress` | Transitive dependency (via gws, permessage-deflate) |
| `fyne.io/systray` | Windows system tray |

## Common Modifications

### Adding a New Controller

1. `internal/gamepad/mapping.go` — add VID/PID → `DeviceMapping` to `knownDevices`
2. `internal/web/frontend/configs/` — add layout JSON
3. `internal/web/frontend/app.js` — add entry to `configMap`

### Changing Poll Rate

`pollDelayNS` in `internal/gamepad/reader.go` (default 16ms ≈ 60Hz).

### Changing Deadzone

`deadzone` in `internal/gamepad/reader.go` (default 0.05); `analogThreshold` in `internal/gamepad/state.go` (default 0.01).

## License

MIT — see [LICENSE](LICENSE)

> **Packaging notice**: Do **not** bundle `overlays/` preset files when distributing InputView. Those files are GPL-2.0 licensed. See [docs/third-party-licenses.md](docs/third-party-licenses.md).
