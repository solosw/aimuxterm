package lsp

import (
	"bufio"
	"strings"
	"testing"
)

func TestReadContentLength(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("Content-Type: application/vscode-jsonrpc; charset=utf-8\r\nContent-Length: 27\r\n\r\n"))
	length, err := readContentLength(reader)
	if err != nil {
		t.Fatalf("readContentLength returned error: %v", err)
	}
	if length != 27 {
		t.Fatalf("got length %d, want 27", length)
	}
}

func TestReadContentLengthRejectsMissingHeader(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("Content-Type: application/json\r\n\r\n"))
	if _, err := readContentLength(reader); err == nil {
		t.Fatal("expected error for missing Content-Length")
	}
}

func TestStatusForUnsupportedLanguage(t *testing.T) {
	info := NewManager(nil).Status("markdown")
	if info.Available || info.Message == "" {
		t.Fatalf("got %+v, want unavailable language with reason", info)
	}
}

func TestSpecForSupportedLanguages(t *testing.T) {
	cases := map[string]string{
		"vue":        "vue-language-server",
		"java":       "jdtls",
		"csharp":     "omnisharp",
		"php":        "intelephense",
		"lua":        "lua-language-server",
		"bash":       "bash-language-server",
		"yaml":       "yaml-language-server",
		"html":       "vscode-html-language-server",
		"css":        "vscode-css-language-server",
		"json":       "vscode-json-language-server",
		"dockerfile": "docker-langserver",
	}
	for language, command := range cases {
		spec, ok := specFor(language)
		if !ok || spec.command != command {
			t.Errorf("specFor(%q) = %+v, %v; want command %q", language, spec, ok, command)
		}
	}
}
