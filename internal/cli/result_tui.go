package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	resultstore "github.com/selimserbes/routurn/internal/result"
	"github.com/selimserbes/routurn/internal/runstate"
	"github.com/spf13/cobra"
)

var errNoResultEntries = errors.New("no recorded runs or materialized results")

type resultChoice struct {
	Task        string
	HasArtifact bool
	LatestRun   runstate.Manifest
	HasRun      bool
}

// The interactive result picker includes ordinary run history, even if a task
// didn't produce artifact files. It never treats the project's public results/
// directory as Routurn-owned without checking private metadata.
func chooseResultTTY(cmd *cobra.Command, root, projectName string) (resultChoice, error) {
	var choices []resultChoice
	var err error
	var discoveryDuration time.Duration
	withPickerLoading(cmd.OutOrStdout(), pickerIsInteractiveTerminal(cmd), "Finding available results...", func() {
		start := time.Now()
		choices, err = listResultChoices(root)
		discoveryDuration = time.Since(start)
	})
	defer func() {
		pickerReportTiming(cmd.ErrOrStderr(), "result", pickerTiming{"result-scan", discoveryDuration})
	}()
	if err != nil {
		return resultChoice{}, err
	}
	if len(choices) == 0 {
		return resultChoice{}, errNoResultEntries
	}
	m := interactiveMenu{Heading: "Select a result", ConfirmHint: "View selected result"}
	for _, choice := range choices {
		detail := "latest artifact"
		if choice.HasRun {
			detail = strings.ToLower(choice.LatestRun.Status)
			if choice.HasArtifact {
				detail += " · files available"
			} else {
				detail += " · no artifact files"
			}
		}
		m.Items = append(m.Items, interactiveMenuItem{Label: choice.Task, Detail: detail})
	}
	selected, err := chooseInteractiveMenu(cmd, projectName, m)
	if err != nil {
		return resultChoice{}, err
	}
	if selected.Index < 0 {
		return resultChoice{}, fmt.Errorf("result selection cancelled")
	}
	return choices[selected.Index], nil
}

func listResultChoices(root string) ([]resultChoice, error) {
	runs, err := runstate.List(root)
	if err != nil {
		return nil, err
	}

	available := map[string]bool{}
	base := filepath.Join(root, ".routurn", "results")
	entries, err := os.ReadDir(base)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(base, entry.Name(), "result.json"))
		if err != nil {
			continue
		}
		var meta resultstore.Metadata
		if json.Unmarshal(data, &meta) == nil && meta.Task != "" {
			available[meta.Task] = true
		}
	}

	choices := make([]resultChoice, 0, len(runs)+len(available))
	seen := map[string]bool{}
	for _, run := range runs {
		if run.Task == "" || seen[run.Task] {
			continue
		}
		choices = append(choices, resultChoice{
			Task:        run.Task,
			HasArtifact: available[run.Task],
			LatestRun:   run,
			HasRun:      true,
		})
		seen[run.Task] = true
	}
	var rest []string
	for name := range available {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	for _, name := range rest {
		choices = append(choices, resultChoice{Task: name, HasArtifact: true})
	}
	return choices, nil
}
