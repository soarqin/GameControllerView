# InputView

Go backend reads gamepad input via Windows XInput API (Xbox-compatible controllers) and Windows Raw Input HID API (PS4/PS5/Switch Pro/generic HID gamepads), plus keyboard/mouse input via Windows Raw Input API, pushes to frontend via WebSocket, and renders real-time gamepad/keyboard/mouse visualization on Canvas.

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

# CLI flags (all optional, defaults work without config file)
go run ./cmd/inputview --addr=:9090 --poll-rate=16 --deadzone=0.05 --mouse-sens=500 --log-level=info

# Config file: place inputview.toml next to executable (see inputview.example.toml for all options)
# CLI flags take priority over config file values

# Open browser at http://localhost:8080
# Health check: GET http://localhost:8080/health → {"status":"ok","version":"0.3.0","uptime_seconds":N,"listeners":{"addr":":8080"}}
```

No external DLL required. Gamepad input uses XInput (`xinput1_4.dll`, built into Windows 8+) for Xbox-compatible controllers and `hid.dll` (built into all Windows versions) for HID gamepads.

## URL Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `p` | Gamepad number (1-based, default 1) | `?p=2` |
| `simple` | Simple mode (transparent background, no UI) | `?simple=1` |
| `alpha` | Gamepad opacity (0.0-1.0) | `?alpha=0.5` |
| `overlay` | Input Overlay config name or variant path (enables texture-atlas renderer) | `?overlay=dualsense`, `?overlay=dualsense/compact` |
| `mouse_sens` | Mouse movement sensitivity divisor (default 500; lower = more sensitive) | `?mouse_sens=300` |

## Project Structure

```
InputView/
├── go.mod                              # module github.com/soar/inputview
├── go.sum
├── build.ps1                           # Windows release build script (-tags release, -H=windowsgui)
├── build.sh                            # Linux/macOS release build script (-tags release)
├── CHANGELOG.md                        # Version history (Keep a Changelog format)
├── inputview.example.toml             # Example config file (copy as inputview.toml next to exe)
├── .github/
│   └── workflows/
│       └── release.yml                 # GitHub Actions: build & publish release on git tag push
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
    ├── config/
    │   └── config.go                   # Config struct + Load(exeDir) — pflag CLI flags + viper TOML parsing + validation
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
    │   ├── sdldb.go                    # SDL GameControllerDB parser: LoadSDLMappingsFromFile/Reader
    │   ├── sdldb_embed.go              # go:embed for bundled gamecontrollerdb.txt
    │   ├── sdldb_test.go               # Tests for SDL DB parsing
    │   ├── gamecontrollerdb.txt        # Bundled SDL_GameControllerDB (embedded at compile time)
    │   ├── reader.go                   # Reader struct: shared fields, Changes()/GetPlayerIndex()/SetActiveByPlayerIndex(), LoadSDLDB(), lookupSDLMapping()
    │   ├── reader_windows.go           # Windows implementation: XInput polling loop (~60Hz) + HID callback handling
    │   ├── reader_other.go             # Non-Windows stub (blocks until ctx cancel)
    │   ├── xinput_windows.go           # XInput API bindings (syscall): GetState, GetStateEx (ordinal 100, Guide button), GetCapabilitiesEx (ordinal 108, VID/PID)
    │   ├── hidinput_shared.go          # Platform-agnostic HID constants, types, and logic (all platforms)
    │   ├── hidinput_windows.go         # HID Raw Input: hid.dll bindings, device init, report parsing (axes/buttons/hat); SDL mapping path
    │   └── hidinput_other.go           # Stub for non-Windows platforms
    ├── hub/
    │   ├── hub.go                      # WebSocket hub: client management, targeted broadcast, main loop
    │   ├── client.go                   # WebSocket client: connection, read/write pumps, message handling
    │   ├── broadcast.go                # State change → targeted JSON broadcast
    │   └── message.go                  # WSMessage type definitions
    ├── server/
    │   ├── server.go                   # HTTP server, graceful shutdown; mounts external overlays/ dir; gzip-aware static file handler
    │   └── handler.go                  # WebSocket upgrade, client message handling
    ├── overlay/
    │   └── scan.go                     # ScanDir(): enumerate overlays/ directory → []Entry (name + URL path)
    ├── tray/
    │   ├── tray.go                     # Windows system tray integration (atomic shutdown flag, non-blocking menu handling)
    │   ├── clipboard_windows.go        # Windows clipboard write via Win32 API (user32/kernel32 syscall)
    │   ├── clipboard_other.go          # Stub for non-Windows platforms
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
        ├── embed.go                    # go:embed embeds frontend/ static files; minifies JS/CSS/HTML/JSON at startup and pre-compresses with gzip; exports FrontendFS() and GzipCache()
        └── frontend/                   # Frontend static files
            ├── index.html
            ├── styles.css
            ├── app.js                  # WebSocket client, URL param parsing, Canvas rendering
            └── configs/                # Gamepad layout JSON configs
                ├── xbox.json
                ├── playstation.json
                └── switch_pro.json
