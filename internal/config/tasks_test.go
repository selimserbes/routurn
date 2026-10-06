package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalTasksMergeAndOverride(t *testing.T) {
	root := t.TempDir()
	project := `version = 1
name = "demo"

[remote]
target = "lab"
path = "/srv/demo"

[sync]
exclude = [".routurn/**"]

[tasks.test]
command = "go test ./..."
`
	if err := os.WriteFile(filepath.Join(root, ProjectFileName), []byte(project), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveLocalTask(root, "dev", Task{Command: "go run ."}); err != nil {
		t.Fatal(err)
	}
	if err := SaveLocalTask(root, "test", Task{Command: "go test -race ./..."}); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Tasks["dev"].Command; got != "go run ." {
		t.Fatalf("dev command = %q", got)
	}
	if got := cfg.Tasks["test"].Command; got != "go test -race ./..." {
		t.Fatalf("override command = %q", got)
	}
}

func TestRemoveLocalTask(t *testing.T) {
	root := t.TempDir()
	if err := SaveLocalTask(root, "dev", Task{Command: "go run ."}); err != nil {
		t.Fatal(err)
	}
	removed, err := RemoveLocalTask(root, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("expected task removal")
	}
	tasks, err := LoadLocalTasks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected no local tasks, got %v", tasks)
	}
}
