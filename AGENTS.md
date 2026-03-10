# InputView

Go backend reads gamepad input via Windows XInput API and keyboard/mouse input via Windows Raw Input API, pushes to frontend via WebSocket, and renders real-time gamepad/keyboard/mouse visualization on Canvas.

## Language Conventions
- Documentation uses markdown format
- All comments are in English
- **Update AGENTS.md**: After completing any task, update AGENTS.md to reflect the changes
  - Document new features, bug fixes, or architectural changes
  - Update relevant sections in this file
  - Keep documentation in sync with code implementation

## Build and Run

```bash
# Dev/console build (default): console window visible, logs go to terminal
go run ./cmd/inputview
go build -o InputView.exe ./cmd/inputview

# Release build: no console window, system tray on Windows
./build.ps1          # Windows (PowerShell)
./build.sh           # Linux/macOS
# Equivalent manual command (Windows):
go build -tags release -ldflags "-s -w -H=windowsgui" -o InputView.exe ./cmd/inputview

# Open browser at http://localhost:8080
```

No external DLL required. Gamepad input uses XInput (`xinput1_4.dll`), which is built into Windows 8+.

## URL Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `p` | Gamepad number (1-based, default 1) | `?p=2` |
| `simple` | Simple mode (transparent background, no UI) | `?simple=1` |
| `alpha` | Gamepad opacity (0.0-1.0) | `?alpha=0.5` |
| `overlay` | Input Overlay config name (enables texture-atlas renderer) | `?overlay=dualsense` |
| `mouse_sens` | Mouse movement sensitivity divisor (default 500; lower = more sensitive) | `?mouse_sens=300` |

## Project Structure

```
InputView/
├── go.mod                              # module github.com/soar/inputview
├── go.sum
├── build.ps1                           # Windows release build script (-tags release, -H=windowsgui)
├── build.sh                            # Linux/macOS release build script (-tags release)
├── docs/
│   ├── input-overlay-format.md        # Input Overlay config format specification
│   ├── gpvskin2overlay.md             # GPV skin converter build & usage guide
│   └── third-party-licenses.md        # Third-party license notices (GPL-2.0 presets)
├── cmd/
│   ├── inputview/
│   │   ├── main.go                     # Entry: component assembly, signal handling
│   │   ├── winres/                     # Windows resource definitions (icon, manifest)
│   │   └── rsrc_windows_amd64.syso     # Compiled Windows resource object
│   └── gpvskin2overlay/
│       └── main.go                     # CLI tool: convert GPV CSS skin → Input Overlay format
└── internal/
    ├── console/
    │   ├── console_windows.go          # Windows console detection & Ctrl+C handler (reusable)
    │   └── console_other.go            # Stub for non-Windows platforms
    ├── input/
    │   ├── state.go                    # KeyMouseState data model, KeyMouseDelta, ComputeKeyMouseDelta()
    │   └── keycode.go                  # Windows Raw Input scancode → uiohook scancode mapping
    ├── rawinput/
    │   ├── rawinput_windows.go         # Windows Raw Input API: global keyboard/mouse capture (HWND_MESSAGE + RIDEV_INPUTSINK)
    │   └── rawinput_other.go           # Stub for non-Windows platforms
    ├── gamepad/
    │   ├── state.go                    # GamepadState data model (includes PlayerIndex)
    │   ├── mapping.go                  # Device mapping types & GetMapping() function
    │   ├── mapping_table.go            # VID/PID device mapping table (550+ entries)
    │   ├── reader.go                   # Reader struct: shared fields, Changes()/GetPlayerIndex()/SetActiveByPlayerIndex()
    │   ├── reader_windows.go           # Windows implementation: XInput polling loop (~60Hz)
    │   ├── reader_other.go             # Non-Windows stub (blocks until ctx cancel)
    │   └── xinput_windows.go           # XInput API bindings (syscall): GetState, GetCapabilitiesEx (ordinal 108)
    ├── hub/
    │   ├── hub.go                      # WebSocket hub: client management, targeted broadcast, main loop
    │   ├── client.go                   # WebSocket client: connection, read/write pumps, message handling
    │   ├── broadcast.go                # State change → targeted JSON broadcast
    │   └── message.go                  # WSMessage type definitions
    ├── server/
    │   ├── server.go                   # HTTP server, graceful shutdown; mounts external overlays/ dir
    │   └── handler.go                  # WebSocket upgrade, client message handling
    ├── tray/
    │   ├── tray.go                     # Windows system tray integration (atomic shutdown flag, non-blocking menu handling)
    │   └── icon.go                     # Embedded tray icon
    ├── gpvskin/
    │   ├── skinmodel.go                # Data types: IOElementType, CSSProperties, SkinElement, etc.
    │   ├── cssparser.go                # CSS loading (HTTP + local), comment stripping, rule parsing
    │   ├── skins.go                    # All 9 GPV skin definitions + CustomSkinDef registry
    │   ├── mapping.go                  # CSS selector resolution + position calculation per element
    │   ├── download.go                 # Image download + SVG/SVGZ→PNG rasterization (rsvg-convert/inkscape)
    │   ├── atlas.go                    # Sprite cropping + IO-convention atlas packing
    │   └── generate.go                 # Input Overlay JSON generation + high-level Convert() pipeline
    └── web/
        ├── embed.go                    # go:embed embeds frontend/ static files, exports FrontendFS()
        └── frontend/                   # Frontend static files
            ├── index.html
            ├── styles.css
            ├── app.js                  # WebSocket client, URL param parsing, Canvas rendering
            └── configs/                # Gamepad layout JSON configs
                ├── xbox.json
                ├── playstation.json
                ├── playstation5.json
                └── switch_pro.json
```