```

Input Overlay presets are **external only** — place them in an `overlays/` directory next to the executable.
The server mounts this directory at `/overlays/`. Presets are **not** embedded in the binary (GPL-2.0 license conflict).

## Architecture Highlights

### Configuration System

`internal/config/config.go` provides `Load(exeDir string) (Config, error)`:

1. **pflag** defines 7 CLI flags (parsed from `os.Args`). `--help` prints usage and exits 0.
2. **viper** reads optional `inputview.toml` from `exeDir` or current directory.
3. CLI flags take priority over TOML file (via `viper.BindPFlags`).
4. Validation: deadzone 0.0–1.0; poll-rate ≥ 1; mouse-sens > 0; log-level ∈ {debug,info,warn,error}.

**Config fields** (7):
| Field | Flag | Default | Purpose |
|-------|------|---------|---------|
| `Addr` | `--addr` | `:8080` | HTTP listen address |
| `PollRate` | `--poll-rate` | `16` | Gamepad poll interval (ms) |
| `Deadzone` | `--deadzone` | `0.05` | Analog stick deadzone |
| `MouseSensitivity` | `--mouse-sens` | `500.0` | Mouse delta divisor |
| `OverlayDir` | `--overlay-dir` | `overlays` | Overlay presets directory |
| `SDLDBPath` | `--sdl-db` | `gamecontrollerdb.txt` | SDL gamecontrollerdb path |
| `LogLevel` | `--log-level` | `info` | Log level |

`main.go` calls `config.Load()` first, then passes values to `reader.SetDeadzone()`, `reader.SetPollDelay()`, `kmReader.SetMouseSensitivity()`, and `server.New()`.

### Logging

All logging uses stdlib `log/slog` with a `TextHandler` writing to `os.Stderr`. Initialized at the very start of `main()` before any subsystems:

```go
slogLevel := &slog.LevelVar{}
slogLevel.Set(slog.LevelInfo)
slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slogLevel})))
```

**`internal/web/embed.go`**: Runs in `init()` before `main()`, so slog is not yet configured. Uses `fmt.Fprintf(os.Stderr, ...)` instead. On walk error, falls back to serving raw (unminified) embedded files rather than panicking.

**`internal/gamepad/reader_windows.go`**: XInput load failure (`procXInputGetState.Find()`) no longer calls `log.Fatalf` — it logs a warning and continues in HID-only mode, allowing PS4/PS5/Switch Pro controllers to work even if XInput DLL is missing.

### XInput Ordinal Exports

`XInputGetStateEx` (ordinal 100, includes Guide button) and `XInputGetCapabilitiesEx` (ordinal 108, includes VID/PID) are **undocumented ordinal-only exports** — they have no named symbol in the DLL. Go's `syscall.LazyProc` with `NewProc("#100")` does NOT perform ordinal lookup; it passes the literal string `"#100"` to `GetProcAddress`, which fails silently. The correct approach is to call the Windows `GetProcAddress` API directly with `MAKEINTRESOURCE(ordinal)` (i.e., pass the ordinal number as a raw `uintptr` for the `lpProcName` parameter, with high word = 0). See `getProcAddressByOrdinal()` in `xinput_windows.go`.

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

The **same HWND_MESSAGE window** also handles HID gamepad events (WM_INPUT with dwType=RIM_TYPEHID):

```
goroutine: rawinput.Reader.Run(ctx)  ← HWND_MESSAGE window (OS-locked)
   ├── WM_INPUT (keyboard)  → keyboard state accumulation
   ├── WM_INPUT (mouse)     → mouse state accumulation
   ├── WM_INPUT (HID)       → registered HID callbacks
   │      └── gamepad.Reader.handleHIDInput()  ← runs on the rawinput goroutine
   │             ↓ (non-XInput HID gamepads only)
   │         parseHIDReport() via hid.dll HidP_* APIs
   │             ↓
   │         GamepadState → chan GamepadState (same channel as XInput)
   └── WM_INPUT_DEVICE_CHANGE → registered device-change callbacks
          └── gamepad.Reader.handleHIDDeviceChange()
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
- `joystickOrder`: Gamepad connection order (list of `joystickKey` values)
- `activeKey`: Unified key of the currently active gamepad
- `joystickKey`: A `uint64` that unifies XInput slots (0-3) and HID device handles. XInput slots use values 0-3 directly; HID device handles are stored as-is (kernel handles are always >= 4 and aligned, so they never collide with XInput slot values 0-3).
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

