package cli

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/selimserbes/routurn/internal/project"
	"github.com/spf13/cobra"
)

func pickerTest(t *testing.T, tasks map[string]config.Task, settings config.PickerConfig, input string) (runnableCommand, string, error) {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(input))
	var output bytes.Buffer
	cmd.SetOut(&output)
	ctx := &projectContext{Resolved: &project.Resolved{
		Root: t.TempDir(), Config: &config.ProjectConfig{Name: "example", Tasks: tasks, Picker: settings},
	}}
	selected, err := chooseRunnableInteractive(cmd, ctx)
	return selected, output.String(), err
}

func TestPickerGenericOrderingIgnoresStageNames(t *testing.T) {
	items := []runnableCommand{{Name: "stage99-run"}, {Name: "test"}, {Name: "build"}, {Name: "stage1"}}
	got := rankRunnableChoices(items)
	want := []string{"build", "stage1", "stage99-run", "test"}
	for i := range want {
		if got[i].Name != want[i] {
			t.Fatalf("got %s at %d, want %s", got[i].Name, i, want[i])
		}
	}
}

func TestPickerRustStyleProjectWorksWithoutConfig(t *testing.T) {
	tasks := map[string]config.Task{"test": {Command: "cargo test"}, "build-release": {Command: "cargo build --release"}, "run": {Command: "cargo run"}}
	selected, screen, err := pickerTest(t, tasks, config.PickerConfig{}, "1\n")
	if err != nil || selected.Name != "build-release" {
		t.Fatalf("selected=%q err=%v", selected.Name, err)
	}
	if !strings.Contains(screen, "Run a task  /  Tasks") || strings.Contains(screen, "Stage ") || strings.Contains(screen, "Other tasks") {
		t.Fatalf("non-generic screen: %s", screen)
	}
	if strings.Contains(screen, "[h] Groups") {
		t.Fatalf("invented groups: %s", screen)
	}
}

func TestPickerUnknownStageNameDoesNotBecomeGroup(t *testing.T) {
	_, screen, _ := pickerTest(t, map[string]config.Task{"stage2-build": {Command: "echo x"}, "run": {Command: "echo y"}}, config.PickerConfig{}, "q\n")
	if !strings.Contains(screen, "stage2-build") || !strings.Contains(screen, "run") || strings.Contains(screen, "Groups (") {
		t.Fatalf("task incorrectly treated as stage metadata: %s", screen)
	}
}

func TestPickerExplicitGroupsAndHomeLabel(t *testing.T) {
	tasks := map[string]config.Task{"stage3a_v4_smoke": {Command: "./scripts/a.sh"}, "stage3a_v4_train": {Command: "./scripts/b.sh"}, "stage3a_v3_train": {Command: "./scripts/c.sh"}, "preflight": {Command: "echo ready"}}
	cfg := config.PickerConfig{DefaultGroup: "Current", Groups: []config.PickerGroup{{Name: "Current", Tasks: []string{"stage3a_v4_*"}}, {Name: "Earlier runs", Tasks: []string{"stage3a_v3_*"}}}, Labels: map[string]string{"stage3a_v4_smoke": "Smoke test", "stage3a_v4_train": "Training"}}
	selected, screen, err := pickerTest(t, tasks, cfg, "1\n")
	if err != nil || selected.Name != "stage3a_v4_smoke" {
		t.Fatalf("selected %q %v", selected.Name, err)
	}
	if !strings.Contains(screen, "Run a task  /  Current") || !strings.Contains(screen, "Smoke test") {
		t.Fatalf("group home missing: %s", screen)
	}
	if strings.Contains(screen, "stage3a_v3_train") || strings.Contains(screen, "preflight") {
		t.Fatalf("home includes unrelated tasks: %s", screen)
	}
}

func TestPickerGroupsAreExplicitAndSelectable(t *testing.T) {
	tasks := map[string]config.Task{"first": {Command: "echo one"}, "later": {Command: "echo two"}, "custom": {Command: "echo three"}}
	cfg := config.PickerConfig{Groups: []config.PickerGroup{{Name: "Alpha", Tasks: []string{"first"}}, {Name: "Beta", Tasks: []string{"later"}}}}
	selected, screen, err := pickerTest(t, tasks, cfg, "h\n2\n1\n")
	if err != nil || selected.Name != "later" {
		t.Fatalf("select group: %q %v", selected.Name, err)
	}
	if !strings.Contains(screen, "Browse groups") || !strings.Contains(screen, "Ungrouped (1)") {
		t.Fatalf("group menu: %s", screen)
	}
}

func TestPickerExplicitGroupWithNoDefaultShowsAllTasks(t *testing.T) {
	cfg := config.PickerConfig{Groups: []config.PickerGroup{{Name: "Only X", Tasks: []string{"x"}}}}
	selected, screen, err := pickerTest(t, map[string]config.Task{"x": {Command: "echo x"}, "y": {Command: "echo y"}}, cfg, "2\n")
	if err != nil || selected.Name != "y" || !strings.Contains(screen, "Run a task  /  Tasks") {
		t.Fatalf("default changed unexpectedly %q %v\n%s", selected.Name, err, screen)
	}
}

