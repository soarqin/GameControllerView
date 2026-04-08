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

    const btnAlphaParam = urlParams.get('btnalpha');
    if (btnAlphaParam !== null) {
        const ba = parseFloat(btnAlphaParam);
        if (!isNaN(ba) && ba >= 0 && ba <= 1) buttonAlpha = ba;
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
            const controllerInfo = document.getElementById('controller-info');
            const statusEl = document.getElementById('status');
            if (controllerInfo) controllerInfo.style.display = 'none';
            if (statusEl) statusEl.style.display = 'none';
        }

        // Determine the order of device params as they appear in the URL query string.
        // This lets users control left-to-right layout via param order, e.g.
        // ?keyboard=wasd&gamepad&mouse=1  ->  keyboard | gamepad | mouse
        const deviceOrder = [];
        const rawQuery = window.location.search.substring(1); // strip leading '?'
        const seen = new Set();
        for (const part of rawQuery.split('&')) {
            const key = decodeURIComponent(part.split('=')[0]);
            if (seen.has(key)) continue;
            seen.add(key);
            if (key === 'gamepad' && hasGamepadParam) deviceOrder.push('gamepad');
            else if (key === 'mouse' && hasMouseParam) deviceOrder.push('mouse');
            else if (key === 'keyboard' && keyboardParam !== null) deviceOrder.push('keyboard');
        }

        for (const device of deviceOrder) {
            if (device === 'gamepad') {
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
            } else if (device === 'mouse') {
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
            } else if (device === 'keyboard') {
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
