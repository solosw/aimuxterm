package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/ssh"

	"just-warp-go/config"
	"just-warp-go/scanner"
	"just-warp-go/snapshot"
	"just-warp-go/terminal"
	"just-warp-go/watcher"
)

// remoteFileEntry holds file metadata for remote workspaces.
// Used for lightweight change detection without downloading file content.
type remoteFileEntry struct {
	path    string
	size    int64
	modTime time.Time
}

func (e remoteFileEntry) fingerprint() string {
	return fmt.Sprintf("%d|%d", e.size, e.modTime.Unix())
}

func entriesToPaths(entries []remoteFileEntry) []string {
	paths := make([]string, len(entries))
	for i, e := range entries {
		paths[i] = e.path
	}
	return paths
}

func entriesToFingerprints(entries []remoteFileEntry) map[string]string {
	m := make(map[string]string, len(entries))
	for _, e := range entries {
		m[e.path] = e.fingerprint()
	}
	return m
}

// Remote file filters — mirrors scanner/scanner.go logic.
var remoteSkipDirs = map[string]bool{
	".git": true, "node_modules": true, ".warp-snapshots": true,
	"dist": true, "build": true, ".next": true, "__pycache__": true,
	"target": true, ".cache": true, "vendor": true, ".yarn": true,
	".pnpm-store": true, "bower_components": true, ".turbo": true,
	".nuxt": true, ".output": true, "coverage": true, ".nyc_output": true,
}

func (a *App) isRemoteNoise(relPath string, isDir bool) bool {
	name := path.Base(relPath)
	if isDir {
		if remoteSkipDirs[name] || (strings.HasPrefix(name, ".") && name != ".gitignore") {
			return true
		}
	}
	for _, seg := range strings.Split(relPath, "/") {
		if remoteSkipDirs[seg] || (strings.HasPrefix(seg, ".") && seg != ".." && seg != "." && seg != ".gitignore") {
			return true
		}
	}
	// Check .gitignore patterns
	if a.remoteGitignore != nil && a.remoteGitignore.Match(relPath) {
		return true
	}
	// Extension-only filter for fast scanning (no download)
	ext := strings.ToLower(path.Ext(relPath))
	return !snapshot.IsTextFile(ext, nil)
}

// fingerprintFor returns the fingerprint for a file from the scanned remote entries.
func (a *App) fingerprintFor(relPath string) string {
	for _, e := range a.scannedRemoteEntries {
		if e.path == relPath {
			return e.fingerprint()
		}
	}
	return ""
}

// remoteIsBinary performs content-based binary check by reading the first bytes of a remote file.
func (a *App) remoteIsBinary(relPath string) bool {
	ext := strings.ToLower(path.Ext(relPath))
	rp := path.Join(a.remotePath, relPath)
	r, err := a.remoteSFTP.Open(rp)
	if err != nil {
		return true
	}
	defer r.Close()
	buf := make([]byte, 512)
	n, _ := r.Read(buf)
	return !snapshot.IsTextFile(ext, buf[:n])
}

// App is the main application struct with bound methods.
type App struct {
	ctx              context.Context
	workspace        string
	workspaceName    string
	startupWorkspace string
	isRemote         bool

	// Remote connection (lifetime = workspace session)
	remoteClient     *ssh.Client
	remoteSFTP       *sftp.Client
	remotePath       string
	remoteSSHCfg     terminal.SSHConfig  // saved for auto-creating SSH terminals
	remotePollCancel context.CancelFunc
	remoteGitignore  *scanner.Gitignore

	snapEng  *snapshot.Engine
	termMgr  *terminal.Manager
	fsw      *watcher.Watcher
	cfgStore *config.Store

	scannedFiles         []string
	scannedRemoteEntries []remoteFileEntry
	mu                   sync.Mutex
}

