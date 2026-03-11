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

// Input Overlay mode: name of the overlay config to load (null = use built-in geometric renderer)
let overlayName = null;
// Input Overlay runtime state
let overlayConfig = null;   // parsed JSON from Input Overlay config file
let overlayTexture = null;  // HTMLImageElement of the texture atlas
let overlayReady = false;   // true once both config and texture are loaded

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

// Whether we have already subscribed to keyboard/mouse events for this session
let kmSubscribed = false;

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
        // Send selected player index to backend, unless the overlay has no gamepad elements
        // (in that case we don't need gamepad data at all).
        if (overlayName === null || overlayHasGamepad) {
            const selectMsg = JSON.stringify({
                type: 'select_player',
                playerIndex: selectedPlayerIndex
            });
            ws.send(selectMsg);
        }

        // Send mouse sensitivity if user provided it via URL parameter
        if (window._mouseSens !== undefined) {
            ws.send(JSON.stringify({ type: 'set_mouse_sens', value: window._mouseSens }));
        }

        // Re-subscribe to keyboard/mouse if we already determined that the overlay needs it
        if (kmSubscribed) {
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
        case 'km_full':
            if (msg.kmState) {
                applyKMFull(msg.kmState);
            }
            break;
        case 'km_delta':
            if (msg.kmDelta) {
                applyKMDelta(msg.kmDelta);
            }
            break;
    }
}

// Apply a full keyboard/mouse state snapshot.
function applyKMFull(data) {
    // Reset and rebuild keys
    Object.keys(kmState.keys).forEach(k => delete kmState.keys[k]);
    if (data.keys) {
        for (const [code, pressed] of Object.entries(data.keys)) {
            if (pressed) kmState.keys[Number(code)] = true;
        }
    }
    // Reset and rebuild mouse buttons
    Object.keys(kmState.mouseButtons).forEach(k => delete kmState.mouseButtons[k]);
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
}

// Apply an incremental keyboard/mouse delta.
function applyKMDelta(delta) {
    // Key down events
    if (delta.keysDown) {
        for (const code of delta.keysDown) {
            kmState.keys[code] = true;
        }
    }
    // Key up events
    if (delta.keysUp) {
        for (const code of delta.keysUp) {
            delete kmState.keys[code];
        }
    }
    // Mouse button down events
    if (delta.buttonsDown) {
        for (const code of delta.buttonsDown) {
            kmState.mouseButtons[code] = true;
        }
    }
    // Mouse button up events
    if (delta.buttonsUp) {
        for (const code of delta.buttonsUp) {
            delete kmState.mouseButtons[code];
        }
    }
    // Mouse movement
    if (delta.mouseMove) {
        kmState.mouseMove.x = delta.mouseMove.x;
        kmState.mouseMove.y = delta.mouseMove.y;
    } else {
        kmState.mouseMove.x = 0;
        kmState.mouseMove.y = 0;
    }
    // Wheel state with timeout
    if (delta.wheelUp || delta.wheelDown) {
        kmState.wheelUp = delta.wheelUp || false;
        kmState.wheelDown = delta.wheelDown || false;
        kmState.wheelTimestamp = performance.now();
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

function loadConfigIfNeeded() {
    // In Input Overlay mode, skip built-in config loading
    if (overlayName !== null) return;

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
// Input Overlay: Config + Texture Loading
// ============================================================

// SDL2 gamepad button codes → InputView state paths
// Matches layout_constants.h and vc.js from input-overlay
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
    15: s => s.buttons.misc,        // Misc1 (e.g. PS5 Mute)
    20: s => s.buttons.touchpad,
};

// Get button pressed state by SDL2 code
function ioButtonPressed(code) {
    const getter = IO_BUTTON_CODE_MAP[code];
    return getter ? !!getter(state) : false;
}

// analyzeOverlayContent inspects the loaded config and sets overlayHasGamepad / overlayHasKM.
// Also sends subscribe_km if the config contains km elements, and hides the gamepad UI
// bar when there are no gamepad elements.
function analyzeOverlayContent(cfg) {
    const gpTypes = new Set([2, 5, 6, 7, 8]);
    const kmTypes = new Set([1, 3, 4, 9]);
    const elements = cfg.elements || [];
    overlayHasGamepad = elements.some(el => gpTypes.has(el.type));
    overlayHasKM = elements.some(el => kmTypes.has(el.type));

    // Hide/show the Player info and controller status bar
    applyGamepadUIVisibility();

    // Subscribe to km events if needed
    if (overlayHasKM && !kmSubscribed) {
        kmSubscribed = true;
        if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'subscribe_km' }));
            console.log('Auto-subscribed to keyboard/mouse events');
        }
    }
}

