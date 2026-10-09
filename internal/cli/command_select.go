package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/spf13/cobra"
)

const commandPickerPageSize = 10

// Keep the picker readable in a narrow terminal without requiring an extra
// terminal-size dependency. COLUMNS is respected when the shell provides it;
// otherwise we use a conservative width that works on ordinary terminals.
func pickerDisplayWidth() int {
	width := 60
	if raw := os.Getenv("COLUMNS"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			width = parsed
		}
	}
	if width < 38 {
		return 38
	}
	if width > 76 {
		return 76
	}
	return width
}

// pickerColorEnabled only enables ANSI decoration for a real interactive TTY.
// NO_COLOR (even if empty), CLICOLOR=0 and TERM=dumb keep plain-text output.
func pickerColorEnabled(cmd *cobra.Command) bool {
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled || os.Getenv("CLICOLOR") == "0" {
		return false
	}
	return pickerIsInteractiveTerminal(cmd)
}

// pickerShortcutLabel returns both a printable-width label and its styled form.
// Calculate padding from the plain label, never from ANSI escape bytes.
func pickerShortcutLabel(option string, color bool) (plain, styled string) {
	key, description, ok := strings.Cut(option, " ")
	if !ok || key == "" {
		return option, option
	}
	plain = "[" + key + "] " + description
	if !color {
		return plain, plain
	}
	return plain, "\x1b[1;96m[" + key + "]\x1b[0m " + description
}

// pickerPrintOptions uses aligned navigation columns, with highlighted shortcut
// keys on color terminals and bracketed keys everywhere else.
func pickerPrintOptions(out io.Writer, width int, options []string, color bool) {
	if width < 56 {
		for _, option := range options {
			_, styled := pickerShortcutLabel(option, color)
			fmt.Fprintf(out, "  %s\n", styled)
		}
		return
	}
	columnWidth := width / 2
	for i := 0; i < len(options); i += 2 {
		leftPlain, leftStyled := pickerShortcutLabel(options[i], color)
		if i+1 >= len(options) {
			fmt.Fprintf(out, "  %s\n", leftStyled)
			continue
		}
		_, rightStyled := pickerShortcutLabel(options[i+1], color)
		padding := columnWidth - 2 - len([]rune(leftPlain))
		if padding < 1 {
			padding = 1
		}
		fmt.Fprintf(out, "  %s%s%s\n", leftStyled, strings.Repeat(" ", padding), rightStyled)
	}
}

// pickerPrintTask prints a configured display label when available; otherwise
// it shows the exact discovered name. Labels never change task identities.
func pickerPrintTask(out io.Writer, width, number int, item runnableCommand, displayLabel string) {
	prefix := fmt.Sprintf("  %2d) ", number)
	name := item.Name
	if item.Saved {
		name += "  · saved"
	}
	label := strings.TrimSpace(displayLabel)
	if label != "" && label != item.Name {
		combined := prefix + fmt.Sprintf("%-15s", label) + name
		if len([]rune(combined)) <= width {
			fmt.Fprintln(out, combined)
			return
		}
		fmt.Fprintln(out, prefix+label)
		prefix = "      "
	}
	// Keep discoverable task names intact even in narrow terminals; extremely
	// long names may naturally wrap in the user's terminal.
	fmt.Fprintln(out, prefix+name)
}

// Generic, deterministic ordering. No assumptions about stages, release numbers
// or project-specific command names.
func rankRunnableChoices(items []runnableCommand) []runnableCommand {
	out := append([]runnableCommand(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		an, bn := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if an != bn {
			return an < bn
		}
		return out[i].Saved && !out[j].Saved
	})
	return out
}

func searchRunnableChoices(items []runnableCommand, query string) []runnableCommand {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return items
	}
	out := make([]runnableCommand, 0)
	for _, item := range items {
		haystack := strings.ToLower(item.Name + " " + item.Source + " " + item.Description + " " + item.Command)
		if strings.Contains(haystack, q) {
			out = append(out, item)
		}
	}
	return out
}

func pickerPage(items []runnableCommand, page int) ([]runnableCommand, int) {
	if page < 0 {
		page = 0
	}
	if len(items) == 0 {
		return nil, 0
	}
	last := (len(items) - 1) / commandPickerPageSize
	if page > last {
		page = last
	}
	start := page * commandPickerPageSize
	end := start + commandPickerPageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], page
}

// pickerGroup holds a project-defined navigation group. Group names and task
// globs come solely from optional routurn.toml metadata.
type pickerGroup struct {
	Name  string
	Items []runnableCommand
}

func pickerMatchesTask(name string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if strings.EqualFold(name, pattern) {
			return true
		}
		match, err := filepath.Match(strings.ToLower(pattern), strings.ToLower(name))
		if err == nil && match {
			return true
		}
	}
	return false
}

