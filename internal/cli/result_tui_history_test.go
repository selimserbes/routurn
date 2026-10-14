package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/selimserbes/routurn/internal/runstate"
)

func TestResultChoicesIncludeRunsWithoutArtifacts(t *testing.T) {
	root := t.TempDir()
	code := 0
	run := runstate.Manifest{
		ID: "20261014T060000Z-abcdef", Task: "smoke", Status: "SUCCEEDED",
		StartedAt: "2026-10-14T06:00:00Z", ExitCode: &code,
	}
	if _, err := runstate.Create(root, run); err != nil {
		t.Fatal(err)
	}
	choices, err := listResultChoices(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(choices) != 1 || choices[0].Task != "smoke" || choices[0].HasArtifact || !choices[0].HasRun {
		t.Fatalf("unexpected result choices: %+v", choices)
	}
	var out bytes.Buffer
	printRunOnlyResult(&out, choices[0].LatestRun)
	for _, want := range []string{"smoke", "SUCCEEDED", "No artifacts collected"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("run summary missing %q: %s", want, out.String())
		}
	}
}

func TestResultChoicesMixArtifactsAndArtifactFreeRuns(t *testing.T) {
	root := t.TempDir()
	for _, task := range []string{"without_files", "with_files"} {
		run := runstate.Manifest{ID: "20261014T060000Z-" + task, Task: task, Status: "SUCCEEDED"}
		if _, err := runstate.Create(root, run); err != nil {
			t.Fatal(err)
		}
	}
	folder := filepath.Join(root, ".routurn", "results", "with_files")
	if err := os.MkdirAll(folder, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "result.json"), []byte(`{"task":"with_files","run_id":"r1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	choices, err := listResultChoices(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(choices) != 2 {
		t.Fatalf("got %d results, want 2: %+v", len(choices), choices)
	}
	m := map[string]resultChoice{}
	for _, choice := range choices {
		m[choice.Task] = choice
	}
	if m["without_files"].HasArtifact || !m["with_files"].HasArtifact {
		t.Fatalf("incorrect artifact labeling: %+v", m)
	}
}

func TestResultChoicesEmptyProject(t *testing.T) {
	choices, err := listResultChoices(t.TempDir())
	if err != nil || len(choices) != 0 {
		t.Fatalf("empty project should not produce filesystem error: choices=%v err=%v", choices, err)
	}
}
