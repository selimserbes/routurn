package cli

import (
	"fmt"

	"github.com/selimserbes/routurn/internal/intake"
	"github.com/selimserbes/routurn/internal/retention"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var recent bool
	var dryRun bool
	var yes bool
	var keepSource bool
	var stripComponents int

	cmd := &cobra.Command{
		Use:   "update [archive]",
		Short: "Select, safely import, and apply an update bundle",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := resolveLocalProjectContext()
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
			if effectiveStrip == 0 {
				if managed, ok, metaErr := intake.FindByPath(archive); metaErr == nil && ok {
					effectiveStrip = managed.StripComponents
				}
			}
			manifest, err := validateUpdateCompatibility(ctx, archive, effectiveStrip)
			if err != nil {
				return err
			}
			if manifest == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "! Legacy update: no routurn-bundle.toml; project/base identity cannot be verified")
			} else if manifest.Base.Fingerprint == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "! Bundle identifies the project but has no base fingerprint; state compatibility cannot be verified")
			}

			if dryRun {
				_, _, err := applyBundle(cmd, ctx.Resolved.Root, archive, applyOptions{
					DryRun:          true,
					Yes:             true,
					StripComponents: effectiveStrip,
				})
				return err
			}

			rememberArchiveDir(ctx.Global, archive)
			imported, err := intake.Import(archive, intake.Options{Consume: !keepSource, StripComponents: effectiveStrip})
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
			if imported.SourceRemoved {
				fmt.Fprintln(cmd.OutOrStdout(), "✓ Original source removed after verified import")
			}
			if imported.Warning != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "! %s\n", imported.Warning)
			}
			fmt.Fprintln(cmd.OutOrStdout())

			record, applied, err := applyBundle(cmd, ctx.Resolved.Root, imported.Path, applyOptions{
				Yes:             yes,
				StripComponents: effectiveStrip,
				ManagedHash:     imported.Hash,
				ManagedName:     name,
			})
			if err != nil {
				return err
			}
			if !applied && record.ID == "" {
				return nil
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
	cmd.Flags().IntVar(&stripComponents, "strip-components", 0, "remove leading path components from archive entries")
	return cmd
}
