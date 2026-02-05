# Console Package

Cross-platform console detection and Windows Ctrl+C signal handling for Go applications.

## Features

- **Smart Console Handling**: Intelligently handles console allocation based on build mode and launch method
  - **Console-mode build + terminal launch**: Reuses existing console
  - **Console-mode build + double-click**: Frees auto-created console (GUI mode)
  - **GUI-mode build + terminal launch**: Allocates new console for output
  - **GUI-mode build + double-click**: No console (pure GUI mode)
- **Reliable Ctrl+C Handling**: Works correctly even when using libraries like SDL3, OpenGL, or other C libraries that call `runtime.LockOSThread()`
- **Cross-Platform**: Gracefully degrades to no-op on non-Windows platforms
- **Re-registration Support**: Allows re-registering the console handler after library initialization (useful when libraries override console handlers)

## Installation

Copy the `console` directory to your project's `internal/` folder:

```
yourproject/
└── internal/
    └── console/
        └── console.go
```

## Usage

### Basic Usage

```go
package main

import (
    "fmt"
    "time"
    "yourproject/internal/console"
)

func main() {
    // Detect if running from console or GUI mode
    if console.IsRunningFromConsole() {
        fmt.Println("Running from terminal - press Ctrl+C to exit")
    } else {
        fmt.Println("Running in GUI mode")
    }

    // Set up Windows console handler
    shutdownChan := make(chan struct{})
    console.SetupConsoleHandler(shutdownChan)

    // Your main application logic here
    go func() {
        for {
            time.Sleep(1 * time.Second)
            fmt.Println("Running...")
        }
    }()

    // Wait for Ctrl+C
    <-shutdownChan
    fmt.Println("Shutting down...")
}
```

### Using with SDL3

```go
package main

import (
    "github.com/jupiterrider/purego-sdl3/sdl"
    "yourproject/internal/console"
    "yourproject/internal/gamepad"
)

func main() {
    // Set up console handler before SDL initialization
    shutdownChan := make(chan struct{})
    registerHandler := console.SetupConsoleHandler(shutdownChan)

    // Create reader with SDL init callback
    reader := gamepad.NewReader()
    reader.SetOnSDLInitCallback(func() {
        // Re-register console handler after SDL initialization
        // (SDL3 overrides console handlers during init)
        registerHandler()
    })

    // Run reader (initializes SDL)
    go reader.Run(ctx)

    // Wait for shutdown
    <-shutdownChan
}
```

## API Reference

### `func IsRunningFromConsole() bool`

Checks if the program is running from a terminal or in GUI mode.

**Returns:**
- `true` if running from a terminal (cmd/PowerShell) on Windows, or always on non-Windows platforms
- `false` if running in GUI mode (double-clicked) on Windows

**Side Effects:**
On Windows, intelligently manages console allocation:
- If a console window already exists (console-mode build or terminal launch), reuses it
- If no console exists and launched from a terminal (GUI-mode build), allocates a new console
- If a console exists but was double-clicked (console-mode build), frees the console to hide the window
- If no console and double-clicked (GUI-mode build), does nothing (pure GUI mode)

**Build Mode Compatibility:**
- **Console-mode build** (`go build`): Works correctly from both terminal and double-click
- **GUI-mode build** (with `-ldflags "-H windowsgui"`): Creates a new console window when launched from terminal, stays silent when double-clicked

**Important Notes for GUI-mode builds:**
- When launched from terminal, uses `AllocConsole()` to create an **independent console window**
  - The parent terminal is released immediately (GUI process doesn't block it)
  - Input/output won't mix with the parent terminal
- After allocating, redirects `os.Stdout`, `os.Stderr`, and `os.Stdin` so `fmt.Printf`, `log.Println`, etc. work correctly
- The log package's default output is also redirected to the new console
- Custom log writers are not affected (you need to update them manually if needed)

### `func SetupConsoleHandler(shutdownChan chan struct{}) func()`

Sets up a Windows console control handler for Ctrl+C and Ctrl+Break.

**Parameters:**
- `shutdownChan`: Channel that will be closed when Ctrl+C or Ctrl+Break is pressed

**Returns:**
- A function that can be called to re-register the handler (useful after library initialization)

**Platform Behavior:**
- **Windows**: Registers `SetConsoleCtrlHandler` with proper callback
- **Other platforms**: Returns a no-op function (does nothing)

**Example:**

```go
shutdownChan := make(chan struct{})
registerHandler := console.SetupConsoleHandler(shutdownChan)

// Re-register after library initialization
registerHandler()

// Wait for shutdown
<-shutdownChan
```

## Why This Package?

### Problem

On Windows, Go's standard `signal.Notify(os.Interrupt)` doesn't work reliably when:

1. Using C libraries like SDL3, OpenGL, GLFW that call `runtime.LockOSThread()`
2. The library overrides Windows console control handlers during initialization

### Solution

This package:

1. Uses Windows `SetConsoleCtrlHandler` API directly via `syscall.NewCallback()`
2. Supports re-registration after library initialization
3. Uses atomic operations to prevent panic from rapid Ctrl+C presses
4. Automatically detects and handles GUI vs console mode

### Comparison

| Method | Works with SDL3 | Works with LockOSThread | Re-registerable | Cross-platform |
|--------|-----------------|-------------------------|-----------------|----------------|
| `signal.Notify()` | ❌ | ❌ | N/A | ✅ |
| `SetConsoleCtrlHandler` | ✅ | ✅ | ❌ | ❌ |
| **This Package** | ✅ | ✅ | ✅ | ✅ |

## Technical Details

### Windows Console Detection

The package intelligently handles console allocation by:

1. Checking if a console window already exists (`GetConsoleWindow()`)
2. If a console exists:
   - Check if parent process is `explorer.exe` (double-click scenario)
   - If yes, free the console (console-mode build was double-clicked)
   - If no, reuse the console (launched from terminal)
3. If no console exists:
   - Check if parent process is `explorer.exe` (double-click scenario)
   - If yes, do nothing (GUI-mode build, no console needed)
   - If no, allocate a console (GUI-mode build launched from terminal)

**Decision Table:**

| Build Mode | Launch Method | Has Console? | Parent Process | Action | Returns |
|------------|---------------|--------------|----------------|--------|---------|
| Console | Terminal | Yes | cmd/powershell | Reuse console | `true` |
| Console | Double-click | Yes | explorer.exe | Free console | `false` |
| GUI | Terminal | No | cmd/powershell | Alloc new console + redirect std | `true` |
| GUI | Double-click | No | explorer.exe | No console | `false` |

**Note:** For GUI-mode builds launched from terminal, the function uses `AllocConsole()` to create a **new independent console window** (instead of attaching to parent with `AttachConsole()`), and then redirects stdout/stderr/stdin so that `fmt.Println`, `log`, etc. work correctly. This prevents input/output confusion between the parent and child processes.

### Handler Re-registration

Some libraries (notably SDL3) override console control handlers during initialization. This package:

1. Returns a `registerHandler` function from `SetupConsoleHandler()`
2. Allows calling this function after library initialization to re-register
3. Can be used with library init callbacks (e.g., `reader.SetOnSDLInitCallback()`)

### Thread Safety

- Uses `atomic.CompareAndSwapInt32()` to ensure shutdown channel is only closed once
- Prevents panic from rapid Ctrl+C presses
- Global state accessible from Windows callback (asynchronously invoked by OS)

## License

Same license as the parent project.
