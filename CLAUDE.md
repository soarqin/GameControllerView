# GameControllerView

Go backend reads gamepad input via SDL3, pushes to frontend via WebSocket, and renders real-time gamepad visualization on Canvas.

## Language Conventions
- Documentation uses markdown format
- All comments are in English

## Build and Run

```bash
cd backend && go run .
# Open browser at http://localhost:8080
```

Requires **SDL3.dll** (>= 3.2.0) in the same directory as the executable or in system PATH. Download from https://github.com/libsdl-org/SDL/releases.

## URL Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `p` | Gamepad number (1-based, default 1) | `?p=2` |
| `simple` | Simple mode (transparent background, no UI) | `?simple=1` |
| `alpha` | Gamepad opacity (0.0-1.0) | `?alpha=0.5` |

Multiple gamepad viewing example:
```bash
# First gamepad
http://localhost:8080/?p=1
# Second gamepad
http://localhost:8080/?p=2
```

## Project Structure

```
backend/
├── main.go                             # Entry: component assembly, signal handling (Ctrl+C)
├── embed.go                            # go:embed embeds frontend/ static files
├── internal/
│   ├── gamepad/
│   │   ├── state.go                    # GamepadState data model (includes PlayerIndex)
│   │   ├── mapping.go                  # Device mapping table (raw axis/button index → semantic names)
│   │   └── reader.go                   # SDL3 Joystick reader, supports multi-gamepad switching
│   ├── hub/
│   │   ├── hub.go                      # WebSocket client management, targeted broadcast
│   │   ├── broadcast.go                # State change → targeted JSON broadcast
│   │   └── message.go                  # WSMessage type definitions
│   └── server/
│       ├── server.go                   # HTTP server, graceful shutdown
│       └── handler.go                  # WebSocket upgrade, client message handling
└── frontend/                           # Frontend static files (embedded via go:embed)
    ├── index.html
    ├── styles.css
    ├── app.js                          # WebSocket client, URL param parsing, Canvas rendering
    └── configs/                        # Gamepad layout JSON configs
        ├── xbox.json
        ├── playstation.json
        ├── playstation5.json
        └── switch_pro.json
```

## Architecture Highlights

### Thread Model

SDL3 must run on the OS main thread. In `main.go`, `reader.Run(ctx)` executes the SDL event loop in a separate goroutine (internally calls `runtime.LockOSThread`), while the main thread waits for signals.

```
goroutine: Reader.Run(ctx)     ← SDL Init → PollEvent + Poll Joystick (~60Hz)
                                   ↓
                            chan GamepadState
                                   ↓
goroutine: Broadcaster.Run()   ← Listen for changes, targeted broadcast to matched clients
                                   ↓
goroutine: Hub.Run()           ← Manage WebSocket client connections
goroutine: HTTP Server         ← Static files + WebSocket endpoint, graceful shutdown
```

### Signal Handling

- Captures `os.Interrupt` (Ctrl+C) and `syscall.SIGTERM`
- Cancels context to stop reader
- Waits for reader to complete cleanup
- Gracefully shuts down HTTP server (5 second timeout)

### Multi-Gamepad Support

Reader maintains a list of connected gamepads (sorted by connection order):
- `joystickOrder`: Gamepad connection order (list of JoystickID)
- `activeID`: JoystickID of the currently active gamepad
- `GetPlayerIndex()`: Get the 1-based number of the current active gamepad
- `SetActiveByPlayerIndex(n)`: Switch to the specified numbered gamepad

### Data Flow

```
Frontend: URL param p=n → Send select_player message on connect
           ↓
Backend: Client.playerIndex = n
           ↓
Backend: Reader.SetActiveByPlayerIndex(n)
           ↓
Reader: Poll specified gamepad → GamepadState{PlayerIndex: n}
           ↓
Broadcaster: BroadcastToPlayer(msg, n)
           ↓
Hub: Only send to clients with playerIndex == n
```

### Using Joystick Low-Level API (Not Gamepad)

Intentionally uses SDL3 Joystick low-level API instead of Gamepad high-level API to avoid conflicts with other applications or games. Joystick API directly reads HID device data, requiring manual maintenance of axis/button index to semantic name mappings (see `mapping.go`).

### WebSocket Message Protocol

**Server → Client:**
- `full`: Complete state snapshot (sent on new client connect, every 5 seconds, and after every 100 deltas)
- `delta`: Only changed fields (regular updates)
- `player_selected`: Confirm gamepad switch success
- All messages include `seq` (incrementing sequence number) and `timestamp` (millisecond timestamp)

**Client → Server:**
- `select_player`: Select gamepad number to listen to

```json
// Client sends
{"type": "select_player", "playerIndex": 2}

// Server responds
{"type": "player_selected", "playerIndex": 2}
```

### Device Mapping System

`mapping.go` matches known devices (Xbox, PlayStation, Switch Pro) via VID/PID, with generic fallback for unknown devices. Mappings define:
- Axis index → stick/trigger correspondence
- Button index → button name correspondence
- Axis value normalization range (sticks -1.0~1.0, triggers 0.0~1.0)
- Whether Y-axis needs inversion

### Frontend Configuration System

`frontend/configs/*.json` defines Canvas drawing layout for each gamepad type (button coordinates, sizes, radii). Frontend automatically loads the corresponding config based on `controllerType` reported by backend.

## Common Modification Guide

### Adding New Gamepad Support

1. `mapping.go`: Add VID/PID → DeviceMapping to `knownDevices` map
2. If button layout differs from existing mappings, create new `DeviceMapping` variable
3. `frontend/configs/`: Add new layout JSON file
4. `frontend/app.js`: Add mapping name → config filename in `configMap`

### Modifying Canvas Rendering

All drawing logic is in `frontend/app.js` in `drawController()` and its sub-functions. Button positions and sizes are controlled by `configs/*.json`, colors by `COLORS` constants.

### Adding New URL Parameters

1. `frontend/app.js`: Parse URL parameters in `init()`
2. Adjust rendering behavior based on parameters (e.g., `simpleMode`, `bodyAlpha`)

### Modifying Poll Frequency

`pollDelayNS` constant in `reader.go` (currently 16ms ≈ 60Hz).

### Modifying Deadzone

`deadzone` constant in `reader.go` (currently 0.05), `analogThreshold` constant in `state.go` (currently 0.01, used for delta comparison).

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/jupiterrider/purego-sdl3` | CGo-free SDL3 Go bindings |
| `github.com/gorilla/websocket` | WebSocket server |
| `github.com/ebitengine/purego` | Transitive dependency, FFI base for purego-sdl3 |
