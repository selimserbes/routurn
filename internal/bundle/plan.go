package bundle

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func BuildPlan(root, archive string, strip int) (Plan, error) {
	entries, err := ReadArchive(archive, strip)
	if err != nil {
		return Plan{}, err
	}
	p := Plan{Archive: filepath.Base(archive), Entries: entries}
	for _, entry := range entries {
		target, err := safeLocalTarget(root, entry.Path)
		if err != nil {
			return Plan{}, err
		}
		info, statErr := os.Lstat(target)
		if statErr == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return Plan{}, fmt.Errorf("refusing to overwrite symlink: %s", entry.Path)
			}
			if !info.Mode().IsRegular() {
				return Plan{}, fmt.Errorf("refusing to overwrite non-regular file: %s", entry.Path)
			}
		} else if !os.IsNotExist(statErr) {
			return Plan{}, fmt.Errorf("inspect local %s: %w", entry.Path, statErr)
		}
		data, err := os.ReadFile(target)
		switch {
		case os.IsNotExist(err):
			p.Added = append(p.Added, entry.Path)
		case err != nil:
			return Plan{}, fmt.Errorf("read local %s: %w", entry.Path, err)
		case bytes.Equal(data, entry.Data):
			p.Unchanged = append(p.Unchanged, entry.Path)
		default:
			p.Modified = append(p.Modified, entry.Path)
		}
	}
	sort.Strings(p.Added)
	sort.Strings(p.Modified)
	sort.Strings(p.Unchanged)
	return p, nil
}

func safeLocalTarget(root, rel string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe local target: %q", rel)
	}
	parts := strings.Split(clean, string(filepath.Separator))
	cur := rootAbs
	for i, part := range parts {
		cur = filepath.Join(cur, part)
		if i == len(parts)-1 {
			break
		}
		info, err := os.Lstat(cur)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("refusing archive path through symlinked directory: %s", filepath.ToSlash(strings.Join(parts[:i+1], "/")))
		}
		if !info.IsDir() {
			return "", fmt.Errorf("archive parent is not a directory: %s", filepath.ToSlash(strings.Join(parts[:i+1], "/")))
		}
	}
	return filepath.Join(rootAbs, clean), nil
}