// applyGamepadUIVisibility shows or hides the controller-info bar based on whether the
// current overlay needs gamepad data.
function applyGamepadUIVisibility() {
    // Only applies in overlay mode; geometric mode always needs the info bar.
    if (overlayName === null) return;
    // In simple mode, header/controller-info are always hidden regardless of gamepad presence.
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

// Load an Input Overlay config: tries external overlays dir first, then embedded
function loadInputOverlayConfig(name) {
    overlayReady = false;
    overlayConfig = null;
    overlayTexture = null;

    // Candidate URL prefixes in priority order:
    // 1. /overlays/<name>/  — external directory next to executable (served by backend)
    // 2. /overlays/<name>/  — embedded under frontend/overlays/ (same path, handled by fallback)
    const baseUrl = `/overlays/${encodeURIComponent(name)}/`;
    const jsonUrl = `${baseUrl}${encodeURIComponent(name)}.json`;

    fetch(jsonUrl)
        .then(r => {
            if (!r.ok) throw new Error(`HTTP ${r.status}`);
            return r.json();
        })
        .then(cfg => {
            overlayConfig = cfg;
            // Analyse content flags and handle km subscription + UI visibility
            analyzeOverlayContent(cfg);
            // Resize canvas to exactly match the overlay's native dimensions — no scaling.
            if (cfg.overlay_width && cfg.overlay_height) {
                canvasW = cfg.overlay_width;
                canvasH = cfg.overlay_height;
                setupCanvas();
            }
            // Derive texture filename: same name as JSON but with .png extension
            const pngUrl = `${baseUrl}${encodeURIComponent(name)}.png`;
            const img = new Image();
            img.onload = () => {
                overlayTexture = img;
                overlayReady = true;
                console.log(`Input Overlay '${name}' loaded (${cfg.overlay_width}x${cfg.overlay_height})`);
            };
            img.onerror = () => {
                console.error(`Failed to load texture: ${pngUrl}`);
                // Still mark ready so we at least show something (will skip texture draws)
                overlayReady = true;
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
    if (down)  return 4;
    return 0;
}

// Draw a single sprite from the texture atlas onto the canvas.
// (sx, sy, sw, sh) = source region in texture; (dx, dy, dw, dh) = destination on canvas.
function ioDrawSprite(sx, sy, sw, sh, dx, dy, dw, dh) {
    if (!overlayTexture || sw <= 0 || sh <= 0 || dw <= 0 || dh <= 0) return;
    ctx.drawImage(overlayTexture, sx, sy, sw, sh, dx, dy, dw, dh);
}

// Draw a partially-filled sprite for progressive trigger rendering.
// fillRatio: 0.0 (empty) … 1.0 (full). direction: 1=up, 2=down, 3=left, 4=right.
function ioDrawTriggerFill(sx, sy, sw, sh, dx, dy, dw, dh, fillRatio, direction) {
    if (!overlayTexture || fillRatio <= 0) return;

    ctx.save();
    // Clip to the fill region on the canvas
    ctx.beginPath();
    switch (direction) {
        case 1: // fill upward (from bottom)
            ctx.rect(dx, dy + dh * (1 - fillRatio), dw, dh * fillRatio);
            break;
        case 2: // fill downward (from top)
            ctx.rect(dx, dy, dw, dh * fillRatio);
            break;
        case 3: // fill leftward (from right)
            ctx.rect(dx + dw * (1 - fillRatio), dy, dw * fillRatio, dh);
            break;
        case 4: // fill rightward (from left)
            ctx.rect(dx, dy, dw * fillRatio, dh);
            break;
        default:
            ctx.rect(dx, dy, dw, dh * fillRatio);
    }
    ctx.clip();
    ctx.drawImage(overlayTexture, sx, sy, sw, sh, dx, dy, dw, dh);
    ctx.restore();
}

// Main Input Overlay draw function
function drawInputOverlay() {
    if (!overlayConfig) {
        // Config not yet loaded. Show "no controller" placeholder only when the overlay
        // will need gamepad data (otherwise show nothing while assets load).
        if (!simpleMode && overlayHasGamepad) drawDisconnected();
        return;
    }
    if (!overlayReady) return; // still loading

    const cfg = overlayConfig;

    // Sort elements by z_level (ascending), stable sort
    const elements = [...(cfg.elements || [])].sort((a, b) => {
        const za = Number(a.z_level) || 0;
        const zb = Number(b.z_level) || 0;
        return za - zb;
    });

    const BORDER = 3; // CFG_INNER_BORDER between sprite states

    for (const el of elements) {
        const type = el.type;
        if (!el.mapping || el.mapping.length < 4) continue;

        const [mu, mv, mw, mh] = el.mapping;
        const [px, py] = el.pos || [0, 0];

        // Destination rectangle on canvas (1:1 overlay coordinates)
        const dx = px;
        const dy = py;
        const dw = mw;
        const dh = mh;

        switch (type) {
            case 0: {
                // Static texture element (e.g. controller body background).
                // Always render, even in simple mode — the controller outline is part
                // of the texture atlas, not the page/canvas background color.
                ioDrawSprite(mu, mv, mw, mh, dx, dy, dw, dh);
                break;
            }

            case 2: {
                // Gamepad button (digital)
                const pressed = ioButtonPressed(el.code);
                const su = mu;
                const sv = pressed ? mv + mh + BORDER : mv;
                ioDrawSprite(su, sv, mw, mh, dx, dy, dw, dh);
                break;
            }

            case 5: {
                // Analog stick
                const side = el.side === 1 ? 'right' : 'left';
                const stickState = state.sticks[side];
                const radius = el.stick_radius || 40;

                // Knob offset in canvas pixels
                const kx = stickState.position.x * radius;
                const ky = -stickState.position.y * radius; // Y inverted

                // Pressed state: use second sprite (offset vertically by mh + BORDER)
                const sv = stickState.pressed ? mv + mh + BORDER : mv;
                ioDrawSprite(mu, sv, mw, mh, dx + kx, dy + ky, dw, dh);
                break;
            }

            case 6: {
                // Trigger (analog or button mode)
                const side = el.side === 1 ? 'rt' : 'lt';
                const value = state.triggers[side].value; // 0.0 … 1.0
                const triggerMode = el.trigger_mode === true;
                const direction = el.direction || 1;

                if (triggerMode) {
                    // Binary button mode
                    const pressed = value > 0.1;
                    const sv = pressed ? mv + mh + BORDER : mv;
                    ioDrawSprite(mu, sv, mw, mh, dx, dy, dw, dh);
                } else {
                    // Progressive fill mode: draw base sprite, then overlay fill
                    ioDrawSprite(mu, mv, mw, mh, dx, dy, dw, dh);
                    if (value > 0) {
                        const pressedV = mv + mh + BORDER;
                        ioDrawTriggerFill(mu, pressedV, mw, mh, dx, dy, dw, dh, value, direction);
                    }
                }
                break;
            }

            case 7: {
                // Gamepad player ID / guide button indicator
                // Sprite sheet: 5 states horizontally (player 1-4, guide pressed)
                // Draw the guide-pressed sprite behind the player sprite when guide is held
                const playerIdx = Math.min(Math.max((selectedPlayerIndex || 1) - 1, 0), 3);
                const guidePressed = !!state.buttons.guide;
                if (guidePressed) {
                    // Guide pressed = sprite index 4
                    const gsx = mu + 4 * (mw + BORDER);
                    ioDrawSprite(gsx, mv, mw, mh, dx, dy, dw, dh);
                }
                // Player number sprite
                const psx = mu + playerIdx * (mw + BORDER);
                ioDrawSprite(psx, mv, mw, mh, dx, dy, dw, dh);
                break;
            }

            case 8: {
                // Composite D-pad (9 direction sprites arranged horizontally)
                const dpadIdx = ioDpadIndex();
                const dsx = mu + dpadIdx * (mw + BORDER);
                ioDrawSprite(dsx, mv, mw, mh, dx, dy, dw, dh);
                break;
            }

            case 1: {
                // Keyboard key — two-frame vertical sprite layout (same as gamepad button)
                // el.code = uiohook scancode
                const pressed = !!kmState.keys[el.code];
                const sv = pressed ? mv + mh + BORDER : mv;
                ioDrawSprite(mu, sv, mw, mh, dx, dy, dw, dh);
                break;
            }

            case 3: {
                // Mouse button — two-frame vertical sprite layout (same as gamepad button)
                // el.code: 1=left, 2=right, 3=middle, 4=X1, 5=X2
                const pressed = !!kmState.mouseButtons[el.code];
                const sv = pressed ? mv + mh + BORDER : mv;
                ioDrawSprite(mu, sv, mw, mh, dx, dy, dw, dh);
                break;
            }

            case 4: {
                // Mouse wheel — 4 horizontal frames:
                //   frame0 [u,        v]: neutral (middle button released)
                //   frame1 [u+w+3,    v]: middle button pressed
                //   frame2 [u+(w+3)*2,v]: scroll up
                //   frame3 [u+(w+3)*3,v]: scroll down
                // Neutral frame is always drawn first; active states are layered on top.
                const now = performance.now();
                const wheelExpired = (now - kmState.wheelTimestamp) > WHEEL_TIMEOUT_MS;
                if (wheelExpired) {
                    kmState.wheelUp = false;
                    kmState.wheelDown = false;
                }
                // Always draw neutral frame
                ioDrawSprite(mu, mv, mw, mh, dx, dy, dw, dh);
                // Middle button pressed
                if (kmState.mouseButtons[3]) {
                    ioDrawSprite(mu + (mw + BORDER), mv, mw, mh, dx, dy, dw, dh);
                }
                // Scroll up overlay
                if (kmState.wheelUp && !wheelExpired) {
                    ioDrawSprite(mu + (mw + BORDER) * 2, mv, mw, mh, dx, dy, dw, dh);
                }
                // Scroll down overlay
                if (kmState.wheelDown && !wheelExpired) {
                    ioDrawSprite(mu + (mw + BORDER) * 3, mv, mw, mh, dx, dy, dw, dh);
                }
                break;
            }

            case 9: {
                // Mouse movement indicator
                // el.mouse_type: 0 = Move (sprite translates), 1 = Point (sprite rotates)
                // el.mouse_radius: max offset in overlay coordinates
                const radius = el.mouse_radius || 40;
                const mx = kmState.mouseMove.x;
                const my = kmState.mouseMove.y;

                if (el.mouse_type === 0) {
                    // Move mode: shift sprite position by movement delta * radius
                    const offsetX = mx * radius;
                    const offsetY = my * radius;
                    ioDrawSprite(mu, mv, mw, mh, dx + offsetX, dy + offsetY, dw, dh);
                } else {
                    // Point mode: rotate sprite to face movement direction
                    if (mx === 0 && my === 0) {
                        // No movement: draw unrotated
                        ioDrawSprite(mu, mv, mw, mh, dx, dy, dw, dh);
                    } else {
                        // Sprite faces up by default, so angle 0 = up.
                        // atan2(mx, -my): up=(0,-1)→0, right=(1,0)→π/2, etc.
                        const angle = Math.atan2(mx, -my);
                        ctx.save();
                        ctx.translate(dx + dw / 2, dy + dh / 2);
                        ctx.rotate(angle);
                        ctx.drawImage(overlayTexture,
                            mu, mv, mw, mh,
                            -dw / 2, -dh / 2, dw, dh);
                        ctx.restore();
                    }
                }
                break;
            }

            default:
                break;
        }
    }
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

        // Scale logical canvas to fit viewport, preserving aspect ratio
        const scaleX = vw / canvasW;
        const scaleY = vh / canvasH;
        const scale = Math.min(scaleX, scaleY);

        // Center
        const offsetX = (vw - canvasW * scale) / 2;
        const offsetY = (vh - canvasH * scale) / 2;

        ctx.setTransform(dpr * scale, 0, 0, dpr * scale, dpr * offsetX, dpr * offsetY);
    } else {
        // In overlay mode, size the canvas element to exactly the overlay dimensions.
        // In geometric mode, the canvas is sized by CSS (max-width layout).
        if (overlayName !== null) {
            canvas.style.width  = canvasW + 'px';
            canvas.style.height = canvasH + 'px';
        }
        const rect = canvas.getBoundingClientRect();
        canvas.width  = rect.width  * dpr;
        canvas.height = rect.height * dpr;
        ctx.scale(dpr, dpr);
    }
}

// Logical canvas dimensions.
// In geometric mode: fixed 500×330.
// In overlay mode: set to overlay_width × overlay_height once the config is loaded.
let canvasW = 500;
let canvasH = 330;

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
    ctx.clearRect(0, 0, canvasW, canvasH);

    if (overlayName !== null) {
        // Input Overlay rendering mode.
        // Render if: gamepad is connected (for gamepad elements), OR the overlay only has
        // keyboard/mouse elements (which are always available regardless of gamepad state).
        const canRender = state.connected || !overlayHasGamepad;
        if (!canRender) {
            if (!simpleMode) drawDisconnected();
        } else {
            drawInputOverlay();
        }
    } else {
        // Built-in geometric rendering mode
        if (!state.connected) {
            if (!simpleMode) {
                drawDisconnected();
            }
        } else {
            drawController();
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

    // Parse overlay parameter: enables Input Overlay rendering mode
    const overlayParam = urlParams.get('overlay');
    if (overlayParam) {
        overlayName = overlayParam;
        // Optimistically hide gamepad UI until the config tells us it's needed.
        // This avoids a visible flash of the controller-info bar on load.
        overlayHasGamepad = false;
        loadInputOverlayConfig(overlayName);
    }

    // Parse mouse_sens parameter: controls sensitivity of mouse_movement elements.
    // Value is sent to the backend via a "set_mouse_sens" message after connection.
    const sensParam = urlParams.get('mouse_sens');
    if (sensParam !== null) {
        const sens = parseFloat(sensParam);
        if (!isNaN(sens) && sens > 0) {
            // Store for sending after WebSocket connection opens
            window._mouseSens = sens;
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

    // Load xbox config as default (only used when not in Input Overlay mode)
    if (overlayName === null) {
        fetch('configs/xbox.json')
            .then(r => r.json())
            .then(config => {
                configCache['xbox'] = config;
                currentConfig = config;
                loadedConfigType = 'xbox';
            })
            .catch(e => console.error('Failed to load default config:', e));
    }

    connectWebSocket();
    requestAnimationFrame(render);
}

init();
