# InputView

实时游戏手柄、键盘和鼠标输入可视化工具。Go 后端读取输入，通过 WebSocket 推送到浏览器，Canvas 实时渲染。

## 功能特性

- **手柄可视化** — 支持 Xbox、PlayStation、Switch Pro 及 550+ 设备（VID/PID 映射）
- **键盘和鼠标可视化** — 通过 Windows Raw Input API 全局捕获；InputView 在后台或 OBS 浏览器源中时同样有效
- **多手柄支持** — 多个浏览器标签页，每个显示不同手柄
- **Input Overlay 支持** — 兼容 [Input Overlay](https://github.com/univrsal/input-overlay) 的 `.json` + `.png` 纹理图集预设，支持全部 10 种元素类型
- **纯键鼠 Overlay** — 不含手柄元素的 Overlay 无需连接手柄即可直接渲染
- **简洁 / OBS 模式** — 透明背景，无 UI 元素（`?simple=1`）
- **系统托盘** — Windows GUI 模式下提供托盘图标和退出菜单
- **零配置二进制** — 单文件可执行程序，内置前端资源

## 环境要求

- **SDL3.dll**（≥ 3.2.0）— 放在可执行文件同目录或系统 PATH 中
  - 从 https://github.com/libsdl-org/SDL/releases 下载
- 键盘/鼠标捕获需要 Windows；仅手柄模式支持 Linux/macOS

## 快速开始

```bash
# 开发/控制台模式运行（终端可见日志）
go run ./cmd/inputview

# Release 构建 — 无控制台窗口，启用系统托盘（Windows）
./build.ps1     # Windows
./build.sh      # Linux/macOS

# 打开浏览器
http://localhost:8080
```

## URL 参数

| 参数 | 描述 | 默认值 | 示例 |
|------|------|--------|------|
| `p` | 手柄编号（1-based） | `1` | `?p=2` |
| `simple` | 透明背景，无 UI | 关闭 | `?simple=1` |
| `alpha` | 手柄主体不透明度（0.0–1.0） | `1.0` | `?alpha=0.5` |
| `overlay` | Input Overlay 预设名称 | — | `?overlay=dualsense` |
| `mouse_sens` | 鼠标移动灵敏度除数（越小越灵敏） | `500` | `?mouse_sens=300` |

### 使用示例

```
# 默认 — 第一个连接的手柄
http://localhost:8080/

# 第二个连接的手柄
http://localhost:8080/?p=2

# 简单模式（透明背景，无 UI — 适用于 OBS 浏览器源）
http://localhost:8080/?simple=1

# 半透明手柄
http://localhost:8080/?alpha=0.5

# Input Overlay 预设
http://localhost:8080/?overlay=dualsense

# 鼠标 Overlay，提高灵敏度
http://localhost:8080/?overlay=mouse&mouse_sens=300&simple=1

# 组合参数
http://localhost:8080/?overlay=dualsense&p=2&simple=1
```

### 多手柄设置

打开多个浏览器标签页，使用不同的 `p` 值：

```
http://localhost:8080/?p=1
http://localhost:8080/?p=2
```

## Input Overlay 预设

将预设目录放在可执行文件旁边：

```
InputView.exe
overlays/
  dualsense/
    dualsense.json
    dualsense.png
  mouse/
    mouse.json
    mouse.png
```

访问 `http://localhost:8080/?overlay=dualsense` 即可使用预设。

仅包含键盘/鼠标元素类型（类型 1、3、4、9）的预设会自动隐藏手柄状态栏，无需连接手柄即可渲染。

[Input Overlay 项目](https://github.com/univrsal/input-overlay/tree/master/presets) 的预设文件采用 **GPL-2.0** 协议授权，**禁止**随 InputView 打包分发。详见 [docs/third-party-licenses.md](docs/third-party-licenses.md)。

完整格式规范请参阅 [docs/input-overlay-format.md](docs/input-overlay-format.md)。

## GPV 皮肤转换器

`cmd/gpvskin2overlay` 可将 [GamepadViewer](https://gamepadviewer.com/) CSS 皮肤转换为 Input Overlay 格式。

```bash
go build -o gpvskin2overlay.exe ./cmd/gpvskin2overlay
gpvskin2overlay -skin xbox -out overlays/gpv-xbox
# 然后访问：http://localhost:8080/?overlay=gpv-xbox
```

完整使用说明请参阅 [docs/gpvskin2overlay.md](docs/gpvskin2overlay.md)。

## 项目结构

```
cmd/
  inputview/          # 主程序入口
  gpvskin2overlay/    # GPV 皮肤转换器 CLI
internal/
  input/              # KeyMouseState 数据模型，扫描码映射（Raw Input → uiohook）
  rawinput/           # Windows Raw Input API 读取器（键盘 + 鼠标，全局捕获）
  gamepad/            # SDL3 手柄读取器，VID/PID 设备映射表（550+ 条目）
  hub/                # WebSocket Hub、广播器、客户端管理
  server/             # HTTP 服务器，WebSocket 升级
  tray/               # Windows 系统托盘集成
  gpvskin/            # GPV 皮肤 → Input Overlay 转换流水线
  web/frontend/       # HTML/CSS/JS 前端 + 手柄布局配置
overlays/             # 外置 Input Overlay 预设（不嵌入二进制）
docs/                 # 格式规范和使用指南
```

## 架构设计

### 线程模型

```
goroutine: gamepad.Reader.Run(ctx)    ← SDL3 手柄轮询 (~60Hz)
                                           ↓ chan GamepadState
goroutine: rawinput.Reader.Run(ctx)   ← Windows Raw Input（键盘 + 鼠标，全局捕获）
                                           ↓ chan KeyMouseState (~60Hz)
goroutine: Broadcaster.Run()          ← 计算增量，广播给 WebSocket 客户端
goroutine: Hub.Run()                  ← WebSocket 客户端注册/注销
goroutine: HTTP Server                ← 静态文件 + /ws WebSocket 端点
```

### WebSocket 协议

**服务端 → 客户端：**

| 类型 | 发送时机 |
|------|---------|
| `full` | 连接时、每 5 秒、每 100 条增量 |
| `delta` | 手柄状态变更时 |
| `player_selected` | 确认 `select_player` 请求 |
| `km_full` | 收到 `subscribe_km` 时（当前键鼠快照） |
| `km_delta` | 键盘/鼠标状态变更时 |

**客户端 → 服务端：**

| 类型 | 用途 |
|------|------|
| `select_player` | 切换到指定手柄 |
| `subscribe_km` | 订阅键鼠事件（Overlay 含键鼠元素时自动发送） |

## 依赖

| 包 | 用途 |
|----|------|
| `github.com/jupiterrider/purego-sdl3` | 无 CGo 的 SDL3 Go 绑定 |
| `github.com/lxzan/gws` | WebSocket 服务端 |
| `github.com/ebitengine/purego` | FFI 基础（间接依赖） |
| `fyne.io/systray` | Windows 系统托盘 |

## 常见修改

### 添加新手柄支持

1. `internal/gamepad/mapping.go` — 在 `knownDevices` 中添加 VID/PID → `DeviceMapping`
2. `internal/web/frontend/configs/` — 添加布局 JSON
3. `internal/web/frontend/app.js` — 在 `configMap` 中添加映射

### 修改轮询频率

`internal/gamepad/reader.go` 中的 `pollDelayNS`（默认 16ms ≈ 60Hz）。

### 修改死区

`internal/gamepad/reader.go` 中的 `deadzone`（默认 0.05）；`internal/gamepad/state.go` 中的 `analogThreshold`（默认 0.01）。

## 许可证

MIT — 详见 [LICENSE](LICENSE)

> **打包注意**：分发 InputView 时，**严禁**将 `overlays/` 目录下的预设文件一同打包，这些文件采用 GPL-2.0 协议。详见 [docs/third-party-licenses.md](docs/third-party-licenses.md)。