func NewApp() *App {
	store, err := config.NewStore()
	if err != nil {
		println("config store init failed:", err.Error())
		store = nil
	}
	return &App{
		termMgr:  terminal.NewManager(),
		cfgStore: store,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetStartupWorkspace() string { return a.startupWorkspace }

func (a *App) OpenInNewWindow(path string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("找不到可执行文件: %w", err)
	}
	return exec.Command(exe, "--workspace", path).Start()
}

func (a *App) shutdown(ctx context.Context) {
	a.termMgr.CloseAll()
	if a.fsw != nil {
		a.fsw.Close()
	}
	a.closeRemote()
}

// ensureGitignore makes sure .warp-snapshots is in the workspace .gitignore.
func ensureGitignore(workspace string) {
	giPath := filepath.Join(workspace, ".gitignore")
	data, err := os.ReadFile(giPath)
	if os.IsNotExist(err) {
		os.WriteFile(giPath, []byte(".warp-snapshots\n"), 0644)
		return
	}
	if err != nil {
		return
	}
	content := string(data)
	if !strings.Contains(content, ".warp-snapshots") {
		f, err := os.OpenFile(giPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer f.Close()
		if !strings.HasSuffix(content, "\n") {
			f.WriteString("\n")
		}
		f.WriteString(".warp-snapshots\n")
	}
}

func (a *App) remoteEnsureGitignore() {
	giPath := path.Join(a.remotePath, ".gitignore")
	data, err := a.readRemoteFileRaw(giPath)
	if err != nil {
		f, ferr := a.remoteSFTP.Create(giPath)
		if ferr != nil {
			return
		}
		defer f.Close()
		f.Write([]byte(".warp-snapshots\n"))
		return
	}
	content := string(data)
	if !strings.Contains(content, ".warp-snapshots") {
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += ".warp-snapshots\n"
		f, ferr := a.remoteSFTP.Create(giPath)
		if ferr != nil {
			return
		}
		defer f.Close()
		f.Write([]byte(content))
	}
}

func (a *App) closeRemote() {
	if a.remotePollCancel != nil {
		a.remotePollCancel()
		a.remotePollCancel = nil
	}
	if a.remoteSFTP != nil {
		a.remoteSFTP.Close()
		a.remoteSFTP = nil
	}
	if a.remoteClient != nil {
		a.remoteClient.Close()
		a.remoteClient = nil
	}
	a.isRemote = false
	a.remotePath = ""
	a.scannedRemoteEntries = nil
}

// ─── Workspace ───────────────────────────────────────

type WorkspaceInfo struct {
	Path         string                `json:"path"`
	Name         string                `json:"name"`
	FileCount    int                   `json:"fileCount"`
	Files        []string              `json:"files"`
	IsRemote     bool                  `json:"isRemote"`
	ChangedFiles []snapshot.FileChange `json:"changedFiles"`
}

func (a *App) SelectWorkspace() (*WorkspaceInfo, error) {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择工作区文件夹",
	})
	if err != nil {
		return nil, fmt.Errorf("选择文件夹失败: %w", err)
	}
	if path == "" {
		return nil, nil
	}
	return a.OpenWorkspace(path)
}

func (a *App) OpenWorkspace(path string) (*WorkspaceInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closeRemote()

	if a.fsw != nil {
		a.fsw.Close()
	}

	a.workspace = path
	ensureGitignore(path)
	a.snapEng = snapshot.NewEngine(path)

	result, err := scanner.Scan(path)
	if err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
	}
	a.scannedFiles = result.Files

	if err := a.snapEng.LoadManifest(); err != nil {
		return nil, fmt.Errorf("加载快照失败: %w", err)
	}
	if !a.snapEng.HasSnapshot() {
		if err := a.snapEng.Init(result.Files); err != nil {
			return nil, fmt.Errorf("创建快照失败: %w", err)
		}
	}

	a.fsw, err = watcher.New(path, func(events []string) { a.onFileChanged() })
	if err != nil {
		return nil, fmt.Errorf("启动文件监听失败: %w", err)
	}

	if a.cfgStore != nil {
		a.cfgStore.SaveWorkspace(path)
	}

	info := a.makeWorkspaceInfo()
	a.emitChanges()
	return info, nil
}

// RemoteDirEntry represents a single directory entry on the remote server.
type RemoteDirEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
}

// ─── Remote Workspace (SFTP Direct) ──────────────────

func (a *App) GetRemoteWorkspaces() ([]config.RemoteWorkspaceEntry, error) {
	if a.cfgStore == nil {
		return nil, nil
	}
	return a.cfgStore.LoadRemoteWorkspaces()
}

func (a *App) SaveRemoteWorkspace(entry config.RemoteWorkspaceEntry) error {
	if a.cfgStore == nil {
		return fmt.Errorf("配置存储不可用")
	}
	return a.cfgStore.SaveRemoteWorkspace(entry)
}

// ListRemoteDir lists entries in a single remote directory (lazy loading).
func (a *App) ListRemoteDir(dir string) ([]RemoteDirEntry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isRemote || a.remoteSFTP == nil {
		return nil, fmt.Errorf("当前不是远程工作区")
	}
	remoteDir := path.Join(a.remotePath, dir)
	if dir == "" {
		remoteDir = a.remotePath
	}
	infos, err := a.remoteSFTP.ReadDir(remoteDir)
	if err != nil {
		return nil, fmt.Errorf("读取远程目录失败: %w", err)
	}
	var entries []RemoteDirEntry
	if dir != "" {
		parent := path.Dir(dir)
		if parent == "." {
			parent = ""
		}
		entries = append(entries, RemoteDirEntry{Name: "..", Path: parent, IsDir: true})
	}
	for _, info := range infos {
		entryPath := path.Join(dir, info.Name())
		if a.isRemoteNoise(entryPath, info.IsDir()) {
			continue
		}
		entries = append(entries, RemoteDirEntry{
			Name:    info.Name(),
			Path:    entryPath,
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Unix(),
		})
	}
	return entries, nil
}