**Critical**: `convertXInputState()` and `parseHIDReport()` build a fresh `GamepadState` from raw input data but do NOT set `PlayerIndex` (they only know about buttons/axes/triggers). The caller must set `PlayerIndex` on the returned state before storing and emitting. If `PlayerIndex` is 0 (zero-value), `BroadcastToPlayer` will never match any client (clients default to `playerIndex=1`), causing all input to be silently dropped.

### Raw Input Implementation Notes

- `RIM_TYPEMOUSE = 0`, `RIM_TYPEKEYBOARD = 1` (from winuser.h). The constants in `rawinput_windows.go` must match exactly — swapping them causes all mouse events to be silently discarded.
- `Client.wantsKeyMouse` is accessed from two goroutines (ReadPump writes, BroadcastKeyMouse reads); use `atomic.Int32` to avoid data race.
- Arrow rotation for `mouse_movement` Point mode (type 9): sprite faces **up** by default, so angle formula is `Math.atan2(mx, -my)` (not the standard `atan2(y, x)`).
- Mouse button codes use `uint16` (not `uint8`) throughout — Go's `encoding/json` serializes `[]uint8` as a base64 string rather than a JSON number array, silently breaking frontend parsing.
- `KeyMouseState` carries `PendingKeysDown/Up` and `PendingButtonsDown/Up` slices (tagged `json:"-"`) to capture button events that occur and complete within a single 16ms tick. `ComputeKeyMouseDelta` uses these pending lists when non-nil, falling back to prev/curr state comparison otherwise.

### HID Gamepad Input (Raw Input + hid.dll)

Non-XInput HID gamepads (PS4/PS5/Switch Pro/generic) are captured via the **same HWND_MESSAGE window** as keyboard/mouse, by registering additional `RAWINPUTDEVICE` entries for `UsagePage=0x01, Usage=0x04` (Joystick) and `Usage=0x05` (Gamepad) with `RIDEV_INPUTSINK | RIDEV_DEVNOTIFY`.

**Callback registration**: `rawinput.Reader.RegisterHIDCallback(usagePage, usage, inputCb, changeCb)` stores the callback and registers the HID device class. HID events (`rimTypeHID`, dwType=2) are routed to matching callbacks via `routeHIDInput()`. Device changes (`WM_INPUT_DEVICE_CHANGE`) are routed via `handleDeviceChange()`.

