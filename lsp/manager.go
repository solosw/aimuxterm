package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type ServerInfo struct {
	Language  string `json:"language"`
	Available bool   `json:"available"`
	Command   string `json:"command"`
	Message   string `json:"message"`
}

type serverSpec struct {
	command string
	args    []string
}

type server struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	writeMu sync.Mutex
}

type Manager struct {
	mu        sync.Mutex
	servers   map[string]*server
	onMessage func(language string, body []byte)
}

func NewManager(handler func(language string, body []byte)) *Manager {
	return &Manager{servers: make(map[string]*server), onMessage: handler}
}

func specFor(language string) (serverSpec, bool) {
	switch language {
	case "go":
		return serverSpec{command: "gopls", args: []string{"serve"}}, true
	case "typescript", "javascript":
		return serverSpec{command: "typescript-language-server", args: []string{"--stdio"}}, true
	case "vue":
		return serverSpec{command: "vue-language-server", args: []string{"--stdio"}}, true
	case "python":
		return serverSpec{command: "pyright-langserver", args: []string{"--stdio"}}, true
	case "rust":
		return serverSpec{command: "rust-analyzer"}, true
	case "c", "cpp", "objectivec":
		return serverSpec{command: "clangd"}, true
	case "java":
		return serverSpec{command: "jdtls"}, true
	case "csharp":
		return serverSpec{command: "omnisharp", args: []string{"--languageserver", "--stdio"}}, true
	case "php":
		return serverSpec{command: "intelephense", args: []string{"--stdio"}}, true
	case "ruby":
		return serverSpec{command: "ruby-lsp"}, true
	case "lua":
		return serverSpec{command: "lua-language-server"}, true
	case "dart":
		return serverSpec{command: "dart", args: []string{"language-server", "--stdio"}}, true
	case "kotlin":
		return serverSpec{command: "kotlin-language-server"}, true
	case "bash":
		return serverSpec{command: "bash-language-server", args: []string{"start"}}, true
	case "powershell":
		return serverSpec{command: "PowerShellEditorServices", args: []string{"--stdio"}}, true
	case "sql":
		return serverSpec{command: "sql-language-server", args: []string{"up", "--method", "stdio"}}, true
	case "yaml":
		return serverSpec{command: "yaml-language-server", args: []string{"--stdio"}}, true
	case "dockerfile":
		return serverSpec{command: "docker-langserver", args: []string{"--stdio"}}, true
	case "html", "xml":
		return serverSpec{command: "vscode-html-language-server", args: []string{"--stdio"}}, true
	case "css", "scss", "less":
		return serverSpec{command: "vscode-css-language-server", args: []string{"--stdio"}}, true
	case "json":
		return serverSpec{command: "vscode-json-language-server", args: []string{"--stdio"}}, true
	default:
		return serverSpec{}, false
	}
}

func (m *Manager) Status(language string) ServerInfo {
	spec, supported := specFor(language)
	if !supported {
		return ServerInfo{Language: language, Message: "该语言暂未配置 LSP"}
	}
	if _, err := exec.LookPath(spec.command); err != nil {
		return ServerInfo{Language: language, Command: spec.command, Message: "未检测到语言服务器"}
	}
	return ServerInfo{Language: language, Available: true, Command: spec.command}
}

func (m *Manager) Start(language, workspace string) error {
	spec, supported := specFor(language)
	if !supported {
		return fmt.Errorf("该语言暂未配置 LSP")
	}
	if _, err := exec.LookPath(spec.command); err != nil {
		return fmt.Errorf("未检测到 %s，请安装后重试", spec.command)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.servers[language]; exists {
		return nil
	}
	cmd := exec.Command(spec.command, spec.args...)
	cmd.Dir = workspace
	configureBackgroundProcess(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 %s 失败: %w", spec.command, err)
	}
	s := &server{cmd: cmd, stdin: stdin}
	m.servers[language] = s
	go m.readMessages(language, stdout)
	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		if m.servers[language] == s {
			delete(m.servers, language)
		}
		m.mu.Unlock()
	}()
	return nil
}

func (m *Manager) Send(language string, message json.RawMessage) error {
	m.mu.Lock()
	s := m.servers[language]
	m.mu.Unlock()
	if s == nil {
		return fmt.Errorf("LSP 服务未启动")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := fmt.Fprintf(s.stdin, "Content-Length: %d\r\n\r\n%s", len(message), message)
	return err
}

func (m *Manager) Stop(language string) {
	m.mu.Lock()
	s := m.servers[language]
	delete(m.servers, language)
	m.mu.Unlock()
	if s != nil {
		_ = s.stdin.Close()
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
	}
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	languages := make([]string, 0, len(m.servers))
	for language := range m.servers {
		languages = append(languages, language)
	}
	m.mu.Unlock()
	for _, language := range languages {
		m.Stop(language)
	}
}

func (m *Manager) readMessages(language string, reader io.Reader) {
	buffered := bufio.NewReader(reader)
	for {
		length, err := readContentLength(buffered)
		if err != nil {
			return
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(buffered, body); err != nil {
			return
		}
		if m.onMessage != nil {
			m.onMessage(language, body)
		}
	}
}

func readContentLength(reader *bufio.Reader) (int, error) {
	length := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return 0, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			if length < 0 {
				return 0, fmt.Errorf("LSP 消息缺少 Content-Length")
			}
			return length, nil
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "Content-Length") {
			continue
		}
		value, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || value < 0 {
			return 0, fmt.Errorf("无效的 Content-Length")
		}
		length = value
	}
}
