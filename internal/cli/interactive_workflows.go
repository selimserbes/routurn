package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/selimserbes/routurn/internal/runstate"
	"github.com/spf13/cobra"
)

// orderByRecentRuns uses Routurn's real run history. No task-name parsing or
// implicit semantics (stage, deploy, version, etc.) are involved.
func orderByRecentRuns(root string, items []runnableCommand) []runnableCommand {
	runs, err := runstate.List(root)
	if err != nil || len(runs) == 0 {
		return items
	}
	priority := map[string]int{}
	for _, run := range runs {
		if run.Task == "" {
			continue
		}
		if _, ok := priority[run.Task]; !ok {
			priority[run.Task] = len(priority)
		}
	}
	result := append([]runnableCommand(nil), items...)
	score := func(item runnableCommand) int {
		if n, ok := priority[item.Name]; ok {
			return n
		}
		if n, ok := priority[commandLabel(item.Command)]; ok {
			return n
		}
		return len(priority) + 1
	}
	// Preserve the original deterministic alpha order for tasks with no history.
	for i := 1; i < len(result); i++ {
		v := result[i]
		s := score(v)
		j := i - 1
		for j >= 0 && score(result[j]) > s {
			result[j+1] = result[j]
			j--
		}
		result[j+1] = v
	}
	return result
}
func runnableMenu(items []runnableCommand, labels map[string]string, heading string, shortcuts []string) interactiveMenu {
	m := interactiveMenu{Heading: "Run a task  /  " + heading, Shortcuts: shortcuts, ConfirmHint: "Run selected task", TextInputLabel: "Command"}
	for _, item := range items {
		label := item.Name
		if v := strings.TrimSpace(labels[item.Name]); v != "" {
			label = v
		}
		detail := ""
		if label != item.Name {
			detail = item.Name
		}
		if item.Saved && detail == "" {
			detail = "saved"
		}
		m.Items = append(m.Items, interactiveMenuItem{Label: label, Detail: detail})
	}
	return m
}
func chooseRunnableTTY(cmd *cobra.Command, ctx *projectContext) (runnableCommand, error) {
	var all, visible []runnableCommand
	var groups []pickerGroup
	var unassigned []runnableCommand
	var discoverDuration, historyDuration time.Duration
	withPickerLoading(cmd.OutOrStdout(), pickerIsInteractiveTerminal(cmd), "Discovering project tasks...", func() {
		start := time.Now()
		all = rankRunnableChoices(discoverRunnableCommands(ctx.Resolved.Root, ctx.Resolved.Config.Tasks))
		discoverDuration = time.Since(start)
		start = time.Now()
		visible = orderByRecentRuns(ctx.Resolved.Root, defaultRunnableCommands(all))
		groups, unassigned = groupPickerTasks(visible, ctx.Resolved.Config.Picker)
		historyDuration = time.Since(start)
	})
	defer func() {
		pickerReportTiming(cmd.ErrOrStderr(), "exec", pickerTiming{"discovery", discoverDuration}, pickerTiming{"history+groups", historyDuration})
	}()
	picker := ctx.Resolved.Config.Picker
	homeMode, homeGroup := "tasks", 0
	for i, g := range groups {
		if picker.DefaultGroup != "" && strings.EqualFold(picker.DefaultGroup, g.Name) {
			homeMode = "group"
			homeGroup = i
			break
		}
	}
	mode, group := "tasks", homeGroup
	if homeMode == "group" {
		mode = "group"
	}
	for {
		heading := "Tasks"
		items := visible
		switch mode {
		case "group":
			heading, items = groups[group].Name, groups[group].Items
		case "all":
			heading, items = "All tasks", all
		case "ungrouped":
			heading, items = "Ungrouped", unassigned
		}
		opts := []string{"a All tasks"}
		if len(groups) > 0 {
			opts = append(opts, "h Groups")
			if len(unassigned) > 0 {
				opts = append(opts, "u Ungrouped")
			}
		}
		if mode != homeMode || (mode == "group" && group != homeGroup) {
			opts = append(opts, "g Home")
		}
		opts = append(opts, "e Custom command")
		if mode == "groups" {
			m := interactiveMenu{Heading: "Browse groups", Shortcuts: opts, ConfirmHint: "Open selected group", TextInputLabel: "Command"}
			m.SearchItems = runnableMenu(visible, picker.Labels, "Tasks", nil).Items
			for _, g := range groups {
				m.Items = append(m.Items, interactiveMenuItem{Label: g.Name, Detail: fmt.Sprintf("%d tasks", len(g.Items))})
			}
			sel, err := chooseInteractiveMenu(cmd, ctx.Resolved.Config.Name, m)
			if err != nil {
				return runnableCommand{}, err
			}
			if sel.Index >= 0 {
				if sel.FromSearchAll {
					return visible[sel.Index], nil
				}
				group = sel.Index
				mode = "group"
				continue
			}
			switch sel.Key {
			case "q":
				return runnableCommand{}, fmt.Errorf("command selection cancelled")
			case "esc":
				mode = homeMode
				group = homeGroup
			case "a":
				mode = "all"
			case "g":
				mode = homeMode
				group = homeGroup
			case "u":
				mode = "ungrouped"
			case "e":
				if sel.Text != "" {
					return parseCustomRunnable(sel.Text)
				}
				selected, inputErr := readCustomRunnable(cmd)
				if errors.Is(inputErr, errTextInputCancelled) {
					continue
				}
				return selected, inputErr
			default:
				mode = "groups"
			}
			continue
		}
		m := runnableMenu(items, picker.Labels, heading, opts)
		if mode != "all" && mode != "tasks" {
			m.SearchItems = runnableMenu(visible, picker.Labels, "Tasks", nil).Items
		}
		sel, err := chooseInteractiveMenu(cmd, ctx.Resolved.Config.Name, m)
		if err != nil {
			return runnableCommand{}, err
		}
		if sel.Index >= 0 {
			if sel.FromSearchAll {
				return visible[sel.Index], nil
			}
			return items[sel.Index], nil
		}
		switch sel.Key {
		case "q":
			return runnableCommand{}, fmt.Errorf("command selection cancelled")
		case "esc":
			if mode == homeMode && group == homeGroup {
				return runnableCommand{}, fmt.Errorf("command selection cancelled")
			}
			mode = homeMode
			group = homeGroup
		case "h":
			if len(groups) > 0 {
				mode = "groups"
			}
		case "u":
			if len(groups) > 0 {
				mode = "ungrouped"
			}
		case "a":
			mode = "all"
		case "g":
			mode = homeMode
			group = homeGroup
		case "e":
			if sel.Text != "" {
				return parseCustomRunnable(sel.Text)
			}
			selected, inputErr := readCustomRunnable(cmd)
			if errors.Is(inputErr, errTextInputCancelled) {
				continue
			}
			return selected, inputErr
		}
	}
}
func readCustomRunnable(cmd *cobra.Command) (runnableCommand, error) {
	raw, err := readMenuText(cmd, "Command (Esc to back)")
	if err != nil {
		return runnableCommand{}, err
	}
	return parseCustomRunnable(raw)
}
func parseCustomRunnable(raw string) (runnableCommand, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return runnableCommand{}, fmt.Errorf("empty custom command")
	}
	return runnableCommand{Name: commandLabel(raw), Command: raw, Source: "Custom", Description: "custom command"}, nil
}
