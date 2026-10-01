package cli

import (
	"fmt"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/selimserbes/routurn/internal/project"
	"github.com/selimserbes/routurn/internal/remote"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <task>",
		Short: "Run a configured task on the remote target with live terminal output",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := project.Resolve(projectName)
			if err != nil {
				return err
			}
			taskName := args[0]
			task, ok := resolved.Config.Tasks[taskName]
			if !ok {
				return fmt.Errorf("task %q is not defined in %s", taskName, config.ProjectFileName)
			}
			if resolved.Config.Remote.Target == "" || resolved.Config.Remote.Path == "" {
				return fmt.Errorf("project remote target/path is not configured in %s", config.ProjectFileName)
			}

			global, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			target, ok := global.Targets[resolved.Config.Remote.Target]
			if !ok {
				return fmt.Errorf("target %q is not registered; use 'routurn target add ...'", resolved.Config.Remote.Target)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Routurn · %s\n", resolved.Config.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "Target  %s\n", resolved.Config.Remote.Target)
			fmt.Fprintf(cmd.OutOrStdout(), "Task    %s\n", taskName)
			fmt.Fprintln(cmd.OutOrStdout(), "────────────────────────────────────────")

			if err := remote.Run(target, resolved.Config.Remote.Path, task.Command, task.Interactive); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "────────────────────────────────────────")
			fmt.Fprintln(cmd.OutOrStdout(), "✓ Completed successfully")
			return nil
		},
	}
}