func (a *App) RemoveRemoteWorkspace(name string) error {
	if a.cfgStore == nil {
		return nil
	}
	return a.cfgStore.RemoveRemoteWorkspace(name)
}

type SSHConfig struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	KeyPath  string `json:"keyPath"`
}

func (a *App) OpenRemoteWorkspace(cfg SSHConfig, remotePath string) (*WorkspaceInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closeRemote()
	if a.fsw != nil {
		a.fsw.Close()
		a.fsw = nil
	}

	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.Password == "" && cfg.KeyPath == "" && a.cfgStore != nil {
		configs, err := a.cfgStore.LoadSSHConfigs()
		if err == nil {
			for _, c := range configs {
				if c.Name == cfg.Name || strings.HasPrefix(cfg.Name, c.Name+":") {
					cfg.Password = c.Password
					cfg.KeyPath = c.KeyPath
					break
				}
			}
		}
	}
	tCfg := terminal.SSHConfig{
		Name: cfg.Name, Host: cfg.Host, Port: cfg.Port,
		User: cfg.User, Password: cfg.Password, KeyPath: cfg.KeyPath,
	}
	auth, err := terminal.BuildSSHAuth(tCfg)
	if err != nil {
		return nil, err
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("SSH连接失败: %w", err)
	}

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("SFTP初始化失败: %w", err)
	}

	// Full Walk for change detection (noise-filtered, fast with skip dirs)
	entries, err := a.listRemoteFiles(sftpClient, remotePath)
	if err != nil {
		sftpClient.Close()
		client.Close()
		return nil, fmt.Errorf("扫描远程目录失败: %w", err)
	}

	a.remoteClient = client
	a.remoteSFTP = sftpClient
	a.remotePath = remotePath
	a.isRemote = true
	a.workspace = workspaceName(cfg.Name, remotePath)
	a.remoteSSHCfg = tCfg
	a.scannedRemoteEntries = entries
	a.scannedFiles = entriesToPaths(entries)
	a.snapEng = snapshot.NewEngine(remotePath)

	// Ensure .warp-snapshots/ exists on remote
	snapDir := path.Join(a.remotePath, ".warp-snapshots")
	if err := sftpClient.MkdirAll(snapDir); err != nil {
		sftpClient.Close()
		client.Close()
		return nil, fmt.Errorf("创建远程快照目录失败: %w", err)
	}

	a.remoteEnsureGitignore()

	// Load remote .gitignore for filtering
	if giData, err := a.readRemoteFileRaw(path.Join(a.remotePath, ".gitignore")); err == nil {
		a.remoteGitignore = scanner.ParseGitignore(string(giData))
	} else {
		a.remoteGitignore = &scanner.Gitignore{}
	}
	// Load manifest from remote; if absent init fresh
	if err := a.remoteLoadManifest(); err != nil {
		sftpClient.Close()
		client.Close()
		return nil, fmt.Errorf("加载远程清单失败: %w", err)
	}
	if !a.snapEng.HasSnapshot() {
		if err := a.remoteInitSnapshots(entries); err != nil {
			sftpClient.Close()
			client.Close()
			return nil, fmt.Errorf("创建远程快照失败: %w", err)
		}
	} else {
		// Clean stale manifest entries (now filtered by gitignore/binary/size)
		currentSet := make(map[string]bool, len(entries))
		for _, e := range entries {
			currentSet[e.path] = true
		}
		a.snapEng.FilterManifest(func(p string) bool { return currentSet[p] })
		for _, e := range entries {
			if _, ok := a.snapEng.GetFileFingerprint(e.path); !ok {
				a.snapEng.SetFileFingerprint(e.path, e.fingerprint())
			}
		}
		a.remoteSaveManifest()
	}

	// Save entry
	if a.cfgStore != nil {
		a.cfgStore.SaveRemoteWorkspace(config.RemoteWorkspaceEntry{
			Name:       workspaceName(cfg.Name, remotePath),
			Host:       cfg.Host,
			Port:       cfg.Port,
			User:       cfg.User,
			RemotePath: remotePath,
		})
	}

	info := a.makeWorkspaceInfo()
	a.emitChanges()
		pollCtx, cancel := context.WithCancel(a.ctx)
		a.remotePollCancel = cancel
		go a.remotePollLoop(pollCtx)

	return info, nil
}

