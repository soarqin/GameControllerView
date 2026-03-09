# Build script for GameControllerView (Windows, release mode)
# Release build: -tags release enables GUI mode (no console window, system tray)
#                -H=windowsgui suppresses the console window at the linker level

Write-Host "Building GameControllerView..."

go build -tags release -ldflags "-s -w -H=windowsgui" -o GameControllerView.exe ./cmd/gamecontrollerview
if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed!"
    exit 1
}

Write-Host "Build complete: GameControllerView.exe"
