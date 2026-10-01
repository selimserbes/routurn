package syncer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/selimserbes/routurn/internal/pathspec"
	resultstore "github.com/selimserbes/routurn/internal/result"
)

type FileEntry struct {
	Path string      `json:"path"`
	Hash string      `json:"hash"`
	Mode fs.FileMode `json:"-"`
	Size int64       `json:"-"`
}

type Manifest struct {
	Files map[string]string `json:"files"`
}

type ScanResult struct {
	Files []FileEntry
}

func Scan(root string, excludes []string) (*ScanResult, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	var files []FileEntry
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if rel == ".git" || strings.HasPrefix(rel, ".git/") || rel == ".routurn" || strings.HasPrefix(rel, ".routurn/") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() && (rel == "results" || rel == "routurn-results") && resultstore.IsManagedPublicRoot(path) {
			return filepath.SkipDir
		}
		if excluded(rel, excludes) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not supported yet: %s", rel)
		}
		if d.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file type: %s", rel)
		}
		if strings.ContainsRune(rel, '\n') || strings.ContainsRune(rel, '\r') {
			return fmt.Errorf("filenames containing newlines are not supported: %q", rel)
		}

		hash, err := hashFile(path)
		if err != nil {
			return err
		}
		files = append(files, FileEntry{Path: rel, Hash: hash, Mode: info.Mode(), Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return &ScanResult{Files: files}, nil
}

func excluded(rel string, patterns []string) bool {
	rel = filepath.ToSlash(rel)
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(filepath.ToSlash(pattern))
		if pattern == "" {
			continue
		}
		if pathspec.Match(pattern, rel) {
			return true
		}
	}
	return false
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func ToManifest(scan *ScanResult) Manifest {
	m := Manifest{Files: make(map[string]string, len(scan.Files))}
	for _, f := range scan.Files {
		m.Files[f.Path] = f.Hash
	}
	return m
}

type Plan struct {
	Changed []FileEntry
	Deleted []string
}

func Diff(previous Manifest, current *ScanResult) Plan {
	currentMap := make(map[string]FileEntry, len(current.Files))
	var changed []FileEntry
	for _, f := range current.Files {
		currentMap[f.Path] = f
		if oldHash, ok := previous.Files[f.Path]; !ok || oldHash != f.Hash {
			changed = append(changed, f)
		}
	}
	var deleted []string
	for path := range previous.Files {
		if _, ok := currentMap[path]; !ok {
			deleted = append(deleted, path)
		}
	}
	sort.Strings(deleted)
	return Plan{Changed: changed, Deleted: deleted}
}

func Fingerprint(scan *ScanResult) string {
	h := sha256.New()
	for _, f := range scan.Files {
		_, _ = io.WriteString(h, f.Path)
		_, _ = h.Write([]byte{0})
		_, _ = io.WriteString(h, f.Hash)
		_, _ = h.Write([]byte{'\n'})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
