@echo off
echo Building GameControllerView...
pushd backend >NUL
go build -ldflags "-s -w -H=windowsgui" -o GameControllerView.exe
if errorlevel 1 (
    echo Build failed!
    exit /b 1
)
popd >NUL
echo Build complete: backend\GameControllerView.exe
pause
