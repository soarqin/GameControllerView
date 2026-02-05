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
├── main.go                             # Entry: component assembly, signal handling
├── embed.go                            # go:embed embeds frontend/ static files
├── internal/
│   ├── console/
│   │   └── console.go                  # Cross-platform console detection & Windows Ctrl+C handler (reusable)
│   ├── gamepad/
│   │   ├── state.go                    # GamepadState data model (includes PlayerIndex)
│   │   ├── mapping.go                  # Device mapping table (raw axis/button index → semantic names)
│   │   └── reader.go                   # SDL3 Joystick reader, supports multi-gamepad switching, SDL init callback
│   ├── hub/
│   │   ├── hub.go                      # WebSocket client management, targeted broadcast
│   │   ├── broadcast.go                # State change → targeted JSON broadcast
│   │   └── message.go                  # WSMessage type definitions
│   ├── server/
│   │   ├── server.go                   # HTTP server, graceful shutdown
│   │   └── handler.go                  # WebSocket upgrade, client message handling
│   └── tray/
│       └── tray.go                     # Windows system tray integration
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
goroutine: Reader.Run(ctx)     ← SDL Init → Callback (re-register Windows handler) → PollEvent + Poll Joystick (~60Hz)
                                   ↓
                            chan GamepadState
                                   ↓
goroutine: Broadcaster.Run()   ← Listen for changes, targeted broadcast to matched clients
                                   ↓
goroutine: Hub.Run()           ← Manage WebSocket client connections
goroutine: HTTP Server         ← Static files + WebSocket endpoint, graceful shutdown
```

**SDL Initialization Callback**: On Windows, a callback is invoked after SDL initialization to re-register the console control handler (SDL3 overrides it during init). Use `reader.SetOnSDLInitCallback(func())` to set platform-specific post-init behavior.

### Signal Handling

- Captures `os.Interrupt` (Ctrl+C) and `syscall.SIGTERM`
- **Windows**: Uses `SetConsoleCtrlHandler` API via `console.SetupConsoleHandler()` because SDL3 with `LockOSThread()` interferes with Go's standard signal handling
  - Handler is re-registered after SDL initialization (SDL3 overrides console handlers during init)
  - Supports both Ctrl+C and Ctrl+Break
  - Uses atomic operations to prevent panic from rapid key presses
- **Unix/Linux**: Uses Go's standard `os.Interrupt` signal handling
- **Console Detection**: `console.IsRunningFromConsole()` intelligently handles console allocation
  - **Console-mode build + terminal**: Reuses existing console
  - **Console-mode build + double-click**: Frees auto-created console (GUI mode)
  - **GUI-mode build + terminal**: Creates independent console window + redirects stdout/stderr/stdin
  - **GUI-mode build + double-click**: No console (pure GUI mode)
  - Console mode shows "Press Ctrl+C or Ctrl+Break to exit" message
  - GUI mode hides console window, uses system tray for exit
- Cancels context to stop reader
- Waits for reader to complete cleanup
- Gracefully shuts down HTTP server (5 second timeout)

### Console Package (Reusable Component)

The `internal/console` package provides cross-platform console detection and Windows Ctrl+C handling that can be reused in other projects:

```go
import "github.com/soar/GameControllerView/backend/internal/console"

// Detect if running from console or GUI mode
if console.IsRunningFromConsole() {
    // Console mode
} else {
    // GUI mode
}

// Set up Windows console handler (works with SDL3, LockOSThread, etc.)
shutdownChan := make(chan struct{})
registerHandler := console.SetupConsoleHandler(shutdownChan)

// Re-register after library initialization (e.g., SDL init)
registerHandler()

// Wait for shutdown
<-shutdownChan
```

**Key Features:**
- Platform-independent: On non-Windows platforms, `SetupConsoleHandler` returns a no-op function
- Smart console allocation: Handles both console-mode and GUI-mode builds correctly
  - Console-mode builds: Frees auto-created console when double-clicked
  - GUI-mode builds: Creates independent console window when launched from terminal (prevents input/output confusion)
- Automatically detects launch method (terminal vs double-click) via parent process check
- Std stream redirection: After console allocation, updates `os.Stdout`, `os.Stderr`, `os.Stdin`, and `log` output
- Re-registration support for libraries that override console handlers during initialization
- Safe for rapid Ctrl+C presses (atomic operations prevent duplicate channel close)

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

### Using SDL Initialization Callback (Platform-Specific Setup)

The `Reader` supports an optional callback that is invoked after SDL initialization completes. This is useful for platform-specific setup that must happen after SDL is initialized (e.g., re-registering Windows console handlers).

```go
reader := gamepad.NewReader()
reader.SetOnSDLInitCallback(func() {
    // Platform-specific setup after SDL initialization
    // Only called if runtime.GOOS matches the platform check
})
```

### Using Console Package in Other Projects

The `internal/console` package is designed to be reusable across projects. To use it in your own project:

1. Copy the `internal/console` directory to your project
2. Import and use the functions:

```go
import "yourproject/internal/console"

func main() {
    // Detect console mode
    if console.IsRunningFromConsole() {
        fmt.Println("Running from terminal - press Ctrl+C to exit")
    } else {
        // GUI mode - hide console, show tray icon, etc.
    }

    // Set up Windows console handler (important for SDL3, LockOSThread, etc.)
    shutdownChan := make(chan struct{})
    registerHandler := console.SetupConsoleHandler(shutdownChan)

    // If using libraries that override console handlers, re-register after init
    // e.g., after sdl.Init()
    registerHandler()

    // Wait for shutdown
    <-shutdownChan
    cleanup()
}
```

**Why use this package:**
- Fixes Ctrl+C not working with SDL3, OpenGL, or other libraries using `runtime.LockOSThread()`
- Smart console allocation handles both console-mode and GUI-mode builds:
  - Console-mode builds: Frees auto-created console when double-clicked
  - GUI-mode builds: Allocates console when launched from terminal
- Automatically detects launch method (terminal vs double-click) via parent process check
- Cross-platform: gracefully degrades to no-op on non-Windows platforms
- Production-tested: handles edge cases like rapid Ctrl+C presses

**Build Mode Examples:**
```bash
# Console-mode build (default)
go build -o myapp.exe
# - From terminal: shows console output
# - Double-clicked: hides console window (GUI mode)

# GUI-mode build (no console window by default)
go build -ldflags "-H windowsgui" -o myapp.exe
# - From terminal: creates new console window + redirects stdout/stderr/stdin
# - Double-clicked: no console (pure GUI)
```

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/jupiterrider/purego-sdl3` | CGo-free SDL3 Go bindings |
| `github.com/gorilla/websocket` | WebSocket server |
| `github.com/ebitengine/purego` | Transitive dependency, FFI base for purego-sdl3 |
