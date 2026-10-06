package cli

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func chooseRunnableInteractive(cmd *cobra.Command, ctx *projectContext) (runnableCommand, error) {
	allChoices := discoverRunnableCommands(ctx.Resolved.Root, ctx.Resolved.Config.Tasks)
	choices := defaultRunnableCommands(allChoices)
	showAll := false
	reader := bufio.NewReader(cmd.InOrStdin())

	for {
		fmt.Fprintf(cmd.OutOrStdout(), "Routurn · %s\n\n", ctx.Resolved.Config.Name)
		fmt.Fprintln(cmd.OutOrStdout(), "Choose what to run")
		printRunnableChoices(cmd, choices, showAll)

		customIndex := len(choices) + 1
		showAllIndex := 0
		fmt.Fprintln(cmd.OutOrStdout(), "  Other")
		fmt.Fprintf(cmd.OutOrStdout(), "  %d) Enter a command\n", customIndex)
		if !showAll && hasHiddenRunnableCommands(allChoices) {
			showAllIndex = customIndex + 1
			fmt.Fprintf(cmd.OutOrStdout(), "  %d) Show all detected commands\n", showAllIndex)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "  q) Cancel")

		fmt.Fprint(cmd.OutOrStdout(), "> ")
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			if err == io.EOF {
				return runnableCommand{}, fmt.Errorf("interactive command selection needs terminal input; use 'routurn exec -- <command>'")
			}
			return runnableCommand{}, err
		}
		value := strings.TrimSpace(line)
		if strings.EqualFold(value, "q") || strings.EqualFold(value, "quit") {
			return runnableCommand{}, fmt.Errorf("command selection cancelled")
		}
		n, convErr := strconv.Atoi(value)
		if convErr != nil {
			fmt.Fprintln(cmd.OutOrStdout(), "Enter a menu number or q.")
			continue
		}
		if n >= 1 && n <= len(choices) {
			return choices[n-1], nil
		}
		if n == customIndex {
			fmt.Fprint(cmd.OutOrStdout(), "Command: ")
			raw, readErr := reader.ReadString('\n')
			if readErr != nil && len(raw) == 0 {
				return runnableCommand{}, readErr
			}
			raw = strings.TrimSpace(raw)
			if raw == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "Command cannot be empty.")
				continue
			}
			return runnableCommand{Name: commandLabel(raw), Command: raw, Source: "Custom", Description: "custom command"}, nil
		}
		if showAllIndex != 0 && n == showAllIndex {
			choices = allChoices
			showAll = true
			fmt.Fprintln(cmd.OutOrStdout())
			continue
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Unknown selection.")
	}
}

func printRunnableChoices(cmd *cobra.Command, choices []runnableCommand, showHiddenReason bool) {
	lastSource := ""
	for i, choice := range choices {
		if choice.Source != lastSource {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", choice.Source)
			lastSource = choice.Source
		}
		desc := strings.TrimSpace(choice.Description)
		if showHiddenReason && choice.Hidden && choice.HiddenReason != "" {
			if desc != "" {
				desc += " · "
			}
			desc += choice.HiddenReason
		}
		if desc != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  %d) %-22s %s\n", i+1, runnableLabel(choice), desc)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  %d) %s\n", i+1, runnableLabel(choice))
		}
		if verbose {
			fmt.Fprintf(cmd.OutOrStdout(), "      %s\n", choice.Command)
		}
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