func (a *App) RefreshLocalWorkspace() (*WorkspaceInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.isRemote || a.workspace == "" {
		return nil, fmt.Errorf("当前不是本地工作区")
	}
	a.refreshScanLocked()
	a.emitChanges()
	return a.makeWorkspaceInfo(), nil
}

func (a *App) RefreshRemoteWorkspace() (*WorkspaceInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isRemote || a.remoteSFTP == nil {
		return nil, fmt.Errorf("当前不是远程工作区")
	}
	entries, err := a.listRemoteFiles(a.remoteSFTP, a.remotePath)
	if err != nil {
		return nil, err
	}
	a.scannedRemoteEntries = entries
	a.scannedFiles = entriesToPaths(entries)
	info := a.makeWorkspaceInfo()
	a.emitChanges()
	return info, nil
}

func (a *App) listRemoteFiles(c *sftp.Client, root string) ([]remoteFileEntry, error) {
	var entries []remoteFileEntry
	w := c.Walk(root)
	for w.Step() {
		if w.Err() != nil {
			continue
		}
		s := w.Stat()
		if s == nil || s.IsDir() {
			continue
		}
		rel := strings.TrimPrefix(path.Clean(w.Path()), path.Clean(root))
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" || a.isRemoteNoise(rel, false) || s.Size() > 5*1024*1024 {
			continue
		}
		entries = append(entries, remoteFileEntry{
			path:    filepath.ToSlash(rel),
			size:    s.Size(),
			modTime: s.ModTime(),
		})
	}
	return entries, nil
}

