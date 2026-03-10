# Third-Party Licenses

## SDL_GameControllerDB (Embedded)

`internal/gamepad/gamecontrollerdb.txt` is bundled from the
[SDL_GameControllerDB](https://github.com/mdqinc/SDL_GameControllerDB) project
and is embedded in the InputView binary at compile time.

> Copyright (C) 1997-2025 Sam Lantinga \<slouken@libsdl.org\>
>
> This software is provided 'as-is', without any express or implied
> warranty. In no event will the authors be held liable for any damages
> arising from the use of this software.
>
> Permission is granted to anyone to use this software for any purpose,
> including commercial applications, and to alter it and redistribute it
> freely, subject to the following restrictions:
>
> 1. The origin of this software must not be misrepresented; you must not
>    claim that you wrote the original software. If you use this software
>    in a product, an acknowledgment in the product documentation would be
>    appreciated but is not required.
> 2. Altered source versions must be plainly marked as such, and must not be
>    misrepresented as being the original software.
> 3. This notice may not be removed or altered from any source distribution.

---

## Input Overlay Presets

The preset files for [Input Overlay](https://github.com/univrsal/input-overlay) (`dualsense`, `xbox-one-controller`, etc.) are **not** part of this repository and are **not** distributed with InputView binaries.

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

InputView is licensed under the **MIT License**. The MIT License is not compatible with GPL-2.0 for combined distribution (GPL-2.0 requires all combined works to also be GPL-2.0). To avoid license conflict:

- Preset files (`.json` + `.png`) are **not embedded** in the binary.
- They must be placed externally in an `overlays/` directory next to the executable.
- They are **not included** in InputView releases.

### Obtaining Presets

You can obtain Input Overlay presets from the official repository:

- https://github.com/univrsal/input-overlay/tree/master/presets

Download and place them in `overlays/<preset-name>/` next to the executable. See the main README for usage instructions.

### Packaging Notice

> **IMPORTANT**: When packaging or distributing InputView, do **NOT** include any Input Overlay preset files (`.json` / `.png` files from the `overlays/` directory). Distributing these files alongside InputView without GPL-2.0 compliance would be a license violation.
>
> Only the InputView source code and binary (which carry the MIT License) may be distributed freely.

---

## Go Module Dependencies

All Go module dependencies carry licenses compatible with the MIT License. See `go.sum` for the full dependency list.

| Package | License |
|---------|---------|
| `github.com/lxzan/gws` | Apache-2.0 |
| `github.com/klauspost/compress` | BSD-3-Clause |
| `fyne.io/systray` | Apache-2.0 |
| `github.com/godbus/dbus/v5` | BSD-2-Clause |
