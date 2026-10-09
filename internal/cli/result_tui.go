package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	resultstore "github.com/selimserbes/routurn/internal/result"
	"github.com/selimserbes/routurn/internal/runstate"
	"github.com/spf13/cobra"
)

// Inspect Routurn's private result metadata, rather than assuming that the
// project's public results/ directory is owned by Routurn.
func chooseResultTTY(cmd *cobra.Command, root, projectName string) (string, error) {
	var names []string
	var err error
	var discoveryDuration time.Duration
	withPickerLoading(cmd.OutOrStdout(), pickerIsInteractiveTerminal(cmd), "Finding available results...", func() {
		start := time.Now()
		names, err = listResultTaskNames(root)
		discoveryDuration = time.Since(start)
	})
	defer func() {
		pickerReportTiming(cmd.ErrOrStderr(), "result", pickerTiming{"result-scan", discoveryDuration})
	}()
	if err != nil {
		return "", err
	}
	m := interactiveMenu{Heading: "Select a result", ConfirmHint: "View selected result"}
	for _, name := range names {
		m.Items = append(m.Items, interactiveMenuItem{Label: name, Detail: "latest artifact"})
	}
	selected, err := chooseInteractiveMenu(cmd, projectName, m)
	if err != nil {
		return "", err
	}
	if selected.Index < 0 {
		return "", fmt.Errorf("result selection cancelled")
	}
	return names[selected.Index], nil
}

func listResultTaskNames(root string) ([]string, error) {
	base := filepath.Join(root, ".routurn", "results")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("no materialized results yet: %w", err)
	}
	available := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, e := os.ReadFile(filepath.Join(base, entry.Name(), "result.json"))
		if e != nil {
			continue
		}
		var meta resultstore.Metadata
		if json.Unmarshal(data, &meta) == nil && meta.Task != "" {
			available[meta.Task] = true
		}
	}
	if len(available) == 0 {
		return nil, fmt.Errorf("no materialized results yet; run a task with artifacts first")
	}
	names := make([]string, 0, len(available))
	seen := map[string]bool{}
	runs, _ := runstate.List(root)
	for _, run := range runs {
		if available[run.Task] && !seen[run.Task] {
			names = append(names, run.Task)
			seen[run.Task] = true
		}
	}
	var rest []string
	for name := range available {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	names = append(names, rest...)
	return names, nil
}
