package cli

import (
	"fmt"

	"github.com/selimserbes/routurn/internal/remote"
	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "stop <run-id|latest>",
		Short: "Stop a detached remote run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := resolveRunRemote(args[0])
			if err != nil {
				return err
			}
			state, _, err := refreshDetachedRun(ctx)
			if err != nil {
				return err
			}
			if !state.Alive {
				fmt.Fprintf(cmd.OutOrStdout(), "Run %s is not running (state %s).\n", ctx.Manifest.ID, state.Status)
				return nil
			}
			if err := remote.StopDetached(ctx.Target, ctx.RemotePath, ctx.Manifest.ID, force); err != nil {
				return err
			}
			if force {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ Force-stop signal sent to run %s\n", ctx.Manifest.ID)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ Stop requested for run %s\n", ctx.Manifest.ID)
				fmt.Fprintf(cmd.OutOrStdout(), "Check    routurn status %s\n", ctx.Manifest.ID)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "send SIGKILL instead of a graceful SIGTERM request")
	return cmd
}