func (a *App) readRemoteFile(relPath string) ([]byte, error) {
	rp := path.Join(a.remotePath, relPath)
	r, err := a.remoteSFTP.Open(rp)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// ─── Remote Snapshot Helpers ─────────────────────────

func (a *App) remoteSnapPath(relPath string) string {
	return path.Join(a.remotePath, ".warp-snapshots", relPath)
}

func (a *App) remoteObjectPath(hash string) string {
	if len(hash) < 4 {
		return ""
	}
	return path.Join(a.remotePath, ".warp-snapshots", "objects", hash[:2], hash[2:])
}

func (a *App) remoteHasObject(hash string) bool {
	rp := a.remoteObjectPath(hash)
	if rp == "" {
		return false
	}
	_, err := a.remoteSFTP.Stat(rp)
	return err == nil
}

func (a *App) remoteWriteObject(hash string, data []byte) error {
	if a.remoteHasObject(hash) {
		return nil // dedup
	}
	rp := a.remoteObjectPath(hash)
	if rp == "" {
		return fmt.Errorf("invalid hash")
	}
	if err := a.remoteSFTP.MkdirAll(path.Dir(rp)); err != nil {
		return err
	}
	f, err := a.remoteSFTP.Create(rp)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(snapshot.Compress(data))
	return err
}

func (a *App) remoteReadObject(hash string) ([]byte, error) {
	rp := a.remoteObjectPath(hash)
	if rp == "" {
		return nil, fmt.Errorf("invalid hash")
	}
	r, err := a.remoteSFTP.Open(rp)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return snapshot.Decompress(raw), nil
}

// remoteReadSnapshotByPath reads a snapshot file, trying object storage first.
func (a *App) remoteReadSnapshotByPath(relPath string) ([]byte, error) {
	if a.snapEng != nil {
		if hash, ok := a.snapEng.GetFileHash(relPath); ok && len(hash) >= 4 {
			if data, err := a.remoteReadObject(hash); err == nil {
				return data, nil
			}
		}
	}
	// Fall back to old path-based layout
	r, err := a.remoteSFTP.Open(a.remoteSnapPath(relPath))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func (a *App) remoteWriteSnapshot(relPath string, data []byte) error {
	h := snapshot.HashBytes(data)
	if err := a.remoteWriteObject(h, data); err != nil {
		return err
	}
	if a.snapEng != nil {
		a.snapEng.SetFileHash(relPath, h)
	}
	return nil
}

func (a *App) remoteRemoveSnapshot(relPath string) error {
	return a.remoteSFTP.Remove(a.remoteSnapPath(relPath))
}

func (a *App) remoteRemoveSnapshotDir() {
	// best-effort cleanup
	a.remoteSFTP.RemoveDirectory(path.Join(a.remotePath, ".warp-snapshots"))
}

func (a *App) remoteLoadManifest() error {
	rp := path.Join(a.remotePath, ".warp-snapshots", "manifest.json")
	data, err := a.readRemoteFileRaw(rp)
	if err != nil {
		// If manifest doesn't exist on remote, start fresh
		a.snapEng = snapshot.NewEngine(a.workspace)
		return nil
	}
	a.snapEng = snapshot.NewEngine(a.workspace)
	return a.snapEng.LoadManifestFrom(data)
}

func (a *App) remoteSaveManifest() error {
	data, err := a.snapEng.MarshalManifest()
	if err != nil {
		return err
	}
	rp := path.Join(a.remotePath, ".warp-snapshots", "manifest.json")
	a.remoteSFTP.MkdirAll(path.Dir(rp))
	f, err := a.remoteSFTP.Create(rp)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// readRemoteFileRaw reads a file by its full remote path (not relative to workspace).
func (a *App) readRemoteFileRaw(fullPath string) ([]byte, error) {
	r, err := a.remoteSFTP.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// remoteInitSnapshots copies text files to remote .warp-snapshots using server-side
// copy via SSH exec. Falls back to per-file SFTP if SSH exec is unavailable.
func (a *App) remoteInitSnapshots(entries []remoteFileEntry) error {
	type textEntry struct {
		path string
		fp   string
	}
	var textEntries []textEntry
	for _, e := range entries {
		ext := strings.ToLower(path.Ext(e.path))
		if !snapshot.IsTextFile(ext, nil) {
			continue
		}
		textEntries = append(textEntries, textEntry{path: e.path, fp: e.fingerprint()})
	}

	if len(textEntries) == 0 {
		return a.remoteSaveManifest()
	}

	runtime.EventsEmit(a.ctx, "snapshot-progress", map[string]interface{}{
		"phase": "start", "total": len(textEntries), "current": 0,
	})

	textPaths := make([]string, len(textEntries))
	for i, te := range textEntries {
		textPaths[i] = te.path
	}

	chunkSize := 1000
	for i := 0; i < len(textPaths); i += chunkSize {
		end := i + chunkSize
		if end > len(textPaths) {
			end = len(textPaths)
		}
		chunk := textPaths[i:end]
		mapping, err := a.remoteExecCopyChunk(chunk)
		if err != nil {
			for _, p := range chunk {
				data, err := a.readRemoteFile(p)
				if err != nil {
					continue
				}
				if err := a.remoteWriteSnapshot(p, data); err != nil {
					return err
				}
			}
		} else {
			for path, sha256 := range mapping {
				a.snapEng.SetFileHash(path, sha256)
				for _, te := range textEntries {
					if te.path == path {
						a.snapEng.SetFileFingerprint(path, te.fp)
						break
					}
				}
			}
		}
		runtime.EventsEmit(a.ctx, "snapshot-progress", map[string]interface{}{
			"phase": "progress", "total": len(textPaths), "current": end,
		})
	}

	return a.remoteSaveManifest()
}

// remoteExecCopyChunk copies files server-side using sha256sum + object storage.
// Returns path→hash mapping parsed from stdout.
func (a *App) remoteExecCopyChunk(paths []string) (map[string]string, error) {
	sess, err := a.remoteClient.NewSession()
	if err != nil {
		return nil, err
	}
	defer sess.Close()

	script := "cd " + shellQuote(a.remotePath) + " || exit 1\n" +
		"mkdir -p .warp-snapshots/objects || exit 1\n" +
		"while IFS= read -r f; do\n" +
		"  [ -z \"$f\" ] && continue\n" +
		"  hash=$(sha256sum \"$f\" | awk '{print $1}')\n" +
		"  p1=\"${hash:0:2}\"\n" +
		"  p2=\"${hash:2}\"\n" +
		"  mkdir -p \".warp-snapshots/objects/$p1\"\n" +
		"  [ -f \".warp-snapshots/objects/$p1/$p2\" ] || gzip -c \"$f\" > \".warp-snapshots/objects/$p1/$p2\"\n" +
		"  echo \"$hash $f\"\n" +
		"done\n"

	sess.Stdin = strings.NewReader(strings.Join(paths, "\n") + "\n")
	output, err := sess.Output(script)
	if err != nil {
		return nil, err
	}
	mapping := make(map[string]string, len(paths))
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			mapping[parts[1]] = parts[0]
		}
	}
	return mapping, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func workspaceName(cfgName, remotePath string) string {
	if strings.HasSuffix(cfgName, ":"+remotePath) {
		return cfgName
	}
	return cfgName + ":" + remotePath
}

// remotePollLoop periodically re-scans the remote directory for changes.
func (a *App) remotePollLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.remotePoll()
		}
	}
}

