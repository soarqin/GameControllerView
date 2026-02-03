@echo off
echo Building GameControllerView...
cd backend
go build -ldflags "-s -w -H=windowsgui" -o GameControllerView.exe
if errorlevel 1 (
    echo Build failed!
    exit /b 1
)
echo Build complete: backend\GameControllerView.exe
pause
