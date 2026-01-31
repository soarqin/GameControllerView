# GameControllerView

A real-time game controller visualization tool that reads gamepad input via SDL3 and streams it to a web browser using WebSocket for live Canvas rendering.

## Features

- **Real-time Input Visualization**: See your gamepad inputs rendered live in the browser
- **WebSocket Streaming**: Low-latency delta updates for smooth performance
- **Multi-Controller Support**: Pre-configured layouts for Xbox, PlayStation, Switch Pro controllers
- **Generic Fallback**: Automatic detection for unknown controllers
- **Zero-Config Binary**: Single executable with embedded frontend assets

## Requirements

- **Go**: 1.25.6 or higher
- **SDL3.dll**: Version 3.2.0 or higher
  - Download from [SDL Releases](https://github.com/libsdl-org/SDL/releases)
  - Place in the same directory as the executable or in system PATH

## Quick Start

```bash
# Clone the repository
git clone https://github.com/soar/GameControllerView.git
cd GameControllerView/backend

# Install dependencies
go mod download

# Run the server
go run .

# Open browser to http://localhost:8080
```

## URL Parameters

The frontend supports the following URL parameters to customize the display:

| Parameter | Description | Example |
|-----------|-------------|---------|
| `p` | Player index (1-based) to select which controller to display. Defaults to `1` (first connected controller). | `?p=2` for the second controller |
| `simple` | Enable simple mode with transparent background and no UI elements. Set to `1` to enable. | `?simple=1` |
| `alpha` | Controller body transparency (0.0 to 1.0). Lower values make the body more transparent. | `?alpha=0.5` |

### Examples

```bash
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
```

### Multi-Controller Setup

To view multiple controllers simultaneously, open multiple browser windows/tabs with different `p` values:

```bash
# First controller
http://localhost:8080/?p=1

# Second controller
http://localhost:8080/?p=2

# Third controller
http://localhost:8080/?p=3
```

## Project Structure

```
backend/
├── main.go                             # Entry point: component assembly, SDL main thread event loop
├── embed.go                            # go:embed frontend static files
├── internal/
│   ├── gamepad/
│   │   ├── state.go                    # GamepadState data model, DeltaChanges, ComputeDelta
│   │   ├── mapping.go                  # Device mapping (raw axis/button indices → semantic names)
│   │   └── reader.go                   # SDL3 Joystick reader (event + polling hybrid loop)
│   ├── hub/
│   │   ├── hub.go                      # WebSocket client management (register/unregister/broadcast)
│   │   ├── broadcast.go                # State changes → JSON broadcast (delta + periodic full sync)
│   │   └── message.go                  # WSMessage type definitions (full/delta/event)
│   └── server/
│       ├── server.go                   # HTTP server, routing (/ static files, /ws WebSocket)
│       └── handler.go                  # WebSocket upgrade handling
└── frontend/                           # Frontend static files (embedded via go:embed)
    ├── index.html
    ├── styles.css
    ├── app.js                          # WebSocket client + state management + Canvas rendering
    └── configs/                        # Controller layout JSON configs
        ├── xbox.json
        ├── playstation.json
        ├── switch_pro.json
        └── generic.json
```

## Architecture

### Thread Model

SDL3 must run on the OS main thread. `main.go` blocks the main thread with `reader.Run(ctx)` executing the SDL event loop, while the Hub and HTTP server run in independent goroutines.

```
Main Thread (runtime.LockOSThread)
├── SDL Init → PollEvent + Joystick Polling (~60Hz)
│
goroutine: Hub.Run()        ← Manages WebSocket client connections
goroutine: Broadcaster.Run() ← Listens to Reader.Changes() channel, broadcasts to Hub
goroutine: HTTP Server       ← Static files + WebSocket endpoint
```

### Data Flow

`Reader` (SDL polling) → `chan GamepadState` → `Broadcaster` → `Hub.Broadcast()` → All WebSocket clients

### Joystick Low-Level API (Not Gamepad)

Intentionally uses SDL3 Joystick low-level API instead of Gamepad high-level API to avoid conflicts with other applications or games. The Joystick API reads HID device data directly and requires manual maintenance of axis/button index to semantic name mappings (see `mapping.go`).

### WebSocket Message Protocol

- `full`: Complete state snapshot (sent on new client connection, every 5 seconds, and after every 100 delta messages)
- `delta`: Only changed fields (normal updates)
- All messages include `seq` (incrementing sequence number) and `timestamp` (millisecond timestamp)

### Device Mapping System

`mapping.go` matches known devices (Xbox, PlayStation, Switch Pro) via VID/PID, with generic fallback for unknown devices. Mappings define:
- Axis indices → stick/trigger correspondence
- Button indices → button name correspondence
- Axis value normalization ranges (sticks -1.0~1.0, triggers 0.0~1.0)
- Whether Y-axis needs inversion

### Frontend Configuration System

`frontend/configs/*.json` defines Canvas drawing layouts for each controller (button coordinates, sizes, radii). The frontend automatically loads the corresponding config based on the `controllerType` reported by the backend.

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/jupiterrider/purego-sdl3` | CGo-free SDL3 Go bindings |
| `github.com/gorilla/websocket` | WebSocket server |
| `github.com/ebitengine/purego` | Indirect dependency, FFI base for purego-sdl3 |

## Common Modifications

### Adding Support for New Controllers

1. `mapping.go`: Add VID/PID → DeviceMapping to the `knownDevices` map
2. If button layout differs from existing mappings, create a new `DeviceMapping` variable
3. `frontend/configs/`: Add a new layout JSON file
4. `frontend/app.js`: Add mapping name → config filename to `configMap`

### Modifying Canvas Rendering

All drawing logic is in `frontend/app.js` in the `drawController()` function and its sub-functions. Button positions and sizes are controlled by `configs/*.json`, colors are controlled by the `COLORS` constant.

### Changing Polling Frequency

The `pollDelayNS` constant in `reader.go` (currently 16ms ≈ 60Hz).

### Changing Deadzone

The `deadzone` constant in `reader.go` (currently 0.05), and the `analogThreshold` constant in `state.go` (currently 0.01, used for delta comparison).

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
