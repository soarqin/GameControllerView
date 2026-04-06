// ============================================================
// State Management
// ============================================================

const state = {
    connected: false,
    controllerType: '',
    name: '',
    buttons: { a: false, b: false, x: false, y: false, lb: false, rb: false, back: false, start: false, guide: false, touchpad: false, capture: false },
    dpad: { up: false, down: false, left: false, right: false },
    sticks: {
        left: { position: { x: 0, y: 0 }, pressed: false },
        right: { position: { x: 0, y: 0 }, pressed: false }
    },
    triggers: {
        lt: { value: 0 },
        rt: { value: 0 }
    }
};

let currentConfig = null;
const configCache = {};
const keyboardConfigCache = {};

// Simple mode: only draw controller elements, no background
let simpleMode = false;
// Body alpha channel (0-1)
let bodyAlpha = 1.0;
// Selected player index (1-based, default 1)
let selectedPlayerIndex = 1;
// Mouse sensitivity divisor (sent to backend after connect)
let mouseSens = 0; // 0 = not set by URL param

// Input Overlay mode: name of the overlay config to load (null = use built-in geometric renderer)
let overlayName = null;
// Input Overlay runtime state
let overlayConfig = null;   // parsed JSON from Input Overlay config file
let overlayTexture = null;  // HTMLImageElement of the texture atlas
let overlayReady = false;   // true once both config and texture are loaded
let overlayLoadFailed = false;  // true if texture failed to load

// Overlay content flags — set once the config JSON is parsed.
// overlayHasGamepad: config contains at least one gamepad element (type 2/5/6/7/8)
// overlayHasKM:      config contains at least one keyboard/mouse element (type 1/3/4/9)
// When overlayHasGamepad is false:
//   - the p= URL parameter is ignored
//   - select_player is not sent to the backend
//   - the Player / controller info bar is hidden
let overlayHasGamepad = true;  // default true until config tells us otherwise
let overlayHasKM = false;

// Keyboard and mouse state (populated from km_full / km_delta WebSocket messages)
const kmState = {
    keys: {},           // uiohook scancode (number) → boolean (pressed)
    mouseButtons: {},   // IO button code (1-5) → boolean (pressed)
    mouseMove: { x: 0, y: 0 },    // normalised [-1,1] movement delta from current tick
    wheelUp: false,
    wheelDown: false,
    wheelTimestamp: 0,  // ms timestamp of last wheel event (for timeout reset)
};
// How long (ms) to keep wheel state active before resetting to neutral
const WHEEL_TIMEOUT_MS = 200;
const RECONNECT_DELAY_INITIAL = 1000;
const RECONNECT_DELAY_MAX = 10000;
const CANVAS_WIDTH = 500;
const CANVAS_HEIGHT = 330;
const TRIGGER_PRESS_THRESHOLD = 0.1;
const OVERLAY_SPRITE_BORDER = 3;
const MAX_PLAYER_INDEX = 16;

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
    // Modifier keys — LShift/RShift/LAlt no E0; RCtrl/RAlt are E0+non-cursor → 0x0E00|makeCode
    'LShift': 0x002A, 'RShift': 0x0036,
    'LCtrl': 0x001D, 'RCtrl': 0x0E1D,   // E0+0x1D → 0x0E00|0x1D
    'LAlt': 0x0038,  'RAlt': 0x0E38,    // E0+0x38 → 0x0E00|0x38
    // Special keys (no E0)
    'Space': 0x0039, 'Enter': 0x001C, 'Backspace': 0x000E,
    'Tab': 0x000F, 'Escape': 0x0001, 'CapsLock': 0x003A,
    // Arrow keys — E0+cursor set → 0xE000|makeCode
    'ArrowUp': 0xE048, 'ArrowDown': 0xE050,
    'ArrowLeft': 0xE04B, 'ArrowRight': 0xE04D,
    // Punctuation (no E0)
    'Minus': 0x000C, 'Equal': 0x000D,
    'BracketLeft': 0x001A, 'BracketRight': 0x001B,
    'Backslash': 0x002B, 'Semicolon': 0x0027,
    'Quote': 0x0028, 'Comma': 0x0033,
    'Period': 0x0034, 'Slash': 0x0035, 'Backquote': 0x0029,
    // Navigation cluster — E0+non-cursor → 0x0E00|makeCode
    'Insert': 0x0E52, 'Delete': 0x0E53,
    'Home': 0x0E47, 'End': 0x0E4F,
    'PageUp': 0x0E49, 'PageDown': 0x0E51,
    // Other
    'PrintScreen': 0x0E37, 'ScrollLock': 0x0046, 'Pause': 0x0045,
    'NumLock': 0x0045,
};