func (a *App) remotePoll() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isRemote || a.remoteSFTP == nil {
		return
	}
	entries, err := a.listRemoteFiles(a.remoteSFTP, a.remotePath)
	if err != nil {
		return
	}
	oldFps := entriesToFingerprints(a.scannedRemoteEntries)
	newFps := entriesToFingerprints(entries)
	if fingerprintsEqual(oldFps, newFps) {
		return
	}
	a.scannedRemoteEntries = entries
	a.scannedFiles = entriesToPaths(entries)
	a.emitChanges()
}

func fingerprintsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// remoteChangedFiles returns changes with line stats.
func (a *App) remoteChangedFiles() []snapshot.FileChange {
	changes := a.snapEng.ChangedFilesByHash(entriesToFingerprints(a.scannedRemoteEntries))
	for i, c := range changes {
		var oldData, newData []byte
		switch c.Status {
		case snapshot.StatusAdded:
			newData, _ = a.readRemoteFile(c.Path)
		case snapshot.StatusModified:
			oldData, _ = a.remoteReadSnapshotByPath(c.Path)
			newData, _ = a.readRemoteFile(c.Path)
		case snapshot.StatusDeleted:
			oldData, _ = a.remoteReadSnapshotByPath(c.Path)
		}
		changes[i].Additions, changes[i].Deletions = snapshot.DiffStats(oldData, newData)
	}
	return changes
}

func (a *App) GetWorkspaceHistory() []config.WorkspaceEntry {
	if a.cfgStore == nil {
		return nil
	}
	entries, _ := a.cfgStore.LoadWorkspaces()
	return entries
}

func (a *App) RemoveWorkspaceFromHistory(path string) error {
	if a.cfgStore == nil {
		return nil
	}
	return a.cfgStore.RemoveWorkspace(path)
}

func (a *App) GetWorkspaceInfo() *WorkspaceInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.workspace == "" || a.snapEng == nil {
		return nil
	}
	return a.makeWorkspaceInfo()
}

func (a *App) makeWorkspaceInfo() *WorkspaceInfo {
	var changes []snapshot.FileChange
	if a.isRemote {
		changes = a.remoteChangedFiles()
	} else {
		changes = a.snapEng.ChangedFiles(a.scannedFiles)
	}
	name := a.workspace
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '\\' || name[i] == '/' {
			name = name[i+1:]
			break
		}
	}
	if a.isRemote {
		name = a.remotePath
		for i := len(name) - 1; i >= 0; i-- {
			if name[i] == '/' {
				name = name[i+1:]
				break
			}
		}
	}
	return &WorkspaceInfo{
		Path:         a.workspace,
		Name:         name,
		FileCount:    len(a.scannedFiles),
		Files:        a.scannedFiles,
		IsRemote:     a.isRemote,
		ChangedFiles: changes,
	}
}

func (a *App) onFileChanged() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.snapEng == nil {
		return
	}
	var changes []snapshot.FileChange
	if a.isRemote {
		changes = a.remoteChangedFiles()
	} else {
		changes = a.snapEng.ChangedFiles(a.scannedFiles)
	}
	runtime.EventsEmit(a.ctx, "file-changes", changes)
}

// ─── File Changes ────────────────────────────────────

func (a *App) GetChangedFiles() []snapshot.FileChange {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.snapEng == nil {
		return nil
	}
	if a.isRemote {
		return a.remoteChangedFiles()
	}
	return a.snapEng.ChangedFiles(a.scannedFiles)
}

func (a *App) AcceptAll() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.snapEng == nil {
		return fmt.Errorf("未选择工作区")
	}
	if a.isRemote {
		fps := entriesToFingerprints(a.scannedRemoteEntries)
		changes := a.snapEng.ChangedFilesByHash(fps)
		for _, c := range changes {
			if a.remoteIsBinary(c.Path) {
				a.snapEng.SetFileFingerprint(c.Path, fps[c.Path])
				continue
			}
			data, err := a.readRemoteFile(c.Path)
			if err != nil {
				continue
			}
			if err := a.remoteWriteSnapshot(c.Path, data); err != nil {
				return err
			}
			a.snapEng.SetFileFingerprint(c.Path, fps[c.Path])
		}
		if err := a.remoteSaveManifest(); err != nil {
			return err
		}
		a.emitChanges()
		return nil
	}
	changes := a.snapEng.ChangedFiles(a.scannedFiles)
	paths := make([]string, len(changes))
	for i, c := range changes {
		paths[i] = c.Path
	}
	if err := a.snapEng.AcceptAll(paths); err != nil {
		return err
	}
	a.refreshScanLocked()
	a.emitChanges()
	return nil
}

