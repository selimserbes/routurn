package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/selimserbes/routurn/internal/intake"
	"github.com/selimserbes/routurn/internal/retention"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var recent bool
	var dryRun bool
	var yes bool
	var keepSource bool
	var fullSnapshot bool
	var stripComponents int

	cmd := &cobra.Command{
		Use:   "update [archive]",
		Short: "Select, safely import, and apply an update bundle",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var ctx *projectContext
			var err error
			if len(args) == 0 && !recent {
				ctx, err = resolveLocalProjectContextForPicker(cmd)
			} else {
				ctx, err = resolveLocalProjectContext()
			}
			if err != nil {
				return err
			}
			if recent && len(args) == 1 {
				return fmt.Errorf("use either an archive path or --recent, not both")
			}

			selector := "select"
			if recent {
				selector = "recent"
			} else if len(args) == 1 {
				selector = args[0]
			}
			archive, err := resolveUpdateInput(cmd, ctx, selector, stripComponents)
			if err != nil {
				return err
			}
			archive = expandUserPath(archive)
			effectiveStrip := stripComponents
			stripExplicit := cmd.Flags().Changed("strip-components")
			if !stripExplicit {
				if managed, ok, metaErr := intake.FindByPath(archive); metaErr == nil && ok && managed.StripComponents > 0 {
					effectiveStrip = managed.StripComponents
					stripExplicit = true // preserve an existing imported mapping
				}
			}
			// Resolve wrapper folders BEFORE import, verification and any
			// source deletion. All update paths use the same mapping policy.
			effectiveStrip, err = resolveArchiveMapping(cmd, ctx.Resolved.Root, archive, effectiveStrip, stripExplicit)
			if err != nil {
				return err
			}
			manifest, compatibility, err := validateUpdateCompatibility(ctx, archive, effectiveStrip)
			if err != nil {
				return err
			}
			if manifest == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "! Archive type cannot be verified automatically; review the update scope")
				if shouldOfferLegacyFullSnapshot(len(args) == 0 && !recent, dryRun, yes, fullSnapshot) {
					fullSnapshot, err = confirmLegacyFullSnapshot(cmd)
					if err != nil {
						return err
					}
				}
			} else if compatibility == updateCompatibilityScoped {
				fmt.Fprintln(cmd.OutOrStdout(), "✓ Bundle target-file preconditions match; unrelated project changes are allowed")
			} else if compatibility == updateCompatibilityUnverified {
				fmt.Fprintln(cmd.OutOrStdout(), "! Bundle identifies the project but has no verifiable base state; review the plan before applying")
			}

			if dryRun {
				_, _, err := applyBundle(cmd, ctx.Resolved.Root, archive, applyOptions{
					DryRun:          true,
					FullSnapshot:    fullSnapshot,
					Yes:             true,
					StripComponents: effectiveStrip,
				})
				return err
			}

			rememberArchiveDir(ctx.Global, archive)
			imported, err := intake.Import(archive, intake.Options{Consume: false, StripComponents: effectiveStrip})
			if err != nil {
				return err
			}
			name := imported.OriginalName
			if imported.Manifest != nil && imported.Manifest.Bundle.Name != "" {
				name = imported.Manifest.Bundle.Name
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Bundle   %s\n", name)
			fmt.Fprintf(cmd.OutOrStdout(), "SHA256   %s\n", imported.Hash)
			if imported.Duplicate {
				fmt.Fprintln(cmd.OutOrStdout(), "✓ Existing managed copy reused")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "✓ Imported into Routurn managed storage")
			}
			if imported.Warning != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "! %s\n", imported.Warning)
			}
			fmt.Fprintln(cmd.OutOrStdout())

			record, applied, err := applyBundle(cmd, ctx.Resolved.Root, imported.Path, applyOptions{
				Yes:             yes,
				FullSnapshot:    fullSnapshot,
				StripComponents: effectiveStrip,
				ManagedHash:     imported.Hash,
				ManagedName:     name,
			})
			if err != nil {
				return err
			}
			// Never consume an original ZIP on cancellation, dry-run or
			// no-op. Delete only after the update was durably applied and the
			// managed copy has been verified by intake.Import.
			if !applied || record.ID == "" {
				return nil
			}
			if len(record.Added) == 0 && len(record.Modified) == 0 && len(record.Deleted) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "✓ Full-snapshot ownership recorded; original archive retained (no file changes)")
			} else {
				// Selecting an archive in the TUI must not unexpectedly consume a
				// file from Downloads. Explicit CLI paths keep their old behavior.
				finishUpdateSource(cmd, archive, imported, keepSource || (len(args) == 0 && !recent))
			}
			_, _ = retention.PruneProject(ctx.Resolved.Root, retention.DefaultRuns, retention.DefaultUpdates, false)
			_ = intake.PruneDefault()
			return nil
		},
	}
	cmd.Flags().BoolVar(&recent, "recent", false, "use a recent compatible Routurn bundle; ask if multiple distinct candidates exist")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what the selected update would change without importing or applying it")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "apply without an interactive confirmation")
	cmd.Flags().BoolVar(&keepSource, "keep-source", false, "keep the original archive after verified import")
	cmd.Flags().BoolVar(&fullSnapshot, "full-snapshot", false, "complete source snapshot; prune only previously tracked unchanged files")
	cmd.Flags().IntVar(&stripComponents, "strip-components", 0, "remove leading path components from archive entries")
	return cmd
}

