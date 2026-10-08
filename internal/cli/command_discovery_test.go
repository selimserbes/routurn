package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/selimserbes/routurn/internal/config"
)

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commandsBySource(items []runnableCommand, source string) map[string]string {
	out := map[string]string{}
	for _, item := range items {
		if item.Source == source {
			out[item.Name] = item.Command
		}
	}
	return out
}

func TestDiscoverNodeCommands(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "package.json", `{"scripts":{"dev":"vite","build":"vite build"}}`)
	writeTestFile(t, root, "pnpm-lock.yaml", "lockfileVersion: 9\n")
	got := commandsBySource(discoverRunnableCommands(root, nil), "Node")
	if got["dev"] != "pnpm run dev" || got["build"] != "pnpm run build" {
		t.Fatalf("unexpected Node commands: %#v", got)
	}
}

func TestDiscoverGoCommands(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/demo\n\ngo 1.22\n")
	writeTestFile(t, root, "cmd/server/main.go", "package main\nfunc main() {}\n")
	got := commandsBySource(discoverRunnableCommands(root, nil), "Go")
	if got["server"] != "go run ./cmd/server" {
		t.Fatalf("unexpected Go commands: %#v", got)
	}
	if got["test"] != "go test ./..." {
		t.Fatalf("missing Go test command: %#v", got)
	}
}

func TestDiscoverRustCommands(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "Cargo.toml", "[package]\nname='demo'\nversion='0.1.0'\n\n[[bin]]\nname='worker'\npath='src/worker.rs'\n")
	got := commandsBySource(discoverRunnableCommands(root, nil), "Rust")
	if got["worker"] != "cargo run --bin worker" {
		t.Fatalf("unexpected Rust commands: %#v", got)
	}
	if got["test"] != "cargo test" {
		t.Fatalf("missing cargo test: %#v", got)
	}
}

func TestDiscoverPythonMakeAndCompose(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "pyproject.toml", "[project]\nname='demo'\nversion='0.1.0'\n[project.scripts]\nserve='demo.cli:main'\n")
	writeTestFile(t, root, "uv.lock", "version = 1\n")
	writeTestFile(t, root, "Makefile", "build:\n\t@echo build\n\ntest:\n\t@echo test\n")
	writeTestFile(t, root, "compose.yaml", "services: {}\n")
	items := discoverRunnableCommands(root, nil)
	py := commandsBySource(items, "Python")
	if py["serve"] != "uv run serve" {
		t.Fatalf("unexpected Python commands: %#v", py)
	}
	makeCmds := commandsBySource(items, "Make")
	if makeCmds["build"] != "make build" {
		t.Fatalf("unexpected Make commands: %#v", makeCmds)
	}
	docker := commandsBySource(items, "Docker")
	if docker["compose-up"] != "docker compose up" {
		t.Fatalf("unexpected Docker commands: %#v", docker)
	}
}

func TestSavedTasksComeFirstAndDeduplicate(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/demo\n\ngo 1.22\n")
	items := discoverRunnableCommands(root, map[string]config.Task{
		"verify": {Command: "go test ./..."},
	})
	if len(items) == 0 || items[0].Name != "verify" || !items[0].Saved {
		t.Fatalf("saved task should be first: %#v", items)
	}
	count := 0
	for _, item := range items {
		if item.Command == "go test ./..." {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected command deduplication, got %d", count)
	}
}

func TestDiscoverExecutableCommandsWithoutScriptsConvention(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "tools/train_model.sh", "#!/usr/bin/env bash\necho train\n")
	if err := os.Chmod(filepath.Join(root, "tools/train_model.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := commandsBySource(discoverRunnableCommands(root, nil), "Project commands")
	if got["train_model"] != "bash tools/train_model.sh" {
		t.Fatalf("unexpected executable commands: %#v", got)
	}
}

func TestShellJoin(t *testing.T) {
	got := shellJoin([]string{"echo", "hello world", "it's"})
	want := `echo 'hello world' 'it'"'"'s'`
	if got != want {
		t.Fatalf("shellJoin() = %q, want %q", got, want)
	}
}

func TestSavedTaskHidesMatchingProjectEntrypointByDefault(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "ops/train.sh", "#!/usr/bin/env bash\necho train\n")
	if err := os.Chmod(filepath.Join(root, "ops/train.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	items := discoverRunnableCommands(root, map[string]config.Task{
		"train": {Command: "export FOO=1\nexec ./ops/train.sh\n"},
	})
	visible := defaultRunnableCommands(items)
	for _, item := range visible {
		if item.ProjectPath == "ops/train.sh" {
			t.Fatalf("matching project entrypoint should be hidden from default menu: %#v", visible)
		}
	}
	foundHidden := false
	for _, item := range items {
		if item.ProjectPath == "ops/train.sh" {
			foundHidden = item.Hidden && strings.Contains(item.HiddenReason, "saved task train")
		}
	}
	if !foundHidden {
		t.Fatalf("expected hidden duplicate entrypoint: %#v", items)
	}
}

func TestMaintenanceHelpersHiddenButStillDiscoverable(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "tools/install_remote_tasks.py", "#!/usr/bin/env python3\nprint('install')\n")
	if err := os.Chmod(filepath.Join(root, "tools/install_remote_tasks.py"), 0o755); err != nil {
		t.Fatal(err)
	}
	items := discoverRunnableCommands(root, nil)
	if !hasHiddenRunnableCommands(items) {
		t.Fatalf("expected hidden helper: %#v", items)
	}
	for _, item := range defaultRunnableCommands(items) {
		if item.ProjectPath == "tools/install_remote_tasks.py" {
			t.Fatalf("maintenance helper leaked into default choices: %#v", item)
		}
	}
}

func TestNonExecutableShebangEntrypointIsDiscovered(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "tools/run_worker.py", "#!/usr/bin/env python3\nprint('worker')\n")
	items := discoverRunnableCommands(root, nil)
	got := commandsBySource(items, "Project commands")
	if got["worker"] != "python3 tools/run_worker.py" {
		t.Fatalf("unexpected non-executable shebang command: %#v", got)
	}
}

func TestNamedEntrypointWithoutExecutableBitOrShebangIsDiscovered(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "tools/run_seed_check.sh", "echo seed\n")
	items := discoverRunnableCommands(root, nil)
	got := commandsBySource(items, "Project commands")
	if got["seed_check"] != "bash tools/run_seed_check.sh" {
		t.Fatalf("unexpected named entrypoint command: %#v", got)
	}
}

func TestSeedCheckRanksAheadOfTrainEntrypoints(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "scripts/run_stage3a_train.sh", "#!/usr/bin/env bash\n")
	writeTestFile(t, root, "scripts/run_stage3a_seed_check.sh", "#!/usr/bin/env bash\n")
	for _, rel := range []string{"scripts/run_stage3a_train.sh", "scripts/run_stage3a_seed_check.sh"} {
		if err := os.Chmod(filepath.Join(root, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	items := defaultRunnableCommands(discoverRunnableCommands(root, nil))
	var project []runnableCommand
	for _, item := range items {
		if item.Source == "Project commands" {
			project = append(project, item)
		}
	}
	if len(project) < 2 || project[0].Name != "stage3a_seed_check" {
		t.Fatalf("seed check should rank first, got %#v", project)
	}
}
