// ============================================================
// Built-in Gamepad Geometric Renderer
// ============================================================

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
            if (!body._parsedVB) {
                const parts = body.viewBox.split(/[\s,]+/).map(Number);
                body._parsedVB = { x: parts[0], y: parts[1], w: parts[2], h: parts[3] };
            }
            const vb = body._parsedVB;
            const targetX = body.x || 0;
            const targetY = body.y || 0;
            const targetWidth = body.width || 500;
            const targetHeight = body.height || 330;
            const scale = Math.min(targetWidth / vb.w, targetHeight / vb.h);
            const scaledWidth = vb.w * scale;
            const scaledHeight = vb.h * scale;
            const offsetX = targetX + (targetWidth - scaledWidth) / 2 - vb.x * scale;
            const offsetY = targetY + (targetHeight - scaledHeight) / 2 - vb.y * scale;
            ctx.translate(offsetX, offsetY);
            ctx.scale(scale, scale);
        }

        if (!body._cachedPath) body._cachedPath = new Path2D(body.path);
        const path = body._cachedPath;
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
    '\u00D7': (x, y, size) => {
        ctx.beginPath();
        ctx.moveTo(x - size, y - size); ctx.lineTo(x + size, y + size);
        ctx.moveTo(x + size, y - size); ctx.lineTo(x - size, y + size);
        ctx.stroke();
    },
    '\u25CB': (x, y, size) => {
        ctx.beginPath(); ctx.arc(x, y, size, 0, Math.PI * 2); ctx.stroke();
    },
    '\u25A1': (x, y, size) => {
        ctx.beginPath(); ctx.rect(x - size, y - size, size * 2, size * 2); ctx.stroke();
    },
    '\u25B3': (x, y, size) => {
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
            ctx.font = cachedFont(fontSize, fontWeight);
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
            ctx.font = cachedFont(fontSize, fontWeight);
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
        ctx.font = cachedFont(fontSize, fontWeight);
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
                ctx.font = cachedFont(fontSize, fontWeight);
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
                ctx.font = cachedFont(fontSize, fontWeight);
                ctx.textAlign = 'center';
                ctx.textBaseline = 'middle';
                ctx.fillText(labelText, b.x + b.width / 2, b.y + b.height / 2 + 1);
            }
        }
    }
}
