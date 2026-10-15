package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompletionInstallUserPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ZSH", "")
	t.Setenv("ZSH_CUSTOM", "")
	root := &cobra.Command{Use: "routurn"}
	root.AddCommand(&cobra.Command{Use: "version"})

	omz := filepath.Join(home, ".oh-my-zsh")
	if err := os.MkdirAll(omz, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(omz, "oh-my-zsh.sh"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("# existing setup\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dest, err := installShellCompletion(root, "zsh")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(omz, "custom", "completions", "_routurn"); dest != want {
		t.Fatalf("got %s want %s", dest, want)
	}
	if stat, err := os.Stat(dest); err != nil || stat.Size() == 0 {
		t.Fatalf("missing zsh completion: %v", err)
	}
	if got, _ := os.ReadFile(zshrc); string(got) != "# existing setup\n" {
		t.Fatalf("Oh My Zsh profile unexpectedly changed: %q", got)
	}

	for _, shell := range []string{"bash", "fish", "powershell"} {
		path, err := installShellCompletion(root, shell)
		if err != nil {
			t.Fatalf("%s: %v", shell, err)
		}
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Fatalf("%s: file missing %v", shell, err)
		}
	}
	if _, err := installShellCompletion(root, "bash"); err != nil {
		t.Fatal(err)
	}
	bashrc, err := os.ReadFile(filepath.Join(home, ".bashrc"))
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(bashrc), "# Routurn shell completion (managed)"); count != 1 {
		t.Fatalf("duplicate Bash profile hook: %d", count)
	}
}
