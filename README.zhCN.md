# GameControllerView

实时游戏手柄可视化工具，通过 SDL3 读取手柄输入，使用 WebSocket 推送到浏览器，Canvas 实时渲染。

## 功能特性

- **实时输入可视化**：在浏览器中实时查看手柄输入状态
- **WebSocket 流式传输**：低延迟的增量更新，流畅性能
- **多手柄支持**：预配置 Xbox、PlayStation、Switch Pro 手柄布局
- **通用降级方案**：自动检测未知手柄
- **零配置二进制**：单文件可执行程序，内置前端资源

## 环境要求

- **Go**: 1.25.6 或更高版本
- **SDL3.dll**: 3.2.0 或更高版本
  - 从 [SDL Releases](https://github.com/libsdl-org/SDL/releases) 下载
  - 放在可执行文件同目录或系统 PATH 中

## 快速开始

```bash
# 克隆仓库
git clone https://github.com/soar/GameControllerView.git
cd GameControllerView/backend

# 安装依赖
go mod download

# 运行服务器
go run .

# 浏览器打开 http://localhost:8080
```

## URL 参数

前端支持以下 URL 参数来自定义显示：

| 参数 | 描述 | 示例 |
|------|------|------|
| `p` | 手柄编号（1-based），选择要显示的手柄。默认为 `1`（第一个连接的手柄） | `?p=2` 显示第二个手柄 |
| `simple` | 启用简单模式，透明背景且无 UI 元素。设置为 `1` 启用 | `?simple=1` |
| `alpha` | 手柄主体透明度（0.0 到 1.0）。值越小越透明 | `?alpha=0.5` |

### 使用示例

```bash
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
```

### 多手柄设置

要同时查看多个手柄，打开多个浏览器窗口/标签页，使用不同的 `p` 值：

```bash
# 第一个手柄
http://localhost:8080/?p=1

# 第二个手柄
http://localhost:8080/?p=2

# 第三个手柄
http://localhost:8080/?p=3
```

## 项目结构

```
backend/
├── main.go                             # 入口：组件组装，SDL 主线程事件循环
├── embed.go                            # go:embed 嵌入前端静态文件
├── internal/
│   ├── gamepad/
│   │   ├── state.go                    # GamepadState 数据模型，DeltaChanges，ComputeDelta
│   │   ├── mapping.go                  # 设备映射（原始轴/按键索引 → 语义名称）
│   │   └── reader.go                   # SDL3 Joystick 读取器（事件 + 轮询混合循环）
│   ├── hub/
│   │   ├── hub.go                      # WebSocket 客户端管理（注册/注销/广播）
│   │   ├── broadcast.go                # 状态变更 → JSON 广播（增量 + 定期全量同步）
│   │   └── message.go                  # WSMessage 类型定义（full/delta/event）
│   └── server/
│       ├── server.go                   # HTTP 服务器，路由（/ 静态文件，/ws WebSocket）
│       └── handler.go                  # WebSocket 升级处理
└── frontend/                           # 前端静态文件（通过 go:embed 嵌入）
    ├── index.html
    ├── styles.css
    ├── app.js                          # WebSocket 客户端 + 状态管理 + Canvas 渲染
    └── configs/                        # 手柄布局 JSON 配置
        ├── xbox.json
        ├── playstation.json
        ├── playstation5.json
        └── switch_pro.json
```

## 架构设计

### 线程模型

SDL3 必须在 OS 主线程运行。`main.go` 阻塞主线程执行 `reader.Run(ctx)` 的 SDL 事件循环，Hub 和 HTTP 服务器在独立的 goroutine 中运行。

```
主线程 (runtime.LockOSThread)
├── SDL Init → PollEvent + Joystick 轮询 (~60Hz)
│
goroutine: Hub.Run()        ← 管理 WebSocket 客户端连接
goroutine: Broadcaster.Run() ← 监听 Reader.Changes() channel，广播给 Hub
goroutine: HTTP Server       ← 静态文件 + WebSocket 端点
```

### 数据流

`Reader` (SDL 轮询) → `chan GamepadState` → `Broadcaster` → `Hub.Broadcast()` → 所有 WebSocket 客户端

### Joystick 低级 API（非 Gamepad）

有意使用 SDL3 Joystick 低级 API 而非 Gamepad 高级 API，以避免与其他应用或游戏冲突。Joystick API 直接读取 HID 设备数据，需要手动维护轴/按键索引到语义名称的映射（见 `mapping.go`）。

### WebSocket 消息协议

- `full`: 完整状态快照（新客户端连接时、每 5 秒、每 100 条增量消息后发送）
- `delta`: 仅包含变更字段（常规更新）
- 所有消息包含 `seq`（递增序列号）和 `timestamp`（毫秒时间戳）

### 设备映射系统

`mapping.go` 通过 VID/PID 匹配已知设备（Xbox、PlayStation、Switch Pro），未知设备使用通用降级方案。映射定义了：
- 轴索引 → 摇杆/扳机对应关系
- 按键索引 → 按钮名称对应关系
- 轴值归一化范围（摇杆 -1.0~1.0，扳机 0.0~1.0）
- Y 轴是否需要反转

### 前端配置系统

`frontend/configs/*.json` 定义了每个手柄的 Canvas 绘制布局（按钮坐标、尺寸、半径）。前端根据后端报告的 `controllerType` 自动加载对应的配置。

## 依赖

| 包 | 用途 |
|---|---|
| `github.com/jupiterrider/purego-sdl3` | 无 CGo 的 SDL3 Go 绑定 |
| `github.com/gorilla/websocket` | WebSocket 服务端 |
| `github.com/ebitengine/purego` | 间接依赖，purego-sdl3 的 FFI 基础 |

## 常见修改

### 添加新手柄支持

1. `mapping.go`: 在 `knownDevices` map 中添加 VID/PID → DeviceMapping
2. 如果按键布局与现有映射不同，创建新的 `DeviceMapping` 变量
3. `frontend/configs/`: 添加新的布局 JSON 文件
4. `frontend/app.js`: 在 `configMap` 中添加映射名称 → 配置文件名

### 修改 Canvas 渲染

所有绘制逻辑在 `frontend/app.js` 的 `drawController()` 函数及其子函数中。按钮位置和尺寸由 `configs/*.json` 控制，颜色由 `COLORS` 常量控制。

### 修改轮询频率

`reader.go` 中的 `pollDelayNS` 常量（当前 16ms ≈ 60Hz）。

### 修改死区

`reader.go` 中的 `deadzone` 常量（当前 0.05），以及 `state.go` 中的 `analogThreshold` 常量（当前 0.01，用于增量比较）。

## 许可证

MIT License

## 贡献

欢迎贡献！请随时提交 Pull Request。
