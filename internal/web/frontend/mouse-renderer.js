// ============================================================
// Built-in Mouse Geometric Renderer
// ============================================================

function drawMouseRenderer(renderer) {
    const c = renderer.ctx;
    const cfg = MOUSE_CONFIG;

    // Body outline (SVG path)
    c.save();
    c.fillStyle = bodyFillColor;
    c.strokeStyle = COLORS.outline;
    c.lineWidth = 2;
    if (!cfg.body._cachedPath) cfg.body._cachedPath = new Path2D(cfg.body.path);
    const bodyPath = cfg.body._cachedPath;
    c.fill(bodyPath);
    c.stroke(bodyPath);
    c.restore();

    // Mouse buttons (middle button skipped -- drawn together with scroll wheel below)
    c.save();
    c.globalAlpha = buttonAlpha;
    const savedCtx = ctx;
    for (const [name, btn] of Object.entries(cfg.buttons)) {
        if (name === 'middle') continue;
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
        c.font = cachedFont(fontSize, 'bold');
        c.textAlign = 'center';
        c.textBaseline = 'middle';
        c.fillText(btn.label, btn.x + btn.width / 2, btn.y + btn.height / 2);
    }
    ctx = savedCtx;

    // Middle button + scroll wheel (combined: middle button is the wheel housing)
    const w = cfg.wheel;
    const mb = cfg.buttons.middle;
    const middlePressed = kmState.mouseButtons[mb.code] === true;
    const wheelActive = kmState.wheelUp || kmState.wheelDown;

    // Middle button border area (fill excludes wheel region to prevent overlap darkening)
    c.fillStyle = middlePressed ? COLORS.buttonPressed : COLORS.buttonDefault;
    c.beginPath();
    c.roundRect(mb.x, mb.y, mb.width, mb.height, mb.radius);
    c.roundRect(w.x, w.y, w.width, w.height, w.radius);
    c.fill('evenodd');
    c.strokeStyle = COLORS.outline;
    c.lineWidth = 1.5;
    c.beginPath();
    c.roundRect(mb.x, mb.y, mb.width, mb.height, mb.radius);
    c.stroke();

    // Scroll wheel indicator (on top of middle button)
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

    // Scroll chevrons when active, M label when idle
    if (wheelActive) {
        const chevronSize = 5;
        const chevronX = w.x + w.width / 2;
        c.strokeStyle = COLORS.buttonLabelPressed;
        c.lineWidth = 2;
        c.lineCap = 'round';
        c.lineJoin = 'round';
        if (kmState.wheelUp) {
            const cy = w.y + w.height * 0.25;
            c.beginPath();
            c.moveTo(chevronX - chevronSize, cy + chevronSize * 0.4);
            c.lineTo(chevronX, cy - chevronSize * 0.4);
            c.lineTo(chevronX + chevronSize, cy + chevronSize * 0.4);
            c.stroke();
        }
        if (kmState.wheelDown) {
            const cy = w.y + w.height * 0.75;
            c.beginPath();
            c.moveTo(chevronX - chevronSize, cy - chevronSize * 0.4);
            c.lineTo(chevronX, cy + chevronSize * 0.4);
            c.lineTo(chevronX + chevronSize, cy - chevronSize * 0.4);
            c.stroke();
        }
    } else {
        c.fillStyle = COLORS.buttonLabel;
        c.font = cachedFont(12, 'bold');
        c.textAlign = 'center';
        c.textBaseline = 'middle';
        c.fillText(mb.label, mb.x + mb.width / 2, mb.y + mb.height / 2);
    }

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
        const minArrowLen = mv.radius * 0.3;
        const arrowLen = Math.min(mv.radius * 0.75, Math.max(minArrowLen, magnitude * mv.radius));
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
            arrowX - headLen * Math.sin(angle + headAngle),
            arrowY + headLen * Math.cos(angle + headAngle)
        );
        c.moveTo(arrowX, arrowY);
        c.lineTo(
            arrowX - headLen * Math.sin(angle - headAngle),
            arrowY + headLen * Math.cos(angle - headAngle)
        );
        c.stroke();
    }
    c.restore();
}
