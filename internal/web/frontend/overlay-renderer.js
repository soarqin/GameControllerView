// ============================================================
// Input Overlay: Texture Atlas Renderer
// ============================================================

function ioButtonPressed(code) {
    const getter = IO_BUTTON_CODE_MAP[code];
    return getter ? !!getter(state) : false;
}

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
    return 4;
}

function ioDrawSprite(sx, sy, sw, sh, dx, dy, dw, dh) {
    if (!overlayTexture || sw <= 0 || sh <= 0 || dw <= 0 || dh <= 0) return;
    ctx.drawImage(overlayTexture, sx, sy, sw, sh, dx, dy, dw, dh);
}

function ioDrawTriggerFill(sx, sy, sw, sh, dx, dy, dw, dh, fillRatio, direction) {
    if (!overlayTexture || fillRatio <= 0) return;
    ctx.save();
    ctx.beginPath();
    switch (direction) {
        case 1: ctx.rect(dx, dy + dh * (1 - fillRatio), dw, dh * fillRatio); break;
        case 2: ctx.rect(dx, dy, dw, dh * fillRatio); break;
        case 3: ctx.rect(dx + dw * (1 - fillRatio), dy, dw * fillRatio, dh); break;
        case 4: ctx.rect(dx, dy, dw * fillRatio, dh); break;
        default: ctx.rect(dx, dy, dw, dh * fillRatio);
    }
    ctx.clip();
    ctx.drawImage(overlayTexture, sx, sy, sw, sh, dx, dy, dw, dh);
    ctx.restore();
}

function drawInputOverlay() {
    if (!overlayConfig) {
        if (!simpleMode && overlayHasGamepad) drawDisconnected();
        return;
    }
    if (overlayLoadFailed) {
        ctx.fillStyle = '#ff0000';
        ctx.font = cachedFont(16);
        ctx.textAlign = 'center';
        ctx.fillText('Texture failed to load', canvasW / 2, canvasH / 2);
        return;
    }
    if (!overlayReady) return;

    const elements = overlayConfig.elements || [];

    for (const el of elements) {
        const type = el.type;
        if (!el.mapping || el.mapping.length < 4) continue;

        const [mu, mv, mw, mh] = el.mapping;
        const [px, py] = el.pos || [0, 0];
        const dx = px, dy = py, dw = mw, dh = mh;

        switch (type) {
            case 0: {
                ioDrawSprite(mu, mv, mw, mh, dx, dy, dw, dh);
                break;
            }
            case 1: {
                // Keyboard key
                const sv = kmState.keys[el.code] ? mv + mh + OVERLAY_SPRITE_BORDER : mv;
                ioDrawSprite(mu, sv, mw, mh, dx, dy, dw, dh);
                break;
            }
            case 2: {
                // Gamepad button (digital)
                const sv = ioButtonPressed(el.code) ? mv + mh + OVERLAY_SPRITE_BORDER : mv;
                ioDrawSprite(mu, sv, mw, mh, dx, dy, dw, dh);
                break;
            }
            case 3: {
                // Mouse button
                const sv = kmState.mouseButtons[el.code] ? mv + mh + OVERLAY_SPRITE_BORDER : mv;
                ioDrawSprite(mu, sv, mw, mh, dx, dy, dw, dh);
                break;
            }
            case 4: {
                // Mouse wheel -- 4 horizontal frames: neutral / middle-pressed / scroll-up / scroll-down
                ioDrawSprite(mu, mv, mw, mh, dx, dy, dw, dh);
                if (kmState.mouseButtons[3]) {
                    ioDrawSprite(mu + (mw + OVERLAY_SPRITE_BORDER), mv, mw, mh, dx, dy, dw, dh);
                }
                if (kmState.wheelUp) {
                    ioDrawSprite(mu + (mw + OVERLAY_SPRITE_BORDER) * 2, mv, mw, mh, dx, dy, dw, dh);
                }
                if (kmState.wheelDown) {
                    ioDrawSprite(mu + (mw + OVERLAY_SPRITE_BORDER) * 3, mv, mw, mh, dx, dy, dw, dh);
                }
                break;
            }
            case 5: {
                // Analog stick
                const side = el.side === 1 ? 'right' : 'left';
                const stickState = state.sticks[side];
                const radius = el.stick_radius || 40;
                const kx = stickState.position.x * radius;
                const ky = -stickState.position.y * radius;
                const sv = stickState.pressed ? mv + mh + OVERLAY_SPRITE_BORDER : mv;
                ioDrawSprite(mu, sv, mw, mh, dx + kx, dy + ky, dw, dh);
                break;
            }
            case 6: {
                // Trigger
                const side = el.side === 1 ? 'rt' : 'lt';
                const value = state.triggers[side].value;
                const direction = el.direction || 1;
                if (el.trigger_mode === true) {
                    const sv = value > TRIGGER_PRESS_THRESHOLD ? mv + mh + OVERLAY_SPRITE_BORDER : mv;
                    ioDrawSprite(mu, sv, mw, mh, dx, dy, dw, dh);
                } else {
                    ioDrawSprite(mu, mv, mw, mh, dx, dy, dw, dh);
                    if (value > 0) {
                        ioDrawTriggerFill(mu, mv + mh + OVERLAY_SPRITE_BORDER, mw, mh, dx, dy, dw, dh, value, direction);
                    }
                }
                break;
            }
            case 7: {
                // Gamepad player ID / guide button indicator
                const playerIdx = Math.min(Math.max((selectedPlayerIndex || 1) - 1, 0), 3);
                if (state.buttons.guide) {
                    ioDrawSprite(mu + 4 * (mw + OVERLAY_SPRITE_BORDER), mv, mw, mh, dx, dy, dw, dh);
                }
                ioDrawSprite(mu + playerIdx * (mw + OVERLAY_SPRITE_BORDER), mv, mw, mh, dx, dy, dw, dh);
                break;
            }
            case 8: {
                // Composite D-pad
                const dsx = mu + ioDpadIndex() * (mw + OVERLAY_SPRITE_BORDER);
                ioDrawSprite(dsx, mv, mw, mh, dx, dy, dw, dh);
                break;
            }
            case 9: {
                // Mouse movement indicator
                const radius = el.mouse_radius || 40;
                const mx = kmState.mouseMove.x;
                const my = kmState.mouseMove.y;
                if (el.mouse_type === 0) {
                    ioDrawSprite(mu, mv, mw, mh, dx + mx * radius, dy + my * radius, dw, dh);
                } else {
                    if (mx === 0 && my === 0) {
                        ioDrawSprite(mu, mv, mw, mh, dx, dy, dw, dh);
                    } else {
                        const angle = Math.atan2(mx, -my);
                        ctx.save();
                        ctx.translate(dx + dw / 2, dy + dh / 2);
                        ctx.rotate(angle);
                        ctx.drawImage(overlayTexture, mu, mv, mw, mh, -dw / 2, -dh / 2, dw, dh);
                        ctx.restore();
                    }
                }
                break;
            }
        }
    }
}
