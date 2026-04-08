// ============================================================
// Configuration Loading (Gamepad, Keyboard, Input Overlay)
// ============================================================

// --- Gamepad Config ---

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

// --- Keyboard Config ---

function loadKeyboardConfig(name) {
    if (keyboardConfigCache[name]) {
        applyKeyboardConfig(name, keyboardConfigCache[name]);
        return;
    }
    // Try external keyboards/ directory first (user overrides embedded)
    fetch('/keyboards/' + name + '.json')
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

    const { canvasW: kw, canvasH: kh } = computeKeyboardDimensions(config);
    renderer.canvasW = kw;
    renderer.canvasH = kh;
    renderer.keyboardConfig = config;
    renderer.keyboardLayout = null; // will be computed by drawKeyboardRenderer

    // Resize canvas DOM element
    renderer.canvas.width = kw;
    renderer.canvas.height = kh;
    setupRendererCanvas(renderer);
    renderer.dirty = true;
}

// --- Input Overlay Config & Texture ---

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
                // PNG failed -- try SVG fallback
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
