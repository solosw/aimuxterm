# Terminal Soft Restore Design

## Context

The application currently owns each local or SSH terminal session inside the Wails process. When the app exits, the backend shuts down the terminal manager and closes every live session. After an OS shutdown, the original terminal processes cannot keep running. The goal is therefore a VSCode-style soft restore: reopen the terminal tabs with their previous titles, working directories, connection type, and recent output buffer, then let the user reconnect when they want a fresh live terminal.

## Goals

- Restore terminal tabs after application restart or accidental OS shutdown.
- Preserve terminal title, type, working directory / remote target, active tab, layout mode, and recent output.
- Do not pretend that old processes are still running after shutdown.
- Keep current live terminal keyboard interaction unchanged.
- Deleting/closing a tab intentionally removes it from persisted terminal state.

## Non-goals

- No tmux/screen process persistence in this feature.
- No automatic replay of previously typed commands.
- No guarantee that interactive process state survives OS shutdown.

## Data model

Add a persisted terminal snapshot model in the config package:

```go
type TerminalSnapshot struct {
    ID        string `json:"id"`
    Title     string `json:"title"`
    Type      string `json:"type"` // local or ssh
    CWD       string `json:"cwd"`
    SSHName   string `json:"sshName,omitempty"`
    Output    string `json:"output"`
    Restored  bool   `json:"restored"`
    Active    bool   `json:"active,omitempty"`
    UpdatedAt string `json:"updatedAt"`
}
```

The store will persist snapshots in `terminal-sessions.json` under the existing app config directory.

Output buffer is capped at approximately 200KB per terminal. Older content is trimmed from the front.

## Backend design

### Config store

Add methods:

- `LoadTerminalSnapshots() ([]TerminalSnapshot, error)`
- `SaveTerminalSnapshots([]TerminalSnapshot) error`

### App methods

Expose Wails methods:

- `GetTerminalSnapshots() ([]config.TerminalSnapshot, error)`
- `SaveTerminalSnapshots([]config.TerminalSnapshot) error`
- `ReconnectTerminal(snapshot config.TerminalSnapshot) (string, error)`

`ReconnectTerminal` creates a new real terminal using the snapshot metadata:

- Local: create a local PTY with saved `CWD`.
- SSH: load the matching SSH config by `SSHName`, create SSH terminal.

It returns the new live terminal ID. The frontend replaces the restored tab with the new ID while keeping the visible title.

Backend shutdown should not delete terminal snapshots. Live sessions can still be closed during shutdown because the restored UI is based on snapshots.

## Frontend design

### Terminal store

Extend `TabItem` with:

```ts
type TerminalType = 'local' | 'ssh'
interface TabItem {
  id: string
  title: string
  type: TerminalType
  cwd: string
  sshName?: string
  restored?: boolean
  output?: string
}
```

Add store behavior:

- On app startup, call `GetTerminalSnapshots()` and create restored tabs.
- Existing live terminal output subscriptions append to the tab output buffer.
- Persist snapshots using a throttled save (about once per second) to avoid excessive disk writes.
- Close tab:
  - If restored only: remove from snapshot list.
  - If live: close backend session and remove from snapshot list.
- Track active tab and layout mode in snapshots.

### TerminalView

For live tabs:

- Keep the current xterm.js behavior unchanged.
- User keyboard input still goes directly to `WriteToTerminal`.
- Output events still write into xterm and update the output buffer.

For restored tabs:

- Do not create an xterm live subscription.
- Render a read-only terminal-like output area containing the saved output.
- Show a banner: `上次终端会话已恢复，原进程已结束。`
- Provide a `重新连接` button. Clicking it calls `ReconnectTerminal`, then switches the tab back to live mode.

## Error handling

- If snapshot loading fails, start with no restored terminals and log the error.
- If reconnect fails, keep the restored tab and show the error in the tab UI.
- If an SSH config referenced by a snapshot no longer exists, show `SSH 配置不存在` and keep the restored output.
- If output persistence fails, do not interrupt the live terminal.

## Testing

Manual checks:

1. Open local terminal, run commands, close app, reopen app: tab and recent output are restored.
2. Reconnect restored local tab: a new live terminal starts in the saved CWD.
3. Open SSH terminal, produce output, close app, reopen app: SSH tab and output restore.
4. Reconnect restored SSH tab: new SSH terminal opens using the saved SSH config.
5. Close a restored tab: it does not return on next restart.
6. Output buffer stays capped and does not grow unbounded.
7. Existing normal terminal interaction still works.
