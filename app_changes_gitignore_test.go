package main

import (
	"os"
	"path/filepath"
	"testing"

	"aimuxterm/scanner"
	"aimuxterm/snapshot"
)

func TestFilterIgnoredFileChangesHidesGitignoredDeleted(t *testing.T) {
	ignore := scanner.ParseGitignore("build/\n*.log\n")
	changes := []snapshot.FileChange{
		{Path: "main.go", Status: snapshot.StatusModified},
		{Path: "build/out.bin", Status: snapshot.StatusDeleted},
		{Path: "debug.log", Status: snapshot.StatusDeleted},
		{Path: "src/app.ts", Status: snapshot.StatusAdded},
		{Path: ".gitignore", Status: snapshot.StatusModified},
	}
	got := filterIgnoredFileChanges(changes, ignore)
	wantKeep := map[string]bool{
		"main.go":    true,
		"src/app.ts": true,
		".gitignore": true,
	}
	if len(got) != len(wantKeep) {
		t.Fatalf("len=%d got=%v", len(got), got)
	}
	for _, c := range got {
		if !wantKeep[c.Path] {
			t.Errorf("unexpected kept %q", c.Path)
		}
		delete(wantKeep, c.Path)
	}
	for p := range wantKeep {
		t.Errorf("missing %q", p)
	}
}

func TestChangedFilesShowsDeletedUntilGitignoreFilters(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("keep.go", "package keep\n")
	write("gone.log", "noise\n")
	write(".gitignore", "")

	eng := snapshot.NewEngine(root)
	if err := eng.Init([]string{"keep.go", "gone.log", ".gitignore"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "gone.log")); err != nil {
		t.Fatal(err)
	}
	before := eng.ChangedFiles([]string{"keep.go", ".gitignore"})
	foundDeleted := false
	for _, c := range before {
		if c.Path == "gone.log" && c.Status == snapshot.StatusDeleted {
			foundDeleted = true
		}
	}
	if !foundDeleted {
		t.Fatalf("expected gone.log deleted before ignore; got %#v", before)
	}

	write(".gitignore", "*.log\n")
	ignore := loadWorkspaceGitignore(root)
	after := filterIgnoredFileChanges(eng.ChangedFiles([]string{"keep.go", ".gitignore"}), ignore)
	for _, c := range after {
		if c.Path == "gone.log" {
			t.Fatalf("gone.log should be filtered after gitignore; got %#v", after)
		}
	}
}
