# GameControllerView

Go 后端通过 SDL3 读取游戏手柄输入，WebSocket 推送到前端，Canvas 实时渲染手柄可视化。

## 构建与运行

```bash
cd backend && go run .
# 浏览器打开 http://localhost:8080
```

运行时需要 **SDL3.dll** (>= 3.2.0) 在可执行文件同目录或系统 PATH 中。从 https://github.com/libsdl-org/SDL/releases 下载。

## 项目结构

```
backend/
├── main.go                             # 入口：组装组件，SDL 主线程事件循环
├── embed.go                            # go:embed 嵌入 frontend/ 静态文件
├── internal/
│   ├── gamepad/
│   │   ├── state.go                    # GamepadState 数据模型，DeltaChanges，ComputeDelta
│   │   ├── mapping.go                  # 设备映射表（原始轴/按键索引 → 语义名称）
│   │   └── reader.go                   # SDL3 Joystick 读取器（事件+轮询混合循环）
│   ├── hub/
│   │   ├── hub.go                      # WebSocket 客户端管理（注册/注销/广播）
│   │   ├── broadcast.go                # 状态变更 → JSON 广播（delta + 定期 full 同步）
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
        ├── switch_pro.json
        └── generic.json
```

## 架构要点

### 线程模型

SDL3 必须在 OS 主线程运行。`main.go` 中 `reader.Run(ctx)` 阻塞主线程执行 SDL 事件循环，Hub 和 HTTP 服务器在独立 goroutine 中运行。

```
主线程 (runtime.LockOSThread)
├── SDL Init → PollEvent + 轮询 Joystick (~60Hz)
│
goroutine: Hub.Run()        ← 管理 WebSocket 客户端连接
goroutine: Broadcaster.Run() ← 监听 Reader.Changes() channel，广播给 Hub
goroutine: HTTP Server       ← 静态文件 + WebSocket 端点
```

### 数据流

`Reader` (SDL 轮询) → `chan GamepadState` → `Broadcaster` → `Hub.Broadcast()` → 所有 WebSocket 客户端

### 使用 Joystick 低级 API（非 Gamepad）

刻意使用 SDL3 Joystick 低级 API 而非 Gamepad 高级 API，避免与其他应用或游戏冲突。Joystick API 直接读取 HID 设备数据，需要自行维护轴索引/按键索引到语义名称的映射表（见 `mapping.go`）。

### WebSocket 消息协议

- `full`: 完整状态快照（新客户端连接时、每 5 秒、每 100 条 delta 后发送）
- `delta`: 仅包含变更字段（常规更新）
- 所有消息包含 `seq`（递增序列号）和 `timestamp`（毫秒时间戳）

### 设备映射系统

`mapping.go` 中通过 VID/PID 匹配已知设备（Xbox、PlayStation、Switch Pro），未知设备使用 generic fallback。映射定义了：
- 轴索引 → 摇杆/扳机的对应关系
- 按键索引 → 按钮名称的对应关系
- 轴值归一化范围（摇杆 -1.0~1.0，扳机 0.0~1.0）
- 是否需要反转 Y 轴

### 前端配置系统

`frontend/configs/*.json` 定义每种手柄的 Canvas 绘制布局（按键坐标、尺寸、半径）。前端根据后端报告的 `controllerType` 自动加载对应配置。

## 常见修改指南

### 添加新手柄支持

1. `mapping.go`: 在 `knownDevices` map 中添加 VID/PID → DeviceMapping
2. 如果按键布局不同于现有映射，创建新的 `DeviceMapping` 变量
3. `frontend/configs/`: 添加新的布局 JSON 文件
4. `frontend/app.js`: 在 `configMap` 中添加映射名称 → 配置文件名

### 修改 Canvas 渲染

所有绘制逻辑在 `frontend/app.js` 的 `drawController()` 及其子函数中。按键位置和尺寸由 `configs/*.json` 控制，颜色由 `COLORS` 常量控制。

### 修改轮询频率

`reader.go` 中的 `pollDelayNS` 常量（当前 16ms ≈ 60Hz）。

### 修改死区

`reader.go` 中的 `deadzone` 常量（当前 0.05），`state.go` 中的 `analogThreshold` 常量（当前 0.01，用于 delta 比较）。

## 依赖

| 包 | 用途 |
|---|---|
| `github.com/jupiterrider/purego-sdl3` | 无 CGo 的 SDL3 Go 绑定 |
| `github.com/gorilla/websocket` | WebSocket 服务端 |
| `github.com/ebitengine/purego` | 间接依赖，purego-sdl3 的 FFI 基础 |
