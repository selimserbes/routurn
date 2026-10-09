package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/selimserbes/routurn/internal/config"
)

func makeProjectFixture(t *testing.T, name string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := "version = 1\nname = '" + name + "'\n[execution]\nmode = 'local'\n"
	if err := os.WriteFile(filepath.Join(root, config.ProjectFileName), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestResolveProjectRegisteredNameAbsoluteRelativeAndNestedPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := makeProjectFixture(t, "local-app")
	cfg, err := config.LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Projects["preferred"] = config.ProjectLink{Root: root}
	if err := config.SaveGlobal(cfg); err != nil {
		t.Fatal(err)
	}
	base, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Dir(root)); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(base)

	nested := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	for _, arg := range []string{"preferred", root, "./local-app", nested} {
		got, err := Resolve(arg)
		if err != nil || got.Root != root || !got.Config.IsLocal() {
			t.Fatalf("Resolve(%q) = %+v / %v", arg, got, err)
		}
	}
	if _, err := Resolve("unknown"); err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("unknown name: %v", err)
	}
	if _, err := Resolve("/definitely/not/an/actual/project"); err == nil {
		t.Fatal("missing path accepted")
	}
}

func TestResolveCWDNotFoundIsDistinguishable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cwd, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	if _, err := Resolve(""); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestResolveMalformedCWDIsNotNotFound(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cwd, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, config.ProjectFileName), []byte("this isn't toml = ["), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	if _, err := Resolve(""); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid project must not be mistaken for missing: %v", err)
	}
}
