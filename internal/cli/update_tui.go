package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/selimserbes/routurn/internal/intake"
	"github.com/spf13/cobra"
)

// Menu chooses the archive source; all existing compatibility checks, preview,
// snapshots and confirmation remain in the update/apply workflow.
func chooseUpdateTTY(cmd *cobra.Command, ctx *projectContext, strip int) (string, error) {
	var choices []updateChoice
	var discoveryDuration time.Duration
	withPickerLoading(cmd.OutOrStdout(), pickerIsInteractiveTerminal(cmd), "Finding update archives...", func() {
		start := time.Now()
		choices, _ = recentDetectedUpdates(ctx, strip)
		choices = orderUpdateChoices(choices)
		discoveryDuration = time.Since(start)
	})
	defer func() {
		pickerReportTiming(cmd.ErrOrStderr(), "update", pickerTiming{"archive-scan", discoveryDuration})
	}()
	for {
		menu := interactiveMenu{Heading: "Choose update source", ConfirmHint: "Review selected update", Shortcuts: []string{"b Browse files", "e Enter path", "m Managed latest"}, TextInputLabel: "Path"}
		for _, choice := range choices {
			name := displayBundleName(choice.Inspection, filepath.Base(choice.Path))
			menu.Items = append(menu.Items, interactiveMenuItem{Label: name, Detail: updateChoiceLabels(choice)})
		}
		sel, err := chooseInteractiveMenu(cmd, ctx.Resolved.Config.Name, menu)
		if err != nil {
			return "", err
		}
		if sel.Index >= 0 {
			return choices[sel.Index].Path, nil
		}
		switch sel.Key {
		case "q", "esc":
			return "", fmt.Errorf("update selection cancelled")
		case "b":
			return browseArchiveTTY(cmd, ctx)
		case "e":
			path, e := sel.Text, error(nil)
			if path == "" {
				path, e = readMenuText(cmd, "Path (Esc to back)")
			}
			if errors.Is(e, errTextInputCancelled) {
				continue
			}
			if e != nil {
				return "", e
			}
			if strings.TrimSpace(path) == "" {
				continue
			}
			return expandUserPath(strings.TrimSpace(path)), nil
		case "m":
			entry, e := intake.LatestForProject(ctx.Resolved.Config.Name)
			if e != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "No managed update: %v\n", e)
				continue
			}
			return entry.Path, nil
		}
	}
}

// The file browser uses the same shared navigation as the archive shortlist.
// Selecting a directory only navigates; update application still happens later
// through the existing validated apply/confirmation workflow.
func browseArchiveTTY(cmd *cobra.Command, ctx *projectContext) (string, error) {
	current := initialBrowseDir(ctx.Global)
	for {
		entries, err := os.ReadDir(current)
		if err != nil {
			return "", err
		}
		type browserItem struct {
			path, name string
			dir        bool
		}
		items := []browserItem{{path: filepath.Dir(current), name: "../", dir: true}}
		var dirs, files []browserItem
		for _, entry := range entries {
			path := filepath.Join(current, entry.Name())
			if entry.IsDir() {
				dirs = append(dirs, browserItem{path, entry.Name() + "/", true})
			} else if supportedArchive(path) {
				files = append(files, browserItem{path, entry.Name(), false})
			}
		}
		sort.Slice(dirs, func(i, j int) bool { return strings.ToLower(dirs[i].name) < strings.ToLower(dirs[j].name) })
		sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i].name) < strings.ToLower(files[j].name) })
		items = append(items, dirs...)
		items = append(items, files...)
		menu := interactiveMenu{Heading: "Browse files  /  " + current, ConfirmHint: "Open selected item", Shortcuts: []string{"e Enter path"}, TextInputLabel: "Path"}
		for _, item := range items {
			detail := "archive"
			if item.dir {
				detail = "folder"
			}
			menu.Items = append(menu.Items, interactiveMenuItem{Label: item.name, Detail: detail})
		}
		choice, err := chooseInteractiveMenu(cmd, ctx.Resolved.Config.Name, menu)
		if err != nil {
			return "", err
		}
		if choice.Index >= 0 {
			v := items[choice.Index]
			if v.dir {
				current = v.path
				continue
			}
			rememberUpdateDir(ctx.Global, filepath.Dir(v.path))
			return v.path, nil
		}
		switch choice.Key {
		case "q":
			return "", fmt.Errorf("update selection cancelled")
		case "esc":
			current = filepath.Dir(current)
		case "e":
			path, e := choice.Text, error(nil)
			if path == "" {
				path, e = readMenuText(cmd, "Path (Esc to back)")
			}
			if errors.Is(e, errTextInputCancelled) {
				continue
			}
			if e != nil {
				return "", e
			}
			if strings.TrimSpace(path) == "" {
				continue
			}
			path = expandUserPath(strings.TrimSpace(path))
			info, e := os.Stat(path)
			if e != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Cannot open: %v\n", e)
				continue
			}
			if info.IsDir() {
				current = path
				continue
			}
			if !supportedArchive(path) {
				fmt.Fprintln(cmd.OutOrStdout(), "Unsupported archive format.")
				continue
			}
			rememberUpdateDir(ctx.Global, filepath.Dir(path))
			return path, nil
		}
	}
}
