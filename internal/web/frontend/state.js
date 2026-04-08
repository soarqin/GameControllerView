// ============================================================
// State Management & Utilities
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

// Keyboard and mouse state (populated from km_full / km_delta WebSocket messages)
const kmState = {
    keys: {},           // uiohook scancode (number) -> boolean (pressed)
    mouseButtons: {},   // IO button code (1-5) -> boolean (pressed)
    mouseMove: { x: 0, y: 0 },    // normalised [-1,1] movement delta from current tick
    wheelUp: false,
    wheelDown: false,
    wheelTimestamp: 0,  // ms timestamp of last wheel event (for timeout reset)
};

// Config caches
let currentConfig = null;
const configCache = {};
const keyboardConfigCache = {};

// URL-driven flags
let simpleMode = false;
let bodyAlpha = 1.0;
let buttonAlpha = 1.0;
let selectedPlayerIndex = 1;
let mouseSens = 0; // 0 = not set by URL param

// Input Overlay state
let overlayName = null;
let overlayConfig = null;
let overlayTexture = null;
let overlayReady = false;
let overlayLoadFailed = false;
let overlayHasGamepad = true;  // default true until config tells us otherwise
let overlayHasKM = false;

// WebSocket state
let ws = null;
let reconnectDelay = RECONNECT_DELAY_INITIAL;
let wsConnected = false;

// Rendering state
let kmSubscribed = false;
let dirty = true;
let wheelDirtyTimeoutId = null;
let activeRenderers = [];
let explicitMode = false;
let hasGamepadParam = false;
let hasMouseParam = false;
let keyboardParam = null;
let forcedGamepadType = null;
let loadedConfigType = '';

// Canvas references (legacy single-canvas)
const canvas = document.getElementById('gamepad-canvas');
let ctx = canvas.getContext('2d');
let canvasW = CANVAS_WIDTH;
let canvasH = CANVAS_HEIGHT;

// Pre-computed fill colors (recalculated when bodyAlpha changes)
let bodyFillColor = '';
let kbBgFillColor = '';

// ============================================================
// Utility Functions
// ============================================================

const _fontCache = {};
function cachedFont(size, weight) {
    weight = weight || 'normal';
    const k = weight + '|' + size;
    return _fontCache[k] || (_fontCache[k] = weight + ' ' + size + 'px "Segoe UI", sans-serif');
}

function hexToRgba(hex, alpha) {
    const r = parseInt(hex.slice(1, 3), 16);
    const g = parseInt(hex.slice(3, 5), 16);
    const b = parseInt(hex.slice(5, 7), 16);
    return `rgba(${r},${g},${b},${alpha})`;
}

function updateBodyFillColor() {
    bodyFillColor = hexToRgba(COLORS.outlineFill, bodyAlpha);
    kbBgFillColor = hexToRgba(COLORS.bg, bodyAlpha);
}

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

// Rounded Rectangle -- uses native ctx.roundRect where available (Chrome 99+,
// Firefox 112+, Safari 15.4+), falling back to manual implementation.
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
