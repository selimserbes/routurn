package cli

import (
	"fmt"

	"github.com/selimserbes/routurn/internal/runstate"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <task>",
		Short: "Run a configured task on the remote target with live terminal output",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := resolveProjectContext()
			if err != nil {
				return err
			}
			taskName := args[0]
			if _, ok := ctx.Resolved.Config.Tasks[taskName]; !ok {
				return fmt.Errorf("task %q is not defined in routurn.toml", taskName)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Routurn · %s\n", ctx.Resolved.Config.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "Target   %s\n", ctx.Resolved.Config.Remote.Target)
			fmt.Fprintf(cmd.OutOrStdout(), "Task     %s\n", taskName)

			result := runTask(ctx, taskName, runstate.Manifest{}, stdinForTask(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if result.RunDir != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Result   %s\n", result.RunDir)
			}
			if result.Err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "✗ Task failed\n")
				return result.Err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "✓ Completed successfully")
			return nil
		},
	}
}
