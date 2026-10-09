package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/spf13/cobra"
)

func addProjectFixture(t *testing.T, name string, local bool) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	mode := "remote"
	if local {
		mode = "local"
	}
	data := "version = 1\nname = '" + name + "'\n[execution]\nmode = '" + mode + "'\n"
	if err := os.WriteFile(filepath.Join(root, "routurn.toml"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRegisteredProjectMenuSortedSkipsMissingAndIsReadOnly(t *testing.T) {
	rootA := addProjectFixture(t, "alpha", true)
	rootZ := addProjectFixture(t, "zulu", false)
	global := &config.GlobalConfig{Projects: map[string]config.ProjectLink{
		"zulu":    {Root: rootZ},
		"missing": {Root: filepath.Join(t.TempDir(), "gone")},
		"alpha":   {Root: rootA},
	}}
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("2\n"))
	var output bytes.Buffer
	cmd.SetOut(&output)
	choice, err := chooseRegisteredProject(cmd, global)
	if err != nil || choice.Name != "zulu" || choice.Mode != "remote" {
		t.Fatalf("choice=%+v err=%v", choice, err)
	}
	got := output.String()
	if strings.Index(got, "alpha") > strings.Index(got, "zulu") || strings.Contains(got, "missing  ·") {
		t.Fatalf("menu isn't sorted or contains invalid project: %s", got)
	}
	if len(global.Projects) != 3 {
		t.Fatal("registry was mutated")
	}
}

func TestRegisteredProjectMenuCancelAndNoValidProjects(t *testing.T) {
	global := &config.GlobalConfig{Projects: map[string]config.ProjectLink{"bad": {Root: t.TempDir()}}}
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("1\n"))
	if _, err := chooseRegisteredProject(cmd, global); err == nil || !strings.Contains(err.Error(), "no usable") {
		t.Fatalf("invalid registry: %v", err)
	}
	global.Projects["good"] = config.ProjectLink{Root: addProjectFixture(t, "good", true)}
	cmd.SetIn(strings.NewReader("q\n"))
	if _, err := chooseRegisteredProject(cmd, global); err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("cancel: %v", err)
	}
}

func TestNoninteractivePickerDoesNotGuessRegisteredProject(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	global, err := config.LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	global.Projects["valid"] = config.ProjectLink{Root: addProjectFixture(t, "valid", true)}
	if err := config.SaveGlobal(global); err != nil {
		t.Fatal(err)
	}
	old, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	previous := projectName
	projectName = ""
	defer func() { projectName = previous }()
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("1\n"))
	cmd.SetOut(&bytes.Buffer{})
	if _, err := resolveLocalProjectContextForPicker(cmd); err == nil || !strings.Contains(err.Error(), "no routurn.toml") {
		t.Fatalf("non-TTY must require explicit --project: %v", err)
	}
}

func TestProjectCheckLocalAndMissingRemote(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	localRoot := addProjectFixture(t, "local-check", true)
	previous := projectName
	projectName = ""
	defer func() { projectName = previous }()
	cmd := newProjectCheckCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{localRoot})
	if err := cmd.Execute(); err != nil || !strings.Contains(output.String(), "Mode    local") {
		t.Fatalf("local check: %v %s", err, output.String())
	}
	remoteRoot := addProjectFixture(t, "remote-check", false)
	cmd = newProjectCheckCmd()
	cmd.SetArgs([]string{remoteRoot})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "missing [remote]") {
		t.Fatalf("invalid remote configuration accepted: %v", err)
	}
}

func TestProjectRegistrationCollisionRequiresReplace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	rootA := addProjectFixture(t, "shared", true)
	rootB := addProjectFixture(t, "other", true)

	add := func(root string, replace bool) error {
		cmd := newProjectAddCmd()
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		args := []string{root, "--name", "shared"}
		if replace {
			args = append(args, "--replace")
		}
		cmd.SetArgs(args)
		return cmd.Execute()
	}
	if err := add(rootA, false); err != nil {
		t.Fatal(err)
	}
	if err := add(rootB, false); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("collision not rejected: %v", err)
	}
	cfg, err := config.LoadGlobal()
	if err != nil || cfg.Projects["shared"].Root != rootA {
		t.Fatalf("registered project silently changed: %+v / %v", cfg, err)
	}
	if err := add(rootB, true); err != nil {
		t.Fatalf("explicit replacement: %v", err)
	}
	cfg, _ = config.LoadGlobal()
	if cfg.Projects["shared"].Root != rootB {
		t.Fatal("--replace did not change the registered path")
	}
}

func TestInitCollisionDoesNotWriteProjectConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	rootA := addProjectFixture(t, "first", true)
	global, err := config.LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	global.Projects["shared"] = config.ProjectLink{Root: rootA}
	if err := config.SaveGlobal(global); err != nil {
		t.Fatal(err)
	}
	rootB := t.TempDir()
	original, _ := os.Getwd()
	if err := os.Chdir(rootB); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(original)
	cmd := newInitCmd()
	cmd.SetArgs([]string{"--name", "shared", "--local"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("init collision not rejected: %v", err)
	}
	if _, err := os.Stat(filepath.Join(rootB, "routurn.toml")); !os.IsNotExist(err) {
		t.Fatalf("init left a project file on collision: %v", err)
	}
}
