package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"just-warp-go/snapshot"
)

// skipDirs are directories always skipped during scanning.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, ".warp-snapshots": true,
	"dist": true, "build": true, ".next": true, "__pycache__": true,
	"target": true, ".cache": true, "vendor": true,
}

// ScanResult holds the result of a workspace scan.
type ScanResult struct {
	Files []string `json:"files"` // relative paths of text files
}

// Scan recursively scans a workspace directory, respecting .gitignore and skipping binary files.
func Scan(workspace string) (*ScanResult, error) {
	ignore := loadGitignore(workspace)
	var files []string

	err := filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible files
		}
		if info.IsDir() {
			if skipDirs[info.Name()] || strings.HasPrefix(info.Name(), ".") && info.Name() != ".gitignore" {
				return filepath.SkipDir
			}
			return nil
		}
		relPath, _ := filepath.Rel(workspace, path)
		if relPath == "" {
			return nil
		}
		if ignore.Match(relPath) {
			return nil
		}
		if isBinaryPath(path) {
			return nil
		}
		if info.Size() > 5*1024*1024 {
			return nil
		}
		files = append(files, relPath)
		return nil
	})

	return &ScanResult{Files: files}, err
}

// gitignore is a simple .gitignore rule matcher.
type gitignore struct {
	patterns []pattern
}

type pattern struct {
	negate  bool
	dirOnly bool
	glob    string
}

func loadGitignore(workspace string) *gitignore {
	f, err := os.Open(filepath.Join(workspace, ".gitignore"))
	if err != nil {
		return &gitignore{}
	}
	defer f.Close()
	gi := &gitignore{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p := pattern{}
		if strings.HasPrefix(line, "!") {
			p.negate = true
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			p.dirOnly = true
			line = line[:len(line)-1]
		}
		p.glob = line
		gi.patterns = append(gi.patterns, p)
	}
	return gi
}

func (gi *gitignore) Match(path string) bool {
	if len(gi.patterns) == 0 {
		return false
	}
	ignored := false
	for _, p := range gi.patterns {
		if p.dirOnly {
			continue // simple impl: skip directory-only patterns for now
		}
		matched := matchGlob(p.glob, path)
		if matched {
			ignored = !p.negate
		}
	}
	return ignored
}

func matchGlob(pattern, path string) bool {
	// Handle ** patterns
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		rest := path
		for i, part := range parts {
			part = strings.Trim(part, "/")
			if part == "" {
				continue
			}
			idx := strings.Index(rest, part)
			if idx < 0 {
				return false
			}
			if i == 0 && !strings.HasPrefix(path, part) && !strings.HasPrefix(pattern, "**") {
				return false
			}
			rest = rest[idx+len(part):]
		}
		return true
	}
	// Simple filepath.Match for basic patterns
	matched, _ := filepath.Match(pattern, filepath.Base(path))
	if matched {
		return true
	}
	// Also try matching against full path
	matched, _ = filepath.Match(pattern, path)
	return matched
}

func isBinaryPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	f, err := os.Open(path)
	if err != nil {
		return true
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	return !snapshot.IsTextFile(ext, buf[:n])
}
