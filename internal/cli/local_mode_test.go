package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/selimserbes/routurn/internal/config"
	resultstore "github.com/selimserbes/routurn/internal/result"
	"github.com/selimserbes/routurn/internal/runstate"
)

func TestLocalExecAndResultWithoutRegisteredSSHTarget(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := `version = 1
name = "local-test"
[execution]
mode = "local"
[remote]
target = "not-a-real-target"
path = "/remote/unused"
[tasks.smoke]
command = "echo local-smoke"
artifacts = ["output/*.txt"]
`
	if err := os.WriteFile(filepath.Join(root, "routurn.toml"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "output"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "output", "data.txt"), []byte("artifact"), 0644); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	oldProject, oldEndpoint := projectName, endpointOverride
	projectName, endpointOverride = "", ""
	defer func() { projectName, endpointOverride = oldProject, oldEndpoint }()
	cmd := newExecCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"smoke"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "local-smoke") || !strings.Contains(out.String(), "Target   local") {
		t.Fatalf("local mode missing output: %s", out.String())
	}
	runs, err := runstate.List(root)
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs=%v err=%v", runs, err)
	}
	if runs[0].Status != "SUCCEEDED" || runs[0].Target != "local" || len(runs[0].Artifacts) != 1 {
		t.Fatalf("unexpected local run: %+v", runs[0])
	}
	_, view, err := resultstore.Read(root, "smoke")
	if err != nil {
		t.Fatalf("result: %v", err)
	}
	if _, err := os.Stat(view); err != nil {
		t.Fatalf("result path %s: %v", view, err)
	}
	// The project contains an intentionally nonexistent SSH target. Success
	// proves the synchronous path did not resolve or use an SSH endpoint.
}

func TestConfigLocalOptInAndLegacyRemote(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "routurn.toml")
	if err := os.WriteFile(file, []byte("version = 1\nname = 'legacy'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadProject(root)
	if err != nil || cfg.IsLocal() {
		t.Fatalf("legacy should be remote: %+v %v", cfg, err)
	}
	if err := os.WriteFile(file, []byte("version = 1\nname = 'invalid'\n[execution]\nmode = 'magic'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.LoadProject(root); err == nil {
		t.Fatal("unknown execution mode accepted")
	}
}

func TestLocalDetachRejectedBeforeRun(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := `version = 1
name = "local-detach"
[execution]
mode = "local"
[tasks.smoke]
command = "echo should-not-run"
`
	if err := os.WriteFile(filepath.Join(root, "routurn.toml"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	oldProject, oldEndpoint := projectName, endpointOverride
	projectName, endpointOverride = "", ""
	defer func() { projectName, endpointOverride = oldProject, oldEndpoint }()
	cmd := newExecCmd()
	cmd.SetArgs([]string{"smoke", "--detach"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "local --detach") {
		t.Fatalf("err=%v output=%s", err, out.String())
	}
	runs, _ := runstate.List(root)
	if len(runs) != 0 {
		t.Fatalf("unexpected run: %+v", runs)
	}
}
