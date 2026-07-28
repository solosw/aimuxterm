# aimuxterm 功能设计文档

## 1. 概述

类似 Warp 的桌面终端管理应用。技术栈：Go (Wails) + Vue 3 + TypeScript。

用户在应用内管理工作区（文件夹），创建多个终端 Tab 执行 CLI 工具（claude、codex 等），应用实时追踪文件变更，支持一键接受/回退所有变更，也可对单个文件操作。不依赖 Git。

## 2. 核心功能

### 2.1 工作区管理

| 功能 | 描述 |
|------|------|
| 选择/切换工作区 | 选择本地文件夹作为工作区根目录 |
| 初始扫描 | 打开工作区时递归扫描所有文件 |
| .gitignore 过滤 | 存在则按规则忽略文件 |
| 二进制跳过 | 跳过非文本文件（按扩展名 + MIME 检测） |
| 初始快照 | 扫描完成后对所有文本文件生成 oldContent |

### 2.2 终端管理

| 功能 | 描述 |
|------|------|
| 多 Tab 终端 | Tab 式管理多个终端会话，每个 Tab 一个 PTY |
| 新建/关闭 Tab | 动态增删终端标签页 |
| 命令执行 | 标准终端交互，支持任意 CLI 工具 |
| 工作目录 | 每个终端默认 CWD 为当前工作区路径 |
| 会话保持 | Tab 切换保持进程运行 |

### 2.3 文件变更追踪

| 功能 | 描述 |
|------|------|
| 实时监听 | 使用 fsnotify 监听工作区文件系统事件 |
| 变更检测 | 与快照对比识别新建/修改/删除 |
| 变更列表 | 前端实时展示变更文件列表及差异内容 |
| 一键全部接受 | 对所有变更文件重新快照（更新 oldContent） |
| 一键全部回退 | 对所有变更文件恢复为 oldContent |
| 单个文件接受 | 对指定文件重新快照 |
| 单个文件回退 | 对指定文件恢复为 oldContent |

### 2.4 快照引擎

| 功能 | 描述 |
|------|------|
| 存储位置 | 工作区根目录下 `.warp-snapshots/` 隐藏目录 |
| 快照格式 | 文件路径 → 内容 SHA256 + 完整内容副本 |
| 快照操作 | Create（初始扫描）/ Update（接受变更）/ Restore（回退变更） |
| 生命周期 | 快照目录随工作区存在，切换工作区时清理 |

## 3. 架构设计

```
┌─────────────────────────────────────────────────┐
│                  Vue 3 Frontend                  │
│  ┌──────────┬──────────────┬──────────────────┐ │
│  │Workspace │   Terminal   │  File Changes     │ │
│  │Selector  │   Panel      │  Panel            │ │
│  │          │  (xterm.js)  │  (diff view)      │ │
│  └──────────┴──────────────┴──────────────────┘ │
│         │           │              │             │
│         └───────────┼──────────────┘             │
│                     │ @wailsapp/runtime          │
├─────────────────────┼───────────────────────────┤
│                     │   Go Backend               │
│  ┌──────────────────┼──────────────────────┐    │
│  │  App (Wails Bindings)                    │    │
│  │  ├── WorkspaceAPI  (SelectFolder, Scan)  │    │
│  │  ├── TerminalAPI   (Create, Write, Kill) │    │
│  │  ├── SnapshotAPI   (Accept, Revert, Diff)│    │
│  │  └── WatcherAPI    (Events → Frontend)   │    │
│  └──────────────────────────────────────────┘    │
│  ┌──────────────────────────────────────────┐    │
│  │  Services                                  │    │
│  │  ├── scanner/   (文件扫描 + .gitignore)    │    │
│  │  ├── watcher/   (fsnotify 文件监听)        │    │
│  │  ├── snapshot/  (快照存取 + 恢复)          │    │
│  │  └── terminal/  (PTY 会话管理)             │    │
│  └──────────────────────────────────────────┘    │
└─────────────────────────────────────────────────┘
```

## 4. 数据流

### 4.1 工作区初始化流程

```
用户选择文件夹 → 扫描文件（过滤.gitignore+二进制）
  → 生成文件列表 → 创建快照(.warp-snapshots/)
  → 启动 fsnotify 监听 → 前端显示工作区状态
```

### 4.2 文件变更追踪流程

```
fsnotify 事件 → 防抖处理(100ms) → 对比快照
  → 识别变更类型(Create/Modify/Delete)
  → Wails Event 推送到前端 → 更新变更列表UI
```

### 4.3 接受/回退流程

```
接受所有：遍历变更列表 → 每个文件更新快照 → 清空变更列表
回退所有：遍历变更列表 → 每个文件从快照恢复 → 清空变更列表
接受单个：指定文件 → 更新快照 → 从变更列表移除
回退单个：指定文件 → 从快照恢复 → 从变更列表移除
```

