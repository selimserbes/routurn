package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/selimserbes/routurn/internal/config"
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
	var saveName string

	cmd := &cobra.Command{
		Use:   "exec [task] [legacy-update-archive] | exec -- <command>",
		Short: "Discover or run a command locally or remotely, with optional update/sync/artifact workflow",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Selecting a task is entirely local. Resolve the SSH route only after
			// a task is chosen, not while the user is browsing the menu.
			dash := cmd.ArgsLenAtDash()
			interactiveChoice := dash < 0 && len(args) == 0
			var ctx *projectContext
			var err error
			if interactiveChoice {
				ctx, err = resolveLocalProjectContextForPicker(cmd)
			} else {
				ctx, err = resolveProjectContext()
			}
			if err != nil {
				return err
			}
			taskName := ""
			adhoc := false
			if dash >= 0 {
				if dash != 0 || len(args) == 0 {
					return fmt.Errorf("usage: routurn exec -- <command>")
				}
				command := shellJoin(args[dash:])
				taskName = commandLabel(command)
				ctx.Resolved.Config.Tasks[taskName] = config.Task{Command: command}
				adhoc = true
			} else if len(args) == 0 {
				choice, chooseErr := chooseRunnableInteractive(cmd, ctx)
				if chooseErr != nil {
					return chooseErr
				}
				taskName = choice.Name
				if !choice.Saved {
					ctx.Resolved.Config.Tasks[taskName] = config.Task{Command: choice.Command}
					adhoc = true
				}
			} else {
				if len(args) > 2 {
					return fmt.Errorf("expected a task name and optional legacy update archive; use '--' before an arbitrary command")
				}
				taskName = args[0]
			}

			if interactiveChoice && !ctx.Resolved.Config.IsLocal() {
				var routeErr error
				withPickerLoading(cmd.OutOrStdout(), pickerIsInteractiveTerminal(cmd), "Selecting remote endpoint...", func() {
					routeErr = resolveRemoteForContext(ctx)
				})
				if routeErr != nil {
					return routeErr
				}
			}

			task, ok := ctx.Resolved.Config.Tasks[taskName]
			if !ok {
				return fmt.Errorf("task %q is not defined; run 'routurn exec' to discover commands or use 'routurn exec -- <command>'", taskName)
			}
			if adhoc && saveName != "" {
				if err := config.SaveLocalTask(ctx.Resolved.Root, saveName, task); err != nil {
					return err
				}
				taskName = saveName
				ctx.Resolved.Config.Tasks[taskName] = task
				fmt.Fprintf(cmd.OutOrStdout(), "✓ Saved command as task %q\n", taskName)
			}
			if detach && ctx.Resolved.Config.IsLocal() {
				return fmt.Errorf("local --detach is not supported yet; run without --detach")
			}
			if detach && task.Interactive {
				return fmt.Errorf("task %q is interactive and cannot be detached", taskName)
			}
			effectiveUpdateSpec := updateSpec
			legacyArchive := ""
			if dash < 0 && len(args) == 2 {
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
				Target:        ctx.TargetName,
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
				selector := taskName
				if adhoc && saveName == "" {
					selector = result.Manifest.ID
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Watch    routurn logs %s --follow\n", selector)
				fmt.Fprintf(cmd.OutOrStdout(), "Status   routurn status %s\n", selector)
				if !adhoc || saveName != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Fetch    routurn fetch %s\n", selector)
				}
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
	cmd.Flags().StringVar(&saveName, "save", "", "save a discovered or arbitrary command as a local task shortcut")
	return cmd
}
