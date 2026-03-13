# Build script for InputView (Windows, release mode)
# Release build: -tags release enables GUI mode (no console window, system tray)
#                -H=windowsgui suppresses the console window at the linker level

# ---------------------------------------------------------------------------
# Update gamecontrollerdb.txt from SDL_GameControllerDB if the remote has
# a newer version.  Comparison is done via the Git blob SHA-1 reported by the
# GitHub Contents API, which equals SHA1("blob <size>\0<content>").
# The update is skipped (with a warning, not an error) when the API is
# unreachable or returns an unexpected response.
# ---------------------------------------------------------------------------
$dbPath = "internal\gamepad\gamecontrollerdb.txt"
$apiUrl = "https://api.github.com/repos/mdqinc/SDL_GameControllerDB/contents/gamecontrollerdb.txt"
$rawUrl = "https://raw.githubusercontent.com/mdqinc/SDL_GameControllerDB/master/gamecontrollerdb.txt"

Write-Host "Checking gamecontrollerdb.txt for updates..."
try {
    $response = Invoke-RestMethod -Uri $apiUrl `
        -Headers @{ "User-Agent" = "InputView-build"; "Accept" = "application/vnd.github.v3+json" } `
        -TimeoutSec 15 -ErrorAction Stop

    $remoteSize = [long]$response.size
    $remoteSha  = $response.sha

    # Compute Git blob SHA-1: SHA1("blob <size>\0<content>")
    $localBytes  = [System.IO.File]::ReadAllBytes((Resolve-Path $dbPath))
    $header      = [System.Text.Encoding]::ASCII.GetBytes("blob $($localBytes.Length)`0")
    $sha1        = [System.Security.Cryptography.SHA1]::Create()
    $localSha    = ($sha1.ComputeHash([byte[]]($header + $localBytes)) |
                    ForEach-Object { $_.ToString("x2") }) -join ""

    if ($localSha -eq $remoteSha) {
        Write-Host "gamecontrollerdb.txt is up to date (sha: $localSha)."
    } else {
        Write-Host "Updating gamecontrollerdb.txt (local: $localSha  remote: $remoteSha)..."
        Invoke-WebRequest -Uri $rawUrl -OutFile $dbPath `
            -Headers @{ "User-Agent" = "InputView-build" } `
            -TimeoutSec 60 -ErrorAction Stop
        Write-Host "gamecontrollerdb.txt updated ($remoteSize bytes)."
    }
} catch {
    Write-Warning "Could not check/update gamecontrollerdb.txt: $_"
}

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
Write-Host "Building InputView..."

go build -tags release -ldflags "-s -w -H=windowsgui" -o InputView.exe ./cmd/inputview
if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed!"
    exit 1
}

Write-Host "Build complete: InputView.exe"
