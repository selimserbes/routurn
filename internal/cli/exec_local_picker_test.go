package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An auto SSH route must not be resolved just to show (or cancel) the task
// picker. The deliberately unregistered target would fail if probed early.
func TestExecPickerCancelDoesNotResolveRemote(t *testing.T) {
	root := t.TempDir()
	projectFile := `version = 1
name = "local-picker-test"

[remote]
target = "unregistered-target"
path = "/remote/work"

[tasks.hello]
command = "echo hello"
`
	if err := os.WriteFile(filepath.Join(root, "routurn.toml"), []byte(projectFile), 0o644); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	oldProject := projectName
	projectName = ""
	defer func() { projectName = oldProject }()

	cmd := newExecCmd()
	cmd.SetIn(strings.NewReader("q\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "selection cancelled") {
		t.Fatalf("expected local task cancellation, got %v; output=%q", err, out.String())
	}
	if strings.Contains(out.String(), "unregistered-target") {
		t.Fatalf("attempted to resolve SSH before menu selection: %s", out.String())
	}
}
