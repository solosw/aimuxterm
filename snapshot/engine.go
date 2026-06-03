package snapshot

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const snapDir = ".warp-snapshots"

// Manifest maps relative file paths to their content hashes.
type Manifest struct {
	Files        map[string]string `json:"files"`
	Fingerprints map[string]string `json:"fingerprints,omitempty"`
}

// Engine manages file snapshots for a workspace.
type Engine struct {
	workspace string
	snapPath  string
	manifest  *Manifest
}

// NewEngine creates a snapshot engine for the given workspace.
func NewEngine(workspace string) *Engine {
	return &Engine{
		workspace: workspace,
		snapPath:  filepath.Join(workspace, snapDir),
		manifest: &Manifest{
			Files:        make(map[string]string),
			Fingerprints: make(map[string]string),
		},
	}
}

// Init creates the snapshot directory and saves initial snapshots for all files.
func (e *Engine) Init(files []string) error {
	if err := os.MkdirAll(e.snapPath, 0755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}
	for _, f := range files {
		if err := e.snapshotFile(f); err != nil {
			return fmt.Errorf("snapshot %s: %w", f, err)
		}
	}
	return e.saveManifest()
}

// AcceptAll re-snapshots all changed files.
func (e *Engine) AcceptAll(files []string) error {
	for _, f := range files {
		if err := e.snapshotFile(f); err != nil {
			// If file was deleted, remove from manifest
			if os.IsNotExist(err) {
				delete(e.manifest.Files, f)
				delete(e.manifest.Fingerprints, f)
				continue
			}
			return err
		}
	}
	return e.saveManifest()
}

// AcceptFile re-snapshots a single file.
func (e *Engine) AcceptFile(path string) error {
	if err := e.snapshotFile(path); err != nil {
		if os.IsNotExist(err) {
			delete(e.manifest.Files, path)
			delete(e.manifest.Fingerprints, path)
			return e.saveManifest()
		}
		return err
	}
	return e.saveManifest()
}

// RevertAll restores all files from their snapshots.
func (e *Engine) RevertAll(files []string) error {
	for _, f := range files {
		if err := e.RevertFile(f); err != nil {
			return err
		}
	}
	return nil
}

// RevertFile restores a single file from its snapshot.
func (e *Engine) RevertFile(path string) error {
	if _, ok := e.manifest.Files[path]; !ok {
		// File was newly created, delete it
		e.deleteFromManifest(path)
		return os.Remove(filepath.Join(e.workspace, path))
	}
	data, err := e.readSnapshotData(path)
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}
	targetPath := filepath.Join(e.workspace, path)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(targetPath, data, 0644)
}

// Diff returns the diff between current file and snapshot.
func (e *Engine) Diff(path string) (oldContent, newContent string, err error) {
	oldData, err := e.readSnapshotData(path)
	if err != nil {
		oldContent = "" // new file
	} else {
		oldContent = string(oldData)
	}
	newData, err := os.ReadFile(filepath.Join(e.workspace, path))
	if err != nil {
		newContent = "" // deleted file
	} else {
		newContent = string(newData)
	}
	return oldContent, newContent, nil
}

// HasSnapshot returns true if the engine has an existing manifest with files.
func (e *Engine) HasSnapshot() bool {
	return len(e.manifest.Files) > 0
}

// ChangedFiles returns files that differ from their snapshots, with line stats.
func (e *Engine) ChangedFiles(currentFiles []string) []FileChange {
	currentSet := make(map[string]bool, len(currentFiles))
	for _, f := range currentFiles {
		currentSet[f] = true
	}
	var changes []FileChange
	for _, f := range currentFiles {
		oldHash, existed := e.manifest.Files[f]
		if !existed {
			adds, _ := DiffStats(nil, nil)
			changes = append(changes, FileChange{Path: f, Status: StatusAdded, Additions: adds})
		} else {
			newHash := hashFile(filepath.Join(e.workspace, f))
			if newHash != oldHash {
				oldData, _ := e.readSnapshotData(f)
				newData, _ := os.ReadFile(filepath.Join(e.workspace, f))
				adds, dels := DiffStats(oldData, newData)
				changes = append(changes, FileChange{Path: f, Status: StatusModified, Additions: adds, Deletions: dels})
			}
		}
	}
	for f := range e.manifest.Files {
		if !currentSet[f] {
			oldData, _ := e.readSnapshotData(f)
			_, dels := DiffStats(oldData, nil)
			changes = append(changes, FileChange{Path: f, Status: StatusDeleted, Deletions: dels})
		}
	}
	return changes
}

