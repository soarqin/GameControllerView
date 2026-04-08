// ============================================================
// Built-in Keyboard Geometric Renderer
// ============================================================

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
        renderer.ctx.font = cachedFont(14);
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

    c.fillStyle = kbBgFillColor;
    c.fillRect(0, 0, renderer.canvasW, renderer.canvasH);

    const savedCtx = ctx;
    c.save();
    c.globalAlpha = buttonAlpha;
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
            ? cachedFont(fontSize, 'bold')
            : cachedFont(Math.max(7, fontSize - 2));

        c.fillText(key.label, key.x + key.w / 2, key.y + key.h / 2);
    }
    c.restore();
    ctx = savedCtx;
}
