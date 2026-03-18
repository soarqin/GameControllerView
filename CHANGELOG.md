# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
