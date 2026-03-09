#!/bin/sh
# Build script for GameControllerView (Linux/macOS, release mode)
# Release build: -tags release enables GUI mode (no console output)

set -e

echo "Building GameControllerView..."

go build -tags release -ldflags "-s -w" -o GameControllerView ./cmd/gamecontrollerview

echo "Build complete: GameControllerView"
