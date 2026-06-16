package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// WorkspaceEntry represents a saved workspace.
type WorkspaceEntry struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	LastOpened string `json:"lastOpened"`
}

// Store manages persistent app configuration.
type Store struct {
	dir string
}

// NewStore creates a config store in the user's config directory.
func NewStore() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir = filepath.Join(dir, "just-warp-go")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// LoadWorkspaces reads the workspace history.
func (s *Store) LoadWorkspaces() ([]WorkspaceEntry, error) {
	path := filepath.Join(s.dir, "workspaces.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []WorkspaceEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// SaveWorkspace adds or updates a workspace entry.
func (s *Store) SaveWorkspace(wsPath string) error {
	entries, _ := s.LoadWorkspaces()

	// Remove existing entry with same path
	filtered := make([]WorkspaceEntry, 0, len(entries))
	for _, e := range entries {
		if e.Path != wsPath {
			filtered = append(filtered, e)
		}
	}

	entry := WorkspaceEntry{
		Path:       wsPath,
		Name:       filepath.Base(wsPath),
		LastOpened: time.Now().Format(time.RFC3339),
	}

	// Prepend (most recent first), keep max 20
	filtered = append([]WorkspaceEntry{entry}, filtered...)
	if len(filtered) > 20 {
		filtered = filtered[:20]
	}

	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "workspaces.json"), data, 0644)
}


// TerminalSnapshot represents a persisted terminal UI/session snapshot.
type TerminalSnapshot struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Workspace string `json:"workspace"`
	CWD       string `json:"cwd"`
	SSHName   string `json:"sshName,omitempty"`
	Output    string `json:"output"`
	Restored  bool   `json:"restored"`
	Active    bool   `json:"active,omitempty"`
	UpdatedAt string `json:"updatedAt"`
}

func (s *Store) LoadTerminalSnapshots() ([]TerminalSnapshot, error) {
	path := filepath.Join(s.dir, "terminal-sessions.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var items []TerminalSnapshot
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) SaveTerminalSnapshots(items []TerminalSnapshot) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "terminal-sessions.json"), data, 0644)
}

// ─── Startup Commands ─────────────────────────────────

func (s *Store) LoadStartupCommands() ([]StartupCommand, error) {
	path := filepath.Join(s.dir, "startup-commands.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cmds []StartupCommand
	if err := json.Unmarshal(data, &cmds); err != nil {
		return nil, err
	}
	return cmds, nil
}

func (s *Store) SaveStartupCommands(cmds []StartupCommand) error {
	data, err := json.MarshalIndent(cmds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "startup-commands.json"), data, 0644)
}

// ─── SSH Configs ────────────────────────────────────

// SSHConfig represents a saved SSH connection.
type SSHConfig struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	KeyPath  string `json:"keyPath"`
}

// RemoteWorkspaceEntry represents a saved remote workspace mirror mapping.
type RemoteWorkspaceEntry struct {
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	RemotePath string `json:"remotePath"`
	CachePath  string `json:"cachePath"`
}

// StartupCommand represents a saved quick-launch command.
type StartupCommand struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

func (s *Store) LoadSSHConfigs() ([]SSHConfig, error) {
	path := filepath.Join(s.dir, "ssh-configs.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cfgs []SSHConfig
	if err := json.Unmarshal(data, &cfgs); err != nil {
		return nil, err
	}
	return cfgs, nil
}

func (s *Store) SaveSSHConfig(cfg SSHConfig) error {
	cfgs, _ := s.LoadSSHConfigs()
	// Update if same name/host, else append
	found := false
	for i, c := range cfgs {
		if c.Name == cfg.Name {
			cfgs[i] = cfg
			found = true
			break
		}
	}
	if !found {
		cfgs = append(cfgs, cfg)
	}
	data, err := json.MarshalIndent(cfgs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "ssh-configs.json"), data, 0644)
}

func (s *Store) RemoveSSHConfig(name string) error {
	cfgs, _ := s.LoadSSHConfigs()
	filtered := make([]SSHConfig, 0, len(cfgs))
	for _, c := range cfgs {
		if c.Name != name {
			filtered = append(filtered, c)
		}
	}
	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "ssh-configs.json"), data, 0644)
}

// ─── Remote Workspaces ───────────────────────────────

func (s *Store) LoadRemoteWorkspaces() ([]RemoteWorkspaceEntry, error) {
	path := filepath.Join(s.dir, "remote-workspaces.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var items []RemoteWorkspaceEntry
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) SaveRemoteWorkspace(item RemoteWorkspaceEntry) error {
	items, _ := s.LoadRemoteWorkspaces()
	found := false
	for i, it := range items {
		if it.Host == item.Host && it.User == item.User && it.RemotePath == item.RemotePath {
			items[i] = item
			found = true
			break
		}
	}
	if !found {
		items = append(items, item)
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "remote-workspaces.json"), data, 0644)
}

func (s *Store) RemoveRemoteWorkspace(name string) error {
	items, _ := s.LoadRemoteWorkspaces()
	filtered := make([]RemoteWorkspaceEntry, 0, len(items))
	for _, it := range items {
		if it.Name != name {
			filtered = append(filtered, it)
		}
	}
	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "remote-workspaces.json"), data, 0644)
}

// ─── Workspace ──────────────────────────────────────

// RemoveWorkspace removes a workspace from history.
func (s *Store) RemoveWorkspace(wsPath string) error {
	entries, err := s.LoadWorkspaces()
	if err != nil {
		return err
	}
	filtered := make([]WorkspaceEntry, 0, len(entries))
	for _, e := range entries {
		if e.Path != wsPath {
			filtered = append(filtered, e)
		}
	}
	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "workspaces.json"), data, 0644)
}
