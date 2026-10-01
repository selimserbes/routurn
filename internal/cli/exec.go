package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/selimserbes/routurn/internal/runstate"
	"github.com/spf13/cobra"
)

func newExecCmd() *cobra.Command {
	var noSync bool
	var noFetch bool
	cmd := &cobra.Command{
		Use:   "exec <task>",
		Short: "Sync changes, run a remote task with live output, and fetch its artifacts",
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

			runID := runstate.NewID()
			manifest := runstate.Manifest{
				ID:        runID,
				Project:   ctx.Resolved.Config.Name,
				Target:    ctx.Resolved.Config.Remote.Target,
				Task:      taskName,
				Status:    "SYNCING",
				StartedAt: time.Now().UTC().Format(time.RFC3339),
			}
			runDir, err := runstate.Create(ctx.Resolved.Root, manifest)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Routurn · %s\n", ctx.Resolved.Config.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "Target   %s\n", ctx.Resolved.Config.Remote.Target)
			fmt.Fprintf(cmd.OutOrStdout(), "Task     %s\n", taskName)
			fmt.Fprintf(cmd.OutOrStdout(), "Run      %s\n", runID)

			var synced syncResult
			if !noSync {
				fmt.Fprintln(cmd.OutOrStdout(), "\n[1/3] Sync")
				synced, err = performSync(ctx, cmd.OutOrStdout(), false, runID)
				if err != nil {
					manifest.Status = "FAILED"
					manifest.FinishedAt = time.Now().UTC().Format(time.RFC3339)
					_ = runstate.Save(ctx.Resolved.Root, manifest)
					return err
				}
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "\n[1/3] Sync skipped")
			}
			manifest.Changed = synced.Changed
			manifest.Deleted = synced.Deleted
			manifest.Snapshot = synced.Snapshot
			_ = runstate.Save(ctx.Resolved.Root, manifest)

			fmt.Fprintln(cmd.OutOrStdout(), "\n[2/3] Run")
			result := runTask(ctx, taskName, manifest, stdinForTask(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if result.RunDir == "" {
				return result.Err
			}

			if !noFetch {
				fmt.Fprintln(cmd.OutOrStdout(), "\n[3/3] Fetch artifacts")
				artifactDir := filepath.Join(result.RunDir, "artifacts")
				files, fetchErr := fetchTaskArtifacts(ctx, taskName, artifactDir)
				if fetchErr != nil {
					if result.Err == nil {
						result.Manifest.Status = "FAILED"
						result.Manifest.FinishedAt = time.Now().UTC().Format(time.RFC3339)
						_ = runstate.Save(ctx.Resolved.Root, result.Manifest)
						return fetchErr
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "! artifact fetch failed: %v\n", fetchErr)
				} else {
					result.Manifest.Artifacts = files
					if len(files) == 0 {
						fmt.Fprintln(cmd.OutOrStdout(), "No matching artifacts found.")
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "✓ %d artifact(s) collected\n", len(files))
					}
				}
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "\n[3/3] Artifact fetch skipped")
			}
			_ = runstate.Save(ctx.Resolved.Root, result.Manifest)

			fmt.Fprintln(cmd.OutOrStdout(), "\n────────────────────────────────────────")
			if result.Err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "✗ Failed · run %s\n", result.Manifest.ID)
				fmt.Fprintf(cmd.OutOrStdout(), "Result   %s\n", runDir)
				return result.Err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Completed successfully · run %s\n", result.Manifest.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "Result   %s\n", runDir)
			return nil
		},
	}
	cmd.Flags().BoolVar(&noSync, "no-sync", false, "run without synchronizing local changes first")
	cmd.Flags().BoolVar(&noFetch, "no-fetch", false, "do not fetch declared task artifacts")
	return cmd
}
