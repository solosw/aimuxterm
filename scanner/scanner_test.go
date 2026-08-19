package scanner

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeFile(t *testing.T, root, rel string, data []byte) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func TestScanSeparatesTextAndNonTextFiles(t *testing.T) {
	root := t.TempDir()

	writeFile(t, root, "main.go", []byte("package main\n"))
	writeFile(t, root, "src/app.ts", []byte("export const a = 1\n"))
	// binary by extension
	writeFile(t, root, "logo.png", []byte("\x89PNG\r\n\x1a\n"))
	// binary by content (NUL byte) despite a text-ish extension
	writeFile(t, root, "data.txt", []byte("abc\x00def"))
	// oversized text file
	writeFile(t, root, "huge.log", make([]byte, maxTextSize+1))

	res, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	wantText := []string{filepath.FromSlash("main.go"), filepath.FromSlash("src/app.ts")}
	for _, w := range wantText {
		if !slices.Contains(res.Files, w) {
			t.Errorf("Files missing %q; got %v", w, res.Files)
		}
	}

	wantOther := []string{
		filepath.FromSlash("logo.png"),
		filepath.FromSlash("data.txt"),
		filepath.FromSlash("huge.log"),
	}
	for _, w := range wantOther {
		if !slices.Contains(res.OtherFiles, w) {
			t.Errorf("OtherFiles missing %q; got %v", w, res.OtherFiles)
		}
		if slices.Contains(res.Files, w) {
			t.Errorf("%q must not be snapshot-tracked in Files", w)
		}
	}
}

func TestScanKeepsGitignoredFilesOutOfBothLists(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".gitignore", []byte("secret.txt\nblobs/\n"))
	writeFile(t, root, "keep.go", []byte("package main\n"))
	writeFile(t, root, "secret.txt", []byte("token\n"))
	writeFile(t, root, "blobs/pic.png", []byte("\x89PNG\r\n\x1a\n"))

	res, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	all := slices.Concat(res.Files, res.OtherFiles)
	for _, unwanted := range []string{"secret.txt", filepath.FromSlash("blobs/pic.png")} {
		if slices.Contains(all, unwanted) {
			t.Errorf("gitignored %q should be excluded; got %v", unwanted, all)
		}
	}
	if !slices.Contains(res.Files, "keep.go") {
		t.Errorf("keep.go should be tracked; got %v", res.Files)
	}
}

func TestScanListsEmptyDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "empty", "nested"), 0755); err != nil {
		t.Fatal(err)
	}

	res, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"empty", "empty/nested"} {
		if !slices.Contains(res.Directories, want) {
			t.Errorf("directories %v do not include %q", res.Directories, want)
		}
	}
}
