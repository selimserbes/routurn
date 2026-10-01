package bundle

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyAndRollback(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "existing.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "update.zip")
	f, _ := os.Create(archive)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("existing.txt")
	_, _ = w.Write([]byte("new"))
	w, _ = zw.Create("added.txt")
	_, _ = w.Write([]byte("added"))
	_ = zw.Close()
	_ = f.Close()

	plan, err := BuildPlan(root, archive, 0)
	if err != nil {
		t.Fatal(err)
	}
	record, err := Apply(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "existing.txt"))
	if string(data) != "new" {
		t.Fatalf("existing not updated: %q", data)
	}
	if _, err := os.Stat(filepath.Join(root, "added.txt")); err != nil {
		t.Fatal(err)
	}

	if _, err := Rollback(root, record.ID); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(root, "existing.txt"))
	if string(data) != "old" {
		t.Fatalf("existing not restored: %q", data)
	}
	if _, err := os.Stat(filepath.Join(root, "added.txt")); !os.IsNotExist(err) {
		t.Fatalf("added file still exists: %v", err)
	}
}

func TestPlanRejectsSymlinkedParent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	archive := filepath.Join(t.TempDir(), "update.zip")
	f, _ := os.Create(archive)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("linked/escape.txt")
	_, _ = w.Write([]byte("bad"))
	_ = zw.Close()
	_ = f.Close()
	if _, err := BuildPlan(root, archive, 0); err == nil {
		t.Fatal("expected symlinked parent rejection")
	}
}
