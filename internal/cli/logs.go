package cli

import (
	"fmt"

	"github.com/selimserbes/routurn/internal/remote"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var follow bool
	var lines int
	cmd := &cobra.Command{
		Use:   "logs <task|run-id|latest>",
		Short: "Show logs for a detached remote run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := resolveRunRemote(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Task     %s\n", ctx.Manifest.Task)
			if verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "Run      %s\n", ctx.Manifest.ID)
			}
			if follow {
				fmt.Fprintln(cmd.OutOrStdout(), "Following remote logs. Ctrl+C disconnects without stopping the task.")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "────────────────────────────────────────")
			return remote.StreamDetachedLogs(ctx.Target, ctx.RemotePath, ctx.Manifest.ID, lines, follow, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow logs until the remote run finishes")
	cmd.Flags().IntVarP(&lines, "lines", "n", 100, "number of trailing lines to show before following")
	return cmd
}
