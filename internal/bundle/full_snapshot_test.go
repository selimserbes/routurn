package bundle

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSnapshotZIP(t *testing.T, files map[string]string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "snapshot.zip")
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for path, data := range files {
		x, err := w.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := x.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return name
}
func putSnapshotFile(t *testing.T, root, name, data string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
func mustSnapshotContents(t *testing.T, root, name, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil || string(data) != want {
		t.Fatalf("%s: got %q, %v; want %q", name, data, err, want)
	}
}
func mustNotExist(t *testing.T, root, name string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(name))); !os.IsNotExist(err) {
		t.Fatalf("%s should be absent, stat: %v", name, err)
	}
}
func fullSnapshotPlan(t *testing.T, root, archive string, excludes []string) Plan {
	t.Helper()
	p, err := BuildPlan(root, archive, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareFullSnapshot(root, &p, excludes); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestFullSnapshotTracksOnlyKnownFilesAndRollsBack(t *testing.T) {
	root := t.TempDir()
	putSnapshotFile(t, root, "legacy.py", "user-old")
	putSnapshotFile(t, root, "notes.txt", "personal")
	firstZIP := writeSnapshotZIP(t, map[string]string{"legacy.py": "managed", "keep.py": "first"})
	first := fullSnapshotPlan(t, root, firstZIP, nil)
	if len(first.Deleted) != 0 || first.SnapshotExisted {
		t.Fatalf("first snapshot should never delete unknown: %+v", first)
	}
	record1, err := Apply(root, first)
	if err != nil {
		t.Fatal(err)
	}
	mustSnapshotContents(t, root, "notes.txt", "personal")
	secondZIP := writeSnapshotZIP(t, map[string]string{"keep.py": "second", "new.py": "new"})
	second := fullSnapshotPlan(t, root, secondZIP, nil)
	if len(second.Deleted) != 1 || second.Deleted[0] != "legacy.py" {
		t.Fatalf("deleted paths: %v", second.Deleted)
	}
	record2, err := Apply(root, second)
	if err != nil {
		t.Fatal(err)
	}
	mustNotExist(t, root, "legacy.py")
	mustSnapshotContents(t, root, "keep.py", "second")
	mustSnapshotContents(t, root, "notes.txt", "personal")
	if _, err := Rollback(root, record2.ID); err != nil {
		t.Fatal(err)
	}
	mustSnapshotContents(t, root, "legacy.py", "managed")
	mustSnapshotContents(t, root, "keep.py", "first")
	mustNotExist(t, root, "new.py")
	// Rolling back the second snapshot reinstates the first ownership list.
	third := fullSnapshotPlan(t, root, secondZIP, nil)
	if len(third.Deleted) != 1 || third.Deleted[0] != "legacy.py" {
		t.Fatalf("ownership state not restored: %v", third.Deleted)
	}
	if _, err := Rollback(root, record1.ID); err != nil {
		t.Fatal(err)
	}
	mustSnapshotContents(t, root, "legacy.py", "user-old")
	mustNotExist(t, root, "keep.py")
	mustNotExist(t, root, snapshotPath)
}
func TestFullSnapshotFailsClosedWhenManagedFileChanged(t *testing.T) {
	root := t.TempDir()
	initial := writeSnapshotZIP(t, map[string]string{"old.py": "tracked", "keep.py": "keep"})
	if _, err := Apply(root, fullSnapshotPlan(t, root, initial, nil)); err != nil {
		t.Fatal(err)
	}
	putSnapshotFile(t, root, "old.py", "user-edited")
	candidate := writeSnapshotZIP(t, map[string]string{"keep.py": "next"})
	plan, err := BuildPlan(root, candidate, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareFullSnapshot(root, &plan, nil); err == nil {
		t.Fatal("refusing to delete edited file expected")
	}
	mustSnapshotContents(t, root, "old.py", "user-edited")
	mustSnapshotContents(t, root, "keep.py", "keep")
}
func TestFullSnapshotRefusesSymlinkForObsoleteFile(t *testing.T) {
	root := t.TempDir()
	initial := writeSnapshotZIP(t, map[string]string{"old.py": "tracked", "keep.py": "keep"})
	if _, err := Apply(root, fullSnapshotPlan(t, root, initial, nil)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "old.py")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "outside"), filepath.Join(root, "old.py")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	candidate := writeSnapshotZIP(t, map[string]string{"keep.py": "next"})
	plan, err := BuildPlan(root, candidate, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareFullSnapshot(root, &plan, nil); err == nil {
		t.Fatal("expected symlink rejection")
	}
}
func TestFullSnapshotPreconditionsCheckedAtApply(t *testing.T) {
	root := t.TempDir()
	initial := writeSnapshotZIP(t, map[string]string{"old.py": "tracked", "keep.py": "keep"})
	if _, err := Apply(root, fullSnapshotPlan(t, root, initial, nil)); err != nil {
		t.Fatal(err)
	}
	candidate := writeSnapshotZIP(t, map[string]string{"keep.py": "next"})
	plan := fullSnapshotPlan(t, root, candidate, nil)
	putSnapshotFile(t, root, "old.py", "changed-after-preview")
	if _, err := Apply(root, plan); err == nil {
		t.Fatal("expected late change rejection")
	}
	mustSnapshotContents(t, root, "old.py", "changed-after-preview")
	mustSnapshotContents(t, root, "keep.py", "keep")
}
func TestFullSnapshotProtectsExcludedPaths(t *testing.T) {
	root := t.TempDir()
	initial := writeSnapshotZIP(t, map[string]string{
		"src/old.py": "old", "src/keep.py": "keep", "node_modules/lib.js": "deps", "cache/cache.py": "cache", "routurn.toml": "keep", ".venv/local": "env",
	})
	if _, err := Apply(root, fullSnapshotPlan(t, root, initial, []string{"cache/**"})); err != nil {
		t.Fatal(err)
	}
	candidate := writeSnapshotZIP(t, map[string]string{"src/keep.py": "keep"})
	plan := fullSnapshotPlan(t, root, candidate, []string{"cache/**"})
	if len(plan.Deleted) != 1 || plan.Deleted[0] != "src/old.py" {
		t.Fatalf("unexpected delete list: %v", plan.Deleted)
	}
	if _, err := Apply(root, plan); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"node_modules/lib.js", "cache/cache.py", "routurn.toml", ".venv/local"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Fatalf("protected %s removed: %v", name, err)
		}
	}
}
func TestFullSnapshotRequiresNonemptyPayloadAndPreservesPartialUpdates(t *testing.T) {
	root := t.TempDir()
	initial := writeSnapshotZIP(t, map[string]string{"old.py": "tracked", "keep.py": "keep"})
	if _, err := Apply(root, fullSnapshotPlan(t, root, initial, nil)); err != nil {
		t.Fatal(err)
	}
	partial := writeSnapshotZIP(t, map[string]string{"keep.py": "partial"})
	plan, err := BuildPlan(root, partial, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Deleted) != 0 || plan.FullSnapshot {
		t.Fatal("partial update is unexpectedly pruning")
	}
	if _, err := Apply(root, plan); err != nil {
		t.Fatal(err)
	}
	mustSnapshotContents(t, root, "old.py", "tracked")
	empty := writeSnapshotZIP(t, map[string]string{})
	p, err := BuildPlan(root, empty, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareFullSnapshot(root, &p, nil); err == nil {
		t.Fatal("empty full snapshot should be rejected")
	}
}
func TestFullSnapshotDeletionOnlyAndNoOpInventory(t *testing.T) {
	root := t.TempDir()
	putSnapshotFile(t, root, "same.txt", "same")
	initial := writeSnapshotZIP(t, map[string]string{"same.txt": "same", "old.txt": "old"})
	first := fullSnapshotPlan(t, root, initial, nil)
	if _, err := Apply(root, first); err != nil {
		t.Fatal(err)
	}
	next := writeSnapshotZIP(t, map[string]string{"same.txt": "same"})
	plan := fullSnapshotPlan(t, root, next, nil)
	if len(plan.Added) != 0 || len(plan.Modified) != 0 || len(plan.Deleted) != 1 {
		t.Fatalf("deletion-only: %+v", plan)
	}
	r, err := Apply(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	mustNotExist(t, root, "old.txt")
	if _, err := Rollback(root, r.ID); err != nil {
		t.Fatal(err)
	}
	mustSnapshotContents(t, root, "old.txt", "old")
}
func TestFullSnapshotRefusesUnsafeTrackedPaths(t *testing.T) {
	root := t.TempDir()
	bad := `{"schema":1,"files":{"../outside.txt":"` + strings.Repeat("0", 64) + `"}}`
	if err := os.MkdirAll(filepath.Join(root, ".routurn"), 0o700); err != nil {
		t.Fatal(err)
	}
	putSnapshotFile(t, root, snapshotPath, bad)
	candidate := writeSnapshotZIP(t, map[string]string{"safe.txt": "hello"})
	p, err := BuildPlan(root, candidate, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareFullSnapshot(root, &p, nil); err == nil {
		t.Fatal("unsafe stored path accepted")
	}
}