**Device info cache**: Usage page/usage per hDevice is cached in `hidDeviceCache` (map[uintptr]hidDeviceUsage) to avoid calling `GetRawInputDeviceInfoW` on every WM_INPUT event. On `GIDC_REMOVAL`, the cache entry is read first (to route the disconnect callback) and then deleted — the handle is never re-queried after removal to prevent stale re-population.

**Disconnected HID handle blocklist**: `gamepad.Reader` maintains a `disconnectedHIDs` set (map[uintptr]struct{}). When `handleHIDDeviceChange` receives `GIDC_REMOVAL`, the handle is added to this set. `handleHIDInput` checks the set before processing any WM_INPUT event and discards residual events for disconnected handles. The handle is removed from the set on the next `GIDC_ARRIVAL` for the same handle value, allowing re-connection to work correctly.

**XInput filtering**: XInput creates a virtual HID device for each Xbox controller. These are identified by `IG_` in the device name (from `GetRawInputDeviceInfoW(RIDI_DEVICENAME)`). `isXInputDevice()` in `hidinput_windows.go` checks for this prefix — matching devices are skipped in the HID path to prevent double-reporting. Additionally, `Reader.xinputVIDPIDs` tracks VID/PID pairs of connected XInput devices; `getOrInitHIDDevice()` checks this set and suppresses HID devices whose VID/PID matches an active XInput device. This handles controllers like Switch Pro where XInput emulation software (Steam, BetterJoy) creates a virtual XInput device but the raw HID interface lacks the `IG_` marker.

**Report ID validation**: HID devices may send reports with multiple report IDs (input reports, feature responses, subcommand replies). Only reports whose ID matches the input value/button caps are valid input data. `hidDeviceInfo.expectedReportIDs` (computed during `initHIDDevice()` via `buildExpectedReportIDs()`) stores the set of valid input report IDs. `parseHIDReport()` checks the first byte of the raw data against this set and returns `(state, false)` for incompatible reports, preventing the caller from emitting a zero-state `GamepadState` that would cause button/axis values to "jump" between real data and zeros.

**Multi-report batching**: When multiple HID reports arrive in a single `WM_INPUT` message (`dwCount > 1`), `handleHIDInput()` extracts only the last report (`rawData[len-reportSize:]`) for parsing, since it contains the most recent input state. This prevents `HidP_*` functions from reading into adjacent reports' data.

**Report parsing**: On each WM_INPUT for a HID gamepad, `parseHIDReport()` is called:
1. Report ID check — incompatible reports are skipped (returns false)
2. `HidP_GetUsageValue` — reads each analog axis value using the `valueCaps` list. Each `valueCaps` entry is iterated over its full `[UsageMin, UsageMax]` range (when `IsRange=1`) to handle controllers that pack multiple axes into a single caps entry.
3. Hat switch (usage 0x39) → mapped to `DpadState` using `hatDirTable` (0=N, 1=NE, … 7=NW, ≥8=center)
4. `HidP_GetUsages` — returns the list of currently pressed button usages (1-based)
5. Button usages → `GamepadState` fields via `resolveButtonTarget()` + `applyButton()`

