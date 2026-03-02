@echo off
echo Building GameControllerView...
go build -ldflags "-s -w -H=windowsgui" -o GameControllerView.exe ./cmd/gamecontrollerview
if errorlevel 1 (
    echo Build failed!
    exit /b 1
)
echo Build complete: GameControllerView.exe
pause