// An ordinary archive cannot prove whether it is a full or partial source update.
// Ask only in the interactive archive-picker path; scripted/explicit-path
// workflows and --yes retain their previous non-deleting defaults.
func shouldOfferLegacyFullSnapshot(picker, dryRun, yes, fullSnapshot bool) bool {
	return picker && !dryRun && !yes && !fullSnapshot
}

func confirmLegacyFullSnapshot(cmd *cobra.Command) (bool, error) {
	fmt.Fprintln(cmd.OutOrStdout(), "! Routurn cannot verify whether this ZIP is a complete project or a partial update.")

	// The shared TUI is used on a normal Unix terminal. Default focus is the
	// non-deleting option; the user must move to option 2 and press Enter to
	// authorize considering deletions. Other terminals use numbered choices.
	if pickerIsInteractiveTerminal(cmd) && runtime.GOOS != "windows" && os.Getenv("TERM") != "" && os.Getenv("TERM") != "dumb" {
		selection, err := chooseInteractiveMenu(cmd, "Update type", interactiveMenu{
			Heading: "What does this ZIP contain?",
			Items: []interactiveMenuItem{
				{Label: "Keep existing files (safe default)", Detail: "Partial update; no deletions"},
				{Label: "Complete project source", Detail: "Review removal of obsolete managed files"},
			},
			ConfirmHint: "Review selected update type",
		})
		if err != nil {
			return false, err
		}
		if selection.Index < 0 {
			return false, fmt.Errorf("update cancelled")
		}
		if selection.Index == 1 {
			fmt.Fprintln(cmd.OutOrStdout(), "! Full-source review selected: check the Deleted list before applying")
			return true, nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), "✓ Safe partial update selected: no obsolete-file deletion")
		return false, nil
	}

	// Keep a deterministic choice for line-based inputs and on Windows.
	// readMenuLine consumes only one line, leaving the later apply answer intact.
	fmt.Fprintln(cmd.OutOrStdout(), "  1) Keep existing files (safe default; no deletions)")
	fmt.Fprintln(cmd.OutOrStdout(), "  2) Complete project source (review managed-file deletions)")
	fmt.Fprint(cmd.OutOrStdout(), "Choose [1]: ")
	answer, err := readMenuLine(cmd.InOrStdin())
	if err != nil && answer == "" {
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		return false, err
	}
	if strings.TrimSpace(answer) == "2" {
		fmt.Fprintln(cmd.OutOrStdout(), "! Full-source review selected: check the Deleted list before applying")
		return true, nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), "✓ Safe partial update selected: no obsolete-file deletion")
	return false, nil
}
