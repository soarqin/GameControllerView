# Input Overlay Config Format

InputView supports the [Input Overlay](https://github.com/univrsal/input-overlay) config format for texture-atlas based rendering. This document describes the complete format, noting which element types are currently supported by InputView.

> **License note**: The Input Overlay project is licensed under [GNU GPL v2](https://www.gnu.org/licenses/old-licenses/gpl-2.0.html). Preset files (`.json` + `.png`) derived from Input Overlay are **not** bundled with InputView. See [third-party-licenses.md](third-party-licenses.md) for details.

## Activation

Add `?overlay=<name>` to the URL. The config is looked up in:

1. `overlays/<name>/<name>.json` — external directory next to the executable (takes priority)
2. Embedded presets compiled into the binary (if any)

```
http://localhost:8080/?overlay=dualsense
http://localhost:8080/?overlay=xbox-one-controller&p=2&simple=1
http://localhost:8080/?overlay=keyboard&mouse_sens=300
```

When the loaded config contains keyboard/mouse element types (`ET_KEYBOARD_KEY`, `ET_MOUSE_BUTTON`, `ET_WHEEL`, or `ET_MOUSE_MOVEMENT`), the frontend automatically subscribes to keyboard and mouse events from the backend. No extra URL parameter is needed.

### `mouse_sens` parameter

Controls the sensitivity of `ET_MOUSE_MOVEMENT` elements. The raw pixel delta reported by the OS is divided by this value and clamped to `[-1.0, 1.0]`. Default: `500`. Smaller values = more sensitive (larger visual movement per physical pixel moved).

## File Pair

Each preset consists of two files with the same base name as the directory:

```
overlays/
└── my-controller/
    ├── my-controller.json   ← element layout
    └── my-controller.png    ← texture atlas
```

## JSON Structure

### Top-Level Object

| Field | Type | Description |
|-------|------|-------------|
| `overlay_width` | integer | Width of the overlay coordinate space (pixels) |
| `overlay_height` | integer | Height of the overlay coordinate space (pixels) |
| `flags` | integer | Bitmask (see below) |
| `elements` | array | List of element objects |
| `default_width` | integer | CCT internal, unused by renderer |
| `default_height` | integer | CCT internal, unused by renderer |
| `space_h` | integer | CCT internal, unused by renderer |
| `space_v` | integer | CCT internal, unused by renderer |
| `version` | integer | Config version (current: `507`), unused by renderer |

**`flags` bitmask:**

| Bit | Value | Meaning |
|-----|-------|---------|
| 0 | `1` | Has left analog stick |
| 1 | `2` | Has right analog stick |
| 2 | `4` | Is a gamepad |
| 3 | `8` | Has mouse |

### Element Object (common fields)

| Field | Type | Description |
|-------|------|-------------|
| `type` | integer | Element type (see table below) |
| `id` | string | Human-readable label (arbitrary, reference only) |
| `pos` | `[x, y]` | Screen position in the overlay coordinate space |
| `mapping` | `[u, v, w, h]` | Base crop rectangle in the texture atlas PNG |
| `z_level` | integer | Rendering order (lower = drawn first / further behind) |

The overlay is scaled to fit the browser canvas while preserving aspect ratio, based on `overlay_width` / `overlay_height`.

### All Element Types

| `type` | Enum name | Description | InputView |
|--------|-----------|-------------|-----------|
| `0` | `ET_TEXTURE` | Static background sprite | Supported |
| `1` | `ET_KEYBOARD_KEY` | Keyboard key | Supported |
| `2` | `ET_GAMEPAD_BUTTON` | Gamepad digital button | Supported |
| `3` | `ET_MOUSE_BUTTON` | Mouse button | Supported |
| `4` | `ET_WHEEL` | Mouse scroll wheel | Supported |
| `5` | `ET_ANALOG_STICK` | Analog stick | Supported |
| `6` | `ET_TRIGGER` | Analog trigger | Supported |
| `7` | `ET_GAMEPAD_ID` | Player number indicator | Supported |
| `8` | `ET_DPAD_STICK` | Composite D-pad | Supported |
| `9` | `ET_MOUSE_MOVEMENT` | Mouse movement indicator | Supported |

---

## Element Type Details

### Type 0 — `ET_TEXTURE` (static sprite) — **Supported**

No extra fields. The sprite at `mapping` is drawn at `pos`.

```json
{
    "type": 0,
    "id": "background",
    "pos": [0, 0],
    "z_level": 0,
    "mapping": [1, 1, 1344, 788]
}
```

---

### Type 1 — `ET_KEYBOARD_KEY` (keyboard key) — **Supported**

| Field | Type | Description |
|-------|------|-------------|
| `code` | integer | uiohook virtual key code (PS/2 Set 1 scan code; E0-extended keys use `0x0E00` or `0xE000` prefix) |

**Sprite layout**: same as `ET_GAMEPAD_BUTTON` — released state at `mapping`, pressed state at `[u, v+h+3]`.

**Input source**: Windows Raw Input API (global capture — works even when InputView is in the background).

---

### Type 2 — `ET_GAMEPAD_BUTTON` (gamepad digital button) — **Supported**

| Field | Type | Description |
|-------|------|-------------|
| `code` | integer | SDL2 gamepad button index (see table below) |

**Sprite layout**: Released state at `mapping` (`[u, v, w, h]`). Pressed state at `[u, v+h+3, w, h]` (3 px gap below the released sprite).

```json
{
    "type": 2,
    "id": "Button A",
    "pos": [914, 420],
    "z_level": 1,
    "mapping": [100, 200, 88, 88],
    "code": 0
}
```

**SDL2 button code mapping:**

| Code | SDL2 constant | Common label |
|------|---------------|--------------|
| `0` | `SDL_GAMEPAD_BUTTON_SOUTH` | A / Cross |
| `1` | `SDL_GAMEPAD_BUTTON_EAST` | B / Circle |
| `2` | `SDL_GAMEPAD_BUTTON_WEST` | X / Square |
| `3` | `SDL_GAMEPAD_BUTTON_NORTH` | Y / Triangle |
| `4` | `SDL_GAMEPAD_BUTTON_BACK` | Back / Share / Select |
| `5` | `SDL_GAMEPAD_BUTTON_GUIDE` | Guide / PS / Home |
| `6` | `SDL_GAMEPAD_BUTTON_START` | Start / Options / Menu |
| `7` | `SDL_GAMEPAD_BUTTON_LEFT_STICK` | L3 / LS (stick click) |
| `8` | `SDL_GAMEPAD_BUTTON_RIGHT_STICK` | R3 / RS (stick click) |
| `9` | `SDL_GAMEPAD_BUTTON_LEFT_SHOULDER` | LB / L1 |
| `10` | `SDL_GAMEPAD_BUTTON_RIGHT_SHOULDER` | RB / R1 |
| `11` | `SDL_GAMEPAD_BUTTON_DPAD_UP` | D-pad Up |
| `12` | `SDL_GAMEPAD_BUTTON_DPAD_DOWN` | D-pad Down |
| `13` | `SDL_GAMEPAD_BUTTON_DPAD_LEFT` | D-pad Left |
| `14` | `SDL_GAMEPAD_BUTTON_DPAD_RIGHT` | D-pad Right |
| `15` | `SDL_GAMEPAD_BUTTON_MISC1` | Mute (PS5) / Share (Xbox) |
| `20` | `SDL_GAMEPAD_BUTTON_TOUCHPAD` | Touchpad click (PS4/PS5) |

> Note: The D-pad can appear as 4 separate `ET_GAMEPAD_BUTTON` elements (codes 11–14) **or** as a single composite `ET_DPAD_STICK` element (type 8). Both are supported.

---

### Type 3 — `ET_MOUSE_BUTTON` (mouse button) — **Supported**

| Field | Type | Description |
|-------|------|-------------|
| `code` | integer | Input Overlay mouse button code: `1`=left, `2`=right, `3`=middle, `4`=X1 (back), `5`=X2 (forward) |

**Sprite layout**: same as `ET_GAMEPAD_BUTTON` — released state at `mapping`, pressed state at `[u, v+h+3]`.

**Input source**: Windows Raw Input API (global capture).

---

### Type 4 — `ET_WHEEL` (mouse scroll wheel) — **Supported**

No extra fields.

**Sprite layout**: 4 sprites arranged **horizontally** in the texture atlas, each `w+3` px apart:

| Offset | State |
|--------|-------|
| `[u, v, w, h]` | Neutral (middle button released) |
| `[u+(w+3), v, w, h]` | Middle button pressed |
| `[u+(w+3)*2, v, w, h]` | Scroll up |
| `[u+(w+3)*3, v, w, h]` | Scroll down |

The neutral state is drawn first, then the middle-button and scroll states are drawn on top as needed. The scroll state is shown only if a scroll event occurred within the last 200 ms.

**Input source**: Windows Raw Input API (global capture).

---

### Type 5 — `ET_ANALOG_STICK` (analog stick) — **Supported**

| Field | Type | Description |
|-------|------|-------------|
| `side` | integer | `0` = left stick, `1` = right stick |
| `stick_radius` | integer | Maximum pixel offset from center position |

**Sprite layout**: Two vertical frames (released/pressed state for L3/R3 stick click), separated by 3 px:

| Crop | State |
|------|-------|
| `[u, v, w, h]` | Stick not pressed (L3/R3 released) |
| `[u, v+h+3, w, h]` | Stick pressed (L3/R3 held) |

The correct frame is selected based on whether the stick click button is held. The sprite is then drawn offset by `(axisX × stick_radius, axisY × stick_radius)` pixels from `pos`, where axis values are in the range `[-1.0, 1.0]`.

```json
{
    "type": 5,
    "id": "Left stick",
    "pos": [389, 517],
    "z_level": 1,
    "mapping": [1359, 1, 157, 156],
    "side": 0,
    "stick_radius": 30
}
```

---

### Type 6 — `ET_TRIGGER` (analog trigger) — **Supported**

| Field | Type | Description |
|-------|------|-------------|
| `side` | integer | `0` = left trigger (LT/L2), `1` = right trigger (RT/R2) |
| `direction` | integer | Fill direction when pressed: `1`=up, `2`=down, `3`=left, `4`=right |
| `trigger_mode` | boolean | `true` = binary (button-like on/off), `false` = progressive fill |

**Sprite layout**: Two vertical frames:

| Crop | State |
|------|-------|
| `[u, v, w, h]` | Released / unfilled background |
| `[u, v+h+3, w, h]` | Pressed / filled foreground |

**`trigger_mode: false` (progressive)**: The background frame is drawn first. A clipped portion of the foreground frame is drawn on top, with its size proportional to the trigger value (0.0–1.0) along the `direction` axis.

**`trigger_mode: true` (binary)**: Only one frame is shown at a time — background when released, foreground when the trigger value exceeds ~0.1.

`direction` is only used in progressive mode and is ignored when `trigger_mode` is `true`.

```json
{
    "type": 6,
    "id": "Left Trigger",
    "pos": [167, 0],
    "z_level": 1,
    "mapping": [1112, 792, 187, 104],
    "side": 0,
    "direction": 1,
    "trigger_mode": false
}
```

---

### Type 7 — `ET_GAMEPAD_ID` (player number indicator) — **Supported**

Displays the current player number. No extra fields.

**Sprite layout**: 5 sprites arranged **horizontally** in the texture atlas, each `w+3` px apart:

| Offset | Meaning |
|--------|---------|
| `[u, v, w, h]` | Player 1 (base / default) |
| `[u+(w+3), v, w, h]` | Player 2 |
| `[u+(w+3)*2, v, w, h]` | Player 3 |
| `[u+(w+3)*3, v, w, h]` | Player 4 |
| `[u+(w+3)*4, v, w, h]` | Guide button pressed |

The player sprite and the guide-pressed sprite may be drawn simultaneously (guide-pressed is drawn on top).

```json
{
    "type": 7,
    "id": "Player ID",
    "pos": [600, 300],
    "z_level": 1,
    "mapping": [0, 900, 40, 40]
}
```

---

### Type 8 — `ET_DPAD_STICK` (composite D-pad) — **Supported**

Composite D-pad element. No extra fields; the direction is computed from the current D-pad button state.

**Sprite layout**: 9 sprites arranged **horizontally** in the texture atlas, each `w+3` px apart:

| Offset | Direction |
|--------|-----------|
| `[u, v, w, h]` | Center (neutral — no direction held) |
| `[u+(w+3), v, w, h]` | Left |
| `[u+(w+3)*2, v, w, h]` | Right |
| `[u+(w+3)*3, v, w, h]` | Up |
| `[u+(w+3)*4, v, w, h]` | Down |
| `[u+(w+3)*5, v, w, h]` | Up-Left |
| `[u+(w+3)*6, v, w, h]` | Up-Right |
| `[u+(w+3)*7, v, w, h]` | Down-Left |
| `[u+(w+3)*8, v, w, h]` | Down-Right |

Only the center sprite (neutral) is drawn when no direction is active. One of the 8 directional sprites is drawn when a direction (or diagonal) is active. Center and directional sprites are mutually exclusive.

```json
{
    "type": 8,
    "id": "D-Pad",
    "pos": [136, 281],
    "z_level": 1,
    "mapping": [1, 1013, 229, 220]
}
```

---

### Type 9 — `ET_MOUSE_MOVEMENT` (mouse movement indicator) — **Supported**

| Field | Type | Description |
|-------|------|-------------|
| `mouse_radius` | integer | Maximum pixel offset from center position (used in Move mode) |
| `mouse_type` | integer | `0` = Move (sprite translates within radius), `1` = Point (sprite rotates to face movement direction) |

**Move mode** (`mouse_type: 0`): The sprite is drawn offset from `pos` by `(moveDelta.x × mouse_radius, moveDelta.y × mouse_radius)` pixels, where `moveDelta` is the normalised `[-1, 1]` movement vector for the current tick.

**Point mode** (`mouse_type: 1`): The sprite is drawn at `pos` rotated by `atan2(moveDelta.y, moveDelta.x)` radians. When there is no movement the sprite is drawn without rotation.

The movement delta is normalised by dividing raw pixel deltas by a configurable sensitivity value (URL parameter `mouse_sens`, default `500`). Larger values make the indicator less sensitive.

**Input source**: Windows Raw Input API (global capture).

---

## Texture Atlas Layout Summary

### Vertical two-frame (button / trigger / stick)

Used by: `ET_KEYBOARD_KEY`, `ET_GAMEPAD_BUTTON`, `ET_MOUSE_BUTTON`, `ET_ANALOG_STICK`, `ET_TRIGGER`

```
y ──► [u, v,     w, h]   normal / released state
      (3 px gap)
      [u, v+h+3, w, h]   pressed / active state
```

### Horizontal multi-frame (composite elements)

Used by: `ET_WHEEL` (4 frames), `ET_GAMEPAD_ID` (5 frames), `ET_DPAD_STICK` (9 frames)

```
x ──► [u,           v, w, h]   frame 0
      [u+(w+3),     v, w, h]   frame 1
      [u+(w+3)*2,   v, w, h]   frame 2
      ...
```

The `mapping` field in the JSON always points to **frame 0**. All other frame positions are computed at load time.

---

## Creating Custom Presets

### Using the CCT (Config Creation Tool)

The official CCT is a web-based tool that generates Input Overlay config files visually. It is distributed as part of the Input Overlay project:

- Open `docs/cct/index.html` from the [input-overlay repository](https://github.com/univrsal/input-overlay)

### Manual Creation

1. Create a PNG texture atlas with all button sprites, following the frame layout conventions above.
2. Write a JSON file referencing each sprite's base `[u, v, w, h]` crop and screen `pos`.
3. Set `overlay_width` / `overlay_height` to the coordinate space dimensions used for `pos` values.
4. Place both files in `overlays/<name>/` next to the InputView executable.
5. Open `http://localhost:8080/?overlay=<name>` in the browser.

### Coordinate System

- `pos` values use the overlay's own coordinate space (`overlay_width` × `overlay_height`), **not** the canvas pixel size.
- The renderer scales the entire overlay to fit the browser canvas while preserving aspect ratio.
- `mapping` values are raw pixel coordinates in the texture atlas PNG (no scaling applied).