func TestPickerUnmatchedTasksRemainsReachable(t *testing.T) {
	cfg := config.PickerConfig{DefaultGroup: "Pinned", Groups: []config.PickerGroup{{Name: "Pinned", Tasks: []string{"build"}}}}
	selected, _, err := pickerTest(t, map[string]config.Task{"build": {Command: "make build"}, "test": {Command: "make test"}}, cfg, "u\n1\n")
	if err != nil || selected.Name != "test" {
		t.Fatalf("ungrouped task lost: %q %v", selected.Name, err)
	}
}

func TestPickerSearchWorksAcrossGroups(t *testing.T) {
	cfg := config.PickerConfig{DefaultGroup: "Main", Groups: []config.PickerGroup{{Name: "Main", Tasks: []string{"build"}}}}
	selected, screen, err := pickerTest(t, map[string]config.Task{"build": {Command: "cargo build"}, "check": {Command: "cargo clippy"}}, cfg, "/clippy\n1\n")
	if err != nil || selected.Name != "check" || !strings.Contains(screen, "Search: clippy") {
		t.Fatalf("search=%q err=%v: %s", selected.Name, err, screen)
	}
}

func TestPickerWildcardMatchingIsCaseInsensitiveAndNamesUnchanged(t *testing.T) {
	if !pickerMatchesTask("Run_Server", []string{"run_*"}) || pickerMatchesTask("run-server", []string{"build*"}) {
		t.Fatal("wrong wildcard result")
	}
	groups, remaining := groupPickerTasks([]runnableCommand{{Name: "run-server"}, {Name: "build"}}, config.PickerConfig{Groups: []config.PickerGroup{{Name: "Development", Tasks: []string{"run-*"}}}})
	if len(groups) != 1 || len(groups[0].Items) != 1 || groups[0].Items[0].Name != "run-server" || len(remaining) != 1 || remaining[0].Name != "build" {
		t.Fatalf("grouping: %#v %#v", groups, remaining)
	}
}

func TestPickerPagingAndSearch(t *testing.T) {
	items := make([]runnableCommand, 23)
	for i := range items {
		items[i] = runnableCommand{Name: "test", Description: "load shift"}
	}
	p0, n0 := pickerPage(items, 0)
	p2, n2 := pickerPage(items, 2)
	if len(p0) != 10 || n0 != 0 || len(p2) != 3 || n2 != 2 || len(searchRunnableChoices(items, "LOAD")) != 23 {
		t.Fatal("bad paging or search")
	}
}

func TestPickerManyTasksPageNavigation(t *testing.T) {
	tasks := map[string]config.Task{}
	for i := 0; i < 24; i++ {
		name := "task-" + strings.Repeat("0", 2-len(strconv.Itoa(i))) + strconv.Itoa(i)
		tasks[name] = config.Task{Command: "echo " + name}
	}
	picked, screen, err := pickerTest(t, tasks, config.PickerConfig{}, "n\n2\n")
	if err != nil || picked.Name != "task-11" || !strings.Contains(screen, "Page 2/3") {
		t.Fatalf("paging: %q %v %s", picked.Name, err, screen)
	}
}

func TestPickerPromptSeparatedAndShortcutsVisible(t *testing.T) {
	_, screen, _ := pickerTest(t, map[string]config.Task{"build": {Command: "cargo build"}}, config.PickerConfig{}, "q\n")
	if !strings.Contains(screen, "[/] Search tasks") || !strings.Contains(screen, "[q] Exit\n\nSelect a task [1-1] or shortcut:\n  > ") {
		t.Fatalf("bad prompt: %s", screen)
	}
}

func TestPickerShortcutsRespectNarrowWidth(t *testing.T) {
	for _, width := range []int{38, 48, 56, 60} {
		var out bytes.Buffer
		pickerPrintOptions(&out, width, []string{"/ Search tasks", "h Groups (8)", "u Ungrouped (3)", "a All tasks", "e Custom command", "q Exit"}, false)
		for _, line := range strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n") {
			if len([]rune(line)) > width {
				t.Fatalf("too wide %d: %q", width, line)
			}
		}
	}
}

func TestPickerShortcutsANSIDoesNotAffectAlignment(t *testing.T) {
	options := []string{"/ Search tasks", "h Groups (8)", "a All tasks", "q Exit"}
	var plain, colored bytes.Buffer
	pickerPrintOptions(&plain, 60, options, false)
	pickerPrintOptions(&colored, 60, options, true)
	if strings.NewReplacer("\x1b[1;96m", "", "\x1b[0m", "").Replace(colored.String()) != plain.String() {
		t.Fatal("ANSI changed alignment")
	}
	t.Setenv("NO_COLOR", "")
	if _, label := pickerShortcutLabel("q Exit", false); label != "[q] Exit" {
		t.Fatal("NO_COLOR text missing")
	}
}

func TestPickerInvalidSelectionNotice(t *testing.T) {
	_, screen, _ := pickerTest(t, map[string]config.Task{"build": {Command: "echo x"}}, config.PickerConfig{}, "42\nq\n")
	if !strings.Contains(screen, "Choose a shown number or a navigation option.") {
		t.Fatalf("notice missing: %s", screen)
	}
}
