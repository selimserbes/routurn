package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/selimserbes/routurn/internal/intake"
	resultstore "github.com/selimserbes/routurn/internal/result"
	"github.com/selimserbes/routurn/internal/retention"
	"github.com/selimserbes/routurn/internal/runstate"
	"github.com/spf13/cobra"
)

func newExecCmd() *cobra.Command {
	var noSync bool
	var noFetch bool
	var yes bool
	var stripComponents int
	var detach bool
	var updateSpec string
	var keepUpdateSource bool

	cmd := &cobra.Command{
		Use:   "exec <task> [legacy-update-archive]",
		Short: "Optionally apply an update, sync changes, run a remote task, and fetch artifacts",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := resolveProjectContext()
			if err != nil {
				return err
			}
			taskName := args[0]
			task, ok := ctx.Resolved.Config.Tasks[taskName]
			if !ok {
				return fmt.Errorf("task %q is not defined in routurn.toml", taskName)
			}
			if detach && task.Interactive {
				return fmt.Errorf("task %q is interactive and cannot be detached", taskName)
			}
			effectiveUpdateSpec := updateSpec
			legacyArchive := ""
			if len(args) == 2 {
				if updateSpec == "select" {
					// pflag optional-value flags do not consume the following token.
					// Treat it as the --update value so both "--update recent" and
					// "--update /path/file.zip" remain natural CLI forms.
					effectiveUpdateSpec = args[1]
				} else if updateSpec != "" {
					return fmt.Errorf("use either the positional update archive or --update, not both")
				} else {
					legacyArchive = args[1]
				}
			}

			hasUpdate := legacyArchive != "" || effectiveUpdateSpec != ""
			totalSteps := 3
			if detach {
				totalSteps = 2
			}
			if hasUpdate {
				totalSteps++
			}
			var updateID, updateArchive string
			if hasUpdate {
				fmt.Fprintf(cmd.OutOrStdout(), "[1/%d] Apply update\n", totalSteps)
				archive := ""
				opts := applyOptions{Yes: yes, StripComponents: stripComponents}

				if effectiveUpdateSpec != "" {
					archive, err = resolveUpdateInput(cmd, ctx, effectiveUpdateSpec, stripComponents)
					if err != nil {
						return err
					}
					archive = expandUserPath(archive)
					effectiveStrip := stripComponents
					if effectiveStrip == 0 {
						if managed, ok, metaErr := intake.FindByPath(archive); metaErr == nil && ok {
							effectiveStrip = managed.StripComponents
						}
					}
					opts.StripComponents = effectiveStrip
					manifest, compatibility, compatErr := validateUpdateCompatibility(ctx, archive, effectiveStrip)
					if compatErr != nil {
						return compatErr
					}
					if manifest == nil {
						fmt.Fprintln(cmd.OutOrStdout(), "! Legacy update: no routurn-bundle.toml; project/base identity cannot be verified")
					} else if compatibility == updateCompatibilityScoped {
						fmt.Fprintln(cmd.OutOrStdout(), "✓ Bundle target-file preconditions match; unrelated project changes are allowed")
					} else if compatibility == updateCompatibilityUnverified {
						fmt.Fprintln(cmd.OutOrStdout(), "! Bundle identifies the project but has no verifiable base state; review the plan before applying")
					}
					rememberArchiveDir(ctx.Global, archive)
					imported, importErr := intake.Import(archive, intake.Options{Consume: !keepUpdateSource, StripComponents: opts.StripComponents})
					if importErr != nil {
						return importErr
					}
					name := imported.OriginalName
					if imported.Manifest != nil && imported.Manifest.Bundle.Name != "" {
						name = imported.Manifest.Bundle.Name
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Bundle   %s\n", name)
					if imported.Duplicate {
						fmt.Fprintln(cmd.OutOrStdout(), "✓ Existing managed copy reused")
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), "✓ Imported into Routurn managed storage")
					}
					if imported.SourceRemoved {
						fmt.Fprintln(cmd.OutOrStdout(), "✓ Original source removed after verified import")
					}
					if imported.Warning != "" {
						fmt.Fprintf(cmd.ErrOrStderr(), "! %s\n", imported.Warning)
					}
					archive = imported.Path
					opts.ManagedHash = imported.Hash
					opts.ManagedName = name
				} else {
					archive = legacyArchive
					fmt.Fprintln(cmd.OutOrStdout(), "! Positional update archives are kept for compatibility; prefer --update")
				}

				record, applied, applyErr := applyBundle(cmd, ctx.Resolved.Root, archive, opts)
				if applyErr != nil {
					return applyErr
				}
				if !applied && record.ID == "" {
					return nil
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
				Endpoint:      ctx.EndpointName,
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
			defer func() {
				_, _ = retention.PruneProject(ctx.Resolved.Root, retention.DefaultRuns, retention.DefaultUpdates, false)
				_ = intake.PruneDefault()
			}()

			fmt.Fprintf(cmd.OutOrStdout(), "Routurn · %s\n", ctx.Resolved.Config.Name)
			printResolvedTarget(cmd.OutOrStdout(), ctx)
			fmt.Fprintf(cmd.OutOrStdout(), "Task     %s\n", taskName)
			if verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "Run      %s\n", runID)
			}

			var synced syncResult
			syncStep := 1
			runStep := 2
			fetchStep := 3
			if hasUpdate {
				syncStep++
				runStep++
				fetchStep++
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
			if detach {
				result := runTaskDetached(ctx, taskName, manifest)
				if result.Err != nil {
					return result.Err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "\n────────────────────────────────────────")
				fmt.Fprintln(cmd.OutOrStdout(), "✓ Detached run started")
				fmt.Fprintf(cmd.OutOrStdout(), "PID      %d\n", result.Manifest.RemotePID)
				fmt.Fprintf(cmd.OutOrStdout(), "Watch    routurn logs %s --follow\n", taskName)
				fmt.Fprintf(cmd.OutOrStdout(), "Status   routurn status %s\n", taskName)
				fmt.Fprintf(cmd.OutOrStdout(), "Fetch    routurn fetch %s\n", taskName)
				if verbose {
					fmt.Fprintf(cmd.OutOrStdout(), "Run      %s\n", result.Manifest.ID)
				}
				return nil
			}

			result := runTask(ctx, taskName, manifest, stdinForTask(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if result.RunDir == "" {
				return result.Err
			}

			stableResult := ""
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
						if result.Err == nil {
							stableResult, _, err = resultstore.Materialize(ctx.Resolved.Root, taskName, result.Manifest.ID, artifactDir)
							if err != nil {
								return fmt.Errorf("materialize task result: %w", err)
							}
						}
					}
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "\n[%d/%d] Artifact fetch skipped\n", fetchStep, totalSteps)
			}
			_ = runstate.Save(ctx.Resolved.Root, result.Manifest)

			fmt.Fprintln(cmd.OutOrStdout(), "\n────────────────────────────────────────")
			if result.Err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "✗ Failed")
				if verbose {
					fmt.Fprintf(cmd.OutOrStdout(), "History  %s\n", runDir)
				}
				return result.Err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "✓ Completed successfully")
			fmt.Fprintf(cmd.OutOrStdout(), "Task     %s\n", taskName)
			if stableResult != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Result   %s\n", stableResult)
			} else if verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "History  %s\n", runDir)
			}
			if verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "Run      %s\n", result.Manifest.ID)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noSync, "no-sync", false, "run without synchronizing local changes first")
	cmd.Flags().BoolVar(&noFetch, "no-fetch", false, "do not fetch declared task artifacts")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "apply an update archive without confirmation")
	cmd.Flags().IntVar(&stripComponents, "strip-components", 0, "remove leading path components from update archive entries")
	cmd.Flags().BoolVar(&detach, "detach", false, "start the remote task in the background; artifact fetching is deferred")
	cmd.Flags().StringVar(&updateSpec, "update", "", "select/import an update (no value opens chooser; use recent, latest, or a path)")
	if flag := cmd.Flags().Lookup("update"); flag != nil {
		flag.NoOptDefVal = "select"
	}
	cmd.Flags().BoolVar(&keepUpdateSource, "keep-update-source", false, "keep the original update archive after verified import")
	return cmd
}
