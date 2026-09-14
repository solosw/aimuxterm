package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeWorkspaceRelPath(t *testing.T) {
	ok, err := sanitizeWorkspaceRelPath(`docs\logo.png`)
	if err != nil || ok != "docs/logo.png" {
		t.Fatalf("got %q %v", ok, err)
	}
	if _, err := sanitizeWorkspaceRelPath("../secret.png"); err == nil {
		t.Fatal("expected reject traversal")
	}
	if _, err := sanitizeWorkspaceRelPath("/etc/passwd"); err == nil {
		t.Fatal("expected reject absolute")
	}
	if _, err := sanitizeWorkspaceRelPath("C:/windows/x.png"); err == nil {
		t.Fatal("expected reject drive")
	}
}

func TestPreviewMIME(t *testing.T) {
	if mime, ok := previewMIME("a/b/photo.PNG"); !ok || mime != "image/png" {
		t.Fatalf("png mime=%q ok=%v", mime, ok)
	}
	if mime, ok := previewMIME("report.pdf"); !ok || mime != "application/pdf" {
		t.Fatalf("pdf mime=%q ok=%v", mime, ok)
	}
	if _, ok := previewMIME("main.go"); ok {
		t.Fatal("go should not be preview binary")
	}
}

func TestGetFilePreviewDataImage(t *testing.T) {
	root := t.TempDir()
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3}
	if err := os.WriteFile(filepath.Join(root, "logo.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{workspace: root}
	url, err := a.GetFilePreviewData("logo.png")
	if err != nil {
		t.Fatal(err)
	}
	prefix := "data:image/png;base64,"
	if !strings.HasPrefix(url, prefix) {
		t.Fatalf("url=%q", url)
	}
	got, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, prefix))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(png) {
		t.Fatalf("decoded mismatch")
	}
}

func TestGetFilePreviewDataRejectsUnknown(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.bin"), []byte{0, 1, 2}, 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{workspace: root}
	if _, err := a.GetFilePreviewData("a.bin"); err == nil {
		t.Fatal("expected reject")
	}
}

func TestParseFileURLPath(t *testing.T) {
	p, err := parseFileURLPath("file:///C:/temp/logo.png")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Clean(p), filepath.Clean(`C:\temp\logo.png`)) &&
		!strings.EqualFold(filepath.Clean(p), filepath.Clean(`C:/temp/logo.png`)) {
		// Accept either slash style from Clean.
		got := filepath.ToSlash(p)
		if !strings.EqualFold(got, "C:/temp/logo.png") {
			t.Fatalf("got %q", p)
		}
	}
}

func TestGetImagePreviewDataAbsolute(t *testing.T) {
	root := t.TempDir()
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 9, 8, 7}
	full := filepath.Join(root, "abs.png")
	if err := os.WriteFile(full, png, 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	url, err := a.GetImagePreviewData(full)
	if err != nil {
		t.Fatal(err)
	}
	prefix := "data:image/png;base64,"
	if !strings.HasPrefix(url, prefix) {
		t.Fatalf("url=%q", url)
	}
	got, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, prefix))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(png) {
		t.Fatal("decoded mismatch")
	}
}

func TestGetImagePreviewDataRelative(t *testing.T) {
	root := t.TempDir()
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 4, 5}
	if err := os.WriteFile(filepath.Join(root, "rel.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{workspace: root}
	url, err := a.GetImagePreviewData("./rel.png")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Fatalf("url=%q", url)
	}
}

func TestGetImagePreviewDataFileURL(t *testing.T) {
	root := t.TempDir()
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 1}
	full := filepath.Join(root, "via-file.png")
	if err := os.WriteFile(full, png, 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	fileURL := "file:///" + filepath.ToSlash(full)
	if filepath.IsAbs(full) && len(full) >= 2 && full[1] == ':' {
		fileURL = "file:///" + strings.ReplaceAll(filepath.ToSlash(full), " ", "%20")
	}
	url, err := a.GetImagePreviewData(fileURL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Fatalf("url=%q from %q", url, fileURL)
	}
}

func TestIsAbsoluteImagePath(t *testing.T) {
	if !isAbsoluteImagePath(`C:\x\a.png`) {
		t.Fatal("windows drive")
	}
	if !isAbsoluteImagePath(`/tmp/a.png`) && filepath.Separator == '/' {
		t.Fatal("unix abs")
	}
	if isAbsoluteImagePath(`docs/a.png`) {
		t.Fatal("relative should be false")
	}
}