Input Overlay presets are **external only** — place them in an `overlays/` directory next to the executable.
The server mounts this directory at `/overlays/`. Presets are **not** embedded in the binary (GPL-2.0 license conflict).

## Architecture Highlights

### Thread Model

XInput is thread-safe and does not require `LockOSThread`. The gamepad reader runs as a plain goroutine.

```
goroutine: Reader.Run(ctx)     ← XInput polling loop (~60Hz, time.Sleep based)
                                   ↓
                            chan GamepadState
                                   ↓
goroutine: Broadcaster.Run()   ← Listen for changes, targeted broadcast to matched clients
                                   ↓
goroutine: Hub.Run()           ← Manage WebSocket client connections
goroutine: HTTP Server         ← Static files + WebSocket endpoint, graceful shutdown
```

```
goroutine: rawinput.Reader.Run(ctx)  ← Windows Raw Input API (HWND_MESSAGE window + RIDEV_INPUTSINK)
                                            ↓ WM_INPUT events (keyboard + mouse, global capture)
                                     internal state accumulation (mutex-protected)
                                            ↓ ~60Hz emitter goroutine
                                     chan KeyMouseState
                                            ↓
goroutine: Broadcaster.Run()         ← Also listens on kmChanges channel, broadcasts to km-subscribed clients
```

### Signal Handling

- Captures `os.Interrupt` (Ctrl+C) and `syscall.SIGTERM`
- **Windows**: Uses `SetConsoleCtrlHandler` API via `console.SetupConsoleHandler()` because `runtime.LockOSThread()` (used by the Raw Input message loop) can interfere with Go's standard signal handling
  - Supports both Ctrl+C and Ctrl+Break
  - Uses atomic operations to prevent panic from rapid key presses
- **Unix/Linux**: Uses Go's standard `os.Interrupt` signal handling
- **Console Detection**: `console.IsRunningFromConsole()` intelligently handles console allocation
  - **Console-mode build + terminal**: Reuses existing console
  - **Console-mode build + double-click**: Frees auto-created console (GUI mode)
  - **GUI-mode build + terminal**: Creates independent console window + redirects stdout/stderr/stdin
  - **GUI-mode build + double-click**: No console (pure GUI mode)

### Multi-Gamepad Support

Reader maintains a list of connected gamepads (sorted by connection order):
- `joystickOrder`: Gamepad connection order (list of XInput user indices 0-3)
- `activeIndex`: XInput user index of the currently active gamepad
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

### Raw Input Implementation Notes

