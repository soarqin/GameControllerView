# GameControllerView

实时游戏手柄可视化工具，通过 SDL3 读取手柄输入，使用 WebSocket 推送到浏览器，Canvas 实时渲染。

## 功能特性

- **实时输入可视化**：在浏览器中实时查看手柄输入状态
- **WebSocket 流式传输**：低延迟的增量更新，流畅性能
- **多手柄支持**：预配置 Xbox、PlayStation、Switch Pro 手柄布局
- **通用降级方案**：自动检测未知手柄
- **零配置二进制**：单文件可执行程序，内置前端资源
- **Input Overlay 支持**：通过 `?overlay=<名称>` 参数启用基于纹理图集的渲染，兼容 [Input Overlay](https://github.com/univrsal/input-overlay) 预设格式

## 环境要求

- **Go**: 1.25.6 或更高版本
- **SDL3.dll**: 3.2.0 或更高版本
  - 从 [SDL Releases](https://github.com/libsdl-org/SDL/releases) 下载
  - 放在可执行文件同目录或系统 PATH 中

## 快速开始

```bash
# 克隆仓库
git clone https://github.com/soarqin/GameControllerView.git
cd GameControllerView

# 安装依赖
go mod download

# 运行服务器
go run ./cmd/gamecontrollerview

# 浏览器打开 http://localhost:8080
```

## URL 参数

前端支持以下 URL 参数来自定义显示：

| 参数 | 描述 | 示例 |
|------|------|------|
| `p` | 手柄编号（1-based），选择要显示的手柄。默认为 `1`（第一个连接的手柄） | `?p=2` 显示第二个手柄 |
| `simple` | 启用简单模式，透明背景且无 UI 元素。设置为 `1` 启用 | `?simple=1` |
| `alpha` | 手柄主体透明度（0.0 到 1.0）。值越小越透明 | `?alpha=0.5` |
| `overlay` | Input Overlay 预设名称。启用纹理图集渲染器，替代内置几何渲染器 | `?overlay=dualsense` |

### 使用示例

```
# 显示第一个手柄（默认）
http://localhost:8080/

# 显示第二个连接的手柄
http://localhost:8080/?p=2

# 简单模式（透明背景，无 UI）
http://localhost:8080/?simple=1

# 半透明手柄，50% 不透明度
http://localhost:8080/?alpha=0.5

# 组合多个参数
http://localhost:8080/?p=2&simple=1&alpha=0.3

# 使用 Input Overlay 预设（需将预设文件放到可执行文件旁的 overlays/dualsense/ 目录）
http://localhost:8080/?overlay=dualsense

# Input Overlay 预设 + 第二个手柄 + 简单模式
http://localhost:8080/?overlay=dualsense&p=2&simple=1
```

### 多手柄设置

要同时查看多个手柄，打开多个浏览器窗口/标签页，使用不同的 `p` 值：

```
http://localhost:8080/?p=1   # 第一个手柄
http://localhost:8080/?p=2   # 第二个手柄
http://localhost:8080/?p=3   # 第三个手柄
```

## 项目结构

```
GameControllerView/
├── go.mod                              # module github.com/soar/gamecontrollerview
├── go.sum
├── build.bat                           # Windows GUI 模式构建脚本
├── docs/
│   ├── input-overlay-format.md        # Input Overlay 配置格式规范
│   └── third-party-licenses.md        # 第三方许可证说明
├── cmd/
│   └── gamecontrollerview/
│       ├── main.go                     # 入口：组件组装，信号处理
│       ├── winres/                     # Windows 资源定义（图标、清单）
│       └── rsrc_windows_amd64.syso     # 编译后的 Windows 资源对象
└── internal/
    ├── console/                        # 跨平台控制台检测 & Windows Ctrl+C 处理
    ├── gamepad/
    │   ├── state.go                    # GamepadState 数据模型，DeltaChanges，ComputeDelta
    │   ├── mapping.go                  # 设备映射（原始轴/按键索引 → 语义名称）
    │   ├── mapping_table.go            # VID/PID 映射表（550+ 条目）
    │   └── reader.go                   # SDL3 Joystick 读取器（事件 + 轮询混合循环）
    ├── hub/
    │   ├── hub.go                      # WebSocket 客户端管理（注册/注销/广播）
    │   ├── client.go                   # WebSocket 客户端（读写泵）
    │   ├── broadcast.go                # 状态变更 → JSON 广播（增量 + 定期全量同步）
    │   └── message.go                  # WSMessage 类型定义（full/delta/player_selected）
    ├── server/
    │   ├── server.go                   # HTTP 服务器，路由（/ 静态文件，/ws WebSocket）
    │   └── handler.go                  # WebSocket 升级处理
    ├── tray/                           # Windows 系统托盘集成
    └── web/
        ├── embed.go                    # go:embed 嵌入前端静态文件，导出 FrontendFS()
        └── frontend/                   # 前端静态文件（构建时嵌入）
            ├── index.html
            ├── styles.css
            ├── app.js                  # WebSocket 客户端 + 状态管理 + Canvas 渲染
            └── configs/                # 手柄布局 JSON 配置
                ├── xbox.json
                ├── playstation.json
                ├── playstation5.json
                └── switch_pro.json
```

### Input Overlay 预设（外置，不随程序分发）

Input Overlay 预设文件（`.json` + `.png` 纹理图集）在运行时放置于可执行文件旁的 `overlays/` 目录，**不**嵌入二进制文件，**不**随 GameControllerView 发布包分发。

```
overlays/              ← 放置在 GameControllerView.exe 旁边
├── dualsense/
│   ├── dualsense.json
│   └── dualsense.png
└── xbox-one-controller/
    ├── xbox-one-controller.json
    └── xbox-one-controller.png
```

可从 [Input Overlay 项目](https://github.com/univrsal/input-overlay/tree/master/presets) 获取预设文件。这些文件采用 **GPL-2.0** 协议授权，**禁止**随 GameControllerView 打包分发。详见 [docs/third-party-licenses.md](docs/third-party-licenses.md)。

### 转换 GamepadViewer 皮肤

内置的 `gpvskin2overlay` 工具可将 [GamepadViewer](https://gamepadviewer.com/) CSS 皮肤转换为 Input Overlay 格式。完整的编译和使用说明请参阅 **[docs/gpvskin2overlay.md](docs/gpvskin2overlay.md)**。

```bash
go build -o gpvskin2overlay.exe ./cmd/gpvskin2overlay
gpvskin2overlay -skin xbox -out overlays/gpv-xbox
# 然后在浏览器访问：http://localhost:8080/?overlay=gpv-xbox
```

## 架构设计

### 线程模型

SDL3 必须在 OS 主线程运行。`reader.Run(ctx)` 在调用 `runtime.LockOSThread` 的 goroutine 中执行，Hub 和 HTTP 服务器在独立的 goroutine 中运行。

```
goroutine: Reader.Run(ctx)     ← SDL 初始化 → 回调 → PollEvent + Joystick 轮询 (~60Hz)
                                   ↓
                            chan GamepadState
                                   ↓
goroutine: Broadcaster.Run()   ← 监听状态变更，广播给匹配的客户端
goroutine: Hub.Run()           ← 管理 WebSocket 客户端连接
goroutine: HTTP Server         ← 静态文件 + WebSocket 端点
```

### 数据流

`Reader`（SDL 轮询）→ `chan GamepadState` → `Broadcaster` → `Hub.BroadcastToPlayer()` → WebSocket 客户端

### Joystick 低级 API（非 Gamepad）

有意使用 SDL3 Joystick 低级 API 而非 Gamepad 高级 API，以避免与其他应用或游戏冲突。Joystick API 直接读取 HID 设备数据，需要手动维护轴/按键索引到语义名称的映射（见 `mapping.go`）。

### WebSocket 消息协议

**服务端 → 客户端：**
- `full`：完整状态快照（新客户端连接时、每 5 秒、每 100 条增量消息后发送）
- `delta`：仅包含变更字段（常规更新）
- `player_selected`：确认手柄切换
- 所有消息包含 `seq`（递增序列号）和 `timestamp`（毫秒时间戳）

**客户端 → 服务端：**
- `select_player`：选择要监听的手柄编号

### 设备映射系统

`mapping.go` 通过 VID/PID 匹配已知设备（Xbox、PlayStation、Switch Pro），未知设备使用通用降级方案。映射定义了：
- 轴索引 → 摇杆/扳机对应关系
- 按键索引 → 按钮名称对应关系
- 轴值归一化范围（摇杆 -1.0~1.0，扳机 0.0~1.0）
- Y 轴是否需要反转

### 前端配置系统

`internal/web/frontend/configs/*.json` 定义了每个手柄的 Canvas 绘制布局（按钮坐标、尺寸、半径）。前端根据后端报告的 `controllerType` 自动加载对应的配置。

## 依赖

| 包 | 用途 |
|---|---|
| `github.com/jupiterrider/purego-sdl3` | 无 CGo 的 SDL3 Go 绑定 |
| `github.com/gorilla/websocket` | WebSocket 服务端 |
| `github.com/ebitengine/purego` | 间接依赖，purego-sdl3 的 FFI 基础 |
| `fyne.io/systray` | Windows 系统托盘集成 |

## 常见修改

### 添加新手柄支持

1. `internal/gamepad/mapping.go`：在 `knownDevices` map 中添加 VID/PID → DeviceMapping
2. 如果按键布局与现有映射不同，创建新的 `DeviceMapping` 变量
3. `internal/web/frontend/configs/`：添加新的布局 JSON 文件
4. `internal/web/frontend/app.js`：在 `configMap` 中添加映射名称 → 配置文件名

### 修改 Canvas 渲染

所有绘制逻辑在 `internal/web/frontend/app.js` 的 `drawController()` 函数及其子函数中。按钮位置和尺寸由 `configs/*.json` 控制，颜色由 `COLORS` 常量控制。

### 修改轮询频率

`internal/gamepad/reader.go` 中的 `pollDelayNS` 常量（当前 16ms ≈ 60Hz）。

### 修改死区

`internal/gamepad/reader.go` 中的 `deadzone` 常量（当前 0.05），以及 `internal/gamepad/state.go` 中的 `analogThreshold` 常量（当前 0.01，用于增量比较）。

## Input Overlay 格式

完整配置格式规范（所有元素类型、精灵布局约定、自定义预设制作说明）请参阅 [docs/input-overlay-format.md](docs/input-overlay-format.md)。

## GPV 皮肤转换器

`gpvskin2overlay` 工具的编译和使用说明请参阅 [docs/gpvskin2overlay.md](docs/gpvskin2overlay.md)，该工具可将 GamepadViewer CSS 皮肤转换为 Input Overlay 格式。

## 许可证

MIT License — 详见 [LICENSE](LICENSE)

### 第三方许可证

Input Overlay 预设文件（`.json` / `.png`）采用 **GPL-2.0** 协议授权，**不**包含在本仓库或 GameControllerView 发布包中。详见 [docs/third-party-licenses.md](docs/third-party-licenses.md)。

> **打包注意事项**：分发 GameControllerView 时，**严禁**将 `overlays/` 目录下的预设文件一同打包。将 GPL-2.0 文件与 MIT 协议软件一同分发而不遵守 GPL 要求，构成协议违规。

## 贡献

欢迎贡献！请随时提交 Pull Request。
