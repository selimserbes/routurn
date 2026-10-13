package cli

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/selimserbes/routurn/internal/runstate"
	"github.com/spf13/cobra"
)

func TestCommonMenuFiltersAcrossTechnologies(t *testing.T) {
	items := []interactiveMenuItem{{Label: "cargo test", Detail: "Rust"}, {Label: "go build", Detail: "Go"}, {Label: "npm run dev", Detail: "Node"}}
	got := menuSearchIndices(items, "RUST")
	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("search: %v", got)
	}
	got = menuSearchIndices(items, "go build")
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("search: %v", got)
	}
}
func TestCommonMenuTenthChoiceUsesZero(t *testing.T) {
	items := make([]interactiveMenuItem, 11)
	for i := range items {
		items[i].Label = "task"
	}
	shown, page, pages := menuPickPage(menuSearchIndices(items, ""), 0)
	if len(shown) != 10 || page != 0 || pages != 2 {
		t.Fatal("page one")
	}
	var screen bytes.Buffer
	menuShow(&screen, "test", interactiveMenu{Heading: "Tasks", Items: items}, shown, page, pages, 0, "", "", false, false)
	if !strings.Contains(screen.String(), "0) task") {
		t.Fatal("tenth task needs 0 shortcut")
	}
}
func TestCommonMenuLineModeKeepsShortcutsAndPipes(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("/cargo\n1\n"))
	var screen bytes.Buffer
	cmd.SetOut(&screen)
	items := []interactiveMenuItem{{Label: "npm test"}, {Label: "cargo test"}}
	sel, err := chooseInteractiveMenu(cmd, "Rust", interactiveMenu{Heading: "Tasks", Items: items})
	if err != nil || sel.Index != 1 {
		t.Fatalf("selected=%+v err=%v", sel, err)
	}
}
func TestCommonMenuSafeNumberRequiresConfirmation(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("0\n"))
	var screen bytes.Buffer
	cmd.SetOut(&screen)
	items := make([]interactiveMenuItem, 10)
	for i := range items {
		items[i].Label = "task"
	}
	sel, err := chooseInteractiveMenu(cmd, "example", interactiveMenu{Heading: "Tasks", Items: items})
	if err != nil || sel.Index != 9 {
		t.Fatalf("selected=%+v err=%v", sel, err)
	}
}

func TestSearchSpansConfiguredHomeGroup(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("/cargo\n1\n"))
	var screen bytes.Buffer
	cmd.SetOut(&screen)
	menu := interactiveMenu{
		Heading: "Current", Items: []interactiveMenuItem{{Label: "train"}},
		SearchItems: []interactiveMenuItem{{Label: "train"}, {Label: "cargo test"}},
	}
	sel, err := chooseInteractiveMenu(cmd, "example", menu)
	if err != nil || sel.Index != 1 || !sel.FromSearchAll {
		t.Fatalf("got %+v, %v", sel, err)
	}
}