## 5. Go 后端模块

```
backend/
├── main.go              # Wails 入口
├── app.go               # App struct + 方法绑定
├── scanner/
│   └── scanner.go       # 递归扫描 + gitignore 解析 + 二进制检测
├── watcher/
│   └── watcher.go       # fsnotify 封装 + 防抖 + 事件队列
├── snapshot/
│   ├── engine.go        # 快照创建/更新/恢复/对比
│   └── store.go         # 文件系统存储 (.warp-snapshots/)
├── terminal/
│   ├── manager.go       # PTY 会话集合管理
│   └── session.go       # 单个 PTY 会话
└── wails.json           # Wails 项目配置
```

## 6. 前端组件

```
frontend/src/
├── main.ts
├── App.vue                    # 根布局（三栏结构）
├── components/
│   ├── WorkspaceBar.vue       # 顶部：工作区路径 + 选择按钮
│   ├── TerminalPanel.vue      # 主区域：Tab 栏 + xterm.js 实例
│   ├── TerminalTab.vue        # 单个终端 Tab
│   ├── FileChangesPanel.vue   # 侧边栏/底部：变更文件列表
│   ├── FileDiffView.vue       # 单个文件 diff 展示
│   └── Toolbar.vue            # 接受全部/回退全部按钮
├── stores/
│   ├── workspace.ts           # 工作区状态 (Pinia)
│   ├── terminal.ts            # 终端会话状态
│   └── fileChanges.ts         # 文件变更状态
└── types/
    └── index.ts               # 类型定义
```

## 7. UI 布局

```
┌─────────────────────────────────────────────────────┐
│  📁 /home/user/projects/my-app          [选择...]   │  ← WorkspaceBar
├──────────────────────────────────────────┬──────────┤
│  [Tab 1] [Tab 2] [Tab 3]        [+]     │ 变更文件  │
│  ┌──────────────────────────────────────┐ │ ┌──────┐ │
│  │ $ claude "refactor this module"      │ │ │M app │ │
│  │ Thinking...                          │ │ │M src │ │
│  │                                      │ │ │+ new │ │
│  │                                      │ │ └──────┘ │
│  │                                      │ │ [接受全部] │
│  │                                      │ │ [回退全部] │
│  └──────────────────────────────────────┘ │          │
│  TerminalPanel (xterm.js)                 │ Changes  │
├──────────────────────────────────────────┴──────────┤
│  Status: 3 files changed | Terminal: running       │  ← StatusBar
└─────────────────────────────────────────────────────┘
```

## 8. Go ↔ 前端绑定

```go
// Wails 暴露的方法（前端通过 runtime 调用）
type App struct { ... }

// 工作区
func (a *App) SelectWorkspace() (path string, err error)    // 打开文件夹选择对话框
func (a *App) GetWorkspaceInfo() (info WorkspaceInfo, err error)

// 终端
func (a *App) CreateTerminal() (tabId string, err error)
func (a *App) WriteToTerminal(tabId string, data string) error
func (a *App) ResizeTerminal(tabId string, cols int, rows int) error
func (a *App) CloseTerminal(tabId string) error

// 文件变更
func (a *App) GetChangedFiles() ([]FileChange, error)
func (a *App) AcceptAll() error
func (a *App) RevertAll() error
func (a *App) AcceptFile(path string) error
func (a *App) RevertFile(path string) error
func (a *App) GetFileDiff(path string) (diff string, error)
```

## 9. 技术选型

| 层面 | 选择 | 理由 |
|------|------|------|
| 框架 | Wails v2 | 成熟稳定，Go+Vue 原生集成 |
| 前端 | Vue 3 + TypeScript | 组合式API，类型安全 |
| 终端 | xterm.js + node-pty | 成熟终端模拟，Wails 社区验证 |
| 构建 | Vite | 快速HMR，Wails 默认集成 |
| 文件监听 | fsnotify (Go) | 跨平台，Go 生态标准库 |
| 状态管理 | Pinia | Vue 3 官方推荐 |
| 快照存储 | 文件系统 (.warp-snapshots/) | 简单可靠，零依赖 |
| .gitignore | 自解析或 go-gitignore | 复用现有规则 |

## 10. 边界与约束

- **不依赖 Git**：完全基于文件系统快照追踪变更
- **仅文本文件**：二进制文件不纳入追踪（按扩展名 + MIME 前512字节检测）
- **仅当前工作区**：不追踪工作区外路径
- **平台**：优先支持 Windows，后续扩展到 macOS/Linux
- **快照生命周期**：切换工作区时清理旧快照，新建快照目录
