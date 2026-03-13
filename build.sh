#!/bin/sh
# Build script for InputView (Linux/macOS, release mode)
# Release build: -tags release enables GUI mode (no console output)

set -e

# ---------------------------------------------------------------------------
# Update gamecontrollerdb.txt from SDL_GameControllerDB if the remote has
# a newer version.  Comparison is done via the Git blob SHA-1 reported by the
# GitHub Contents API, which equals SHA1("blob <size>\0<content>").
# The update is skipped (with a warning, not an error) when curl/sha1sum/
# shasum is unavailable or the API is unreachable.
# ---------------------------------------------------------------------------
DB_PATH="internal/gamepad/gamecontrollerdb.txt"
API_URL="https://api.github.com/repos/mdqinc/SDL_GameControllerDB/contents/gamecontrollerdb.txt"
RAW_URL="https://raw.githubusercontent.com/mdqinc/SDL_GameControllerDB/master/gamecontrollerdb.txt"

echo "Checking gamecontrollerdb.txt for updates..."

# Determine available SHA-1 tool
if command -v sha1sum > /dev/null 2>&1; then
    SHA1_CMD="sha1sum"
elif command -v shasum > /dev/null 2>&1; then
    SHA1_CMD="shasum -a 1"
else
    SHA1_CMD=""
fi

if [ -z "$SHA1_CMD" ] || ! command -v curl > /dev/null 2>&1; then
    echo "Warning: curl or sha1sum/shasum not available; skipping gamecontrollerdb.txt update check."
else
    # Fetch remote metadata from GitHub Contents API
    API_JSON=$(curl -sf --max-time 15 \
        -H "User-Agent: InputView-build" \
        -H "Accept: application/vnd.github.v3+json" \
        "$API_URL" 2>/dev/null) || true

    if [ -z "$API_JSON" ]; then
        echo "Warning: Could not reach GitHub API; skipping gamecontrollerdb.txt update check."
    else
        REMOTE_SHA=$(printf '%s' "$API_JSON" | grep -o '"sha": *"[^"]*"' | head -1 | grep -o '"[^"]*"$' | tr -d '"')

        if [ -z "$REMOTE_SHA" ]; then
            echo "Warning: Could not parse remote sha; skipping update check."
        else
            # Compute Git blob SHA-1: SHA1("blob <size>\0<content>")
            LOCAL_SIZE=$(wc -c < "$DB_PATH" | tr -d ' ')
            # Use printf to produce "blob <size>\0" header, then append file
            LOCAL_SHA=$(
                { printf "blob %s\0" "$LOCAL_SIZE"; cat "$DB_PATH"; } \
                | $SHA1_CMD | awk '{print $1}'
            )

            if [ "$LOCAL_SHA" = "$REMOTE_SHA" ]; then
                echo "gamecontrollerdb.txt is up to date (sha: $LOCAL_SHA)."
            else
                echo "Updating gamecontrollerdb.txt (local: $LOCAL_SHA  remote: $REMOTE_SHA)..."
                curl -f --max-time 60 \
                    -H "User-Agent: InputView-build" \
                    -o "$DB_PATH" "$RAW_URL" \
                    && echo "gamecontrollerdb.txt updated." \
                    || echo "Warning: Failed to download gamecontrollerdb.txt; keeping existing file."
            fi
        fi
    fi
fi

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
echo "Building InputView..."

go build -tags release -ldflags "-s -w" -o InputView ./cmd/inputview

echo "Build complete: InputView"
