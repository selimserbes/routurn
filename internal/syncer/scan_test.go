package syncer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanAndDiff(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(rel, data string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite("main.go", "one")
	mustWrite("node_modules/skip.js", "skip")
	first, err := Scan(root, []string{"node_modules/**"})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Files) != 1 || first.Files[0].Path != "main.go" {
		t.Fatalf("unexpected first scan: %#v", first.Files)
	}
	prev := ToManifest(first)

	mustWrite("main.go", "two")
	mustWrite("pkg/new.go", "new")
	second, err := Scan(root, []string{"node_modules/**"})
	if err != nil {
		t.Fatal(err)
	}
	plan := Diff(prev, second)
	if len(plan.Changed) != 2 {
		t.Fatalf("expected 2 changed files, got %d", len(plan.Changed))
	}
	if len(plan.Deleted) != 0 {
		t.Fatalf("expected no deletions, got %#v", plan.Deleted)
	}

	if err := os.Remove(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}
	third, err := Scan(root, []string{"node_modules/**"})
	if err != nil {
		t.Fatal(err)
	}
	plan = Diff(ToManifest(second), third)
	if len(plan.Deleted) != 1 || plan.Deleted[0] != "main.go" {
		t.Fatalf("unexpected deletions: %#v", plan.Deleted)
	}
}
