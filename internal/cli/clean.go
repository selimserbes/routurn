package cli

import (
	"fmt"

	"github.com/selimserbes/routurn/internal/intake"
	"github.com/selimserbes/routurn/internal/remote"
	"github.com/selimserbes/routurn/internal/retention"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	var dryRun bool
	var keepRuns int
	var keepUpdates int
	var keepBundles int
	var keepSnapshots int
	var keepRemoteRuns int
	var localOnly bool
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Prune old Routurn-managed history and bundle storage",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := resolveLocalProjectContext()
			if err != nil {
				return err
			}
			report, err := retention.PruneProject(ctx.Resolved.Root, keepRuns, keepUpdates, dryRun)
			if err != nil {
				return err
			}
			bundles, err := intake.Prune(keepBundles, dryRun)
			if err != nil {
				return err
			}
			verb := "Removed"
			if dryRun {
				verb = "Would remove"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d run(s), %d update backup(s), %d managed bundle(s)\n", verb, len(report.Runs), len(report.Updates), len(bundles))
			if verbose || dryRun {
				for _, path := range report.Runs {
					fmt.Fprintf(cmd.OutOrStdout(), "  run     %s\n", path)
				}
				for _, path := range report.Updates {
					fmt.Fprintf(cmd.OutOrStdout(), "  update  %s\n", path)
				}
				for _, path := range bundles {
					fmt.Fprintf(cmd.OutOrStdout(), "  bundle  %s\n", path)
				}
			}

			if !localOnly && ctx.Resolved.Config.Remote.Target != "" && ctx.Resolved.Config.Remote.Path != "" {
				if dryRun {
					fmt.Fprintln(cmd.OutOrStdout(), "Remote cleanup is not executed during --dry-run; automatic retention remains enabled after sync/detached starts.")
				} else {
					selected, selectErr := remote.ResolveEndpoint(ctx.Global, ctx.Resolved.Config.Remote.Target, endpointOverride)
					if selectErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "! remote cleanup skipped: %v\n", selectErr)
					} else {
						snapshots, snapErr := remote.PruneSnapshots(selected.Endpoint, ctx.Resolved.Config.Remote.Path, keepSnapshots)
						runs, runErr := remote.PruneDetachedRuns(selected.Endpoint, ctx.Resolved.Config.Remote.Path, keepRemoteRuns)
						if snapErr != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "! remote snapshot cleanup: %v\n", snapErr)
						}
						if runErr != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "! remote detached-run cleanup: %v\n", runErr)
						}
						if snapErr == nil && runErr == nil {
							fmt.Fprintf(cmd.OutOrStdout(), "Remote (%s): removed %d old snapshot(s), %d old completed detached run(s)\n", selected.EndpointName, len(snapshots), len(runs))
						}
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show local cleanup candidates without deleting anything")
	cmd.Flags().IntVar(&keepRuns, "keep-runs", retention.DefaultRuns, "number of newest local run histories to keep")
	cmd.Flags().IntVar(&keepUpdates, "keep-updates", retention.DefaultUpdates, "number of newest update backups to keep")
	cmd.Flags().IntVar(&keepBundles, "keep-bundles", 20, "number of newest managed update bundles to keep")
	cmd.Flags().IntVar(&keepSnapshots, "keep-snapshots", remote.DefaultSnapshotRetention, "number of newest remote sync snapshots to keep")
	cmd.Flags().IntVar(&keepRemoteRuns, "keep-remote-runs", remote.DefaultDetachedRetention, "number of newest completed remote detached-run states to keep")
	cmd.Flags().BoolVar(&localOnly, "local-only", false, "skip remote cleanup")
	return cmd
}
