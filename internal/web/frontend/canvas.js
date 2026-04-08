// ============================================================
// Canvas Setup & Render Loop
// ============================================================

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
    ctx.font = cachedFont(20);
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText('No controller connected', canvasW / 2, canvasH / 2);
    ctx.font = cachedFont(14);
    ctx.fillText('Connect a gamepad and it will appear here', canvasW / 2, canvasH / 2 + 30);
}