// LoadManifest loads existing manifest from disk.
func (e *Engine) LoadManifest() error {
	data, err := os.ReadFile(filepath.Join(e.snapPath, "manifest.json"))
	if err != nil {
		if os.IsNotExist(err) {
			e.manifest = &Manifest{Files: make(map[string]string), Fingerprints: make(map[string]string)}
			return nil
		}
		return err
	}
	e.manifest = &Manifest{Files: make(map[string]string), Fingerprints: make(map[string]string)}
	return json.Unmarshal(data, e.manifest)
}

func (e *Engine) snapshotFile(relPath string) error {
	src := filepath.Join(e.workspace, relPath)
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if !IsTextFile(strings.ToLower(filepath.Ext(relPath)), FirstBytes(data)) {
		return fmt.Errorf("skip binary file")
	}
	h := hashBytes(data)
	if err := e.writeObject(h, data); err != nil {
		return err
	}
	e.manifest.Files[relPath] = h
	return nil
}

// FirstBytes returns the first 512 bytes of data for content-type detection.
func FirstBytes(data []byte) []byte {
	if len(data) > 512 {
		return data[:512]
	}
	return data
}

func (e *Engine) saveManifest() error {
	data, err := json.MarshalIndent(e.manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(e.snapPath, "manifest.json"), data, 0644)
}

