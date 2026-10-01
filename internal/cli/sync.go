package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Safely synchronize project changes to the configured remote target",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := resolveProjectContext()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Routurn · %s\n", ctx.Resolved.Config.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "Target   %s\n", ctx.Resolved.Config.Remote.Target)
			fmt.Fprintf(cmd.OutOrStdout(), "Remote   %s\n", ctx.Resolved.Config.Remote.Path)
			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "Mode     dry-run")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "────────────────────────────────────────")
			_, err = performSync(ctx, cmd.OutOrStdout(), dryRun, "")
			return err
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show the sync plan without changing the remote project")
	return cmd
}
