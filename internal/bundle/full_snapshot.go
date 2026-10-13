package bundle

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/selimserbes/routurn/internal/pathspec"
)

// Full snapshots are explicitly opted into; old partial updates never prune files.
// Only paths recorded by an earlier full snapshot are eligible for removal.
const snapshotPath = ".routurn/full-snapshot.json"

type snapshotState struct {
	Schema int               `json:"schema"`
	Files  map[string]string `json:"files"`
}

func protectedSnapshotPath(rel string, excludes []string) bool {
	rel = filepath.ToSlash(rel)
	if rel == "routurn.toml" || rel == "routurn.example.toml" {
		return true
	}
	segments := strings.Split(rel, "/")
	for _, segment := range segments {
		switch segment {
		case ".git", ".routurn", ".venv", "venv", "node_modules", "target", "__pycache__", ".tox", ".mypy_cache", ".pytest_cache":
			return true
		}
	}
	for _, pattern := range excludes {
		pattern = strings.TrimSpace(pattern)
		if pattern != "" && pathspec.Match(pattern, rel) {
			return true
		}
		// A directory exclude may match the folder rather than the child.
		for i := 1; i < len(segments); i++ {
			if pattern != "" && pathspec.Match(pattern, strings.Join(segments[:i], "/")) {
				return true
			}
		}
	}
	return false
}

func loadSnapshot(root string) (snapshotState, []byte, bool, error) {
	target, err := safeLocalTarget(root, snapshotPath)
	if err != nil {
		return snapshotState{}, nil, false, err
	}
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		return snapshotState{Schema: 1, Files: map[string]string{}}, nil, false, nil
	}
	if err != nil {
		return snapshotState{}, nil, false, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4<<20 {
		return snapshotState{}, nil, false, fmt.Errorf("invalid full-snapshot state")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return snapshotState{}, nil, false, err
	}
	var state snapshotState
	if err := json.Unmarshal(data, &state); err != nil {
		return snapshotState{}, nil, false, fmt.Errorf("read snapshot state: %w", err)
	}
	if state.Schema != 1 || state.Files == nil || len(state.Files) > MaxEntries {
		return snapshotState{}, nil, false, fmt.Errorf("invalid snapshot schema or file count")
	}
	for rel, hash := range state.Files {
		if err := validateSnapshotKey(rel, hash); err != nil {
			return snapshotState{}, nil, false, err
		}
	}
	return state, data, true, nil
}

func validateSnapshotKey(rel, hash string) error {
	normalized, skip, err := normalizeArchivePath(rel, 0)
	if err != nil || skip || normalized != rel {
		return fmt.Errorf("invalid snapshot path: %q", rel)
	}
	if len(hash) != 64 {
		return fmt.Errorf("invalid snapshot digest for %q", rel)
	}
	b, err := hex.DecodeString(hash)
	if err != nil || len(b) != sha256.Size {
		return fmt.Errorf("invalid snapshot digest for %q", rel)
	}
	return nil
}

// PrepareFullSnapshot marks a plan as a complete snapshot. The first snapshot
// only establishes ownership; pre-existing unknown files are not deleted.
// Later snapshots can remove an absent file ONLY when its content still
// matches the hash that Routurn recorded during an earlier full snapshot.
func PrepareFullSnapshot(root string, plan *Plan, excludes []string) error {
	if len(plan.Entries) == 0 {
		return fmt.Errorf("a full snapshot must contain at least one file")
	}
	previous, original, existed, err := loadSnapshot(root)
	if err != nil {
		return err
	}
	next := make(map[string]string, len(plan.Entries))
	for _, entry := range plan.Entries {
		if protectedSnapshotPath(entry.Path, excludes) {
			continue
		}
		if err := validateSnapshotKey(entry.Path, strings.Repeat("0", 64)); err != nil {
			return err
		}
		hash := sha256.Sum256(entry.Data)
		next[entry.Path] = hex.EncodeToString(hash[:])
	}
	if len(next) == 0 {
		return fmt.Errorf("full snapshot has no manageable source files")
	}
	deleted := []string{}
	hashes := map[string]string{}
	for rel, previousHash := range previous.Files {
		if _, ok := next[rel]; ok || protectedSnapshotPath(rel, excludes) {
			continue
		}
		full, err := safeLocalTarget(root, rel)
		if err != nil {
			return err
		}
		info, err := os.Lstat(full)
		if os.IsNotExist(err) {
			continue
		} // already removed elsewhere
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("obsolete managed file is not regular: %s", rel)
		}
		if err := checkDeleteHash(root, rel, previousHash); err != nil {
			return err
		}
		deleted = append(deleted, rel)
		hashes[rel] = previousHash
	}
	sort.Strings(deleted)
	plan.FullSnapshot = true
	plan.SnapshotExisted = existed
	plan.SnapshotBefore = original
	plan.SnapshotFiles = next
	plan.Deleted = deleted
	plan.DeleteHashes = hashes
	return nil
}

func checkDeleteHash(root, rel, want string) error {
	full, err := safeLocalTarget(root, rel)
	if err != nil {
		return err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return err
	}
	actual := sha256.Sum256(content)
	if hex.EncodeToString(actual[:]) != want {
		return fmt.Errorf("file changed after being managed; refusing to delete")
	}
	return nil
}

func checkSnapshotPreconditions(root string, plan Plan) error {
	_, data, exists, err := loadSnapshot(root)
	if err != nil {
		return err
	}
	if exists != plan.SnapshotExisted || !bytes.Equal(data, plan.SnapshotBefore) {
		return fmt.Errorf("snapshot tracking changed since plan was built")
	}
	for rel, hash := range plan.DeleteHashes {
		if err := checkDeleteHash(root, rel, hash); err != nil {
			return fmt.Errorf("obsolete %s: %w", rel, err)
		}
	}
	return nil
}

func atomicSnapshotWrite(root string, data []byte) error {
	target := filepath.Join(root, filepath.FromSlash(snapshotPath))
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	// Never overwrite a user-supplied symlink or special file.
	info, err := os.Lstat(target)
	if err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("snapshot path is not a regular file")
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(target), ".snapshot-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), target)
}

func saveSnapshot(root string, files map[string]string) error {
	data, err := json.MarshalIndent(snapshotState{Schema: 1, Files: files}, "", "  ")
	if err != nil {
		return err
	}
	return atomicSnapshotWrite(root, append(data, '\n'))
}

func restoreSnapshot(root, updateDir string, record Record) error {
	if !record.FullSnapshot {
		return nil
	}
	target := filepath.Join(root, filepath.FromSlash(snapshotPath))
	if !record.HadSnapshot {
		info, err := os.Lstat(target)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to remove non-regular snapshot state")
		}
		return os.Remove(target)
	}
	previous, err := os.ReadFile(filepath.Join(updateDir, "previous-snapshot.json"))
	if err != nil {
		return err
	}
	return atomicSnapshotWrite(root, previous)
}

// Remove only empty directories previously occupied by a deleted managed file.
// Never remove the project root or follow symlinks. Nonempty directories and
// untracked sibling files are always preserved.
func removeEmptyAncestors(root, rel string) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return
	}
	dir := filepath.Dir(filepath.Join(rootAbs, filepath.FromSlash(rel)))
	for dir != rootAbs && dir != "." {
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
