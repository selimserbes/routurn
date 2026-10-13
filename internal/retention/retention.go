package retention

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/selimserbes/routurn/internal/bundle"
	"github.com/selimserbes/routurn/internal/runstate"
)

const (
	DefaultRuns    = 10
	DefaultUpdates = 10
)

type Report struct {
	Runs    []string
	Updates []string
}

func PruneProject(root string, keepRuns, keepUpdates int, dryRun bool) (Report, error) {
	var report Report

	runs, err := runstate.List(root)
	if err != nil {
		return report, err
	}
	if keepRuns < 0 {
		keepRuns = 0
	}
	if len(runs) > keepRuns {
		for _, run := range runs[keepRuns:] {
			path := runstate.Dir(root, run.ID)
			report.Runs = append(report.Runs, path)
			if !dryRun {
				if err := os.RemoveAll(path); err != nil {
					return report, err
				}
			}
		}
	}

	updates, err := bundle.List(root)
	if err != nil {
		return report, err
	}
	if keepUpdates < 0 {
		keepUpdates = 0
	}
	if len(updates) > keepUpdates {
		// bundle.List is newest-first, but keep this explicit and stable.
		sort.Slice(updates, func(i, j int) bool { return updates[i].ID > updates[j].ID })
		for _, update := range updates[keepUpdates:] {
			path := filepath.Join(root, ".routurn", "updates", update.ID)
			report.Updates = append(report.Updates, path)
			if !dryRun {
				if err := os.RemoveAll(path); err != nil {
					return report, err
				}
			}
		}
	}
	return report, nil
}
