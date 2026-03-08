# Third-Party Licenses

## Input Overlay Presets

The preset files for [Input Overlay](https://github.com/univrsal/input-overlay) (`dualsense`, `xbox-one-controller`, etc.) are **not** part of this repository and are **not** distributed with GameControllerView binaries.

These preset files are derived from the Input Overlay project, which is licensed under the **GNU General Public License v2.0 (GPL-2.0)**:

> Copyright (C) Alex and contributors  
> https://github.com/univrsal/input-overlay
>
> This program is free software; you can redistribute it and/or modify  
> it under the terms of the GNU General Public License as published by  
> the Free Software Foundation; either version 2 of the License, or  
> (at your option) any later version.
>
> This program is distributed in the hope that it will be useful,  
> but WITHOUT ANY WARRANTY; without even the implied warranty of  
> MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the  
> GNU General Public License for more details.
>
> You should have received a copy of the GNU General Public License  
> along with this program. If not, see <https://www.gnu.org/licenses/old-licenses/gpl-2.0.html>.

### Why Presets Are External

GameControllerView is licensed under the **MIT License**. The MIT License is not compatible with GPL-2.0 for combined distribution (GPL-2.0 requires all combined works to also be GPL-2.0). To avoid license conflict:

- Preset files (`.json` + `.png`) are **not embedded** in the binary.
- They must be placed externally in an `overlays/` directory next to the executable.
- They are **not included** in GameControllerView releases.

### Obtaining Presets

You can obtain Input Overlay presets from the official repository:

- https://github.com/univrsal/input-overlay/tree/master/presets

Download and place them in `overlays/<preset-name>/` next to the executable. See the main README for usage instructions.

### Packaging Notice

> **IMPORTANT**: When packaging or distributing GameControllerView, do **NOT** include any Input Overlay preset files (`.json` / `.png` files from the `overlays/` directory). Distributing these files alongside GameControllerView without GPL-2.0 compliance would be a license violation.
>
> Only the GameControllerView source code and binary (which carry the MIT License) may be distributed freely.

---

## Other Dependencies

All Go module dependencies used by GameControllerView carry licenses compatible with the MIT License. See `go.sum` for the full dependency list.

| Package | License |
|---------|---------|
| `github.com/jupiterrider/purego-sdl3` | MIT |
| `github.com/gorilla/websocket` | BSD-2-Clause |
| `github.com/ebitengine/purego` | Apache-2.0 |
| `fyne.io/systray` | MIT |
| `github.com/godbus/dbus/v5` | BSD-2-Clause |
