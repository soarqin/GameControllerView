// ============================================================
// Constants & Lookup Tables
// ============================================================

const RECONNECT_DELAY_INITIAL = 1000;
const RECONNECT_DELAY_MAX = 10000;
const CANVAS_WIDTH = 500;
const CANVAS_HEIGHT = 330;
const TRIGGER_PRESS_THRESHOLD = 0.1;
const OVERLAY_SPRITE_BORDER = 3;
const MAX_PLAYER_INDEX = 16;
const WHEEL_TIMEOUT_MS = 200;

// Maps key names (used in keyboard layout configs) to uiohook scancodes.
// Values match exactly what keycode.go:RawScanToUIohook() produces:
//   - Regular keys (no E0): uiohook = makeCode directly
//   - E0 cursor keys (Up/Down/Left/Right, Clear): uiohook = 0xE000 | makeCode
//   - Other E0-extended keys (RCtrl, RAlt, Insert, etc.): uiohook = 0x0E00 | makeCode
const KEY_NAME_TO_SCANCODE = {
    // Letters (makeCode direct, no E0)
    'A': 0x001E, 'B': 0x0030, 'C': 0x002E, 'D': 0x0020, 'E': 0x0012,
    'F': 0x0021, 'G': 0x0022, 'H': 0x0023, 'I': 0x0017, 'J': 0x0024,
    'K': 0x0025, 'L': 0x0026, 'M': 0x0032, 'N': 0x0031, 'O': 0x0018,
    'P': 0x0019, 'Q': 0x0010, 'R': 0x0013, 'S': 0x001F, 'T': 0x0014,
    'U': 0x0016, 'V': 0x002F, 'W': 0x0011, 'X': 0x002D, 'Y': 0x0015,
    'Z': 0x002C,
    // Number row (makeCode direct, no E0)
    '1': 0x0002, '2': 0x0003, '3': 0x0004, '4': 0x0005, '5': 0x0006,
    '6': 0x0007, '7': 0x0008, '8': 0x0009, '9': 0x000A, '0': 0x000B,
    // Function keys (makeCode direct, no E0)
    'F1': 0x003B, 'F2': 0x003C, 'F3': 0x003D, 'F4': 0x003E,
    'F5': 0x003F, 'F6': 0x0040, 'F7': 0x0041, 'F8': 0x0042,
    'F9': 0x0043, 'F10': 0x0044, 'F11': 0x0057, 'F12': 0x0058,
    // Modifier keys -- LShift/RShift/LAlt no E0; RCtrl/RAlt are E0+non-cursor
    'LShift': 0x002A, 'RShift': 0x0036,
    'LCtrl': 0x001D, 'RCtrl': 0x0E1D,
    'LAlt': 0x0038,  'RAlt': 0x0E38,
    // Special keys (no E0)
    'Space': 0x0039, 'Enter': 0x001C, 'Backspace': 0x000E,
    'Tab': 0x000F, 'Escape': 0x0001, 'CapsLock': 0x003A,
    // Arrow keys -- E0+cursor set
    'ArrowUp': 0xE048, 'ArrowDown': 0xE050,
    'ArrowLeft': 0xE04B, 'ArrowRight': 0xE04D,
    // Punctuation (no E0)
    'Minus': 0x000C, 'Equal': 0x000D,
    'BracketLeft': 0x001A, 'BracketRight': 0x001B,
    'Backslash': 0x002B, 'Semicolon': 0x0027,
    'Quote': 0x0028, 'Comma': 0x0033,
    'Period': 0x0034, 'Slash': 0x0035, 'Backquote': 0x0029,
    // Navigation cluster -- E0+non-cursor
    'Insert': 0x0E52, 'Delete': 0x0E53,
    'Home': 0x0E47, 'End': 0x0E4F,
    'PageUp': 0x0E49, 'PageDown': 0x0E51,
    // Other
    'PrintScreen': 0x0E37, 'ScrollLock': 0x0046, 'Pause': 0x0045,
    'NumLock': 0x0045,
};

