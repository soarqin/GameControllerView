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

// Simple mode: only draw controller elements, no background
let simpleMode = false;
// Body alpha channel (0-1)
let bodyAlpha = 1.0;
// Selected player index (1-based, default 1)
let selectedPlayerIndex = 1;

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
        // Send selected player index to backend
        const selectMsg = JSON.stringify({
            type: 'select_player',
            playerIndex: selectedPlayerIndex
        });
        ws.send(selectMsg);
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
        case 'player_selected':
            if (msg.playerIndex !== undefined) {
                console.log(`Player ${msg.playerIndex} selected`);
            }
            break;
    }
}

// Helper: Merge nested state objects
function mergeState(target, source) {
    if (source.connected !== undefined) {
        target.connected = source.connected;
    }
    if (source.controllerType !== undefined) {
        target.controllerType = source.controllerType;
    }
    if (source.name !== undefined) {
        target.name = source.name;
    }
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
    
    if (source.triggers) {
        if (source.triggers.lt) target.triggers.lt.value = source.triggers.lt.value ?? 0;
        if (source.triggers.rt) target.triggers.rt.value = source.triggers.rt.value ?? 0;
    }
}

function applyFullState(data) {
    mergeState(state, data);
    updateControllerInfo();
    loadConfigIfNeeded();
}

function applyDelta(changes) {
    mergeState(state, changes);
    updateControllerInfo();
    loadConfigIfNeeded();
}

function updateControllerInfo() {
    const el = document.getElementById('controller-name');
    if (state.connected && state.name) {
        el.textContent = `Player ${selectedPlayerIndex}: ${state.name} (${state.controllerType})`;
    } else {
        el.textContent = `Player ${selectedPlayerIndex}: No controller detected`;
    }
    updateStatusIndicator();
}

// ============================================================
// Configuration Loading
// ============================================================

let loadedConfigType = '';