// Tasks always remain visible through Tasks, Search or All tasks, even if no
// configured group matches them. A task can appear in more than one group.
func groupPickerTasks(items []runnableCommand, picker config.PickerConfig) ([]pickerGroup, []runnableCommand) {
	groups := make([]pickerGroup, 0, len(picker.Groups))
	patterns := make([][]string, 0, len(picker.Groups))
	unassigned := make([]runnableCommand, 0)
	for _, definition := range picker.Groups {
		if strings.TrimSpace(definition.Name) == "" {
			continue
		}
		groups = append(groups, pickerGroup{Name: definition.Name})
		patterns = append(patterns, definition.Tasks)
	}
	for _, item := range items {
		assigned := false
		for i := range groups {
			if pickerMatchesTask(item.Name, patterns[i]) {
				groups[i].Items = append(groups[i].Items, item)
				assigned = true
			}
		}
		if !assigned {
			unassigned = append(unassigned, item)
		}
	}
	return groups, unassigned
}

func pickerPageBounds(total, page int) (start, end, actual, pages int) {
	pages = 1
	if total > 0 {
		pages = (total + commandPickerPageSize - 1) / commandPickerPageSize
	}
	actual = page
	if actual < 0 {
		actual = 0
	}
	if actual >= pages {
		actual = pages - 1
	}
	start = actual * commandPickerPageSize
	end = start + commandPickerPageSize
	if end > total {
		end = total
	}
	return
}

// Clearing the screen is reserved for real terminal sessions. Captured output and
// piped input remain plain text, so scripts and tests are unaffected.
func pickerIsInteractiveTerminal(cmd *cobra.Command) bool {
	input, inputOK := cmd.InOrStdin().(*os.File)
	output, outputOK := cmd.OutOrStdout().(*os.File)
	if !inputOK || !outputOK || os.Getenv("TERM") == "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	inInfo, inErr := input.Stat()
	outInfo, outErr := output.Stat()
	return inErr == nil && outErr == nil && inInfo.Mode()&os.ModeCharDevice != 0 && outInfo.Mode()&os.ModeCharDevice != 0
}