**Preparsed data**: Fetched once per device via `GetRawInputDeviceInfoW(RIDI_PREPARSEDDATA)` and cached in `hidDeviceInfo`. Required for all `HidP_*` calls. Allocation must be at least 8-byte aligned (Go's `make([]byte)` satisfies this on all supported architectures).

**Struct layout critical**: `hidpValueCaps` and `hidpButtonCaps` in `hidinput_windows.go` must exactly match the Windows SDK `HIDP_VALUE_CAPS` / `HIDP_BUTTON_CAPS` binary layout. The correct field order for `HIDP_VALUE_CAPS` after `IsAbsolute` is: `HasNull(1) Reserved(1) BitSize(2) ReportCount(2) Reserved2[5](10) UnitsExp(4) Units(4) LogicalMin(4) LogicalMax(4) PhysicalMin(4) PhysicalMax(4) [union 16 bytes]` — total 72 bytes. Any deviation causes all `UsageMin`/`UsageMax`/`LogicalMin`/`LogicalMax` fields to read as zero, silently breaking all axis and hat-switch parsing.

**Axis normalization**: HID axis values are unsigned integers with `LogicalMin`/`LogicalMax` from value caps. `normalizeHIDAxis()` maps `[LogicalMin, LogicalMax]` to `[-1, 1]` for sticks or `[0, 1]` for triggers. Handles edge case where `LogicalMax < LogicalMin` due to sign-extension of a smaller type (detected via `BitSize`).

**HID Y-axis direction**: HID Y axes are positive-downward. The HID path negates the Y value before storing it in `GamepadState` so that it matches the XInput convention (positive-upward). The frontend canvas renderer then inverts Y again (`knobY = s.y - position.y * maxTravel`), which correctly maps positive-up to upward knob movement. XInput Y axes are already positive-upward, so no inversion is applied in the XInput path.

**Device-specific mappings**: `DeviceMapping` now has two optional HID fields:
- `HIDAxes map[uint16]string` — maps HID usage codes to semantic axis targets. If nil, `defaultHIDAxes` is used (covers most standard gamepads).
- `HIDButtons map[uint16]string` — maps 1-based button usage numbers to button target names. If nil, `defaultButtonOrder` is used.
- PS4/PS5: `playStationHIDAxes` (Z=right_x, Rz=right_y, Rx=lt, Ry=rt) — different from generic default which assigns Z to right trigger.
- Switch Pro: `switchProHIDAxes` (Z=right_x, Rz=right_y, no analog triggers in HID report). `switchProHIDButtons` remaps face buttons (Y=1, B=2, A=3, X=4 — Nintendo layout differs from default). `switchProMapping` with `Name: "switch_pro"` ensures the frontend loads the correct layout config.

**SDL GameControllerDB mapping path** (takes priority over legacy HID path when available):
- `LoadSDLDB(externalPath)` in `reader.go` always loads the **embedded** `gamecontrollerdb.txt` (via `go:embed` in `sdldb_embed.go`) as a base, then overlays an external file at `externalPath` if present. External entries take priority, allowing users to place an updated `gamecontrollerdb.txt` next to the executable without recompiling.
- `lookupSDLMapping(vid, pid)` in `reader.go` looks up `globalSDLMappings` by VID/PID.
- `initHIDDevice()` calls `lookupSDLMapping()` and stores `*SDLMapping` in `hidDeviceInfo.sdlMap`.
- `buildAxisOrder()` enumerates value caps in report order (hat switch excluded), producing an ordered `[]hidAxisEntry` that maps SDL's 0-based axis index to (usagePage, usage).
- `parseHIDReport()` dispatches to `parseHIDReportSDL()` when `sdlMap != nil`, otherwise falls back to the legacy usage-code path.
- `parseHIDReportSDL()` handles all SDL binding types:
  - `leftx:a0` → axis index 0 → full axis → `left_x`
  - `lefttrigger:+a2` → positive half of axis 2 → `lt`
  - `righttrigger:-a3` → negative half of axis 3 (flipped) → `rt`
  - `lefty:a1~` → axis 1 inverted → `left_y`
  - `dpdown:+a1` → positive half of axis 1 > 0.5 threshold → `Dpad.Down`
  - `dpdown:h0.4` → hat switch (parsed by `parseHatSwitch()`)
  - `dpdown:b11` → button 11 → `Dpad.Down`
  - `+rightx:b9,-rightx:b4` → half-axis from buttons (N64 C-stick pattern) → `right_x`
- SDL GUID byte layout: `[bus LE16][crc LE16][vid LE16][0x0000][pid LE16][0x0000][ver LE16][sig][data]`. VID/PID are little-endian uint16 at bytes 4-5 and 8-9.
- `sdlNameToControllerType()` maps SDL controller names (e.g. "DualSense", "PS5") to frontend `controllerType` identifiers.
- `sdlPlatformName()` maps `runtime.GOOS` to the platform string used in gamecontrollerdb.txt: `"windows"` → `"Windows"`, `"linux"` → `"Linux"`, `"darwin"` → `"Mac OS X"`.
- File location: place `gamecontrollerdb.txt` next to the executable (from [SDL_GameControllerDB](https://github.com/mdqinc/SDL_GameControllerDB)) to override bundled entries.

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
- `set_mouse_sens`: Set mouse movement sensitivity divisor (sent on connect when `?mouse_sens=N` URL param is present)

```json
// Client sends
{"type": "select_player", "playerIndex": 2}

// Server responds
{"type": "player_selected", "playerIndex": 2}

// Client sends (when ?mouse_sens=300 URL param is set)
{"type": "set_mouse_sens", "value": 300}
```

`ClientMessage.Value` (float64) carries the numeric payload for `set_mouse_sens`. The backend routes it to `rawinput.Reader.SetMouseSensitivity()`.

### Device Mapping System

`mapping.go` matches known devices (Xbox, PlayStation, Switch Pro) via VID/PID, with generic fallback for unknown devices. Mappings define:
- Axis index → stick/trigger correspondence (XInput path)
- Button index → button name correspondence (XInput path)
- Axis value normalization range (sticks -1.0~1.0, triggers 0.0~1.0)
- Whether Y-axis needs inversion (XInput path)
- `HIDAxes map[uint16]string` — HID usage code → semantic axis target (HID path)
- `HIDButtons map[uint16]string` — 1-based HID button usage → button name (HID path)

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
- **Thread locking**: `Tray.Run()` calls `runtime.LockOSThread()` before `systray.Run()` because the systray library's `init()` locks the main goroutine (assuming `Run()` is called from `main()`), but InputView calls it from a spawned goroutine. Without explicit locking, Go's async preemption can migrate the goroutine between OS threads, breaking the Windows message loop (which is thread-bound). This caused the tray icon to become completely unresponsive after some time.
- **Non-blocking menu handling**: Menu clicks processed in a dedicated goroutine to prevent Windows message loop deadlocks
- **Atomic shutdown flag**: Prevents duplicate shutdown requests and race conditions
- **`openBrowserURL` runs in its own goroutine**: `exec.Command(...).Start()` can stall on Windows under certain conditions (antivirus scanning, disk pressure). If `openBrowserURL` blocked inside `handleMenuClicks`, the select loop would stop draining `ClickedCh`; since `systray` uses a non-blocking send to that channel, all subsequent clicks would be silently dropped, causing the menu to become permanently unresponsive.
- **"Open Browser" sub-menu**: At startup, `overlay.ScanDir()` enumerates the `overlays/` directory. "Open Browser" becomes a parent menu item with sub-items:
  - **Default** — opens `http://localhost:8080`
  - One sub-item per overlay entry (primary and variants) — opens `http://localhost:8080?overlay=<name>`
  - Overlay sub-item clicks are aggregated via a shared `chan string` so the main `select` loop avoids a dynamic number of cases.
- **"Copy URL for Streaming" sub-menu**: Mirrors "Open Browser" with identical sub-items, but instead of opening a browser the URL is written to the Windows clipboard. All URLs include `?simple=1` (transparent background, no UI chrome) for use in streaming software (OBS Browser Source, etc.). Sub-item URLs also include the `overlay=` parameter. Clipboard writes use `user32.dll`/`kernel32.dll` Win32 API directly (no external dependency).
- **Overlay variants**: A variant config `overlays/dualsense/compact.json` appears as sub-item "dualsense/compact", opening the page with `?overlay=dualsense/compact`. Its PNG texture atlas is still `overlays/dualsense/dualsense.png`.

### Input Overlay Rendering

Two rendering engines coexist in `app.js`, selected by the `?overlay=` URL parameter:

| Mode | Renderer |
|------|----------|
| Built-in geometric | Canvas shapes (SVG path / rounded rects) |
| Input Overlay (`?overlay=<name>`) | Texture atlas (PNG sprite sheet) |

**Supported element types:** texture (0), keyboard_button (1), gamepad_button (2), mouse_button (3), mouse_wheel (4), analog_stick (5), trigger (6), gamepad_id (7), dpad (8), mouse_movement (9)

**Canvas sizing**: In overlay mode, `canvasW`/`canvasH` are set to `overlay_width`/`overlay_height` from the config once loaded. In simple mode (`?simple=1`) the canvas is stretched to fill the viewport while preserving aspect ratio. In geometric/overlay non-simple mode, `setupCanvas()` reads the CSS-constrained width via `getBoundingClientRect()`, derives height from `canvasH * scale` to preserve aspect ratio, and applies `ctx.setTransform(dpr * scale, 0, 0, dpr * scale, 0, 0)` so that drawing coordinates always stay in the `[0, canvasW] × [0, canvasH]` logical space — this prevents content clipping when `max-width: 100%` CSS causes the canvas element to be narrower than the overlay's native dimensions. In geometric mode the canvas stays at the fixed 500×330 logical size.

**Dirty-flag rendering**: `render()` only redraws the canvas when `dirty = true`. The flag is set whenever WebSocket state changes arrive (`applyFullState`, `applyDelta`, `applyKMFull`, `applyKMDelta`) or when assets finish loading. This eliminates unnecessary GPU work at idle (~60 draws/sec → 0 when nothing changes).

**Overlay element sorting**: `cfg.elements` are sorted by `z_level` once when the config JSON is loaded (`loadInputOverlayConfig`), not on every animation frame. The sorted array is mutated in-place on the `overlayConfig` object.

**Simple mode** (`?simple=1`): makes the page background transparent. In Input Overlay mode, type=0 static texture elements (controller body) are always rendered — the controller outline is part of the atlas, not the page background.

**Keyboard/mouse-only overlays**: if the config contains no gamepad element types (2/5/6/7/8), the Player info bar and controller status bar are hidden, `select_player` is not sent, and rendering starts immediately without waiting for a gamepad connection.

**Overlay variants**: A single PNG texture atlas can be shared by multiple JSON configs. Place variant configs in the same subdirectory with different filenames — e.g. `overlays/dualsense/compact.json` — and access them via `?overlay=dualsense/compact`. The PNG is always loaded from `overlays/<dir>/<dir>.png` regardless of the variant name.

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

Override via `--poll-rate=<ms>` CLI flag or `poll-rate = <ms>` in `inputview.toml`.

### Modifying Deadzone

`deadzone` constant in `internal/gamepad/reader_windows.go` (currently 0.05), `analogThreshold` constant in `internal/gamepad/state.go` (currently 0.01, used for delta comparison).

Override via `--deadzone=<value>` CLI flag or `deadzone = <value>` in `inputview.toml`.

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/lxzan/gws` | WebSocket server |
| `github.com/klauspost/compress` | Transitive dependency (via gws, permessage-deflate) |
| `fyne.io/systray` | Windows system tray integration |
| `github.com/godbus/dbus/v5` | Transitive (via systray, Linux only) |
| `github.com/tdewolff/minify/v2` | JS/CSS/HTML/JSON minifier — runs at startup to pre-process embedded frontend assets |
| `github.com/tdewolff/parse/v2` | Transitive dependency (via tdewolff/minify) |
| `github.com/spf13/viper` | Configuration file + CLI flag parsing (TOML + pflag) |
| `github.com/spf13/pflag` | POSIX-compatible CLI flag library (used by viper) |