// Maps key names to display labels shown on canvas key caps.
const KEY_DEFAULT_LABELS = {
    'Space': '␣', 'Enter': '↵', 'Backspace': '⌫', 'Tab': '⇥',
    'Escape': 'Esc', 'CapsLock': 'Caps',
    'LShift': '⇧', 'RShift': '⇧',
    'LCtrl': 'Ctrl', 'RCtrl': 'Ctrl',
    'LAlt': 'Alt', 'RAlt': 'Alt',
    'ArrowUp': '↑', 'ArrowDown': '↓',
    'ArrowLeft': '←', 'ArrowRight': '→',
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

// Whether we have already subscribed to keyboard/mouse events for this session
let kmSubscribed = false;

// Dirty flag: set to true whenever state changes. render() only redraws when dirty.
let dirty = true;

let activeRenderers = [];
let explicitMode = false;
let hasGamepadParam = false;
let hasMouseParam = false;
let keyboardParam = null;
let forcedGamepadType = null;

function markRendererDirty(types) {
    for (const renderer of activeRenderers) {
        if (types.includes(renderer.type)) renderer.dirty = true;
    }
}

function enforceForcedGamepadType() {
    if (explicitMode && forcedGamepadType && forcedGamepadType !== 'true' && forcedGamepadType !== '') {
        state.controllerType = forcedGamepadType;
    }
}

// ============================================================
// WebSocket Connection
// ============================================================

let ws = null;
let reconnectDelay = RECONNECT_DELAY_INITIAL;
const maxReconnectDelay = RECONNECT_DELAY_MAX;

function connectWebSocket() {
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${location.host}/ws`;

    ws = new WebSocket(url);

    ws.onopen = () => {
        reconnectDelay = RECONNECT_DELAY_INITIAL;
        setWSStatus(true);
        // Send selected player index to backend, unless the overlay has no gamepad elements
        // (in that case we don't need gamepad data at all).
        if ((overlayName === null && (!explicitMode || hasGamepadParam)) || (overlayName !== null && overlayHasGamepad)) {
            ws.send(JSON.stringify({ type: 'select_player', playerIndex: selectedPlayerIndex }));
        }

        // Send mouse sensitivity if user provided it via URL parameter
        if (mouseSens > 0) {
            ws.send(JSON.stringify({ type: 'set_mouse_sens', value: mouseSens }));
        }

        // Re-subscribe to keyboard/mouse if we already determined that the overlay needs it
        if (kmSubscribed) {
            ws.send(JSON.stringify({ type: 'subscribe_km' }));
        }

        if (explicitMode && (hasMouseParam || keyboardParam !== null) && !kmSubscribed) {
            kmSubscribed = true;
            ws.send(JSON.stringify({ type: 'subscribe_km' }));
        }
    };

    ws.onclose = () => {
        setWSStatus(false);
        scheduleReconnect();
    };

    ws.onerror = () => {
        ws.close();
    };

    ws.onmessage = (event) => {
        try {
            handleMessage(JSON.parse(event.data));
        } catch (e) {
            console.error('Failed to parse message:', e);
        }
    };
}

function scheduleReconnect() {
    setTimeout(() => {
        connectWebSocket();
        reconnectDelay = Math.min(reconnectDelay * 1.5, maxReconnectDelay);
    }, reconnectDelay);
}

let wsConnected = false;

function setWSStatus(connected) {
    wsConnected = connected;
    updateStatusIndicator();
}

function updateStatusIndicator() {
    // In overlay mode with no gamepad elements, the status indicator is hidden — skip update.
    if (overlayName !== null && !overlayHasGamepad) return;
    const dot = document.getElementById('ws-status');
    const text = document.getElementById('ws-text');
    if (!wsConnected) {
        dot.className = 'status-dot disconnected';
        text.textContent = 'Server Disconnected';
    } else if (!state.connected) {
        dot.className = 'status-dot disconnected';
        text.textContent = 'No Controller';
    } else {
        dot.className = 'status-dot connected';
        text.textContent = 'Controller Connected';
    }
}

// ============================================================
// Message Handling
// ============================================================

function handleMessage(msg) {
    switch (msg.type) {
        case 'full':
            if (msg.data) applyFullState(msg.data);
            break;
        case 'delta':
            if (msg.changes) applyDelta(msg.changes);
            break;
        case 'player_selected':
            // Acknowledged — nothing to do on frontend
            break;
        case 'km_full':
            if (msg.kmState) applyKMFull(msg.kmState);
            break;
        case 'km_delta':
            if (msg.kmDelta) applyKMDelta(msg.kmDelta);
            break;
    }
}

// Apply a full keyboard/mouse state snapshot.
function applyKMFull(data) {
    // Rebuild keys map
    for (const k in kmState.keys) delete kmState.keys[k];
    if (data.keys) {
        for (const [code, pressed] of Object.entries(data.keys)) {
            if (pressed) kmState.keys[Number(code)] = true;
        }
    }
    // Rebuild mouse buttons map
    for (const k in kmState.mouseButtons) delete kmState.mouseButtons[k];
    if (data.mouseButtons) {
        for (const [code, pressed] of Object.entries(data.mouseButtons)) {
            if (pressed) kmState.mouseButtons[Number(code)] = true;
        }
    }
    // Mouse movement and wheel are per-tick — not meaningful in a full snapshot
    kmState.mouseMove.x = 0;
    kmState.mouseMove.y = 0;
    kmState.wheelUp = false;
    kmState.wheelDown = false;
    dirty = true;
    markRendererDirty(['mouse', 'keyboard']);
}

// Apply an incremental keyboard/mouse delta.
function applyKMDelta(delta) {
    if (delta.keysDown) {
        for (const code of delta.keysDown) kmState.keys[code] = true;
    }
    if (delta.keysUp) {
        for (const code of delta.keysUp) delete kmState.keys[code];
    }
    if (delta.buttonsDown) {
        for (const code of delta.buttonsDown) kmState.mouseButtons[code] = true;
    }
    if (delta.buttonsUp) {
        for (const code of delta.buttonsUp) delete kmState.mouseButtons[code];
    }
    kmState.mouseMove.x = delta.mouseMove ? delta.mouseMove.x : 0;
    kmState.mouseMove.y = delta.mouseMove ? delta.mouseMove.y : 0;
    if (delta.wheelUp || delta.wheelDown) {
        kmState.wheelUp = delta.wheelUp || false;
        kmState.wheelDown = delta.wheelDown || false;
        kmState.wheelTimestamp = performance.now();
    }
    dirty = true;
    markRendererDirty(['mouse', 'keyboard']);
}

// Merge gamepad state fields. For delta messages some fields may be absent (undefined).
// Pointer-typed delta fields (buttons, dpad, sticks, triggers) are always full objects
// when present — replace the whole sub-object, not individual properties.
function mergeState(target, source) {
    if (source.connected !== undefined) target.connected = source.connected;
    if (source.controllerType !== undefined) target.controllerType = source.controllerType;
    if (source.name !== undefined) target.name = source.name;
    if (source.buttons) Object.assign(target.buttons, source.buttons);
    if (source.dpad) Object.assign(target.dpad, source.dpad);
    if (source.sticks) {
        if (source.sticks.left) {
            if (source.sticks.left.position) Object.assign(target.sticks.left.position, source.sticks.left.position);
            if (source.sticks.left.pressed !== undefined) target.sticks.left.pressed = source.sticks.left.pressed;
        }
        if (source.sticks.right) {
            if (source.sticks.right.position) Object.assign(target.sticks.right.position, source.sticks.right.position);
            if (source.sticks.right.pressed !== undefined) target.sticks.right.pressed = source.sticks.right.pressed;
        }
    }
    // triggers: backend sends the complete TriggersState object when either value changes.
    if (source.triggers) {
        if (source.triggers.lt !== undefined) target.triggers.lt.value = source.triggers.lt.value ?? 0;
        if (source.triggers.rt !== undefined) target.triggers.rt.value = source.triggers.rt.value ?? 0;
    }
}

function applyFullState(data) {
    mergeState(state, data);
    enforceForcedGamepadType();
    updateControllerInfo();
    loadConfigIfNeeded();
    dirty = true;
    markRendererDirty(['gamepad']);
}

function applyDelta(changes) {
    mergeState(state, changes);
    enforceForcedGamepadType();
    updateControllerInfo();
    loadConfigIfNeeded();
    dirty = true;
    markRendererDirty(['gamepad']);
}

function updateControllerInfo() {
    // In overlay mode with no gamepad elements, the info bar is hidden — skip update.
    if (overlayName !== null && !overlayHasGamepad) return;
    const el = document.getElementById('controller-name');
    if (state.connected && state.name) {
        el.textContent = `Player ${selectedPlayerIndex}: ${state.name} (${state.controllerType})`;
    } else {
        el.textContent = `Player ${selectedPlayerIndex}: No controller detected`;
    }
    updateStatusIndicator();
}

// ============================================================
// Configuration Loading (geometric renderer)
// ============================================================

let loadedConfigType = '';

function configNameForType(type) {
    const configMap = {
        'xbox': 'xbox',
        'playstation': 'playstation',
        'playstation5': 'playstation',
        'switch_pro': 'switch_pro',
    };
    return configMap[type] || 'xbox';
}

function loadConfigIfNeeded() {
    // In Input Overlay mode, skip built-in config loading
    if (overlayName !== null) return;
    if (explicitMode && !hasGamepadParam) return;

    const type = state.controllerType || 'xbox';
    if (type === loadedConfigType) return;

    loadedConfigType = type;

    if (configCache[type]) {
        currentConfig = configCache[type];
        return;
    }

    const configName = configNameForType(type);

    fetch(`configs/${configName}.json`)
        .then(r => {
            if (!r.ok) throw new Error('HTTP ' + r.status);
            return r.json();
        })
        .then(config => {
            configCache[type] = config;
            currentConfig = config;
            dirty = true;
            markRendererDirty(['gamepad']);
        })
        .catch(e => {
            console.warn('Config load failed:', configName, e);
            // Fallback to xbox
            if (type !== 'xbox') {
                loadedConfigType = '';
                state.controllerType = 'xbox';
                loadConfigIfNeeded();
            }
        });
}

// ============================================================
// Keyboard Config Loading
// ============================================================

function loadKeyboardConfig(name) {
    if (keyboardConfigCache[name]) {
        applyKeyboardConfig(name, keyboardConfigCache[name]);
        return;
    }
    // Try external keyboards/ directory first (user overrides embedded)
    fetch('/keyboards/' + encodeURIComponent(name) + '.json')
        .then(r => {
            if (!r.ok) throw new Error('HTTP ' + r.status);
            return r.json();
        })
        .then(config => {
            keyboardConfigCache[name] = config;
            applyKeyboardConfig(name, config);
        })
        .catch(() => {
            // Fallback: try embedded configs/keyboard_{name}.json
            fetch('configs/keyboard_' + name + '.json')
                .then(r => {
                    if (!r.ok) throw new Error('HTTP ' + r.status);
                    return r.json();
                })
                .then(config => {
                    keyboardConfigCache[name] = config;
                    applyKeyboardConfig(name, config);
                })
                .catch(e => {
                    console.error('[InputView] Failed to load keyboard config:', name, e);
                });
        });
}

function computeKeyboardDimensions(config) {
    const keySize = config.keySize || 40;
    const gap = config.gap || 4;
    const padding = config.padding || 8;
    const rows = config.rows || [];

    let maxRowWidth = 0;
    for (const row of rows) {
        let rowWidth = 0;
        for (const entry of row) {
            if (entry.pad !== undefined) {
                rowWidth += entry.pad * (keySize + gap);
            } else {
                const w = (entry.width || 1.0) * keySize + ((entry.width || 1.0) - 1) * gap;
                rowWidth += w + gap;
            }
        }
        if (rowWidth > 0) rowWidth -= gap; // remove trailing gap
        if (rowWidth > maxRowWidth) maxRowWidth = rowWidth;
    }

    const canvasW = Math.ceil(maxRowWidth + 2 * padding);
    const canvasH = Math.ceil(rows.length * (keySize + gap) - gap + 2 * padding);
    return { canvasW, canvasH };
}

function applyKeyboardConfig(name, config) {
    // Find keyboard renderer in activeRenderers
    const renderer = activeRenderers.find(r => r.type === 'keyboard' && r.configName === name);
    if (!renderer) return;

    const { canvasW, canvasH } = computeKeyboardDimensions(config);
    renderer.canvasW = canvasW;
    renderer.canvasH = canvasH;
    renderer.keyboardConfig = config;
    renderer.keyboardLayout = null; // will be computed by drawKeyboardRenderer

    // Resize canvas DOM element
    renderer.canvas.width = canvasW;
    renderer.canvas.height = canvasH;
    setupRendererCanvas(renderer);
    renderer.dirty = true;
}

// ============================================================
// Input Overlay: Config + Texture Loading
// ============================================================

// SDL2 gamepad button codes → InputView state paths
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

function ioButtonPressed(code) {
    const getter = IO_BUTTON_CODE_MAP[code];
    return getter ? !!getter(state) : false;
}

function analyzeOverlayContent(cfg) {
    const gpTypes = new Set([2, 5, 6, 7, 8]);
    const kmTypes = new Set([1, 3, 4, 9]);
    const elements = cfg.elements || [];
    overlayHasGamepad = elements.some(el => gpTypes.has(el.type));
    overlayHasKM = elements.some(el => kmTypes.has(el.type));

    applyGamepadUIVisibility();

    if (overlayHasKM && !kmSubscribed) {
        kmSubscribed = true;
        if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'subscribe_km' }));
        }
    }
}

function applyGamepadUIVisibility() {
    if (overlayName === null) return;
    if (simpleMode) return;
    const infoBar = document.getElementById('controller-info');
    const statusEl = document.getElementById('status');
    if (!overlayHasGamepad) {
        if (infoBar) infoBar.style.display = 'none';
        if (statusEl) statusEl.style.display = 'none';
    } else {
        if (infoBar) infoBar.style.display = '';
        if (statusEl) statusEl.style.display = '';
    }
}

// Load an Input Overlay config.
// name may be a plain overlay name ("dualsense") or a variant path ("dualsense/compact").
function loadInputOverlayConfig(name) {
    overlayReady = false;
    overlayConfig = null;
    overlayTexture = null;

    const slashIdx = name.indexOf('/');
    let dirName, jsonName;
    if (slashIdx >= 0) {
        dirName = name.substring(0, slashIdx);
        jsonName = name.substring(slashIdx + 1);
    } else {
        dirName = name;
        jsonName = name;
    }

    const baseUrl = `/overlays/${encodeURIComponent(dirName)}/`;
    const jsonUrl = `${baseUrl}${encodeURIComponent(jsonName)}.json`;
    // PNG texture atlas is always named after the directory (shared by all variants).
    const pngUrl = `${baseUrl}${encodeURIComponent(dirName)}.png`;

    fetch(jsonUrl)
        .then(r => {
            if (!r.ok) throw new Error(`HTTP ${r.status}`);
            return r.json();
        })
        .then(cfg => {
            if (!cfg.overlay_width || !cfg.overlay_height || cfg.overlay_width <= 0 || cfg.overlay_height <= 0) {
                console.warn('Invalid overlay config: missing or invalid overlay_width/overlay_height');
            }
            if (cfg.elements && !Array.isArray(cfg.elements)) {
                console.warn('Invalid overlay config: elements must be an array');
            }
            overlayConfig = cfg;
            analyzeOverlayContent(cfg);
            if (cfg.overlay_width && cfg.overlay_height) {
                canvasW = cfg.overlay_width;
                canvasH = cfg.overlay_height;
                setupCanvas();
            }
            // Sort elements by z_level once at load time, not every frame.
            if (cfg.elements) {
                cfg.elements.sort((a, b) => (Number(a.z_level) || 0) - (Number(b.z_level) || 0));
            }
            const onTextureLoaded = (loadedImg, src) => {
                overlayTexture = loadedImg;
                overlayReady = true;
                dirty = true;
                console.log(`Input Overlay '${name}' loaded from ${src} (${cfg.overlay_width}x${cfg.overlay_height})`);
            };
            const onAllFailed = () => {
                overlayLoadFailed = true;
                dirty = true;
                console.error('Failed to load overlay texture (tried PNG and SVG):', pngUrl);
                const statusEl = document.getElementById('status');
                if (statusEl) {
                    statusEl.textContent = `Error: failed to load texture ${dirName}`;
                    statusEl.style.display = '';
                }
            };
            const img = new Image();
            img.onload = () => onTextureLoaded(img, pngUrl);
            img.onerror = () => {
                // PNG failed — try SVG fallback
                const svgUrl = `${baseUrl}${encodeURIComponent(dirName)}.svg`;
                const svgImg = new Image();
                svgImg.onload = () => onTextureLoaded(svgImg, svgUrl);
                svgImg.onerror = onAllFailed;
                svgImg.src = svgUrl;
            };
            img.src = pngUrl;
        })
        .catch(err => {
            console.error(`Failed to load Input Overlay config '${name}': ${err}`);
        });
}

// ============================================================
// Input Overlay: Texture Atlas Renderer
// ============================================================

// Compute D-pad direction index (0=neutral, 1=left, 2=right, 3=up, 4=down,
// 5=up-left, 6=up-right, 7=down-left, 8=down-right)
function ioDpadIndex() {
    const { up, down, left, right } = state.dpad;
    if (!up && !down && !left && !right) return 0;
    if (up && left)  return 5;
    if (up && right) return 6;
    if (down && left)  return 7;
    if (down && right) return 8;
    if (left)  return 1;
    if (right) return 2;
    if (up)    return 3;
    return 4;
}

function ioDrawSprite(sx, sy, sw, sh, dx, dy, dw, dh) {
    if (!overlayTexture || sw <= 0 || sh <= 0 || dw <= 0 || dh <= 0) return;
    ctx.drawImage(overlayTexture, sx, sy, sw, sh, dx, dy, dw, dh);
}

function ioDrawTriggerFill(sx, sy, sw, sh, dx, dy, dw, dh, fillRatio, direction) {
    if (!overlayTexture || fillRatio <= 0) return;
    ctx.save();
    ctx.beginPath();
    switch (direction) {
        case 1: ctx.rect(dx, dy + dh * (1 - fillRatio), dw, dh * fillRatio); break;
        case 2: ctx.rect(dx, dy, dw, dh * fillRatio); break;
        case 3: ctx.rect(dx + dw * (1 - fillRatio), dy, dw * fillRatio, dh); break;
        case 4: ctx.rect(dx, dy, dw * fillRatio, dh); break;
        default: ctx.rect(dx, dy, dw, dh * fillRatio);
    }
    ctx.clip();
    ctx.drawImage(overlayTexture, sx, sy, sw, sh, dx, dy, dw, dh);
    ctx.restore();
}

function drawInputOverlay() {
     if (!overlayConfig) {
         if (!simpleMode && overlayHasGamepad) drawDisconnected();
         return;
     }
     if (overlayLoadFailed) {
         ctx.fillStyle = '#ff0000';
         ctx.font = '16px sans-serif';
         ctx.textAlign = 'center';
         ctx.fillText('Texture failed to load', canvasW / 2, canvasH / 2);
         return;
     }
     if (!overlayReady) return;
 
     const elements = overlayConfig.elements || [];

    for (const el of elements) {
        const type = el.type;
        if (!el.mapping || el.mapping.length < 4) continue;

        const [mu, mv, mw, mh] = el.mapping;
        const [px, py] = el.pos || [0, 0];
        const dx = px, dy = py, dw = mw, dh = mh;

        switch (type) {
            case 0: {
                ioDrawSprite(mu, mv, mw, mh, dx, dy, dw, dh);
                break;
            }
             case 1: {
                 // Keyboard key
                 const sv = kmState.keys[el.code] ? mv + mh + OVERLAY_SPRITE_BORDER : mv;
                 ioDrawSprite(mu, sv, mw, mh, dx, dy, dw, dh);
                 break;
             }
             case 2: {
                 // Gamepad button (digital)
                 const sv = ioButtonPressed(el.code) ? mv + mh + OVERLAY_SPRITE_BORDER : mv;
                 ioDrawSprite(mu, sv, mw, mh, dx, dy, dw, dh);
                 break;
             }
             case 3: {
                 // Mouse button
                 const sv = kmState.mouseButtons[el.code] ? mv + mh + OVERLAY_SPRITE_BORDER : mv;
                 ioDrawSprite(mu, sv, mw, mh, dx, dy, dw, dh);
                 break;
             }
             case 4: {
                 // Mouse wheel — 4 horizontal frames: neutral / middle-pressed / scroll-up / scroll-down
                 const now = performance.now();
                 const wheelExpired = (now - kmState.wheelTimestamp) > WHEEL_TIMEOUT_MS;
                 if (wheelExpired) {
                     kmState.wheelUp = false;
                     kmState.wheelDown = false;
                 }
                 ioDrawSprite(mu, mv, mw, mh, dx, dy, dw, dh);
                 if (kmState.mouseButtons[3]) {
                     ioDrawSprite(mu + (mw + OVERLAY_SPRITE_BORDER), mv, mw, mh, dx, dy, dw, dh);
                 }
                 if (kmState.wheelUp && !wheelExpired) {
                     ioDrawSprite(mu + (mw + OVERLAY_SPRITE_BORDER) * 2, mv, mw, mh, dx, dy, dw, dh);
                 }
                 if (kmState.wheelDown && !wheelExpired) {
                     ioDrawSprite(mu + (mw + OVERLAY_SPRITE_BORDER) * 3, mv, mw, mh, dx, dy, dw, dh);
                 }
                 break;
             }
             case 5: {
                 // Analog stick
                 const side = el.side === 1 ? 'right' : 'left';
                 const stickState = state.sticks[side];
                 const radius = el.stick_radius || 40;
                 const kx = stickState.position.x * radius;
                 const ky = -stickState.position.y * radius;
                 const sv = stickState.pressed ? mv + mh + OVERLAY_SPRITE_BORDER : mv;
                 ioDrawSprite(mu, sv, mw, mh, dx + kx, dy + ky, dw, dh);
                 break;
             }
             case 6: {
                 // Trigger
                 const side = el.side === 1 ? 'rt' : 'lt';
                 const value = state.triggers[side].value;
                 const direction = el.direction || 1;
                 if (el.trigger_mode === true) {
                     const sv = value > TRIGGER_PRESS_THRESHOLD ? mv + mh + OVERLAY_SPRITE_BORDER : mv;
                     ioDrawSprite(mu, sv, mw, mh, dx, dy, dw, dh);
                 } else {
                     ioDrawSprite(mu, mv, mw, mh, dx, dy, dw, dh);
                     if (value > 0) {
                         ioDrawTriggerFill(mu, mv + mh + OVERLAY_SPRITE_BORDER, mw, mh, dx, dy, dw, dh, value, direction);
                     }
                 }
                 break;
             }
             case 7: {
                 // Gamepad player ID / guide button indicator
                 const playerIdx = Math.min(Math.max((selectedPlayerIndex || 1) - 1, 0), 3);
                 if (state.buttons.guide) {
                     ioDrawSprite(mu + 4 * (mw + OVERLAY_SPRITE_BORDER), mv, mw, mh, dx, dy, dw, dh);
                 }
                 ioDrawSprite(mu + playerIdx * (mw + OVERLAY_SPRITE_BORDER), mv, mw, mh, dx, dy, dw, dh);
                 break;
             }
             case 8: {
                 // Composite D-pad
                 const dsx = mu + ioDpadIndex() * (mw + OVERLAY_SPRITE_BORDER);
                 ioDrawSprite(dsx, mv, mw, mh, dx, dy, dw, dh);
                 break;
             }
            case 9: {
                // Mouse movement indicator
                const radius = el.mouse_radius || 40;
                const mx = kmState.mouseMove.x;
                const my = kmState.mouseMove.y;
                if (el.mouse_type === 0) {
                    ioDrawSprite(mu, mv, mw, mh, dx + mx * radius, dy + my * radius, dw, dh);
                } else {
                    if (mx === 0 && my === 0) {
                        ioDrawSprite(mu, mv, mw, mh, dx, dy, dw, dh);
                    } else {
                        const angle = Math.atan2(mx, -my);
                        ctx.save();
                        ctx.translate(dx + dw / 2, dy + dh / 2);
                        ctx.rotate(angle);
                        ctx.drawImage(overlayTexture, mu, mv, mw, mh, -dw / 2, -dh / 2, dw, dh);
                        ctx.restore();
                    }
                }
                break;
            }
        }
    }
}

// ============================================================
// Canvas Rendering
// ============================================================

const canvas = document.getElementById('gamepad-canvas');
let ctx = canvas.getContext('2d');

// Logical canvas dimensions.
// In geometric mode: fixed CANVAS_WIDTH × CANVAS_HEIGHT.
// In overlay mode: set to overlay_width × overlay_height once the config is loaded.
let canvasW = CANVAS_WIDTH;
let canvasH = CANVAS_HEIGHT;

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

// Pre-computed body fill color (recalculated when bodyAlpha changes)
let bodyFillColor = '';
function updateBodyFillColor() {
    const r = parseInt(COLORS.outlineFill.slice(1, 3), 16);
    const g = parseInt(COLORS.outlineFill.slice(3, 5), 16);
    const b = parseInt(COLORS.outlineFill.slice(5, 7), 16);
    bodyFillColor = `rgba(${r},${g},${b},${bodyAlpha})`;
}

// High-DPI canvas setup
function setupCanvas() {
    const dpr = window.devicePixelRatio || 1;

    if (simpleMode) {
        const vw = window.innerWidth;
        const vh = window.innerHeight;
        canvas.style.width = vw + 'px';
        canvas.style.height = vh + 'px';
        canvas.width = vw * dpr;
        canvas.height = vh * dpr;

        const scale = Math.min(vw / canvasW, vh / canvasH);
        const offsetX = (vw - canvasW * scale) / 2;
        const offsetY = (vh - canvasH * scale) / 2;
        ctx.setTransform(dpr * scale, 0, 0, dpr * scale, dpr * offsetX, dpr * offsetY);
    } else {
        if (overlayName !== null) {
            canvas.style.width  = canvasW + 'px';
            canvas.style.height = canvasH + 'px';
        }

        const cssWidth = canvas.getBoundingClientRect().width;
        const scale = cssWidth / canvasW;
        const cssHeight = canvasH * scale;

        canvas.style.height = cssHeight + 'px';
        canvas.width  = cssWidth  * dpr;
        canvas.height = cssHeight * dpr;

        ctx.setTransform(dpr * scale, 0, 0, dpr * scale, 0, 0);
    }
    dirty = true;
}

function setupRendererNormal(renderer) {
    const dpr = window.devicePixelRatio || 1;
    const cssWidth = renderer.canvas.getBoundingClientRect().width || renderer.canvasW;
    const scale = cssWidth / renderer.canvasW;
    const cssHeight = renderer.canvasH * scale;
    renderer.canvas.style.height = cssHeight + 'px';
    renderer.canvas.width = cssWidth * dpr;
    renderer.canvas.height = cssHeight * dpr;
    renderer.ctx.setTransform(dpr * scale, 0, 0, dpr * scale, 0, 0);
    renderer.dirty = true;
}

function setupRendererCanvas(renderer) {
    const dpr = window.devicePixelRatio || 1;
    if (simpleMode) {
        const vw = window.innerWidth;
        const vh = window.innerHeight;
        if (activeRenderers.length === 1) {
            renderer.canvas.style.width = vw + 'px';
            renderer.canvas.style.height = vh + 'px';
            renderer.canvas.width = vw * dpr;
            renderer.canvas.height = vh * dpr;
            const scale = Math.min(vw / renderer.canvasW, vh / renderer.canvasH);
            const offsetX = (vw - renderer.canvasW * scale) / 2;
            const offsetY = (vh - renderer.canvasH * scale) / 2;
            renderer.ctx.setTransform(dpr * scale, 0, 0, dpr * scale, dpr * offsetX, dpr * offsetY);
        } else {
            setupRendererNormal(renderer);
        }
    } else {
        setupRendererNormal(renderer);
    }
    renderer.dirty = true;
}

function drawGamepadRenderer(renderer) {
    const savedCtx = ctx;
    const savedW = canvasW;
    const savedH = canvasH;
    ctx = renderer.ctx;
    canvasW = renderer.canvasW;
    canvasH = renderer.canvasH;
    if (!state.connected) {
        if (!simpleMode) drawDisconnected();
    } else {
        drawController();
    }
    ctx = savedCtx;
    canvasW = savedW;
    canvasH = savedH;
}

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
        x1:     { x: 5,  y: 110, width: 22,  height: 35, radius: 5,  label: 'X1', code: 4 },
        x2:     { x: 5,  y: 150, width: 22,  height: 35, radius: 5,  label: 'X2', code: 5 },
    },
    wheel: {
        x: 68, y: 18, width: 24, height: 50, radius: 12
    },
    movement: {
        x: 80, y: 210, radius: 40
    }
};

function drawMouseRenderer(renderer) {
    const c = renderer.ctx;
    const cfg = MOUSE_CONFIG;

    // Body outline (SVG path)
    c.save();
    c.fillStyle = bodyFillColor;
    c.strokeStyle = COLORS.outline;
    c.lineWidth = 2;
    const bodyPath = new Path2D(cfg.body.path);
    c.fill(bodyPath);
    c.stroke(bodyPath);
    c.restore();

    // Scroll wheel
    const w = cfg.wheel;
    const wheelActive = (kmState.wheelUp || kmState.wheelDown) &&
        (performance.now() - kmState.wheelTimestamp <= WHEEL_TIMEOUT_MS);

    if (wheelActive && kmState.wheelUp) {
        c.fillStyle = COLORS.buttonPressed;
        c.beginPath();
        c.roundRect(w.x, w.y, w.width, w.height / 2, [w.radius, w.radius, 0, 0]);
        c.fill();
        c.fillStyle = COLORS.buttonDefault;
        c.beginPath();
        c.roundRect(w.x, w.y + w.height / 2, w.width, w.height / 2, [0, 0, w.radius, w.radius]);
        c.fill();
    } else if (wheelActive && kmState.wheelDown) {
        c.fillStyle = COLORS.buttonDefault;
        c.beginPath();
        c.roundRect(w.x, w.y, w.width, w.height / 2, [w.radius, w.radius, 0, 0]);
        c.fill();
        c.fillStyle = COLORS.buttonPressed;
        c.beginPath();
        c.roundRect(w.x, w.y + w.height / 2, w.width, w.height / 2, [0, 0, w.radius, w.radius]);
        c.fill();
    } else {
        c.fillStyle = COLORS.buttonDefault;
        c.beginPath();
        c.roundRect(w.x, w.y, w.width, w.height, w.radius);
        c.fill();
    }
    c.strokeStyle = COLORS.outline;
    c.lineWidth = 1;
    c.beginPath();
    c.roundRect(w.x, w.y, w.width, w.height, w.radius);
    c.stroke();

    // Mouse buttons
    const savedCtx = ctx;
    for (const [, btn] of Object.entries(cfg.buttons)) {
        const pressed = kmState.mouseButtons[btn.code] === true;
        c.fillStyle = pressed ? COLORS.buttonPressed : COLORS.buttonDefault;
        c.strokeStyle = COLORS.outline;
        c.lineWidth = 1.5;
        ctx = c;
        drawRoundRect(btn.x, btn.y, btn.width, btn.height, btn.radius);
        c.fill();
        c.stroke();

        c.fillStyle = pressed ? COLORS.buttonLabelPressed : COLORS.buttonLabel;
        const fontSize = btn.width > 30 ? 12 : 9;
        c.font = `bold ${fontSize}px "Segoe UI", sans-serif`;
        c.textAlign = 'center';
        c.textBaseline = 'middle';
        c.fillText(btn.label, btn.x + btn.width / 2, btn.y + btn.height / 2);
    }
    ctx = savedCtx;

    // Movement indicator
    const mv = cfg.movement;
    const mx = kmState.mouseMove.x;
    const my = kmState.mouseMove.y;

    c.fillStyle = COLORS.stickBase;
    c.strokeStyle = COLORS.outline;
    c.lineWidth = 1.5;
    c.beginPath();
    c.arc(mv.x, mv.y, mv.radius, 0, Math.PI * 2);
    c.fill();
    c.stroke();

    const magnitude = Math.sqrt(mx * mx + my * my);
    if (magnitude > 0.01) {
        const angle = Math.atan2(mx, -my);
        const arrowLen = Math.min(mv.radius * 0.75, magnitude * mv.radius);
        const arrowX = mv.x + Math.sin(angle) * arrowLen;
        const arrowY = mv.y - Math.cos(angle) * arrowLen;

        c.strokeStyle = COLORS.buttonPressed;
        c.lineWidth = 3;
        c.lineCap = 'round';
        c.beginPath();
        c.moveTo(mv.x, mv.y);
        c.lineTo(arrowX, arrowY);
        c.stroke();

        const headLen = 8;
        const headAngle = Math.PI / 6;
        c.beginPath();
        c.moveTo(arrowX, arrowY);
        c.lineTo(
            arrowX - headLen * Math.cos(angle - headAngle + Math.PI / 2),
            arrowY + headLen * Math.sin(angle - headAngle + Math.PI / 2)
        );
        c.moveTo(arrowX, arrowY);
        c.lineTo(
            arrowX - headLen * Math.cos(angle + headAngle + Math.PI / 2),
            arrowY + headLen * Math.sin(angle + headAngle + Math.PI / 2)
        );
        c.stroke();
    }
}

function computeKeyboardLayout(config) {
    const keySize = config.keySize || 40;
    const gap = config.gap || 4;
    const padding = config.padding || 8;
    const rows = config.rows || [];
    const layout = [];

    for (let rowIdx = 0; rowIdx < rows.length; rowIdx++) {
        const row = rows[rowIdx];
        let cursorX = padding;
        const y = padding + rowIdx * (keySize + gap);

        for (const entry of row) {
            if (entry.pad !== undefined) {
                cursorX += entry.pad * (keySize + gap);
                continue;
            }

            const widthUnits = entry.width || 1.0;
            const w = widthUnits * keySize + (widthUnits - 1) * gap;
            const h = keySize;

            let label;
            if (entry.label !== undefined) {
                label = entry.label;
            } else if (KEY_DEFAULT_LABELS[entry.key] !== undefined) {
                label = KEY_DEFAULT_LABELS[entry.key];
            } else {
                label = entry.key;
            }

            const scancode = KEY_NAME_TO_SCANCODE[entry.key];

            layout.push({ key: entry.key, label, scancode, x: cursorX, y, w, h });
            cursorX += w + gap;
        }
    }

    return layout;
}

function drawKeyboardRenderer(renderer) {
    const cfg = renderer.keyboardConfig;
    if (!cfg) {
        renderer.ctx.fillStyle = COLORS.buttonDefault;
        renderer.ctx.fillRect(0, 0, renderer.canvasW, renderer.canvasH);
        renderer.ctx.fillStyle = COLORS.textDim;
        renderer.ctx.font = '14px "Segoe UI", sans-serif';
        renderer.ctx.textAlign = 'center';
        renderer.ctx.textBaseline = 'middle';
        renderer.ctx.fillText('Loading keyboard...', renderer.canvasW / 2, renderer.canvasH / 2);
        return;
    }

    if (!renderer.keyboardLayout) {
        renderer.keyboardLayout = computeKeyboardLayout(cfg);
    }

    const c = renderer.ctx;
    const keySize = cfg.keySize || 40;

    c.fillStyle = COLORS.bg;
    c.fillRect(0, 0, renderer.canvasW, renderer.canvasH);

    const savedCtx = ctx;
    for (const key of renderer.keyboardLayout) {
        const pressed = key.scancode !== undefined && kmState.keys[key.scancode] === true;

        c.fillStyle = pressed ? COLORS.buttonPressed : COLORS.buttonDefault;
        c.strokeStyle = COLORS.outline;
        c.lineWidth = 1.5;

        ctx = c;
        drawRoundRect(key.x, key.y, key.w, key.h, 4);
        c.fill();
        c.stroke();

        c.fillStyle = pressed ? COLORS.buttonLabelPressed : COLORS.buttonLabel;
        c.textAlign = 'center';
        c.textBaseline = 'middle';

        const baseFontSize = Math.min(keySize * 0.4, key.w * 0.5);
        const fontSize = Math.max(8, Math.min(14, baseFontSize));
        const isSingleChar = key.label.length === 1;
        c.font = isSingleChar
            ? `bold ${fontSize}px "Segoe UI", sans-serif`
            : `${Math.max(7, fontSize - 2)}px "Segoe UI", sans-serif`;

        c.fillText(key.label, key.x + key.w / 2, key.y + key.h / 2);
    }
    ctx = savedCtx;
}

function render() {
    if (activeRenderers.length > 0) {
        for (const renderer of activeRenderers) {
            if (renderer.dirty) {
                renderer.dirty = false;
                renderer.ctx.clearRect(0, 0, renderer.canvasW, renderer.canvasH);
                renderer.draw(renderer);
            }
        }
    } else {
        if (dirty) {
            dirty = false;
            ctx.clearRect(0, 0, canvasW, canvasH);

            if (overlayName !== null) {
                const canRender = state.connected || !overlayHasGamepad;
                if (!canRender) {
                    if (!simpleMode) drawDisconnected();
                } else {
                    drawInputOverlay();
                }
            } else {
                if (!state.connected) {
                    if (!simpleMode) drawDisconnected();
                } else {
                    drawController();
                }
            }
        }
    }

    requestAnimationFrame(render);
}

function drawDisconnected() {
    ctx.fillStyle = COLORS.textDim;
    ctx.font = '20px "Segoe UI", sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText('No controller connected', canvasW / 2, canvasH / 2);
    ctx.font = '14px "Segoe UI", sans-serif';
    ctx.fillText('Connect a gamepad and it will appear here', canvasW / 2, canvasH / 2 + 30);
}

function drawController() {
    const cfg = currentConfig;
    if (!cfg) {
        if (!simpleMode) drawDisconnected();
        return;
    }

    drawTriggers(cfg);
    drawBody(cfg);
    drawDpad(cfg);
    drawFaceButtons(cfg);
    drawShoulderButtons(cfg);
    drawSticks(cfg);
    drawTouchpad(cfg);
    drawCenterButtons(cfg);
    drawTriggerLabels(cfg);
}

// --- Body outline ---
function drawBody(cfg) {
    const body = cfg.body;
    if (!body) return;

    ctx.fillStyle = bodyFillColor;
    ctx.strokeStyle = COLORS.outline;
    ctx.lineWidth = 2;

    if (body.path) {
        ctx.save();

        if (body.viewBox) {
            const [vbX, vbY, vbWidth, vbHeight] = body.viewBox.split(/[\s,]+/).map(Number);
            const targetX = body.x || 0;
            const targetY = body.y || 0;
            const targetWidth = body.width || 500;
            const targetHeight = body.height || 330;
            const scale = Math.min(targetWidth / vbWidth, targetHeight / vbHeight);
            const scaledWidth = vbWidth * scale;
            const scaledHeight = vbHeight * scale;
            const offsetX = targetX + (targetWidth - scaledWidth) / 2 - vbX * scale;
            const offsetY = targetY + (targetHeight - scaledHeight) / 2 - vbY * scale;
            ctx.translate(offsetX, offsetY);
            ctx.scale(scale, scale);
        }

        const path = new Path2D(body.path);
        ctx.fill(path);
        ctx.stroke(path);
        ctx.restore();
        return;
    }

    // Fallback: rounded rectangle
    const { x, y, width: w, height: h, radius } = body;
    const r = radius || 30;
    drawRoundRect(x, y, w, h, r);
    ctx.fill();
    ctx.stroke();
}

// --- D-pad ---
function drawDpadDirection(cx, cy, size, arm, direction, pressed) {
    const halfArm = arm / 2;
    const points = [];

    switch (direction) {
        case 'up':
            points.push([cx - halfArm, cy - size], [cx + halfArm, cy - size],
                        [cx + halfArm, cy - halfArm], [cx, cy], [cx - halfArm, cy - halfArm]);
            break;
        case 'down':
            points.push([cx - halfArm, cy + size], [cx + halfArm, cy + size],
                        [cx + halfArm, cy + halfArm], [cx, cy], [cx - halfArm, cy + halfArm]);
            break;
        case 'left':
            points.push([cx - size, cy - halfArm], [cx - size, cy + halfArm],
                        [cx - halfArm, cy + halfArm], [cx, cy], [cx - halfArm, cy - halfArm]);
            break;
        case 'right':
            points.push([cx + size, cy - halfArm], [cx + size, cy + halfArm],
                        [cx + halfArm, cy + halfArm], [cx, cy], [cx + halfArm, cy - halfArm]);
            break;
    }

    ctx.fillStyle = pressed ? COLORS.dpadPressed : COLORS.dpadBg;
    ctx.beginPath();
    ctx.moveTo(points[0][0], points[0][1]);
    for (let i = 1; i < points.length; i++) ctx.lineTo(points[i][0], points[i][1]);
    ctx.closePath();
    ctx.fill();

    ctx.strokeStyle = COLORS.outline;
    ctx.lineWidth = 1.5;
    ctx.stroke();
}

function drawDpad(cfg) {
    const dpad = cfg.dpad;
    if (!dpad) return;
    const { x: cx, y: cy } = dpad;
    const size = dpad.size || 30;
    const arm = dpad.armWidth || 22;
    drawDpadDirection(cx, cy, size, arm, 'up',    state.dpad.up);
    drawDpadDirection(cx, cy, size, arm, 'down',  state.dpad.down);
    drawDpadDirection(cx, cy, size, arm, 'left',  state.dpad.left);
    drawDpadDirection(cx, cy, size, arm, 'right', state.dpad.right);
}

// --- Face Buttons (A, B, X, Y) ---
const psSymbols = {
    '×': (x, y, size) => {
        ctx.beginPath();
        ctx.moveTo(x - size, y - size); ctx.lineTo(x + size, y + size);
        ctx.moveTo(x + size, y - size); ctx.lineTo(x - size, y + size);
        ctx.stroke();
    },
    '○': (x, y, size) => {
        ctx.beginPath(); ctx.arc(x, y, size, 0, Math.PI * 2); ctx.stroke();
    },
    '□': (x, y, size) => {
        ctx.beginPath(); ctx.rect(x - size, y - size, size * 2, size * 2); ctx.stroke();
    },
    '△': (x, y, size) => {
        ctx.beginPath();
        ctx.moveTo(x, y - size); ctx.lineTo(x + size, y + size); ctx.lineTo(x - size, y + size);
        ctx.closePath(); ctx.stroke();
    },
};

function drawFaceButtons(cfg) {
    const buttons = cfg.faceButtons;
    if (!buttons) return;

    const r = buttons.radius || 18;
    const defaultLabelConfig = buttons.label || { fontSize: r, fontWeight: 'normal' };

    const btnDefs = [
        { key: 'a', defaultColor: COLORS.faceA },
        { key: 'b', defaultColor: COLORS.faceB },
        { key: 'x', defaultColor: COLORS.faceX },
        { key: 'y', defaultColor: COLORS.faceY },
    ];

    for (const def of btnDefs) {
        const pos = buttons[def.key];
        if (!pos) continue;

        const pressed = state.buttons[def.key];
        const posLabel = pos.label || {};
        const labelText = posLabel.text;
        const labelColor = posLabel.color || def.defaultColor;
        const fontSize = posLabel.fontSize || defaultLabelConfig.fontSize || r;
        const fontWeight = posLabel.fontWeight || defaultLabelConfig.fontWeight || 'normal';

        ctx.beginPath();
        ctx.arc(pos.x, pos.y, r, 0, Math.PI * 2);
        ctx.fillStyle = pressed ? labelColor : COLORS.buttonDefault;
        ctx.fill();
        ctx.strokeStyle = labelColor;
        ctx.lineWidth = 2;
        ctx.stroke();

        if (!labelText) continue;

        ctx.fillStyle = pressed ? COLORS.buttonLabelPressed : labelColor;
        ctx.strokeStyle = pressed ? COLORS.buttonLabelPressed : labelColor;
        ctx.lineWidth = 2;

        if (psSymbols[labelText]) {
            psSymbols[labelText](pos.x, pos.y, r * 0.5);
        } else {
            ctx.font = `${fontWeight} ${fontSize}px "Segoe UI", sans-serif`;
            ctx.textAlign = 'center';
            ctx.textBaseline = 'middle';
            ctx.fillText(labelText, pos.x, pos.y + 1);
        }
    }
}

// --- Shoulder Buttons (LB, RB) ---
function drawShoulderButtons(cfg) {
    const shoulders = cfg.shoulders;
    if (!shoulders) return;

    const defaultLabelConfig = shoulders.label || { fontSize: 15, fontWeight: 'normal' };

    for (const side of ['lb', 'rb']) {
        const s = shoulders[side];
        if (!s) continue;
        const pressed = state.buttons[side];
        const sLabel = s.label || {};
        const labelText = sLabel.text;
        const fontSize = sLabel.fontSize || defaultLabelConfig.fontSize || 15;
        const fontWeight = sLabel.fontWeight || defaultLabelConfig.fontWeight || 'normal';

        ctx.fillStyle = pressed ? COLORS.buttonPressed : COLORS.buttonDefault;
        ctx.strokeStyle = COLORS.outline;
        ctx.lineWidth = 2;
        drawRoundRect(s.x, s.y, s.width, s.height, s.radius || 6);
        ctx.fill();
        ctx.stroke();

        if (labelText) {
            ctx.fillStyle = pressed ? COLORS.buttonLabelPressed : COLORS.buttonLabel;
            ctx.font = `${fontWeight} ${fontSize}px "Segoe UI", sans-serif`;
            ctx.textAlign = 'center';
            ctx.textBaseline = 'middle';
            ctx.fillText(labelText, s.x + s.width / 2, s.y + s.height / 2);
        }
    }
}

// --- Triggers (LT, RT) ---
function drawTriggers(cfg) {
    const triggers = cfg.triggers;
    if (!triggers) return;

    for (const side of ['lt', 'rt']) {
        const t = triggers[side];
        if (!t) continue;
        const value = state.triggers[side].value;

        ctx.fillStyle = COLORS.triggerBg;
        ctx.strokeStyle = COLORS.outline;
        ctx.lineWidth = 2;
        drawRoundRect(t.x, t.y, t.width, t.height, t.radius || 6);
        ctx.fill();
        ctx.stroke();

        if (value > 0) {
            const fillHeight = t.height * value;
            ctx.fillStyle = COLORS.triggerFill;
            drawRoundRect(t.x, t.y + t.height - fillHeight, t.width, fillHeight, t.radius || 6);
            ctx.fill();
        }
    }
}

// --- Trigger Labels ---
function drawTriggerLabels(cfg) {
    const triggers = cfg.triggers;
    if (!triggers) return;

    const defaultLabelConfig = triggers.label || { fontSize: 13, fontWeight: 'normal' };

    for (const side of ['lt', 'rt']) {
        const t = triggers[side];
        if (!t) continue;
        const tLabel = t.label || {};
        const labelText = tLabel.text;
        if (!labelText) continue;

        const fontSize = tLabel.fontSize || defaultLabelConfig.fontSize || 13;
        const fontWeight = tLabel.fontWeight || defaultLabelConfig.fontWeight || 'normal';
        ctx.fillStyle = COLORS.buttonLabel;
        ctx.font = `${fontWeight} ${fontSize}px "Segoe UI", sans-serif`;
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(labelText, t.x + t.width / 2, t.y + t.height * 0.3);
    }
}

// --- Sticks ---
function drawSticks(cfg) {
    const sticks = cfg.sticks;
    if (!sticks) return;

    for (const side of ['left', 'right']) {
        const s = sticks[side];
        if (!s) continue;
        const stickState = state.sticks[side];

        const baseR = s.baseRadius || 40;
        const knobR = s.knobRadius || 22;
        const maxTravel = baseR - knobR + 5;

        ctx.beginPath();
        ctx.arc(s.x, s.y, baseR, 0, Math.PI * 2);
        ctx.fillStyle = COLORS.stickBase;
        ctx.fill();
        ctx.strokeStyle = COLORS.outline;
        ctx.lineWidth = 2;
        ctx.stroke();

        const knobX = s.x + stickState.position.x * maxTravel;
        const knobY = s.y - stickState.position.y * maxTravel;

        ctx.beginPath();
        ctx.arc(knobX, knobY, knobR, 0, Math.PI * 2);
        ctx.fillStyle = stickState.pressed ? COLORS.stickKnobPressed : COLORS.stickKnob;
        ctx.fill();
        ctx.strokeStyle = COLORS.outline;
        ctx.lineWidth = 1.5;
        ctx.stroke();
    }
}

// --- Touchpad (PlayStation) ---
function drawTouchpad(cfg) {
    const touchpad = cfg.touchpad;
    if (!touchpad) return;

    const pressed = state.buttons.touchpad;
    ctx.fillStyle = pressed ? COLORS.buttonPressed : COLORS.buttonDefault;
    ctx.strokeStyle = COLORS.outline;
    ctx.lineWidth = 2;
    drawRoundRect(touchpad.x, touchpad.y, touchpad.width, touchpad.height, touchpad.radius || 6);
    ctx.fill();
    ctx.stroke();

    ctx.strokeStyle = pressed ? COLORS.buttonLabelPressed : COLORS.buttonLabel;
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(touchpad.x + touchpad.width / 2, touchpad.y + 5);
    ctx.lineTo(touchpad.x + touchpad.width / 2, touchpad.y + touchpad.height - 5);
    ctx.stroke();
}

// --- Center Buttons (Select, Start, Home) ---
function drawCenterButtons(cfg) {
    const center = cfg.centerButtons;
    if (!center) return;

    const defaultLabelConfig = center.label || { fontSize: 14, fontWeight: 'bold' };

    const shapeDrawers = {
        'triangle_right': (b) => {
            ctx.beginPath();
            ctx.moveTo(b.x, b.y);
            ctx.lineTo(b.x + b.width, b.y + b.height / 2);
            ctx.lineTo(b.x, b.y + b.height);
            ctx.closePath();
        }
    };

    for (const key of ['back', 'start', 'guide', 'capture']) {
        const b = center[key];
        if (!b) continue;
        const pressed = state.buttons[key];
        const bLabel = b.label || {};
        const labelText = bLabel.text;
        const fontSize = bLabel.fontSize || defaultLabelConfig.fontSize || 14;
        const fontWeight = bLabel.fontWeight || defaultLabelConfig.fontWeight || 'bold';

        const isCircular = b.radius && !b.width && !b.height;

        ctx.fillStyle = pressed ? COLORS.buttonPressed : COLORS.buttonDefault;
        ctx.strokeStyle = COLORS.outline;

        if (isCircular) {
            ctx.lineWidth = 2;
            ctx.beginPath();
            ctx.arc(b.x, b.y, b.radius, 0, Math.PI * 2);
            ctx.fill();
            ctx.stroke();

            if (labelText) {
                ctx.fillStyle = pressed ? COLORS.buttonLabelPressed : COLORS.buttonLabel;
                ctx.font = `${fontWeight} ${fontSize}px "Segoe UI", sans-serif`;
                ctx.textAlign = 'center';
                ctx.textBaseline = 'middle';
                ctx.fillText(labelText, b.x, b.y + 1);
            }
        } else {
            ctx.lineWidth = 1.5;
            const shapeDrawer = b.shape ? shapeDrawers[b.shape] : null;
            if (shapeDrawer) {
                shapeDrawer(b);
            } else {
                drawRoundRect(b.x, b.y, b.width, b.height, b.radius || 4);
            }
            ctx.fill();
            ctx.stroke();

            if (labelText) {
                ctx.fillStyle = pressed ? COLORS.buttonLabelPressed : COLORS.buttonLabel;
                ctx.font = `${fontWeight} ${fontSize}px "Segoe UI", sans-serif`;
                ctx.textAlign = 'center';
                ctx.textBaseline = 'middle';
                ctx.fillText(labelText, b.x + b.width / 2, b.y + b.height / 2 + 1);
            }
        }
    }
}

// --- Utility: Rounded Rectangle ---
// Uses native ctx.roundRect where available (Chrome 99+, Firefox 112+, Safari 15.4+),
// falling back to a manual implementation for older environments.
function drawRoundRect(x, y, w, h, r) {
    if (w < 0) { x += w; w = -w; }
    if (h < 0) { y += h; h = -h; }
    r = Math.min(r, w / 2, h / 2);
    if (ctx.roundRect) {
        ctx.beginPath();
        ctx.roundRect(x, y, w, h, r);
    } else {
        ctx.beginPath();
        ctx.moveTo(x + r, y);
        ctx.lineTo(x + w - r, y);
        ctx.quadraticCurveTo(x + w, y, x + w, y + r);
        ctx.lineTo(x + w, y + h - r);
        ctx.quadraticCurveTo(x + w, y + h, x + w - r, y + h);
        ctx.lineTo(x + r, y + h);
        ctx.quadraticCurveTo(x, y + h, x, y + h - r);
        ctx.lineTo(x, y + r);
        ctx.quadraticCurveTo(x, y, x + r, y);
        ctx.closePath();
    }
}

// ============================================================
// Initialization
// ============================================================

function init() {
    const urlParams = new URLSearchParams(window.location.search);
    simpleMode = urlParams.get('simple') === '1';

    const playerParam = urlParams.get('p');
    if (playerParam !== null) {
        const p = parseInt(playerParam, 10);
        if (!isNaN(p) && p >= 1 && p <= MAX_PLAYER_INDEX) selectedPlayerIndex = p;
    }

    const alphaParam = urlParams.get('alpha');
    if (alphaParam !== null) {
        const alpha = parseFloat(alphaParam);
        if (!isNaN(alpha) && alpha >= 0 && alpha <= 1) bodyAlpha = alpha;
    }

    const overlayParam = urlParams.get('overlay');
    hasGamepadParam = urlParams.has('gamepad');
    hasMouseParam = urlParams.get('mouse') === '1';
    keyboardParam = urlParams.get('keyboard');
    forcedGamepadType = urlParams.get('gamepad');
    explicitMode = hasGamepadParam || hasMouseParam || (keyboardParam !== null);

    if (overlayParam && explicitMode) {
        console.warn('[InputView] ?overlay= and ?gamepad/?mouse/?keyboard cannot be combined. Ignoring gamepad/mouse/keyboard params.');
        explicitMode = false;
        hasGamepadParam = false;
        hasMouseParam = false;
        keyboardParam = null;
        forcedGamepadType = null;
    }

    if (overlayParam) {
        overlayName = overlayParam;
        // Optimistically hide gamepad UI until the config tells us it's needed.
        overlayHasGamepad = false;
        loadInputOverlayConfig(overlayName);
    }

    const sensParam = urlParams.get('mouse_sens');
    if (sensParam !== null) {
        const sens = parseFloat(sensParam);
        if (!isNaN(sens) && sens >= 1 && sens <= 10000) mouseSens = sens;
    }

    // Pre-compute body fill color after bodyAlpha is resolved
    updateBodyFillColor();

    if (simpleMode) {
        document.body.classList.add('simple-mode');
        document.body.style.backgroundColor = 'transparent';
        canvas.style.backgroundColor = 'transparent';
        canvas.style.border = 'none';
        canvas.style.borderRadius = '0';
        const header = document.getElementById('header');
        const controllerInfo = document.getElementById('controller-info');
        const app = document.getElementById('app');
        if (header) header.style.display = 'none';
        if (controllerInfo) controllerInfo.style.display = 'none';
        if (app) { app.style.padding = '0'; app.style.maxWidth = 'none'; }
    }

    if (explicitMode && !overlayParam) {
        const container = document.getElementById('device-container');
        container.style.display = 'flex';
        if (simpleMode) container.classList.add('simple-mode');

        canvas.style.display = 'none';

        if (!hasGamepadParam) {
            const playerInfo = document.getElementById('player-info');
            const controllerInfo = document.getElementById('controller-info');
            const statusEl = document.getElementById('status');
            if (playerInfo) playerInfo.style.display = 'none';
            if (controllerInfo) controllerInfo.style.display = 'none';
            if (statusEl) statusEl.style.display = 'none';
        }

        if (hasGamepadParam) {
            const gamepadCanvas = document.createElement('canvas');
            gamepadCanvas.className = 'device-canvas';
            gamepadCanvas.dataset.device = 'gamepad';
            gamepadCanvas.width = 500;
            gamepadCanvas.height = 330;
            container.appendChild(gamepadCanvas);
            activeRenderers.push({
                canvas: gamepadCanvas,
                ctx: gamepadCanvas.getContext('2d'),
                canvasW: 500,
                canvasH: 330,
                dirty: true,
                draw: drawGamepadRenderer,
                type: 'gamepad'
            });
        }

        if (hasMouseParam) {
            const mouseCanvas = document.createElement('canvas');
            mouseCanvas.className = 'device-canvas';
            mouseCanvas.dataset.device = 'mouse';
            mouseCanvas.width = 160;
            mouseCanvas.height = 270;
            container.appendChild(mouseCanvas);
            activeRenderers.push({
                canvas: mouseCanvas,
                ctx: mouseCanvas.getContext('2d'),
                canvasW: 160,
                canvasH: 270,
                dirty: true,
                draw: drawMouseRenderer,
                type: 'mouse'
            });
        }

        if (keyboardParam !== null) {
            const keyboardCanvas = document.createElement('canvas');
            keyboardCanvas.className = 'device-canvas';
            keyboardCanvas.dataset.device = 'keyboard';
            keyboardCanvas.width = 400;
            keyboardCanvas.height = 240;
            container.appendChild(keyboardCanvas);
            activeRenderers.push({
                canvas: keyboardCanvas,
                ctx: keyboardCanvas.getContext('2d'),
                canvasW: 400,
                canvasH: 240,
                dirty: true,
                draw: drawKeyboardRenderer,
                type: 'keyboard',
                configName: keyboardParam
            });
        }

        setTimeout(() => {
            for (const renderer of activeRenderers) setupRendererCanvas(renderer);
        }, 0);

        // Load keyboard configs for any keyboard renderers
        for (const renderer of activeRenderers) {
            if (renderer.type === 'keyboard' && renderer.configName) {
                loadKeyboardConfig(renderer.configName);
            }
        }
    }

    setupCanvas();
    window.addEventListener('resize', () => {
        setupCanvas();
        for (const renderer of activeRenderers) setupRendererCanvas(renderer);
    });

    // Preload default xbox config (only in geometric mode)
    if (overlayName === null && (!explicitMode || hasGamepadParam)) {
        if (explicitMode) enforceForcedGamepadType();
        const preloadType = (explicitMode && forcedGamepadType && forcedGamepadType !== 'true' && forcedGamepadType !== '') ? forcedGamepadType : 'xbox';
        const preloadConfigName = configNameForType(preloadType);
        fetch(`configs/${preloadConfigName}.json`)
            .then(r => r.json())
            .then(config => {
                configCache[preloadType] = config;
                currentConfig = config;
                loadedConfigType = preloadType;
                dirty = true;
                markRendererDirty(['gamepad']);
            })
            .catch(e => console.error('Failed to load default config:', e));
    }

    connectWebSocket();
    requestAnimationFrame(render);
}

init();