// chooseRunnableInteractive lists all discovered tasks by default. Projects can
// optionally choose a home group and friendly labels via [picker] in routurn.toml.
// Neither discovery nor command execution depends on these display settings.
func chooseRunnableInteractive(cmd *cobra.Command, ctx *projectContext) (runnableCommand, error) {
	if pickerIsInteractiveTerminal(cmd) {
		return chooseRunnableTTY(cmd, ctx)
	}
	all := rankRunnableChoices(discoverRunnableCommands(ctx.Resolved.Root, ctx.Resolved.Config.Tasks))
	visible := defaultRunnableCommands(all)
	picker := ctx.Resolved.Config.Picker
	groups, unassigned := groupPickerTasks(visible, picker)
	homeMode, homeGroup := "tasks", 0
	for i, group := range groups {
		if picker.DefaultGroup != "" && strings.EqualFold(group.Name, picker.DefaultGroup) {
			homeMode, homeGroup = "group", i
			break
		}
	}
	mode, currentGroup := homeMode, homeGroup
	page, search := 0, ""
	reader := bufio.NewReader(cmd.InOrStdin())
	out := cmd.OutOrStdout()
	clearScreen := pickerIsInteractiveTerminal(cmd)
	color := pickerColorEnabled(cmd)
	width := pickerDisplayWidth()
	notice := ""

	for {
		var choices []runnableCommand
		heading := "Tasks"
		switch mode {
		case "tasks":
			choices = visible
		case "group":
			heading, choices = groups[currentGroup].Name, groups[currentGroup].Items
		case "groups":
			heading = "Browse groups"
		case "ungrouped":
			heading, choices = "Ungrouped tasks", unassigned
		case "search":
			heading, choices = "Search: "+search, searchRunnableChoices(visible, search)
		case "all":
			heading, choices = "All tasks (including helpers)", all
		}
		count := len(choices)
		if mode == "groups" {
			count = len(groups)
		}
		start, end, actualPage, pages := pickerPageBounds(count, page)
		page = actualPage
		if clearScreen {
			fmt.Fprint(out, "\x1b[H\x1b[2J")
		}
		fmt.Fprintf(out, "Routurn · %s\n", ctx.Resolved.Config.Name)
		fmt.Fprintln(out, strings.Repeat("─", width))
		fmt.Fprintf(out, "Run a task  /  %s\n", heading)
		if pages > 1 {
			fmt.Fprintf(out, "Page %d/%d\n", page+1, pages)
		}
		fmt.Fprintln(out)
		if mode == "groups" {
			for i := start; i < end; i++ {
				g := groups[i]
				fmt.Fprintf(out, "  %2d) %s  (%d tasks)\n", i-start+1, g.Name, len(g.Items))
			}
		} else {
			for i := start; i < end; i++ {
				pickerPrintTask(out, width, i-start+1, choices[i], picker.Labels[choices[i].Name])
			}
		}
		if count == 0 {
			fmt.Fprintln(out, "  No matching tasks. Try Search or All tasks.")
		}
		fmt.Fprintln(out, "\n"+strings.Repeat("─", width))
		fmt.Fprintln(out, "Shortcuts")
		options := []string{"/ Search tasks"}
		if len(groups) > 0 {
			options = append(options, fmt.Sprintf("h Groups (%d)", len(groups)))
			if len(unassigned) > 0 {
				options = append(options, fmt.Sprintf("u Ungrouped (%d)", len(unassigned)))
			}
		}
		options = append(options, "a All tasks")
		if mode != homeMode || (mode == "group" && currentGroup != homeGroup) {
			options = append(options, "g Home")
		}
		options = append(options, "e Custom command")
		if pages > 1 {
			options = append(options, "n Next", "p Previous")
		}
		if mode == "search" && search != "" {
			options = append(options, "c Clear search")
		}
		options = append(options, "q Exit")
		pickerPrintOptions(out, width, options, color)
		fmt.Fprintln(out)
		if notice != "" {
			fmt.Fprintf(out, "  %s\n\n", notice)
			notice = ""
		}
		if end > start {
			fmt.Fprintf(out, "Select a task [1-%d] or shortcut:\n", end-start)
		} else {
			fmt.Fprintln(out, "Select a shortcut:")
		}
		if color {
			fmt.Fprint(out, "  \x1b[1;96m>\x1b[0m ")
		} else {
			fmt.Fprint(out, "  > ")
		}
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			if err == io.EOF {
				return runnableCommand{}, fmt.Errorf("interactive command selection needs terminal input; use 'routurn exec -- <command>'")
			}
			return runnableCommand{}, err
		}
		value := strings.TrimSpace(line)
		switch strings.ToLower(value) {
		case "q", "quit":
			return runnableCommand{}, fmt.Errorf("command selection cancelled")
		case "h", "history", "groups":
			if len(groups) > 0 {
				mode, page = "groups", 0
			} else {
				notice = "No groups configured for this project."
			}
		case "u", "ungrouped":
			if len(groups) > 0 {
				mode, page = "ungrouped", 0
			} else {
				notice = "No groups configured for this project."
			}
		case "g", "b", "back", "home":
			mode, currentGroup, page = homeMode, homeGroup, 0
		case "a", "all":
			mode, page = "all", 0
		case "n", "next":
			if page+1 < pages {
				page++
			}
		case "p", "prev", "previous":
			if page > 0 {
				page--
			}
		case "s", "search", "/":
			fmt.Fprint(out, "Search task name or command: ")
			raw, readErr := reader.ReadString('\n')
			if readErr != nil && len(raw) == 0 {
				return runnableCommand{}, readErr
			}
			search, mode, page = strings.TrimSpace(raw), "search", 0
		case "c", "clear":
			search, mode, page = "", "search", 0
		case "e", "enter":
			fmt.Fprint(out, "Command: ")
			raw, readErr := reader.ReadString('\n')
			if readErr != nil && len(raw) == 0 {
				return runnableCommand{}, readErr
			}
			raw = strings.TrimSpace(raw)
			if raw == "" {
				notice = "Command cannot be empty."
				break
			}
			return runnableCommand{Name: commandLabel(raw), Command: raw, Source: "Custom", Description: "custom command"}, nil
		default:
			if strings.HasPrefix(value, "/") {
				search, mode, page = strings.TrimSpace(strings.TrimPrefix(value, "/")), "search", 0
				break
			}
			n, convErr := strconv.Atoi(value)
			if convErr != nil || n < 1 || n > end-start {
				notice = "Choose a shown number or a navigation option."
				break
			}
			if mode == "groups" {
				currentGroup, mode, page = start+n-1, "group", 0
			} else {
				return choices[start+n-1], nil
			}
		}
		fmt.Fprintln(out)
	}
}

func commandLabel(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "command"
	}
	label := fields[0]
	label = strings.TrimPrefix(label, "./")
	label = strings.ReplaceAll(label, "/", "-")
	var cleaned strings.Builder
	for _, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			cleaned.WriteRune(r)
		} else {
			cleaned.WriteRune('-')
		}
	}
	label = strings.Trim(cleaned.String(), "-._")
	if label == "" {
		return "command"
	}
	return "cmd-" + label
}