func (a *App) RevertAll() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.snapEng == nil {
		return fmt.Errorf("未选择工作区")
	}
	if a.isRemote {
		changes := a.snapEng.ChangedFilesByHash(entriesToFingerprints(a.scannedRemoteEntries))
		for _, c := range changes {
			snapData, err := a.remoteReadSnapshotByPath(c.Path)
			if err != nil {
				continue // no snapshot, skip
			}
			rp := path.Join(a.remotePath, c.Path)
			f, err := a.remoteSFTP.Create(rp)
			if err != nil {
				return err
			}
			if _, err := f.Write(snapData); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
		a.refreshScanLocked()
		for _, c := range changes {
			a.snapEng.SetFileFingerprint(c.Path, a.fingerprintFor(c.Path))
		}
		if err := a.remoteSaveManifest(); err != nil {
			return err
		}
		a.emitChanges()
		return nil
	}
	changes := a.snapEng.ChangedFiles(a.scannedFiles)
	paths := make([]string, len(changes))
	for i, c := range changes {
		paths[i] = c.Path
	}
	if err := a.snapEng.RevertAll(paths); err != nil {
		return err
	}
	a.refreshScanLocked()
	a.emitChanges()
	return nil
}

func (a *App) AcceptFile(p string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.snapEng == nil {
		return fmt.Errorf("未选择工作区")
	}
	if a.isRemote {
		data, err := a.readRemoteFile(p)
		if err != nil {
			a.remoteRemoveSnapshot(p)
			a.snapEng.RemoveFromManifest([]string{p})
			return a.remoteSaveManifest()
		}
		ext := strings.ToLower(path.Ext(p))
		fp := a.fingerprintFor(p)
		if !snapshot.IsTextFile(ext, snapshot.FirstBytes(data)) {
			a.snapEng.SetFileFingerprint(p, fp)
			return a.remoteSaveManifest()
		}
		if err := a.remoteWriteSnapshot(p, data); err != nil {
			return err
		}
		a.snapEng.SetFileFingerprint(p, fp)
		if err := a.remoteSaveManifest(); err != nil {
			return err
		}
		a.emitChanges()
		return nil
	}
	if err := a.snapEng.AcceptFile(p); err != nil {
		return err
	}
	a.refreshScanLocked()
	a.emitChanges()
	return nil
}

func (a *App) RevertFile(p string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.snapEng == nil {
		return fmt.Errorf("未选择工作区")
	}
	if a.isRemote {
		snapData, err := a.remoteReadSnapshotByPath(p)
		if err != nil {
			return a.remoteSaveManifest()
		}
		rp := path.Join(a.remotePath, p)
		a.remoteSFTP.MkdirAll(path.Dir(rp))
		f, err := a.remoteSFTP.Create(rp)
		if err != nil {
			return err
		}
		if _, err := f.Write(snapData); err != nil {
			f.Close()
			return err
		}
		f.Close()
		a.refreshScanLocked()
		a.snapEng.SetFileFingerprint(p, a.fingerprintFor(p))
		if err := a.remoteSaveManifest(); err != nil {
			return err
		}
		a.emitChanges()
		return nil
	}
	if err := a.snapEng.RevertFile(p); err != nil {
		return err
	}
	a.refreshScanLocked()
	a.emitChanges()
	return nil
}

func (a *App) GetFileDiff(path string) (map[string]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.snapEng == nil {
		return nil, fmt.Errorf("未选择工作区")
	}
	if a.isRemote {
		newData, err := a.readRemoteFile(path)
		if err != nil {
			return nil, err
		}
		oldData, err := a.remoteReadSnapshotByPath(path)
		if err != nil {
			oldData = nil // new file, no snapshot
		}
		return map[string]string{
			"old": string(oldData),
			"new": string(newData),
		}, nil
	}
	oldC, newC, err := a.snapEng.Diff(path)
	if err != nil {
		return nil, err
	}
	return map[string]string{"old": oldC, "new": newC}, nil
}