function loadConfigIfNeeded() {
    const type = state.controllerType || 'xbox';
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
        'playstation5': 'playstation5',
        'switch_pro': 'switch_pro',
    };
    const configName = configMap[type] || 'xbox';

    fetch(`configs/${configName}.json`)
        .then(r => r.json())
        .then(config => {
            configCache[type] = config;
            currentConfig = config;
        })
        .catch(() => {
            // Fallback to xbox
            if (type !== 'xbox') {
                loadedConfigType = '';
                state.controllerType = 'xbox';
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

    if (simpleMode) {
        // Full viewport in simple mode
        const vw = window.innerWidth;
        const vh = window.innerHeight;
        canvas.style.width = vw + 'px';
        canvas.style.height = vh + 'px';
        canvas.width = vw * dpr;
        canvas.height = vh * dpr;

        // Calculate scale to fit WxH while maintaining aspect ratio
        const scaleX = vw / W;
        const scaleY = vh / H;
        const scale = Math.min(scaleX, scaleY);

        // Center the controller
        const offsetX = (vw - W * scale) / 2;
        const offsetY = (vh - H * scale) / 2;

        ctx.setTransform(dpr * scale, 0, 0, dpr * scale, dpr * offsetX, dpr * offsetY);
    } else {
        const rect = canvas.getBoundingClientRect();
        canvas.width = rect.width * dpr;
        canvas.height = rect.height * dpr;
        ctx.scale(dpr, dpr);
    }
}

const W = 500;
const H = 330;

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

// Helper: Convert hex color to rgba with alpha
function hexToRgba(hex, alpha) {
    const r = parseInt(hex.slice(1, 3), 16);
    const g = parseInt(hex.slice(3, 5), 16);
    const b = parseInt(hex.slice(5, 7), 16);
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

// Helper: Resolve label config with fallbacks
function resolveLabelConfig(posLabelConfig, defaultLabelConfig, defaults) {
    return {
        text: posLabelConfig.text,
        fontSize: posLabelConfig.fontSize || defaultLabelConfig.fontSize || defaults.fontSize,
        fontWeight: posLabelConfig.fontWeight || defaultLabelConfig.fontWeight || defaults.fontWeight,
        color: posLabelConfig.color || defaults.color,
    };
}

function render() {
    ctx.clearRect(0, 0, W, H);

    if (!state.connected) {
        if (!simpleMode) {
            drawDisconnected();
        }
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
        if (!simpleMode) {
            drawDisconnected();
        }
        return;
    }

    // Draw triggers first (will be partially covered by body)
    drawTriggers(cfg);
    // Draw body (keep in simple mode)
    drawBody(cfg);
    drawDpad(cfg);
    drawFaceButtons(cfg);
    drawShoulderButtons(cfg);
    drawSticks(cfg);
    drawTouchpad(cfg);
    drawCenterButtons(cfg);
    // Redraw trigger labels on top
    drawTriggerLabels(cfg);
}

// --- Body outline ---
function drawBody(cfg) {
    const body = cfg.body;
    if (!body) return;

    ctx.fillStyle = hexToRgba(COLORS.outlineFill, bodyAlpha);
    ctx.strokeStyle = COLORS.outline;
    ctx.lineWidth = 2;

    // Check for SVG path property (new feature)
    if (body.path) {
        ctx.save();

        // If viewBox is provided, scale the SVG path to fit the body dimensions
        if (body.viewBox) {
            // Parse viewBox: "min-x min-y width height"
            const [vbX, vbY, vbWidth, vbHeight] = body.viewBox.split(/[\s,]+/).map(Number);

            // Calculate target position and size from body config
            const targetX = body.x || 0;
            const targetY = body.y || 0;
            const targetWidth = body.width || 500;
            const targetHeight = body.height || 330;

            // Calculate scale to fit viewBox into target area
            // Use uniform scaling to preserve aspect ratio
            const scaleX = targetWidth / vbWidth;
            const scaleY = targetHeight / vbHeight;
            const scale = Math.min(scaleX, scaleY);

            // Calculate centered position
            const scaledWidth = vbWidth * scale;
            const scaledHeight = vbHeight * scale;
            const offsetX = targetX + (targetWidth - scaledWidth) / 2 - vbX * scale;
            const offsetY = targetY + (targetHeight - scaledHeight) / 2 - vbY * scale;

            // Apply transformation: translate to position, then scale
            ctx.translate(offsetX, offsetY);
            ctx.scale(scale, scale);
        }

        // Use Path2D API to draw custom controller outline
        const path = new Path2D(body.path);
        ctx.fill(path);
        ctx.stroke(path);

        ctx.restore();
        return;
    }

    // Fallback: Draw rounded rectangle (backward compatibility)
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

// Helper function to draw a single D-pad direction (pentagon shape)
function drawDpadDirection(cx, cy, size, arm, direction, pressed) {
    const points = [];
    const halfArm = arm / 2;
    
    switch (direction) {
        case 'up':
            points.push([cx - halfArm, cy - size]);
            points.push([cx + halfArm, cy - size]);
            points.push([cx + halfArm, cy - halfArm]);
            points.push([cx, cy]);
            points.push([cx - halfArm, cy - halfArm]);
            break;
        case 'down':
            points.push([cx - halfArm, cy + size]);
            points.push([cx + halfArm, cy + size]);
            points.push([cx + halfArm, cy + halfArm]);
            points.push([cx, cy]);
            points.push([cx - halfArm, cy + halfArm]);
            break;
        case 'left':
            points.push([cx - size, cy - halfArm]);
            points.push([cx - size, cy + halfArm]);
            points.push([cx - halfArm, cy + halfArm]);
            points.push([cx, cy]);
            points.push([cx - halfArm, cy - halfArm]);
            break;
        case 'right':
            points.push([cx + size, cy - halfArm]);
            points.push([cx + size, cy + halfArm]);
            points.push([cx + halfArm, cy + halfArm]);
            points.push([cx, cy]);
            points.push([cx + halfArm, cy - halfArm]);
            break;
    }
    
    ctx.fillStyle = pressed ? COLORS.dpadPressed : COLORS.dpadBg;
    ctx.beginPath();
    ctx.moveTo(points[0][0], points[0][1]);
    for (let i = 1; i < points.length; i++) {
        ctx.lineTo(points[i][0], points[i][1]);
    }
    ctx.closePath();
    ctx.fill();
    
    ctx.strokeStyle = COLORS.outline;
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.moveTo(points[0][0], points[0][1]);
    for (let i = 1; i < points.length; i++) {
        ctx.lineTo(points[i][0], points[i][1]);
    }
    ctx.closePath();
    ctx.stroke();
}

function drawDpad(cfg) {
    const dpad = cfg.dpad;
    if (!dpad) return;

    const cx = dpad.x;
    const cy = dpad.y;
    const size = dpad.size || 30;
    const arm = dpad.armWidth || 22;

    drawDpadDirection(cx, cy, size, arm, 'up', state.dpad.up);
    drawDpadDirection(cx, cy, size, arm, 'down', state.dpad.down);
    drawDpadDirection(cx, cy, size, arm, 'left', state.dpad.left);
    drawDpadDirection(cx, cy, size, arm, 'right', state.dpad.right);
}

// --- Face Buttons (A, B, X, Y) ---
function drawFaceButtons(cfg) {
    const buttons = cfg.faceButtons;
    if (!buttons) return;

    const r = buttons.radius || 18;
    const defaultLabelConfig = buttons.label || { fontSize: r, fontWeight: 'normal' };

    // Button definitions - colors are defaults, overridden by config
    const btnDefs = [
        { key: 'a', defaultColor: COLORS.faceA },
        { key: 'b', defaultColor: COLORS.faceB },
        { key: 'x', defaultColor: COLORS.faceX },
        { key: 'y', defaultColor: COLORS.faceY },
    ];

    // PlayStation symbol drawing helpers
    const psSymbols = {
        '×': (ctx, x, y, size) => {
            ctx.beginPath();
            ctx.moveTo(x - size, y - size);
            ctx.lineTo(x + size, y + size);
            ctx.moveTo(x + size, y - size);
            ctx.lineTo(x - size, y + size);
            ctx.stroke();
        },
        '○': (ctx, x, y, size) => {
            ctx.beginPath();
            ctx.arc(x, y, size, 0, Math.PI * 2);
            ctx.stroke();
        },
        '□': (ctx, x, y, size) => {
            ctx.beginPath();
            ctx.rect(x - size, y - size, size * 2, size * 2);
            ctx.stroke();
        },
        '△': (ctx, x, y, size) => {
            ctx.beginPath();
            ctx.moveTo(x, y - size);
            ctx.lineTo(x + size, y + size);
            ctx.lineTo(x - size, y + size);
            ctx.closePath();
            ctx.stroke();
        }
    };

    for (const def of btnDefs) {
        const pos = buttons[def.key];
        if (!pos) continue;

        const pressed = state.buttons[def.key];
        const labelConfig = resolveLabelConfig(pos.label || {}, defaultLabelConfig, {
            fontSize: r,
            fontWeight: 'normal',
            color: def.defaultColor,
        });
        const labelText = labelConfig.text;
        const labelColor = labelConfig.color;
        const fontSize = labelConfig.fontSize;
        const fontWeight = labelConfig.fontWeight;

        // Draw button background
        ctx.beginPath();
        ctx.arc(pos.x, pos.y, r, 0, Math.PI * 2);
        ctx.fillStyle = pressed ? labelColor : COLORS.buttonDefault;
        ctx.fill();
        ctx.strokeStyle = labelColor;
        ctx.lineWidth = 2;
        ctx.stroke();

        // Draw label if text exists
        if (!labelText) continue;

        ctx.fillStyle = pressed ? COLORS.buttonLabelPressed : labelColor;
        ctx.strokeStyle = pressed ? COLORS.buttonLabelPressed : labelColor;
        ctx.lineWidth = 2;

        // Check if this is a PlayStation symbol
        if (psSymbols[labelText]) {
            const symbolSize = r * 0.5;
            psSymbols[labelText](ctx, pos.x, pos.y, symbolSize);
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

        const labelConfig = resolveLabelConfig(s.label || {}, defaultLabelConfig, {
            fontSize: 15,
            fontWeight: 'normal',
        });
        const labelText = labelConfig.text;
        const fontSize = labelConfig.fontSize;
        const fontWeight = labelConfig.fontWeight;

        ctx.fillStyle = pressed ? COLORS.buttonPressed : COLORS.buttonDefault;
        ctx.strokeStyle = COLORS.outline;
        ctx.lineWidth = 2;

        roundRect(ctx, s.x, s.y, s.width, s.height, s.radius || 6);
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
    }
}

// --- Trigger Labels (drawn on top of body) ---
function drawTriggerLabels(cfg) {
    const triggers = cfg.triggers;
    if (!triggers) return;

    const defaultLabelConfig = triggers.label || { fontSize: 13, fontWeight: 'normal' };
    const defaults = { fontSize: 13, fontWeight: 'normal' };

    for (const side of ['lt', 'rt']) {
        const t = triggers[side];
        if (!t) continue;

        const labelConfig = resolveLabelConfig(t.label || {}, defaultLabelConfig, defaults);
        const labelText = labelConfig.text;

        if (labelText) {
            ctx.fillStyle = COLORS.buttonLabel;
            ctx.font = `${labelConfig.fontWeight} ${labelConfig.fontSize}px "Segoe UI", sans-serif`;
            ctx.textAlign = 'center';
            ctx.textBaseline = 'middle';
            ctx.fillText(labelText, t.x + t.width / 2, t.y + t.height * 0.3);
        }
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

// --- Touchpad (PlayStation) ---
function drawTouchpad(cfg) {
    const touchpad = cfg.touchpad;
    if (!touchpad) return;

    const pressed = state.buttons.touchpad;

    ctx.fillStyle = pressed ? COLORS.buttonPressed : COLORS.buttonDefault;
    ctx.strokeStyle = COLORS.outline;
    ctx.lineWidth = 2;

    roundRect(ctx, touchpad.x, touchpad.y, touchpad.width, touchpad.height, touchpad.radius || 6);
    ctx.fill();
    ctx.stroke();

    // Draw divider line for two sides of touchpad
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

    // Button keys to iterate over
    const btnKeys = ['back', 'start', 'guide', 'capture'];

    // Get default label config from centerButtons
    const defaultLabelConfig = center.label || { fontSize: 14, fontWeight: 'bold' };

    // Shape drawing functions
    const shapeDrawers = {
        'triangle_right': (b) => {
            const w = b.width;
            const h = b.height;
            ctx.beginPath();
            ctx.moveTo(b.x, b.y); // left top
            ctx.lineTo(b.x + w, b.y + h / 2); // right point (tip)
            ctx.lineTo(b.x, b.y + h); // left bottom
            ctx.closePath();
        }
    };

    for (const key of btnKeys) {
        const b = center[key];
        if (!b) continue;
        const pressed = state.buttons[key];

        const labelConfig = resolveLabelConfig(b.label || {}, defaultLabelConfig, {
            fontSize: 14,
            fontWeight: 'bold',
        });
        const labelText = labelConfig.text;
        const fontSize = labelConfig.fontSize;
        const fontWeight = labelConfig.fontWeight;

        // Check if button is circular (has radius but no width/height)
        const isCircular = b.radius && !b.width && !b.height;

        if (isCircular) {
            const r = b.radius;
            ctx.beginPath();
            ctx.arc(b.x, b.y, r, 0, Math.PI * 2);
            ctx.fillStyle = pressed ? COLORS.buttonPressed : COLORS.buttonDefault;
            ctx.fill();
            ctx.strokeStyle = COLORS.outline;
            ctx.lineWidth = 2;
            ctx.stroke();

            // Show label if text exists
            if (labelText) {
                ctx.fillStyle = pressed ? COLORS.buttonLabelPressed : COLORS.buttonLabel;
                ctx.font = `${fontWeight} ${fontSize}px "Segoe UI", sans-serif`;
                ctx.textAlign = 'center';
                ctx.textBaseline = 'middle';
                ctx.fillText(labelText, b.x, b.y + 1);
            }
        } else {
            // Rectangular or special shape buttons
            ctx.fillStyle = pressed ? COLORS.buttonPressed : COLORS.buttonDefault;
            ctx.strokeStyle = COLORS.outline;
            ctx.lineWidth = 1.5;

            // Check for special shapes from config
            const shapeDrawer = b.shape ? shapeDrawers[b.shape] : null;
            if (shapeDrawer) {
                shapeDrawer(b);
            } else {
                roundRect(ctx, b.x, b.y, b.width, b.height, b.radius || 4);
            }
            ctx.fill();
            ctx.stroke();

            // Show label if text exists
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
    // Check URL parameters
    const urlParams = new URLSearchParams(window.location.search);
    simpleMode = urlParams.get('simple') === '1';

    // Parse player index parameter (p)
    const playerParam = urlParams.get('p');
    if (playerParam !== null) {
        const p = parseInt(playerParam, 10);
        if (!isNaN(p) && p >= 1) {
            selectedPlayerIndex = p;
        }
    }

    // Parse alpha parameter (0-1)
    const alphaParam = urlParams.get('alpha');
    if (alphaParam !== null) {
        const alpha = parseFloat(alphaParam);
        if (!isNaN(alpha) && alpha >= 0 && alpha <= 1) {
            bodyAlpha = alpha;
        }
    }

    // Apply simple mode styles
    if (simpleMode) {
        document.body.style.backgroundColor = 'transparent';
        canvas.style.backgroundColor = 'transparent';
        canvas.style.border = 'none';
        canvas.style.borderRadius = '0';
        // Hide header and controller info in simple mode
        const header = document.getElementById('header');
        const controllerInfo = document.getElementById('controller-info');
        const app = document.getElementById('app');
        if (header) header.style.display = 'none';
        if (controllerInfo) controllerInfo.style.display = 'none';
        if (app) {
            app.style.padding = '0';
            app.style.maxWidth = 'none';
        }
    }

    setupCanvas();
    window.addEventListener('resize', setupCanvas);

    // Load xbox config as default
    fetch('configs/xbox.json')
        .then(r => r.json())
        .then(config => {
            configCache['xbox'] = config;
            currentConfig = config;
            loadedConfigType = 'xbox';
        })
        .catch(e => console.error('Failed to load default config:', e));

    connectWebSocket();
    requestAnimationFrame(render);
}

init();
