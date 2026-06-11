# Just Warp Go

A [Warp](https://www.warp.dev/)-inspired desktop terminal management app built with **Wails v2 + Go + Vue 3 + TypeScript**.

Manage workspaces, run multi-tab terminals (local PTY & SSH), and track file changes — all with a beautiful GUI. **No Git required.**

## Features

### Workspace Management
- Select local folders or connect to remote servers via SSH
- Automatic `.gitignore`-aware file scanning
- Binary / noise directory skipping
- Recently-opened workspace history

### Terminal
- Multi-tab terminal with local PTY (ConPTY on Windows, Unix PTY on Linux/macOS) and SSH support
- xterm.js-based terminal emulator
- One-click SSH terminals from saved configurations
- Per-tab CWD tracking, sessions persist across tab switches

### File Change Tracking
- Real-time filesystem monitoring via fsnotify (local)
- Periodic auto-polling (remote, 15s interval)
- Snapshot-based change detection (no Git dependency)
- **Git-style object storage** — content-addressed `.warp-snapshots/objects/` with SHA-256 dedup
- **Gzip compression** — all objects compressed, ~60-70% space savings
- Diff view with line-level additions/deletions
- **Accept All** / **Revert All** for bulk operations
- **Accept File** / **Revert File** for single-file operations

### Remote Workspace (SSH + SFTP)
- Connect to remote servers and browse the filesystem
- Server-side snapshot creation via SSH exec (sha256sum, zero network transfer)
- File change tracking via fingerprint comparison (size + mod time)
- 15-second auto-polling for remote changes
- Auto-create `.gitignore` with `.warp-snapshots` entry on remote
- Read remote file diffs on demand
- Snapshots deduplicated and gzip-compressed server-side

### Quality of Life
- Startup commands — auto-run CLI tools on terminal launch
- SSH config management — save and reuse connection profiles
- File tree browsing with click-to-open
- Code preview with syntax highlighting

## Architecture

```
Wails Desktop App
├── Go Backend
│   ├── app.go            — bound methods (workspace, terminal, snapshot, SSH)
│   ├── scanner/          — recursive file scan + .gitignore + binary detection
│   ├── watcher/          — fsnotify wrapper with debounce
│   ├── snapshot/         — content-addressed snapshot engine (SHA-256 + gzip, Git-style objects)
│   ├── terminal/         — PTY (Windows ConPTY / Unix PTY) & SSH session management
│   └── config/           — persistent JSON store for workspaces, SSH, commands
├── Vue 3 Frontend
│   ├── WorkspaceBar      — workspace path + selector + history
│   ├── TerminalPanel     — tab bar + xterm.js instances
│   ├── FileChangesPanel  — change list with accept/revert buttons
│   ├── FileTreePanel     — workspace file browser
│   ├── FilePreviewPanel  — file content with diff view
│   └── DiffView          — side-by-side diff display
```

## Prerequisites

- [Go](https://go.dev/) 1.25+
- [Node.js](https://nodejs.org/) 18+
- [Wails v2](https://wails.io/docs/gettingstarted/installation)

## Getting Started

```bash
# Clone
git clone https://github.com/solosw/just-warp-go.git
cd just-warp-go

# Install frontend dependencies
cd frontend && npm install && cd ..

# Run in dev mode
wails dev

# Build for production
wails build
```

The dev server starts at `http://localhost:34115` with hot reload.

## Tech Stack

| Layer | Choice |
|-------|--------|
| Framework | Wails v2 |
| Backend | Go |
| Frontend | Vue 3 + TypeScript + Vite |
| Terminal | xterm.js + ConPTY (Windows) / Unix PTY (Linux/macOS) |
| State | Pinia |
| File Watch | fsnotify |
| Snapshot | SHA-256 + gzip + content-addressed objects (.warp-snapshots/objects/) |

## License

MIT
