#!/bin/sh
# Build script for InputView (Linux/macOS, release mode)
# Release build: -tags release enables GUI mode (no console output)

set -e

echo "Building InputView..."

go build -tags release -ldflags "-s -w" -o InputView ./cmd/inputview

echo "Build complete: InputView"
