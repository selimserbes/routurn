package cli

import (
	"fmt"

	"github.com/selimserbes/routurn/internal/runstate"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var detach bool
	cmd := &cobra.Command{
		Use:   "run <task>",
		Short: "Run a configured task on the remote target",
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

			if detach {
				result := runTaskDetached(ctx, taskName, runstate.Manifest{})
				if result.Err != nil {
					fmt.Fprintln(cmd.OutOrStdout(), "✗ Failed to start detached task")
					return result.Err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Run      %s\n", result.Manifest.ID)
				fmt.Fprintf(cmd.OutOrStdout(), "PID      %d\n", result.Manifest.RemotePID)
				fmt.Fprintln(cmd.OutOrStdout(), "✓ Detached task started")
				fmt.Fprintf(cmd.OutOrStdout(), "Watch    routurn logs %s --follow\n", result.Manifest.ID)
				fmt.Fprintf(cmd.OutOrStdout(), "Status   routurn status %s\n", result.Manifest.ID)
				return nil
			}

			result := runTask(ctx, taskName, runstate.Manifest{}, stdinForTask(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if result.RunDir != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Result   %s\n", result.RunDir)
			}
			if result.Err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "✗ Task failed")
				return result.Err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "✓ Completed successfully")
			return nil
		},
	}
	cmd.Flags().BoolVar(&detach, "detach", false, "start the remote task in the background and return immediately")
	return cmd
}
