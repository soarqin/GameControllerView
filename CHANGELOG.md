# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
