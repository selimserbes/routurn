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
	var yes bool
	var stripComponents int
	cmd := &cobra.Command{
		Use:   "exec <task> [update-archive]",
		Short: "Optionally apply an update, sync changes, run a remote task, and fetch artifacts",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := resolveProjectContext()
			if err != nil {
				return err
			}
			taskName := args[0]
			if _, ok := ctx.Resolved.Config.Tasks[taskName]; !ok {
				return fmt.Errorf("task %q is not defined in routurn.toml", taskName)
			}

			hasUpdate := len(args) == 2
			totalSteps := 3
			if hasUpdate {
				totalSteps = 4
			}
			var updateID, updateArchive string
			if hasUpdate {
				fmt.Fprintf(cmd.OutOrStdout(), "[1/%d] Apply update\n", totalSteps)
				record, applied, err := applyBundle(cmd, ctx.Resolved.Root, args[1], applyOptions{Yes: yes, StripComponents: stripComponents})
				if err != nil {
					return err
				}
				if !applied {
					if record.ID == "" {
						return nil
					}
				}
				updateID = record.ID
				updateArchive = record.Archive
				fmt.Fprintln(cmd.OutOrStdout())
			}

			runID := runstate.NewID()
			manifest := runstate.Manifest{
				ID:            runID,
				Project:       ctx.Resolved.Config.Name,
				Target:        ctx.Resolved.Config.Remote.Target,
				Task:          taskName,
				Status:        "SYNCING",
				StartedAt:     time.Now().UTC().Format(time.RFC3339),
				UpdateID:      updateID,
				UpdateArchive: updateArchive,
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
			syncStep := 1
			runStep := 2
			fetchStep := 3
			if hasUpdate {
				syncStep, runStep, fetchStep = 2, 3, 4
			}
			if !noSync {
				fmt.Fprintf(cmd.OutOrStdout(), "\n[%d/%d] Sync\n", syncStep, totalSteps)
				synced, err = performSync(ctx, cmd.OutOrStdout(), false, runID)
				if err != nil {
					manifest.Status = "FAILED"
					manifest.FinishedAt = time.Now().UTC().Format(time.RFC3339)
					_ = runstate.Save(ctx.Resolved.Root, manifest)
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "\n[%d/%d] Sync skipped\n", syncStep, totalSteps)
			}
			manifest.Changed = synced.Changed
			manifest.Deleted = synced.Deleted
			manifest.Snapshot = synced.Snapshot
			_ = runstate.Save(ctx.Resolved.Root, manifest)

			fmt.Fprintf(cmd.OutOrStdout(), "\n[%d/%d] Run\n", runStep, totalSteps)
			result := runTask(ctx, taskName, manifest, stdinForTask(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if result.RunDir == "" {
				return result.Err
			}

			if !noFetch {
				fmt.Fprintf(cmd.OutOrStdout(), "\n[%d/%d] Fetch artifacts\n", fetchStep, totalSteps)
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
				fmt.Fprintf(cmd.OutOrStdout(), "\n[%d/%d] Artifact fetch skipped\n", fetchStep, totalSteps)
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
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "apply an update archive without confirmation")
	cmd.Flags().IntVar(&stripComponents, "strip-components", 0, "remove leading path components from update archive entries")
	return cmd
}