func (e *Engine) objectsDir() string  { return filepath.Join(e.snapPath, "objects") }
func (e *Engine) objectPath(hash string) string {
	if len(hash) < 4 {
		return ""
	}
	return filepath.Join(e.objectsDir(), hash[:2], hash[2:])
}
func (e *Engine) readObject(hash string) ([]byte, error) {
	raw, err := os.ReadFile(e.objectPath(hash))
	if err != nil {
		return nil, err
	}
	return Decompress(raw), nil
}
func (e *Engine) writeObject(hash string, data []byte) error {
	objPath := e.objectPath(hash)
	if objPath == "" {
		return fmt.Errorf("invalid hash")
	}
	if _, err := os.Stat(objPath); err == nil {
		return nil // already stored, dedup
	}
	if err := os.MkdirAll(filepath.Dir(objPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(objPath, Compress(data), 0644)
}

// snapFilePath returns the old-style path-based snapshot location (for backward compat).
func (e *Engine) snapFilePath(relPath string) string {
	return filepath.Join(e.snapPath, relPath)
}

func (e *Engine) readSnapshotData(relPath string) ([]byte, error) {
	if hash, ok := e.manifest.Files[relPath]; ok && len(hash) >= 4 {
		if data, err := e.readObject(hash); err == nil {
			return data, nil
		}
	}
	// Fall back to old path-based layout
	return os.ReadFile(e.snapFilePath(relPath))
}

func (e *Engine) deleteFromManifest(path string) {
	delete(e.manifest.Files, path)
	delete(e.manifest.Fingerprints, path)
	e.saveManifest()
}

// FileChange types
const (
	StatusAdded    = "added"
	StatusModified = "modified"
	StatusDeleted  = "deleted"
)

type FileChange struct {
	Path      string `json:"path"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

func hashFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return hashBytes(data)
}

func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// HashBytes returns the hex SHA-256 of data (public).
func HashBytes(data []byte) string {
	return hashBytes(data)
}

// Common binary file extensions.
var binaryExts = map[string]bool{
	".exe": true, ".dll": true, ".so": true, ".dylib": true,
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".7z": true, ".rar": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".ico": true, ".webp": true, ".svg": true,
	".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".mkv": true, ".wmv": true, ".flv": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
	".o": true, ".obj": true, ".a": true, ".lib": true,
	".class": true, ".pyc": true, ".pyo": true,
	".jar": true, ".war": true, ".ear": true,
	".db": true, ".sqlite": true, ".sqlite3": true,
	".wasm": true,
}

// IsTextFile checks whether a file is a text file based on its extension and content.
// ext should be the lowercase file extension (e.g. ".go").
// peek should be the first up-to-512 bytes of the file (or nil if unavailable).
func IsTextFile(ext string, peek []byte) bool {
	if binaryExts[ext] {
		return false
	}
	if len(peek) > 0 {
		for _, b := range peek {
			if b == 0 {
				return false
			}
		}
	}
	return true
}

// DiffStats computes line additions and deletions between old and new content.
func DiffStats(oldData, newData []byte) (additions, deletions int) {
	oldLines := readLinesFromBytes(oldData)
	newLines := readLinesFromBytes(newData)
	oldCount := make(map[string]int, len(oldLines))
	for _, l := range oldLines {
		oldCount[l]++
	}
	newCount := make(map[string]int, len(newLines))
	for _, l := range newLines {
		newCount[l]++
	}
	for l, n := range newCount {
		o := oldCount[l]
		if n > o {
			additions += n - o
		}
	}
	for l, o := range oldCount {
		n := newCount[l]
		if o > n {
			deletions += o - n
		}
	}
	return
}

func readLinesFromBytes(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// ChangedFilesByHash compares current hashes against stored manifest without reading files.
func (e *Engine) ChangedFilesByHash(currentHashes map[string]string) []FileChange {
	// Use fingerpints for comparison if available (remote workspace), else files (local).
	ref := e.manifest.Files
	if len(e.manifest.Fingerprints) > 0 {
		ref = e.manifest.Fingerprints
	}
	currentSet := make(map[string]bool, len(currentHashes))
	for f := range currentHashes {
		currentSet[f] = true
	}
	var changes []FileChange
	for f, newHash := range currentHashes {
		oldHash, existed := ref[f]
		if !existed {
			changes = append(changes, FileChange{Path: f, Status: StatusAdded})
		} else if newHash != oldHash {
			changes = append(changes, FileChange{Path: f, Status: StatusModified})
		}
	}
	for f := range ref {
		if !currentSet[f] {
			changes = append(changes, FileChange{Path: f, Status: StatusDeleted})
		}
	}
	return changes
}

// SetFileHash sets a single file hash in the manifest in-memory (does not save).
func (e *Engine) SetFileHash(path, hash string) {
	e.manifest.Files[path] = hash
}

// GetFileHash returns the stored hash for a path.
func (e *Engine) GetFileHash(path string) (string, bool) {
	h, ok := e.manifest.Files[path]
	return h, ok
}

// GetFileFingerprint returns the stored fingerprint for a path.
func (e *Engine) GetFileFingerprint(path string) (string, bool) {
	if e.manifest.Fingerprints == nil {
		return "", false
	}
	fp, ok := e.manifest.Fingerprints[path]
	return fp, ok
}

// SetFileFingerprint sets the fingerprint for a file (used by remote workspaces).
func (e *Engine) SetFileFingerprint(path, fp string) {
	if e.manifest.Fingerprints == nil {
		e.manifest.Fingerprints = make(map[string]string)
	}
	e.manifest.Fingerprints[path] = fp
}

// LoadManifestFrom parses manifest JSON from bytes.
func (e *Engine) LoadManifestFrom(data []byte) error {
	e.manifest = &Manifest{}
	return json.Unmarshal(data, e.manifest)
}

// MarshalManifest returns the manifest as JSON bytes.
func (e *Engine) MarshalManifest() ([]byte, error) {
	return json.MarshalIndent(e.manifest, "", "  ")
}

// RemoveFromManifest deletes entries from manifest (for remote revert).
func (e *Engine) RemoveFromManifest(paths []string) error {
	for _, path := range paths {
		delete(e.manifest.Files, path)
		delete(e.manifest.Fingerprints, path)
	}
	return e.saveManifest()
}

// FilterManifest removes entries that don't match the keep predicate.
func (e *Engine) FilterManifest(keep func(path string) bool) {
	cleaned := make(map[string]string, len(e.manifest.Files))
	for p, h := range e.manifest.Files {
		if keep(p) {
			cleaned[p] = h
		}
	}
	e.manifest.Files = cleaned
	if e.manifest.Fingerprints != nil {
		cleanedFp := make(map[string]string, len(e.manifest.Fingerprints))
		for p, fp := range e.manifest.Fingerprints {
			if keep(p) {
				cleanedFp[p] = fp
			}
		}
		e.manifest.Fingerprints = cleanedFp
	}
	e.saveManifest()
}

// ReadFileContent reads a file from workspace.
func ReadFileContent(workspace, relPath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(workspace, relPath))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Compress gzip-compresses data.
func Compress(data []byte) []byte {
	var buf bytes.Buffer
	w, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	w.Write(data)
	w.Close()
	return buf.Bytes()
}

// Decompress tries gzip decompression; returns raw data if not compressed.
func Decompress(data []byte) []byte {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return data
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return data
	}
	return out
}
