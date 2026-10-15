package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/spf13/cobra"
)

func TestCompletionFilter(t *testing.T) {
	got := completionFilter([]string{"beta", "alpha", "alpine", "alpha"}, "al")
	want := []string{"alpha", "alpha", "alpine"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDynamicCompletionLocalOnly(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	projectDir := filepath.Join(home, "example")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := []byte("version = 1\nname = \"example\"\n[execution]\nmode = \"local\"\n[tasks.smoke]\ncommand = \"echo test\"\n[tasks.train]\ncommand = \"echo train\"\n")
	if err := os.WriteFile(filepath.Join(projectDir, config.ProjectFileName), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	global := &config.GlobalConfig{
		Version:  1,
		Targets:  map[string]config.Target{"lab-test": {Host: "127.0.0.1", User: "test", Port: 22}},
		Projects: map[string]config.ProjectLink{"example": {Root: projectDir}},
	}
	if err := config.SaveGlobal(global); err != nil {
		t.Fatal(err)
	}
	original := projectName
	projectName = "example"
	t.Cleanup(func() { projectName = original })

	root := &cobra.Command{Use: "routurn"}
	root.PersistentFlags().String("project", "", "")
	root.PersistentFlags().String("endpoint", "", "")
	root.AddCommand(newTargetCmd(), newProjectCmd(), newExecCmd(), newRunCmd(), newResultCmd(), newTaskCmd())
	configureDynamicCompletion(root)

	get := func(parts ...string) *cobra.Command {
		node := root
		for _, part := range parts {
			var found *cobra.Command
			for _, child := range node.Commands() {
				if child.Name() == part {
					found = child
					break
				}
			}
			if found == nil {
				t.Fatalf("command missing: %q", part)
			}
			node = found
		}
		if node.ValidArgsFunction == nil {
			t.Fatalf("missing completion: %q", parts)
		}
		return node
	}
	require := func(cmd *cobra.Command, args []string, prefix string, expected []string) {
		got, directive := cmd.ValidArgsFunction(cmd, args, prefix)
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("%s got %q want %q", cmd.CommandPath(), got, expected)
		}
		if directive == cobra.ShellCompDirectiveError {
			t.Fatalf("unexpected completion error")
		}
	}
	require(get("target", "test"), nil, "lab", []string{"lab-test"})
	require(get("project", "show"), nil, "ex", []string{"example"})
	require(get("exec"), nil, "sm", []string{"smoke"})
	require(get("run"), nil, "tr", []string{"train"})
	require(get("target", "test"), []string{"lab-test"}, "", nil)
}
