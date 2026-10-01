package cli

import (
	"fmt"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Safely synchronize project changes to the configured remote target",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ctx *projectContext
			var err error
			if dryRun {
				ctx, err = resolveLocalProjectContext()
				if err == nil {
					if ctx.Resolved.Config.Remote.Target == "" || ctx.Resolved.Config.Remote.Path == "" {
						return fmt.Errorf("project remote target/path is not configured in %s", config.ProjectFileName)
					}
					if _, ok := ctx.Global.Targets[ctx.Resolved.Config.Remote.Target]; !ok {
						return fmt.Errorf("target %q is not registered; use 'routurn target add ...'", ctx.Resolved.Config.Remote.Target)
					}
				}
			} else {
				ctx, err = resolveProjectContext()
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Routurn · %s\n", ctx.Resolved.Config.Name)
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Target   %s\n", ctx.Resolved.Config.Remote.Target)
			} else {
				printResolvedTarget(cmd.OutOrStdout(), ctx)
			}
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