- `RIM_TYPEMOUSE = 0`, `RIM_TYPEKEYBOARD = 1` (from winuser.h). The constants in `rawinput_windows.go` must match exactly — swapping them causes all mouse events to be silently discarded.
- `Client.wantsKeyMouse` is accessed from two goroutines (ReadPump writes, BroadcastKeyMouse reads); use `atomic.Int32` to avoid data race.
- Arrow rotation for `mouse_movement` Point mode (type 9): sprite faces **up** by default, so angle formula is `Math.atan2(mx, -my)` (not the standard `atan2(y, x)`).
- Mouse button codes use `uint16` (not `uint8`) throughout — Go's `encoding/json` serializes `[]uint8` as a base64 string rather than a JSON number array, silently breaking frontend parsing.
- `KeyMouseState` carries `PendingKeysDown/Up` and `PendingButtonsDown/Up` slices (tagged `json:"-"`) to capture button events that occur and complete within a single 16ms tick. `ComputeKeyMouseDelta` uses these pending lists when non-nil, falling back to prev/curr state comparison otherwise.

### WebSocket Message Protocol

**Server → Client:**
- `full`: Complete state snapshot (sent on new client connect, every 5 seconds, and after every 100 deltas)
- `delta`: Only changed fields (regular updates)
- `player_selected`: Confirm gamepad switch success
- `km_full`: Complete keyboard/mouse state snapshot (sent when client subscribes)
- `km_delta`: Keyboard/mouse incremental update (keys down/up, buttons, mouse move, wheel)
- All messages include `seq` (incrementing sequence number) and `timestamp` (millisecond timestamp)

**Client → Server:**
- `select_player`: Select gamepad number to listen to
- `subscribe_km`: Subscribe to keyboard/mouse event stream (automatically sent when overlay config contains km element types)

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

`internal/web/frontend/configs/*.json` defines Canvas drawing layout for each gamepad type (button coordinates, sizes, radii). Frontend automatically loads the corresponding config based on `controllerType` reported by backend.

#### Body Shape Configuration

Gamepad body outlines are defined in the `body` section of each config file. The system supports two rendering methods:

**1. SVG Path (Recommended)** — with optional `viewBox` for automatic coordinate scaling:
```json
{
  "body": {
    "path": "M60.3 48.3c-6.8 1.9...",
    "viewBox": "0 0 256 256",
    "x": 10, "y": 40, "width": 480, "height": 280
  }
}
```

**2. Rounded Rectangle (Legacy)**:
```json
{
  "body": { "x": 10, "y": 40, "width": 480, "height": 280, "radius": 40 }
}
```

### System Tray (Windows GUI Mode)

The system tray provides menu access when running in GUI mode (double-clicked executable). Key points:
- **Non-blocking menu handling**: Menu clicks processed in a dedicated goroutine to prevent Windows message loop deadlocks
- **Atomic shutdown flag**: Prevents duplicate shutdown requests and race conditions

### Input Overlay Rendering

Two rendering engines coexist in `app.js`, selected by the `?overlay=` URL parameter:

| Mode | Renderer |
|------|----------|
| Built-in geometric | Canvas shapes (SVG path / rounded rects) |
| Input Overlay (`?overlay=<name>`) | Texture atlas (PNG sprite sheet) |

**Supported element types:** texture (0), keyboard_button (1), gamepad_button (2), mouse_button (3), mouse_wheel (4), analog_stick (5), trigger (6), gamepad_id (7), dpad (8), mouse_movement (9)

**Canvas sizing**: In overlay mode, `canvasW`/`canvasH` are set to `overlay_width`/`overlay_height` from the config once loaded, and overlay elements are rendered at 1:1 pixel coordinates with no scaling. In simple mode (`?simple=1`) the canvas is stretched to fill the viewport while preserving aspect ratio. In geometric mode the canvas stays at the fixed 500×330 logical size.

**Simple mode** (`?simple=1`): makes the page background transparent. In Input Overlay mode, type=0 static texture elements (controller body) are always rendered — the controller outline is part of the atlas, not the page background.

**Keyboard/mouse-only overlays**: if the config contains no gamepad element types (2/5/6/7/8), the Player info bar and controller status bar are hidden, `select_player` is not sent, and rendering starts immediately without waiting for a gamepad connection.