// Maps key names to display labels shown on canvas key caps.
const KEY_DEFAULT_LABELS = {
    'Space': '\u2423', 'Enter': '\u21B5', 'Backspace': '\u232B', 'Tab': '\u21E5',
    'Escape': 'Esc', 'CapsLock': 'Caps',
    'LShift': '\u21E7', 'RShift': '\u21E7',
    'LCtrl': 'Ctrl', 'RCtrl': 'Ctrl',
    'LAlt': 'Alt', 'RAlt': 'Alt',
    'ArrowUp': '\u2191', 'ArrowDown': '\u2193',
    'ArrowLeft': '\u2190', 'ArrowRight': '\u2192',
    'Backquote': '`', 'Minus': '-', 'Equal': '=',
    'BracketLeft': '[', 'BracketRight': ']',
    'Backslash': '\\', 'Semicolon': ';', 'Quote': "'",
    'Comma': ',', 'Period': '.', 'Slash': '/',
    'Insert': 'Ins', 'Delete': 'Del',
    'Home': 'Home', 'End': 'End',
    'PageUp': 'PgUp', 'PageDown': 'PgDn',
    'PrintScreen': 'PrtSc', 'ScrollLock': 'ScrLk', 'Pause': 'Pause',
    'NumLock': 'NumLk',
};

// SDL2 gamepad button codes -> InputView state paths
const IO_BUTTON_CODE_MAP = {
    0:  s => s.buttons.a,
    1:  s => s.buttons.b,
    2:  s => s.buttons.x,
    3:  s => s.buttons.y,
    4:  s => s.buttons.back,
    5:  s => s.buttons.guide,
    6:  s => s.buttons.start,
    7:  s => s.sticks.left.pressed,
    8:  s => s.sticks.right.pressed,
    9:  s => s.buttons.lb,
    10: s => s.buttons.rb,
    11: s => s.dpad.up,
    12: s => s.dpad.down,
    13: s => s.dpad.left,
    14: s => s.dpad.right,
    15: s => s.buttons.capture,
    20: s => s.buttons.touchpad,
};

// Colors
const COLORS = {
    bg: '#16213e',
    outline: '#4a4a6a',
    outlineFill: '#1e2a4a',
    buttonDefault: '#2a2a4a',
    buttonPressed: '#4ade80',
    buttonLabel: '#a0a0b0',
    buttonLabelPressed: '#1a1a2e',
    stickBase: '#2a2a4a',
    stickKnob: '#6a6a8a',
    stickKnobPressed: '#4ade80',
    triggerBg: '#2a2a4a',
    triggerFill: '#4ade80',
    dpadBg: '#4a5568',
    dpadPressed: '#4ade80',
    textDim: '#6a6a8a',
    faceA: '#4ade80',
    faceB: '#f87171',
    faceX: '#60a5fa',
    faceY: '#fbbf24',
};

// Mouse device config (positions, sizes)
const MOUSE_CONFIG = {
    canvasW: 160,
    canvasH: 270,
    body: {
        path: 'M 80,15 C 40,15 20,35 18,70 L 15,180 C 14,210 25,250 80,255 C 135,250 146,210 145,180 L 142,70 C 140,35 120,15 80,15 Z',
        viewBox: '0 0 160 270',
        x: 0, y: 0, width: 160, height: 270
    },
    buttons: {
        left:   { x: 5,  y: 15,  width: 65,  height: 80, radius: 8,  label: 'L',  code: 1 },
        right:  { x: 90, y: 15,  width: 65,  height: 80, radius: 8,  label: 'R',  code: 2 },
        middle: { x: 62, y: 15,  width: 36,  height: 55, radius: 6,  label: 'M',  code: 3 },
        x1:     { x: 5,  y: 150, width: 22,  height: 35, radius: 5,  label: 'X1', code: 4 },
        x2:     { x: 5,  y: 110, width: 22,  height: 35, radius: 5,  label: 'X2', code: 5 },
    },
    wheel: {
        x: 68, y: 18, width: 24, height: 50, radius: 12
    },
    movement: {
        x: 80, y: 210, radius: 40
    }
};
