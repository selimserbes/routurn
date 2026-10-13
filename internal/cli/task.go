package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/selimserbes/routurn/internal/project"
	"github.com/spf13/cobra"
)

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage local command shortcuts",
	}
	cmd.AddCommand(newTaskListCmd(), newTaskSaveCmd(), newTaskRemoveCmd())
	return cmd
}

func newTaskListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List saved and configured task shortcuts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := project.Resolve(projectName)
			if err != nil {
				return err
			}
			if len(resolved.Config.Tasks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No saved tasks. Use 'routurn exec' to discover commands or 'routurn task save <name> -- <command>'.")
				return nil
			}
			names := make([]string, 0, len(resolved.Config.Tasks))
			for name := range resolved.Config.Tasks {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %s\n", name, resolved.Config.Tasks[name].Command)
			}
			return nil
		},
	}
}

func newTaskSaveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "save <name> -- <command>",
		Short: "Save a command shortcut without editing routurn.toml",
		Args: func(cmd *cobra.Command, args []string) error {
			if cmd.ArgsLenAtDash() != 1 || len(args) < 2 {
				return fmt.Errorf("usage: routurn task save <name> -- <command>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := project.Resolve(projectName)
			if err != nil {
				return err
			}
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("task name cannot be empty")
			}
			dash := cmd.ArgsLenAtDash()
			command := shellJoin(args[dash:])
			task := config.Task{Command: command}
			if err := config.SaveLocalTask(resolved.Root, name, task); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Saved task %q\n", name)
			fmt.Fprintf(cmd.OutOrStdout(), "Run: routurn exec %s\n", name)
			return nil
		},
	}
	return cmd
}

func newTaskRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a Routurn-managed local task shortcut",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := project.Resolve(projectName)
			if err != nil {
				return err
			}
			removed, err := config.RemoveLocalTask(resolved.Root, args[0])
			if err != nil {
				return err
			}
			if !removed {
				return fmt.Errorf("local task %q was not found", args[0])
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Removed local task %q\n", args[0])
			return nil
		},
	}
}