Presets are served from `overlays/<name>/` next to the executable (mounted at `/overlays/`). See [docs/input-overlay-format.md](docs/input-overlay-format.md) for full format specification.

## GPV Skin → Input Overlay Converter

`cmd/gpvskin2overlay` converts [GamepadViewer](https://gamepadviewer.com/) CSS skins into Input Overlay format. See **[docs/gpvskin2overlay.md](docs/gpvskin2overlay.md)** for full build and usage instructions.

### Internal Package (`internal/gpvskin`)

| File | Purpose |
|------|---------|
| `skinmodel.go` | Data types: `IOElementType`, `CSSProperties`, `SkinElement`, etc. |
| `cssparser.go` | CSS loading (HTTP + local), comment stripping, rule/property parsing |
| `skins.go` | All 9 GPV skin definitions + `CustomSkinDef` registry |
| `mapping.go` | CSS selector resolution + absolute position calculation per element |
| `download.go` | Image download + SVG/SVGZ→PNG rasterization (rsvg-convert/inkscape) |
| `atlas.go` | Sprite cropping + IO-convention atlas packing |
| `generate.go` | Input Overlay JSON generation + high-level `Convert()` pipeline |

### Key Implementation Details

**CSS selector matching**: `Lookup()` merges all matching CSS rules in file order. Later rules override earlier ones. Exception: a `background` value without `url()` will not overwrite an existing URL value (prevents `.xbox { background: no-repeat center }` from erasing the background image).

**Positive `background-position` values**: GPV sometimes uses positive background-position (e.g. `48px 0` for the Y button). In CSS this shifts the image right, meaning the crop starts at a negative coordinate. In `atlas.go`, negative cropX/cropY are corrected as `cropX = imageWidth + cropX`.

**Sub-container DOM layout**: GPV's DOM nests elements inside intermediate containers (`.triggers`, `.bumpers`, `.arrows`, `.abxy`, `.sticks`, `.dpad`). `mapping.go` resolves each container's absolute position first, then computes element positions within.

**PressedOpacity atlas layout**: Most GPV skins use `opacity:0` for normal state and `opacity:1` for pressed state. Atlas layout:
- frame0 (at `[u, v, w, h]`): transparent
- frame1 (at `[u, v+h+3, w, h]`): the actual sprite

All dpad directions and triggers use `PressedOpacity`. All triggers use `trigger_mode: false` (progressive fill). NES dpad uses `PressedSprite` (sprite-based pressed state).

**Guide/Home button**: GPV cannot detect the guide button via browser APIs. The `.meta` element is not mapped in any skin.

**SVGZ support**: `isSVG()` recognizes both `.svg` and `.svgz`. `rasterizeSVG()` checks the gzip magic bytes (`0x1f 0x8b`) before attempting decompression — some servers return plain SVG with a `.svgz` extension.

## Common Modification Guide

### Adding New Gamepad Support

1. `internal/gamepad/mapping.go`: Add VID/PID → DeviceMapping to `knownDevices` map
2. If button layout differs from existing mappings, create new `DeviceMapping` variable
3. `internal/web/frontend/configs/`: Add new layout JSON file
4. `internal/web/frontend/app.js`: Add mapping name → config filename in `configMap`

### Modifying Canvas Rendering

All drawing logic is in `internal/web/frontend/app.js` in `drawController()` and its sub-functions. Button positions and sizes are controlled by `configs/*.json`, colors by `COLORS` constants.

### Modifying Poll Frequency

`pollDelay` constant in `internal/gamepad/reader_windows.go` (currently 16ms ≈ 60Hz).

### Modifying Deadzone

`deadzone` constant in `internal/gamepad/reader_windows.go` (currently 0.05), `analogThreshold` constant in `internal/gamepad/state.go` (currently 0.01, used for delta comparison).

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/lxzan/gws` | WebSocket server |
| `github.com/klauspost/compress` | Transitive dependency (via gws, permessage-deflate) |
| `fyne.io/systray` | Windows system tray integration |
| `github.com/godbus/dbus/v5` | Transitive (via systray, Linux only) |