func TestRecentRunsAreGenericAndRespectHistory(t *testing.T) {
	root := t.TempDir()
	_, err := runstate.Create(root, runstate.Manifest{ID: "20261008T120000Z-000001", Task: "cargo-test"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = runstate.Create(root, runstate.Manifest{ID: "20261008T140000Z-000002", Task: "npm-build"})
	if err != nil {
		t.Fatal(err)
	}
	got := orderByRecentRuns(root, []runnableCommand{{Name: "alpha"}, {Name: "cargo-test"}, {Name: "npm-build"}})
	if got[0].Name != "npm-build" || got[1].Name != "cargo-test" || got[2].Name != "alpha" {
		t.Fatalf("recent: %+v", got)
	}
}

func TestTTYNavigationHasNoTextInputCaret(t *testing.T) {
	items := []interactiveMenuItem{{Label: "build"}, {Label: "cargo test"}}
	menu := interactiveMenu{Heading: "Tasks", Items: items}
	shown := []int{0, 1}
	var screen bytes.Buffer
	menuShow(&screen, "Rust", menu, shown, 0, 1, 1, "", "", false, true)
	got := screen.String()
	if !strings.Contains(got, "\x1b[?25l") || !strings.Contains(got, "Selected: cargo test") || strings.Contains(got, "Select a task or shortcut:") {
		t.Fatalf("navigation mode looks like an input field: %q", got)
	}
	if !strings.Contains(got, "[↑↓] Navigate") || !strings.Contains(got, "[Enter] Confirm") {
		t.Fatalf("navigation keys should appear in shortcuts: %q", got)
	}
}

func TestTTYFooterShowsActionSpecificConfirmation(t *testing.T) {
	items := []interactiveMenuItem{{Label: "cargo test"}}
	for _, tc := range []struct {
		name, hint, want string
	}{
		{"exec", "Run selected task", "[Enter] Run selected task"},
		{"update", "Review selected update", "[Enter] Review selected update"},
		{"result", "View selected result", "[Enter] View selected result"},
		{"fallback", "", "[Enter] Confirm selection"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var screen bytes.Buffer
			menu := interactiveMenu{Heading: "Tasks", Items: items, ConfirmHint: tc.hint}
			menuShow(&screen, "example", menu, []int{0}, 0, 1, 0, "", "", false, true)
			got := screen.String()
			if !strings.Contains(got, "Selected: cargo test\n"+tc.want+"\n") {
				t.Fatalf("unexpected footer: %q", got)
			}
			if strings.Contains(got, "Press Enter to confirm; use arrow keys") {
				t.Fatalf("old verbose footer still present: %q", got)
			}
		})
	}
}

func TestTTYSearchShowsTextCursor(t *testing.T) {
	menu := interactiveMenu{Heading: "Tasks", Items: []interactiveMenuItem{{Label: "cargo test"}}, Searching: true}
	var screen bytes.Buffer
	menuShow(&screen, "Rust", menu, []int{0}, 0, 1, 0, "cargo", "", true, true)
	if !strings.Contains(screen.String(), "\x1b[?25h") || !strings.Contains(screen.String(), "Search tasks (Enter: finish, Esc: cancel):") {
		t.Fatalf("search should be visible text entry: %q", screen.String())
	}
}

func TestTextEntryCancelReturnsToMenuInLineMode(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("\x1b\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	if _, err := readMenuText(cmd, "Command"); !errors.Is(err, errTextInputCancelled) {
		t.Fatalf("Esc did not cancel text entry: %v", err)
	}
}

func TestNestedLineModeCustomInputEscReturnsToTasks(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("e\n\x1b\n1\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	menu := interactiveMenu{Heading: "Tasks", Items: []interactiveMenuItem{{Label: "cargo test"}}, Shortcuts: []string{"e Custom command"}}
	sel, err := chooseInteractiveMenu(cmd, "Rust", menu)
	if err != nil || sel.Key != "e" {
		t.Fatalf("did not enter text input: %+v %v", sel, err)
	}
	_, err = readMenuText(cmd, "Command")
	if !errors.Is(err, errTextInputCancelled) {
		t.Fatalf("Esc did not cancel: %v", err)
	}
	sel, err = chooseInteractiveMenu(cmd, "Rust", menu)
	if err != nil || sel.Index != 0 {
		t.Fatalf("did not return to task picker: %+v %v", sel, err)
	}
}

// stripTestSGR removes colors but keeps cursor-control sequences.
func stripTestSGR(s string) string {
	return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(s, "")
}

func TestTTYInlineEditorKeepsTaskListAndReplacesSelectionFooter(t *testing.T) {
	menu := interactiveMenu{
		Heading: "Run a task  /  Tasks", Items: []interactiveMenuItem{{Label: "cargo test"}, {Label: "cargo build"}},
		TextInputLabel: "Command", EnteringText: true, TextInputValue: "echo ok",
	}
	var screen bytes.Buffer
	menuShow(&screen, "rust-project", menu, []int{0, 1}, 0, 1, 0, "", "", true, true)
	got := stripTestSGR(screen.String())
	for _, want := range []string{"Run a task  /  Tasks", "1) cargo test", "2) cargo build", "Shortcuts", "[Enter] Run command", "[Esc] Back", "Command:\n  ❯ echo ok"} {
		if !strings.Contains(got, want) {
			t.Fatalf("command editor lost %q: %q", want, got)
		}
	}
	if !strings.Contains(got, "\x1b[?25h") || !strings.HasSuffix(got, "  ❯ echo ok") {
		t.Fatalf("cursor must be visible at the input: %q", got)
	}
	if strings.Contains(got, "Selected:") || strings.Contains(got, "❯ 1) cargo test") || strings.Contains(got, "[q] Exit") {
		t.Fatalf("navigation and text modes overlap: %q", got)
	}
}

func TestTTYInlineEditorReturnScreenContainsPersistentCancellationNotice(t *testing.T) {
	menu := interactiveMenu{Heading: "Tasks", Items: []interactiveMenuItem{{Label: "cargo test"}}, TextInputLabel: "Command"}
	var screen bytes.Buffer
	menuShow(&screen, "example", menu, []int{0}, 0, 1, 0, "", "Command cancelled; nothing was run.", false, true)
	got := screen.String()
	if !strings.Contains(got, "Command cancelled; nothing was run.") || !strings.Contains(got, "Selected: cargo test") {
		t.Fatalf("cancel must restore task view with notice: %q", got)
	}
}

func TestTTYInlinePathEditorKeepsUpdateChoicesVisible(t *testing.T) {
	menu := interactiveMenu{Heading: "Choose update source", Items: []interactiveMenuItem{{Label: "update.zip"}}, TextInputLabel: "Path", EnteringText: true, TextInputValue: "/tmp/archive.zip"}
	var screen bytes.Buffer
	menuShow(&screen, "example", menu, []int{0}, 0, 1, 0, "", "", false, true)
	got := stripTestSGR(screen.String())
	if !strings.Contains(got, "1) update.zip") || !strings.Contains(got, "Path:\n  ❯ /tmp/archive.zip") || !strings.Contains(got, "[Enter] Use path") || strings.Contains(got, "Selected:") {
		t.Fatalf("update path editor should keep choices while editing footer: %q", got)
	}
}
