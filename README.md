# GameControllerView

A real-time game controller visualization tool that reads gamepad input via SDL3 and streams it to a web browser using WebSocket for live Canvas rendering.

## Features

- **Real-time Input Visualization**: See your gamepad inputs rendered live in the browser
- **WebSocket Streaming**: Low-latency delta updates for smooth performance
- **Multi-Controller Support**: Pre-configured layouts for Xbox, PlayStation, Switch Pro controllers
- **Generic Fallback**: Automatic detection for unknown controllers
- **Zero-Config Binary**: Single executable with embedded frontend assets
- **Input Overlay Support**: Texture-atlas based rendering using [Input Overlay](https://github.com/univrsal/input-overlay) preset format (`?overlay=<name>`)

## Requirements

- **Go**: 1.25.6 or higher
- **SDL3.dll**: Version 3.2.0 or higher
  - Download from [SDL Releases](https://github.com/libsdl-org/SDL/releases)
  - Place in the same directory as the executable or in system PATH

## Quick Start

```bash
# Clone the repository
git clone https://github.com/soarqin/GameControllerView.git
cd GameControllerView

# Install dependencies
go mod download

# Run in dev/console mode (logs visible in terminal)
go run ./cmd/gamecontrollerview

# Or build a release binary (no console window, system tray on Windows)
./build.ps1   # Windows
./build.sh    # Linux/macOS

# Open browser to http://localhost:8080
```

## URL Parameters

The frontend supports the following URL parameters to customize the display:

| Parameter | Description | Example |
|-----------|-------------|---------|
| `p` | Player index (1-based) to select which controller to display. Defaults to `1` (first connected controller). | `?p=2` for the second controller |
| `simple` | Enable simple mode with transparent background and no UI elements. Set to `1` to enable. | `?simple=1` |
| `alpha` | Controller body transparency (0.0 to 1.0). Lower values make the body more transparent. | `?alpha=0.5` |
| `overlay` | Input Overlay preset name. Enables texture-atlas renderer instead of the built-in geometric renderer. | `?overlay=dualsense` |

### Examples

```
# Display the first controller (default)
http://localhost:8080/

# Display the second connected controller
http://localhost:8080/?p=2

# Simple mode (transparent background, no UI)
http://localhost:8080/?simple=1

# Semi-transparent controller with 50% opacity
http://localhost:8080/?alpha=0.5

# Combine multiple parameters
http://localhost:8080/?p=2&simple=1&alpha=0.3

# Use an Input Overlay preset (place files in overlays/dualsense/ next to the exe)
http://localhost:8080/?overlay=dualsense

# Input Overlay preset with player 2, simple mode
http://localhost:8080/?overlay=dualsense&p=2&simple=1
```

### Multi-Controller Setup

To view multiple controllers simultaneously, open multiple browser windows/tabs with different `p` values:

```
http://localhost:8080/?p=1   # First controller
http://localhost:8080/?p=2   # Second controller
http://localhost:8080/?p=3   # Third controller
```

## Project Structure

```
GameControllerView/
├── go.mod                              # module github.com/soar/gamecontrollerview
├── go.sum
├── build.ps1                           # Windows release build script (-tags release, -H=windowsgui)
├── build.sh                            # Linux/macOS release build script (-tags release)
├── docs/
│   ├── input-overlay-format.md        # Input Overlay config format specification
│   └── third-party-licenses.md        # Third-party license notices
├── cmd/
│   └── gamecontrollerview/
│       ├── main.go                     # Entry point: component assembly, signal handling
│       ├── winres/                     # Windows resource definitions (icon, manifest)
│       └── rsrc_windows_amd64.syso     # Compiled Windows resource object
└── internal/
    ├── console/                        # Cross-platform console detection & Windows Ctrl+C handler
    ├── gamepad/
    │   ├── state.go                    # GamepadState data model, DeltaChanges, ComputeDelta
    │   ├── mapping.go                  # Device mapping (raw axis/button indices → semantic names)
    │   ├── mapping_table.go            # VID/PID mapping table (550+ entries)
    │   └── reader.go                   # SDL3 Joystick reader (event + polling hybrid loop)
    ├── hub/
    │   ├── hub.go                      # WebSocket client management (register/unregister/broadcast)
    │   ├── client.go                   # WebSocket client (read/write pumps)
    │   ├── broadcast.go                # State changes → JSON broadcast (delta + periodic full sync)
    │   └── message.go                  # WSMessage type definitions (full/delta/player_selected)
    ├── server/
    │   ├── server.go                   # HTTP server, routing (/ static files, /ws WebSocket)
    │   └── handler.go                  # WebSocket upgrade handling
    ├── tray/                           # Windows system tray integration
    └── web/
        ├── embed.go                    # go:embed frontend static files, exports FrontendFS()
        └── frontend/                   # Frontend static files (embedded at build time)
            ├── index.html
            ├── styles.css
            ├── app.js                  # WebSocket client + state management + Canvas rendering
            └── configs/                # Controller layout JSON configs
                ├── xbox.json
                ├── playstation.json
                ├── playstation5.json
                └── switch_pro.json
```

### Input Overlay Presets (external, not included)

Input Overlay presets (`.json` + `.png` texture atlas pairs) are placed in an `overlays/` directory **next to the executable** at runtime. They are **not** embedded in the binary and **not** distributed with GameControllerView releases.

```
overlays/              ← place next to GameControllerView.exe
├── dualsense/
│   ├── dualsense.json
│   └── dualsense.png
└── xbox-one-controller/
    ├── xbox-one-controller.json
    └── xbox-one-controller.png
```

Presets from the [Input Overlay project](https://github.com/univrsal/input-overlay/tree/master/presets) are licensed under **GPL-2.0** and must not be bundled with GameControllerView distributions. See [docs/third-party-licenses.md](docs/third-party-licenses.md) for details.

### Converting GamepadViewer Skins

The included `gpvskin2overlay` tool converts [GamepadViewer](https://gamepadviewer.com/) CSS skins into Input Overlay format. See **[docs/gpvskin2overlay.md](docs/gpvskin2overlay.md)** for build and usage instructions.

```bash
go build -o gpvskin2overlay.exe ./cmd/gpvskin2overlay
gpvskin2overlay -skin xbox -out overlays/gpv-xbox
# Then open: http://localhost:8080/?overlay=gpv-xbox
```

## Architecture

### Thread Model

SDL3 must run on the OS main thread. `reader.Run(ctx)` is executed in a goroutine that calls `runtime.LockOSThread`, while the Hub and HTTP server run in independent goroutines.

```
goroutine: Reader.Run(ctx)     ← SDL Init → Callback → PollEvent + Joystick Polling (~60Hz)
                                   ↓
                            chan GamepadState
                                   ↓
goroutine: Broadcaster.Run()   ← Listens for changes, broadcasts to matched clients
goroutine: Hub.Run()           ← Manages WebSocket client connections
goroutine: HTTP Server         ← Static files + WebSocket endpoint
```

### Data Flow

`Reader` (SDL polling) → `chan GamepadState` → `Broadcaster` → `Hub.BroadcastToPlayer()` → WebSocket clients

### Joystick Low-Level API (Not Gamepad)

Intentionally uses SDL3 Joystick low-level API instead of Gamepad high-level API to avoid conflicts with other applications or games. The Joystick API reads HID device data directly and requires manual maintenance of axis/button index to semantic name mappings (see `mapping.go`).

### WebSocket Message Protocol

**Server → Client:**
- `full`: Complete state snapshot (sent on new client connection, every 5 seconds, and after every 100 delta messages)
- `delta`: Only changed fields (normal updates)
- `player_selected`: Confirms gamepad switch
- All messages include `seq` (incrementing sequence number) and `timestamp` (millisecond timestamp)

**Client → Server:**
- `select_player`: Select which gamepad to listen to

### Device Mapping System

`mapping.go` matches known devices (Xbox, PlayStation, Switch Pro) via VID/PID, with generic fallback for unknown devices. Mappings define:
- Axis indices → stick/trigger correspondence
- Button indices → button name correspondence
- Axis value normalization ranges (sticks -1.0~1.0, triggers 0.0~1.0)
- Whether Y-axis needs inversion

### Frontend Configuration System

`internal/web/frontend/configs/*.json` defines Canvas drawing layouts for each controller (button coordinates, sizes, radii). The frontend automatically loads the corresponding config based on the `controllerType` reported by the backend.

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/jupiterrider/purego-sdl3` | CGo-free SDL3 Go bindings |
| `github.com/gorilla/websocket` | WebSocket server |
| `github.com/ebitengine/purego` | Indirect dependency, FFI base for purego-sdl3 |
| `fyne.io/systray` | Windows system tray integration |

## Common Modifications

### Adding Support for New Controllers

1. `internal/gamepad/mapping.go`: Add VID/PID → DeviceMapping to the `knownDevices` map
2. If button layout differs from existing mappings, create a new `DeviceMapping` variable
3. `internal/web/frontend/configs/`: Add a new layout JSON file
4. `internal/web/frontend/app.js`: Add mapping name → config filename to `configMap`

### Modifying Canvas Rendering

All drawing logic is in `internal/web/frontend/app.js` in the `drawController()` function and its sub-functions. Button positions and sizes are controlled by `configs/*.json`, colors are controlled by the `COLORS` constant.

### Changing Polling Frequency

The `pollDelayNS` constant in `internal/gamepad/reader.go` (currently 16ms ≈ 60Hz).

### Changing Deadzone

The `deadzone` constant in `internal/gamepad/reader.go` (currently 0.05), and the `analogThreshold` constant in `internal/gamepad/state.go` (currently 0.01, used for delta comparison).

## Input Overlay Format

See [docs/input-overlay-format.md](docs/input-overlay-format.md) for the full config format specification, including all element types, sprite layout conventions, and instructions for creating custom presets.

## GPV Skin Converter

See [docs/gpvskin2overlay.md](docs/gpvskin2overlay.md) for build and usage instructions for the `gpvskin2overlay` tool, which converts GamepadViewer CSS skins into Input Overlay format.

## License

MIT License — see [LICENSE](LICENSE)

### Third-Party

Input Overlay preset files (`.json` / `.png`) are licensed under **GPL-2.0**. They are **not** included in this repository or in GameControllerView releases. See [docs/third-party-licenses.md](docs/third-party-licenses.md).

> **Packaging notice**: Do **NOT** bundle `overlays/` preset files when distributing GameControllerView. Distributing GPL-2.0 files alongside MIT-licensed software without GPL compliance is a license violation.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
