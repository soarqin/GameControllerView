// ============================================================
// State Management
// ============================================================

const state = {
    connected: false,
    controllerType: '',
    name: '',
    buttons: { a: false, b: false, x: false, y: false, lb: false, rb: false, select: false, start: false, home: false },
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

// ============================================================
// WebSocket Connection
// ============================================================

let ws = null;
let reconnectDelay = 1000;
const maxReconnectDelay = 10000;

function connectWebSocket() {
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${location.host}/ws`;

    ws = new WebSocket(url);

    ws.onopen = () => {
        reconnectDelay = 1000;
        setWSStatus(true);
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
            const msg = JSON.parse(event.data);
            handleMessage(msg);
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
            if (msg.data) {
                applyFullState(msg.data);
            }
            break;
        case 'delta':
            if (msg.changes) {
                applyDelta(msg.changes);
            }
            break;
        case 'event':
            if (msg.data) {
                applyFullState(msg.data);
            }
            break;
    }
}

function applyFullState(data) {
    Object.assign(state, {
        connected: data.connected ?? state.connected,
        controllerType: data.controllerType ?? state.controllerType,
        name: data.name ?? state.name,
    });
    if (data.buttons) Object.assign(state.buttons, data.buttons);
    if (data.dpad) Object.assign(state.dpad, data.dpad);
    if (data.sticks) {
        if (data.sticks.left) {
            if (data.sticks.left.position) Object.assign(state.sticks.left.position, data.sticks.left.position);
            state.sticks.left.pressed = data.sticks.left.pressed ?? state.sticks.left.pressed;
        }
        if (data.sticks.right) {
            if (data.sticks.right.position) Object.assign(state.sticks.right.position, data.sticks.right.position);
            state.sticks.right.pressed = data.sticks.right.pressed ?? state.sticks.right.pressed;
        }
    }
    if (data.triggers) {
        if (data.triggers.lt) state.triggers.lt.value = data.triggers.lt.value ?? 0;
        if (data.triggers.rt) state.triggers.rt.value = data.triggers.rt.value ?? 0;
    }
    updateControllerInfo();
    loadConfigIfNeeded();
}

function applyDelta(changes) {
    if (changes.connected !== undefined && changes.connected !== null) {
        state.connected = changes.connected;
    }
    if (changes.controllerType !== undefined && changes.controllerType !== null) {
        state.controllerType = changes.controllerType;
    }
    if (changes.name !== undefined && changes.name !== null) {
        state.name = changes.name;
    }
    if (changes.buttons) Object.assign(state.buttons, changes.buttons);
    if (changes.dpad) Object.assign(state.dpad, changes.dpad);
    if (changes.sticks) {
        if (changes.sticks.left) {
            if (changes.sticks.left.position) Object.assign(state.sticks.left.position, changes.sticks.left.position);
            if (changes.sticks.left.pressed !== undefined) state.sticks.left.pressed = changes.sticks.left.pressed;
        }
        if (changes.sticks.right) {
            if (changes.sticks.right.position) Object.assign(state.sticks.right.position, changes.sticks.right.position);
            if (changes.sticks.right.pressed !== undefined) state.sticks.right.pressed = changes.sticks.right.pressed;
        }
    }
    if (changes.triggers) {
        if (changes.triggers.lt) state.triggers.lt.value = changes.triggers.lt.value ?? state.triggers.lt.value;
        if (changes.triggers.rt) state.triggers.rt.value = changes.triggers.rt.value ?? state.triggers.rt.value;
    }
    updateControllerInfo();
    loadConfigIfNeeded();
}

function updateControllerInfo() {
    const el = document.getElementById('controller-name');
    if (state.connected && state.name) {
        el.textContent = `${state.name} (${state.controllerType})`;
    } else {
        el.textContent = 'No controller detected';
    }
    updateStatusIndicator();
}

// ============================================================
// Configuration Loading
// ============================================================

let loadedConfigType = '';

function loadConfigIfNeeded() {
    const type = state.controllerType || 'generic';
    if (type === loadedConfigType) return;

    loadedConfigType = type;

    if (configCache[type]) {
        currentConfig = configCache[type];
        return;
    }

    // Map controller type to config file
    const configMap = {
        'xbox': 'xbox',
        'playstation': 'playstation',
        'switch_pro': 'switch_pro',
        'generic': 'generic'
    };
    const configName = configMap[type] || 'generic';

    fetch(`configs/${configName}.json`)
        .then(r => r.json())
        .then(config => {
            configCache[type] = config;
            currentConfig = config;
        })
        .catch(() => {
            // Fallback to generic
            if (type !== 'generic') {
                loadedConfigType = '';
                state.controllerType = 'generic';
                loadConfigIfNeeded();
            }
        });
}

// ============================================================
// Canvas Rendering
// ============================================================

const canvas = document.getElementById('gamepad-canvas');
const ctx = canvas.getContext('2d');

// High-DPI support
function setupCanvas() {
    const dpr = window.devicePixelRatio || 1;
    const rect = canvas.getBoundingClientRect();
    canvas.width = rect.width * dpr;
    canvas.height = rect.height * dpr;
    ctx.scale(dpr, dpr);
}

const W = 800;
const H = 500;

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
    dpadBg: '#2a2a4a',
    dpadPressed: '#4ade80',
    textDim: '#6a6a8a',
    faceA: '#4ade80',
    faceB: '#f87171',
    faceX: '#60a5fa',
    faceY: '#fbbf24',
};

function render() {
    ctx.clearRect(0, 0, W, H);

    if (!state.connected) {
        drawDisconnected();
    } else {
        drawController();
    }

    requestAnimationFrame(render);
}

function drawDisconnected() {
    ctx.fillStyle = COLORS.textDim;
    ctx.font = '20px "Segoe UI", sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText('No controller connected', W / 2, H / 2);
    ctx.font = '14px "Segoe UI", sans-serif';
    ctx.fillText('Connect a gamepad and it will appear here', W / 2, H / 2 + 30);
}

function drawController() {
    const cfg = currentConfig;
    if (!cfg) {
        drawDisconnected();
        return;
    }

    drawBody(cfg);
    drawDpad(cfg);
    drawFaceButtons(cfg);
    drawShoulderButtons(cfg);
    drawTriggers(cfg);
    drawSticks(cfg);
    drawCenterButtons(cfg);
}

// --- Body outline ---
function drawBody(cfg) {
    const body = cfg.body;
    if (!body) return;

    ctx.fillStyle = COLORS.outlineFill;
    ctx.strokeStyle = COLORS.outline;
    ctx.lineWidth = 2;

    // Draw a rounded controller shape
    const x = body.x, y = body.y, w = body.width, h = body.height, r = body.radius || 30;

    ctx.beginPath();
    // Top-left grip curve
    ctx.moveTo(x + r, y);
    // Top edge
    ctx.lineTo(x + w - r, y);
    // Top-right corner
    ctx.quadraticCurveTo(x + w, y, x + w, y + r);
    // Right edge down to grip
    ctx.lineTo(x + w, y + h - r);
    // Bottom-right corner
    ctx.quadraticCurveTo(x + w, y + h, x + w - r, y + h);
    // Bottom edge
    ctx.lineTo(x + r, y + h);
    // Bottom-left corner
    ctx.quadraticCurveTo(x, y + h, x, y + h - r);
    // Left edge
    ctx.lineTo(x, y + r);
    // Top-left corner
    ctx.quadraticCurveTo(x, y, x + r, y);
    ctx.closePath();

    ctx.fill();
    ctx.stroke();
}

// --- D-pad ---
function drawDpad(cfg) {
    const dpad = cfg.dpad;
    if (!dpad) return;

    const cx = dpad.x;
    const cy = dpad.y;
    const size = dpad.size || 30;
    const arm = dpad.armWidth || 22;
    const half = arm / 2;

    // Draw cross background
    ctx.fillStyle = COLORS.dpadBg;
    // Vertical bar
    ctx.fillRect(cx - half, cy - size, arm, size * 2);
    // Horizontal bar
    ctx.fillRect(cx - size, cy - half, size * 2, arm);

    // Highlight pressed directions
    if (state.dpad.up) {
        ctx.fillStyle = COLORS.dpadPressed;
        ctx.fillRect(cx - half, cy - size, arm, size);
    }
    if (state.dpad.down) {
        ctx.fillStyle = COLORS.dpadPressed;
        ctx.fillRect(cx - half, cy, arm, size);
    }
    if (state.dpad.left) {
        ctx.fillStyle = COLORS.dpadPressed;
        ctx.fillRect(cx - size, cy - half, size, arm);
    }
    if (state.dpad.right) {
        ctx.fillStyle = COLORS.dpadPressed;
        ctx.fillRect(cx, cy - half, size, arm);
    }
}

// --- Face Buttons (A, B, X, Y) ---
function drawFaceButtons(cfg) {
    const buttons = cfg.faceButtons;
    if (!buttons) return;

    const r = buttons.radius || 18;

    const btnDefs = [
        { key: 'a', label: 'A', color: COLORS.faceA },
        { key: 'b', label: 'B', color: COLORS.faceB },
        { key: 'x', label: 'X', color: COLORS.faceX },
        { key: 'y', label: 'Y', color: COLORS.faceY },
    ];

    for (const def of btnDefs) {
        const pos = buttons[def.key];
        if (!pos) continue;

        const pressed = state.buttons[def.key];

        ctx.beginPath();
        ctx.arc(pos.x, pos.y, r, 0, Math.PI * 2);
        ctx.fillStyle = pressed ? def.color : COLORS.buttonDefault;
        ctx.fill();
        ctx.strokeStyle = def.color;
        ctx.lineWidth = 2;
        ctx.stroke();

        ctx.fillStyle = pressed ? COLORS.buttonLabelPressed : def.color;
        ctx.font = `bold ${r}px "Segoe UI", sans-serif`;
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(def.label, pos.x, pos.y);
    }
}

// --- Shoulder Buttons (LB, RB) ---
function drawShoulderButtons(cfg) {
    const shoulders = cfg.shoulders;
    if (!shoulders) return;

    for (const side of ['lb', 'rb']) {
        const s = shoulders[side];
        if (!s) continue;
        const pressed = state.buttons[side];

        ctx.fillStyle = pressed ? COLORS.buttonPressed : COLORS.buttonDefault;
        ctx.strokeStyle = COLORS.outline;
        ctx.lineWidth = 2;

        roundRect(ctx, s.x, s.y, s.width, s.height, s.radius || 6);
        ctx.fill();
        ctx.stroke();

        ctx.fillStyle = pressed ? COLORS.buttonLabelPressed : COLORS.buttonLabel;
        ctx.font = '13px "Segoe UI", sans-serif';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(side.toUpperCase(), s.x + s.width / 2, s.y + s.height / 2);
    }
}

// --- Triggers (LT, RT) ---
function drawTriggers(cfg) {
    const triggers = cfg.triggers;
    if (!triggers) return;

    for (const side of ['lt', 'rt']) {
        const t = triggers[side];
        if (!t) continue;
        const value = side === 'lt' ? state.triggers.lt.value : state.triggers.rt.value;

        // Background
        ctx.fillStyle = COLORS.triggerBg;
        ctx.strokeStyle = COLORS.outline;
        ctx.lineWidth = 2;
        roundRect(ctx, t.x, t.y, t.width, t.height, t.radius || 6);
        ctx.fill();
        ctx.stroke();

        // Fill based on value
        if (value > 0) {
            const fillHeight = t.height * value;
            ctx.fillStyle = COLORS.triggerFill;
            roundRect(ctx, t.x, t.y + t.height - fillHeight, t.width, fillHeight, t.radius || 6);
            ctx.fill();
        }

        // Label
        ctx.fillStyle = COLORS.buttonLabel;
        ctx.font = '12px "Segoe UI", sans-serif';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(side.toUpperCase(), t.x + t.width / 2, t.y + t.height + 14);
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

        // Base circle
        ctx.beginPath();
        ctx.arc(s.x, s.y, baseR, 0, Math.PI * 2);
        ctx.fillStyle = COLORS.stickBase;
        ctx.fill();
        ctx.strokeStyle = COLORS.outline;
        ctx.lineWidth = 2;
        ctx.stroke();

        // Knob position
        const knobX = s.x + stickState.position.x * maxTravel;
        const knobY = s.y - stickState.position.y * maxTravel; // Y is inverted in canvas

        ctx.beginPath();
        ctx.arc(knobX, knobY, knobR, 0, Math.PI * 2);
        ctx.fillStyle = stickState.pressed ? COLORS.stickKnobPressed : COLORS.stickKnob;
        ctx.fill();
        ctx.strokeStyle = COLORS.outline;
        ctx.lineWidth = 1.5;
        ctx.stroke();
    }
}

// --- Center Buttons (Select, Start, Home) ---
function drawCenterButtons(cfg) {
    const center = cfg.centerButtons;
    if (!center) return;

    const btnDefs = [
        { key: 'select', label: 'SEL' },
        { key: 'start', label: 'STA' },
        { key: 'home', label: 'H' },
    ];

    for (const def of btnDefs) {
        const b = center[def.key];
        if (!b) continue;
        const pressed = state.buttons[def.key];

        if (def.key === 'home') {
            // Home button is circular
            const r = b.radius || 14;
            ctx.beginPath();
            ctx.arc(b.x, b.y, r, 0, Math.PI * 2);
            ctx.fillStyle = pressed ? COLORS.buttonPressed : COLORS.buttonDefault;
            ctx.fill();
            ctx.strokeStyle = COLORS.outline;
            ctx.lineWidth = 2;
            ctx.stroke();

            ctx.fillStyle = pressed ? COLORS.buttonLabelPressed : COLORS.buttonLabel;
            ctx.font = `bold ${r}px "Segoe UI", sans-serif`;
            ctx.textAlign = 'center';
            ctx.textBaseline = 'middle';
            ctx.fillText(def.label, b.x, b.y);
        } else {
            // Select/Start are rectangular
            ctx.fillStyle = pressed ? COLORS.buttonPressed : COLORS.buttonDefault;
            ctx.strokeStyle = COLORS.outline;
            ctx.lineWidth = 1.5;
            roundRect(ctx, b.x, b.y, b.width, b.height, b.radius || 4);
            ctx.fill();
            ctx.stroke();

            ctx.fillStyle = pressed ? COLORS.buttonLabelPressed : COLORS.buttonLabel;
            ctx.font = '10px "Segoe UI", sans-serif';
            ctx.textAlign = 'center';
            ctx.textBaseline = 'middle';
            ctx.fillText(def.label, b.x + b.width / 2, b.y + b.height / 2);
        }
    }
}

// --- Utility: Rounded Rectangle ---
function roundRect(ctx, x, y, w, h, r) {
    if (w < 0) { x += w; w = -w; }
    if (h < 0) { y += h; h = -h; }
    r = Math.min(r, w / 2, h / 2);
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

// ============================================================
// Initialization
// ============================================================

function init() {
    setupCanvas();
    window.addEventListener('resize', setupCanvas);

    // Load generic config as default
    fetch('configs/generic.json')
        .then(r => r.json())
        .then(config => {
            configCache['generic'] = config;
            currentConfig = config;
            loadedConfigType = 'generic';
        })
        .catch(e => console.error('Failed to load default config:', e));

    connectWebSocket();
    requestAnimationFrame(render);
}

init();