func (a *App) GetFileContent(path string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.workspace == "" || a.snapEng == nil {
		return "", fmt.Errorf("未选择工作区")
	}
	if a.isRemote {
		data, err := a.readRemoteFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return snapshot.ReadFileContent(a.workspace, path)
}

func (a *App) SaveFile(relPath, content string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.workspace == "" || a.snapEng == nil {
		return fmt.Errorf("未选择工作区")
	}
	if a.isRemote {
		if a.remoteSFTP == nil {
			return fmt.Errorf("远程连接不可用")
		}
		rp := path.Join(a.remotePath, relPath)
		a.remoteSFTP.MkdirAll(path.Dir(rp))
		f, err := a.remoteSFTP.Create(rp)
		if err != nil {
			return fmt.Errorf("写入远程文件失败: %w", err)
		}
		defer f.Close()
		if _, err := f.Write([]byte(content)); err != nil {
			return fmt.Errorf("写入远程文件失败: %w", err)
		}
		f.Close()
		a.remoteWriteSnapshot(relPath, []byte(content))
		a.refreshScanLocked()
		a.snapEng.SetFileFingerprint(relPath, a.fingerprintFor(relPath))
		if err := a.remoteSaveManifest(); err != nil {
			return fmt.Errorf("更新清单失败: %w", err)
		}
		a.emitChanges()
		return nil
	}
	fullPath := filepath.Join(a.workspace, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}
	if err := a.snapEng.AcceptFile(relPath); err != nil {
		return fmt.Errorf("更新快照失败: %w", err)
	}
	a.refreshScanLocked()
	a.emitChanges()
	return nil
}

// ─── Terminal ────────────────────────────────────────

func (a *App) CreateTerminal() (string, error) {
	var id string
	var err error
	if a.isRemote {
		id, err = a.termMgr.CreateSSH(a.remoteSSHCfg)
	} else {
		id, err = a.termMgr.Create(a.workspace)
	}
	if err != nil {
		return "", err
	}
	sess, _ := a.termMgr.Get(id)
	go a.readTerminalOutput(id, sess)

	if a.isRemote && a.remotePath != "" {
		sess.Write([]byte("cd '" + a.remotePath + "'\n"))
	}
	return id, nil
}

func (a *App) WriteToTerminal(tabId, data string) error {
	sess, err := a.termMgr.Get(tabId)
	if err != nil {
		return err
	}
	_, err = sess.Write([]byte(data))
	return err
}

func (a *App) ResizeTerminal(tabId string, cols, rows int) error {
	sess, err := a.termMgr.Get(tabId)
	if err != nil {
		return err
	}
	return sess.Resize(uint16(rows), uint16(cols))
}

func (a *App) CloseTerminal(tabId string) error {
	return a.termMgr.Close(tabId)
}

// ─── SSH ─────────────────────────────────────────────

func (a *App) CreateSSHTerminal(cfg SSHConfig) (string, error) {
	tCfg := terminal.SSHConfig{
		Name: cfg.Name, Host: cfg.Host, Port: cfg.Port,
		User: cfg.User, Password: cfg.Password, KeyPath: cfg.KeyPath,
	}
	id, err := a.termMgr.CreateSSH(tCfg)
	if err != nil {
		return "", err
	}
	sess, _ := a.termMgr.Get(id)
	go a.readTerminalOutput(id, sess)
	return id, nil
}

func (a *App) GetSSHConfigs() ([]config.SSHConfig, error) {
	if a.cfgStore == nil {
		return nil, nil
	}
	return a.cfgStore.LoadSSHConfigs()
}

func (a *App) SaveSSHConfig(cfg config.SSHConfig) error {
	if a.cfgStore == nil {
		return fmt.Errorf("配置存储不可用")
	}
	return a.cfgStore.SaveSSHConfig(cfg)
}

func (a *App) RemoveSSHConfig(name string) error {
	if a.cfgStore == nil {
		return nil
	}
	return a.cfgStore.RemoveSSHConfig(name)
}

// ─── Startup Commands ──────────────────────────────────

func (a *App) GetStartupCommands() ([]config.StartupCommand, error) {
	if a.cfgStore == nil {
		return nil, nil
	}
	return a.cfgStore.LoadStartupCommands()
}

func (a *App) SaveStartupCommands(cmds []config.StartupCommand) error {
	if a.cfgStore == nil {
		return fmt.Errorf("配置存储不可用")
	}
	return a.cfgStore.SaveStartupCommands(cmds)
}

func (a *App) readTerminalOutput(id string, sess *terminal.Session) {
	buf := make([]byte, 4096)
	for {
		n, err := sess.Read(buf)
		if err != nil {
			runtime.EventsEmit(a.ctx, "terminal-output:"+id, "\r\n[终端已关闭]")
			return
		}
		if n > 0 {
			runtime.EventsEmit(a.ctx, "terminal-output:"+id, string(buf[:n]))
		}
	}
}

func (a *App) refreshScan() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.refreshScanLocked()
}

func (a *App) refreshScanLocked() {
	if a.isRemote {
		entries, err := a.listRemoteFiles(a.remoteSFTP, a.remotePath)
		if err != nil {
			return
		}
		a.scannedRemoteEntries = entries
		a.scannedFiles = entriesToPaths(entries)
		return
	}
	result, err := scanner.Scan(a.workspace)
	if err != nil {
		return
	}
	a.scannedFiles = result.Files
}

func (a *App) emitChanges() {
	var changes []snapshot.FileChange
	if a.isRemote {
		changes = a.remoteChangedFiles()
	} else {
		changes = a.snapEng.ChangedFiles(a.scannedFiles)
	}
	runtime.EventsEmit(a.ctx, "file-changes", changes)
}

