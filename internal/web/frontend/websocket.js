// ============================================================
// WebSocket Connection & Message Handling
// ============================================================

function connectWebSocket() {
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${location.host}/ws`;

    ws = new WebSocket(url);

    ws.onopen = () => {
        reconnectDelay = RECONNECT_DELAY_INITIAL;
        setWSStatus(true);
        // Send selected player index to backend, unless the overlay has no gamepad elements
        // (in that case we don't need gamepad data at all).
        if ((overlayName === null && (!explicitMode || hasGamepadParam)) || (overlayName !== null && overlayHasGamepad)) {
            ws.send(JSON.stringify({ type: 'select_player', playerIndex: selectedPlayerIndex }));
        }

        // Send mouse sensitivity if user provided it via URL parameter
        if (mouseSens > 0) {
            ws.send(JSON.stringify({ type: 'set_mouse_sens', value: mouseSens }));
        }

        // Re-subscribe to keyboard/mouse if we already determined that the overlay needs it
        if (kmSubscribed) {
            ws.send(JSON.stringify({ type: 'subscribe_km' }));
        }

        if (explicitMode && (hasMouseParam || keyboardParam !== null) && !kmSubscribed) {
            kmSubscribed = true;
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
            handleMessage(JSON.parse(event.data));
        } catch (e) {
            console.error('Failed to parse message:', e);
        }
    };
}

function scheduleReconnect() {
    setTimeout(() => {
        connectWebSocket();
        reconnectDelay = Math.min(reconnectDelay * 1.5, RECONNECT_DELAY_MAX);
    }, reconnectDelay);
}

function setWSStatus(connected) {
    wsConnected = connected;
    updateStatusIndicator();
}

function updateStatusIndicator() {
    // In overlay mode with no gamepad elements, the status indicator is hidden -- skip update.
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
// Message Dispatch & State Merging
// ============================================================

function handleMessage(msg) {
    switch (msg.type) {
        case 'full':
            if (msg.data) applyFullState(msg.data);
            break;
        case 'delta':
            if (msg.changes) applyDelta(msg.changes);
            break;
        case 'player_selected':
            // Acknowledged -- nothing to do on frontend
            break;
        case 'km_full':
            if (msg.kmState) applyKMFull(msg.kmState);
            break;
        case 'km_delta':
            if (msg.kmDelta) applyKMDelta(msg.kmDelta);
            break;
    }
}

// Apply a full keyboard/mouse state snapshot.
function applyKMFull(data) {
    // Rebuild keys map
    for (const k in kmState.keys) delete kmState.keys[k];
    if (data.keys) {
        for (const [code, pressed] of Object.entries(data.keys)) {
            if (pressed) kmState.keys[Number(code)] = true;
        }
    }
    // Rebuild mouse buttons map
    for (const k in kmState.mouseButtons) delete kmState.mouseButtons[k];
    if (data.mouseButtons) {
        for (const [code, pressed] of Object.entries(data.mouseButtons)) {
            if (pressed) kmState.mouseButtons[Number(code)] = true;
        }
    }
    // Mouse movement and wheel are per-tick -- not meaningful in a full snapshot
    kmState.mouseMove.x = 0;
    kmState.mouseMove.y = 0;
    kmState.wheelUp = false;
    kmState.wheelDown = false;
    kmState.wheelTimestamp = 0;
    if (wheelDirtyTimeoutId) {
        clearTimeout(wheelDirtyTimeoutId);
        wheelDirtyTimeoutId = null;
    }
    dirty = true;
    markRendererDirty(['mouse', 'keyboard']);
}

// Apply an incremental keyboard/mouse delta.
function applyKMDelta(delta) {
    if (delta.keysDown) {
        for (const code of delta.keysDown) kmState.keys[code] = true;
    }
    if (delta.keysUp) {
        for (const code of delta.keysUp) delete kmState.keys[code];
    }
    if (delta.buttonsDown) {
        for (const code of delta.buttonsDown) kmState.mouseButtons[code] = true;
    }
    if (delta.buttonsUp) {
        for (const code of delta.buttonsUp) delete kmState.mouseButtons[code];
    }
    kmState.mouseMove.x = delta.mouseMove ? delta.mouseMove.x : 0;
    kmState.mouseMove.y = delta.mouseMove ? delta.mouseMove.y : 0;
    if (delta.wheelUp || delta.wheelDown) {
        kmState.wheelUp = delta.wheelUp || false;
        kmState.wheelDown = delta.wheelDown || false;
        kmState.wheelTimestamp = performance.now();
        if (wheelDirtyTimeoutId) clearTimeout(wheelDirtyTimeoutId);
        wheelDirtyTimeoutId = setTimeout(() => {
            wheelDirtyTimeoutId = null;
            kmState.wheelUp = false;
            kmState.wheelDown = false;
            dirty = true;
            markRendererDirty(['mouse']);
        }, WHEEL_TIMEOUT_MS + 20);
    }
    dirty = true;
    markRendererDirty(['mouse', 'keyboard']);
}

// Merge gamepad state fields. For delta messages some fields may be absent (undefined).
// Pointer-typed delta fields (buttons, dpad, sticks, triggers) are always full objects
// when present -- replace the whole sub-object, not individual properties.
function mergeState(target, source) {
    if (source.connected !== undefined) target.connected = source.connected;
    if (source.controllerType !== undefined) target.controllerType = source.controllerType;
    if (source.name !== undefined) target.name = source.name;
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
    // triggers: backend sends the complete TriggersState object when either value changes.
    if (source.triggers) {
        if (source.triggers.lt !== undefined) target.triggers.lt.value = source.triggers.lt.value ?? 0;
        if (source.triggers.rt !== undefined) target.triggers.rt.value = source.triggers.rt.value ?? 0;
    }
}

function applyFullState(data) {
    mergeState(state, data);
    enforceForcedGamepadType();
    updateControllerInfo();
    loadConfigIfNeeded();
    dirty = true;
    markRendererDirty(['gamepad']);
}

function applyDelta(changes) {
    mergeState(state, changes);
    enforceForcedGamepadType();
    updateControllerInfo();
    loadConfigIfNeeded();
    dirty = true;
    markRendererDirty(['gamepad']);
}

function updateControllerInfo() {
    // In overlay mode with no gamepad elements, the info bar is hidden -- skip update.
    if (overlayName !== null && !overlayHasGamepad) return;
    const el = document.getElementById('controller-name');
    if (state.connected && state.name) {
        el.textContent = `Player ${selectedPlayerIndex}: ${state.name} (${state.controllerType})`;
    } else {
        el.textContent = `Player ${selectedPlayerIndex}: No controller detected`;
    }
    updateStatusIndicator();
}
