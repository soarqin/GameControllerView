# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] - 2026-04-01

### Added

- Nintendo Switch Pro custom HID report parser: bypasses HidP_* pipeline entirely for Nintendo controllers (VID 0x057E) whose USB HID descriptors define a fake standard layout. Parses the actual proprietary byte format directly for report ID 0x30 (full mode, 60Hz) and 0x3F (simple HID mode).
- SVG texture atlas fallback for Input Overlay: when the PNG atlas (`<dir>/<dir>.png`) is not found, the loader automatically attempts `<dir>/<dir>.svg`. Both formats work through the same `HTMLImageElement` + `ctx.drawImage()` pipeline.

### Fixed

- Switch Pro controller stick Y-axis was inverted in 0x30 full mode. The proprietary protocol uses Y positive-upward (matching XInput convention), but the parser incorrectly negated it assuming standard HID positive-downward convention. Removed the negation for 0x30; kept it for 0x3F simple mode which does follow HID convention.
- Switch Pro built-in geometric renderer: redesigned SVG body shape (wider, flatter, flat top edge matching real controller silhouette), fixed face button positions to match Nintendo layout (A=east, B=south, X=north, Y=west — previously used Xbox layout), fixed D-pad overlapping with left stick (was only 25px apart, now properly separated), corrected L/R shoulder button positions to visually connect with ZL/ZR triggers.
- Input Overlay button code 15 (`SDL_CONTROLLER_BUTTON_MISC1`) mapped to non-existent `state.buttons.misc` instead of `state.buttons.capture`. Switch Pro Capture, Xbox Series Share, and PS5 Mic buttons now respond correctly in overlay mode.
- Switch Pro wired connection caused erratic button state jumping. The HID descriptor's fake layout made HidP_* parse the timer byte as button state, toggling random buttons every frame.

### Changed

- Switch Pro overlay preset (`switch-pro-controller-sdl.json`): changed Home and Capture elements from type 0 (static texture) to type 2 (gamepad_button) with SDL codes 5 and 15 respectively, enabling press visualization.
- Resolved data races (`atomic.Int32` for `Client.wantsKeyMouse`), resource leaks (unclosed HTTP response bodies), and unchecked errors across the codebase.

## [0.2.1] - 2026-03-19

### Fixed

- System tray icon becomes completely unresponsive (right-click menu stops appearing) after the program runs for some time. Root cause: `systray.Run()` was called from a spawned goroutine without `runtime.LockOSThread()`, allowing Go's async preemption to migrate the goroutine between OS threads and break the Windows message loop. Fixed by locking the goroutine to its OS thread before entering the systray event loop.

### Changed

- Consolidated PS4, PS5, and Switch Pro controller types into a single PlayStation layout: updated `playstation.json` to PS5 body shape with touchpad, removed separate `playstation5.json` and `switch_pro.json` configs.

## [0.2.0] - 2026-03-13

### Added

- Dirty-flag rendering: canvas only redraws when WebSocket state changes arrive, eliminating ~60 wasted redraws/sec at idle.
- Pre-minified and pre-gzipped static file serving: JS/CSS/HTML/JSON assets are minified at startup via `tdewolff/minify` and pre-compressed with gzip; the server sends compressed bytes directly when the client supports it (app.js 53KB → 6KB, -88%).
- Build scripts (`build.ps1`, `build.sh`, GitHub Actions workflow) now auto-update `gamecontrollerdb.txt` from SDL_GameControllerDB before building if the remote has a newer version.

### Fixed

- `set_mouse_sens` WebSocket message was silently dropped by the backend; now properly routed to `rawinput.Reader.SetMouseSensitivity()`.
- Overlay elements are sorted by `z_level` once at config load time instead of every animation frame.

### Changed

- `app.js` refactored and reduced from ~1500 to ~750 lines.
- Updated bundled `gamecontrollerdb.txt` (added Atari CX Wireless Controller mappings).

## [0.1.2] - 2026-03-13

### Added

- Overlay variant support: multiple JSON configs can share a single PNG texture atlas within the same overlay directory. Access variants via `?overlay=<dir>/<variant>` (e.g. `?overlay=dualsense/compact`).
- System tray "Open Browser" is now a sub-menu listing all available overlays (primary and variants) discovered at startup. Clicking a sub-item opens the browser directly to that overlay.
- System tray "Copy URL for Streaming" sub-menu: mirrors "Open Browser" but copies the URL (with `?simple=1` for transparent background) to the clipboard instead of opening the browser. Useful for pasting into OBS Browser Source and similar streaming tools.

## [0.1.1] - 2026-03-12

### Fixed

- Fix canvas crop
- Remove player info in Simple Mode

## [0.1.0] - 2026-03-10

### Added

- Initial release

<!-- When releasing a new version, add a new section above this line using the format below:

## [x.y.z] - YYYY-MM-DD

### Added
- ...

### Changed
- ...

### Fixed
- ...

### Removed
- ...

-->

## [Unreleased]

### Added

- Built-in mouse renderer: top-down view with left/right/middle/X1/X2 button states, scroll wheel indicator (200ms timeout), and movement direction arrow
- Built-in keyboard renderer: config-driven row-based layout, pressed-key highlighting, WASD gaming preset included
- URL parameters `?gamepad[=type]`, `?mouse=1`, `?keyboard=<preset>` for explicit device selection; freely combinable (e.g. `?gamepad&mouse=1&keyboard=wasd&simple=1`)
- Multi-canvas layout: each device gets an independent Canvas element arranged via CSS flexbox with 16px gap
- External keyboard config support: place JSON files in `keyboards/` directory next to executable; external configs take priority over built-in ones
- `--keyboard-dir` CLI flag and `keyboard-dir` TOML option (default: `keyboards`) for external keyboard layout directory
- `KEY_NAME_TO_SCANCODE` lookup table mapping 80+ key names to uiohook scancodes
- Simple mode (`?simple=1`) works with multi-canvas: transparent background, all canvases visible at natural per-device dimensions
