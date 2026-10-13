package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/selimserbes/routurn/internal/bundle"
	"github.com/selimserbes/routurn/internal/config"
	"github.com/selimserbes/routurn/internal/project"
	"github.com/selimserbes/routurn/internal/retention"
	"github.com/selimserbes/routurn/internal/syncer"
	"github.com/spf13/cobra"
)

type applyOptions struct {
	DryRun          bool
	FullSnapshot    bool
	Yes             bool
	StripComponents int
	ManagedHash     string
	ManagedName     string
}

func newApplyCmd() *cobra.Command {
	var opts applyOptions
	cmd := &cobra.Command{
		Use:   "apply <update-archive>",
		Short: "Safely inspect and apply an update archive to the local project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := project.Resolve(projectName)
			if err != nil {
				return err
			}
			defer func() {
				_, _ = retention.PruneProject(resolved.Root, retention.DefaultRuns, retention.DefaultUpdates, false)
			}()
			opts.StripComponents, err = resolveArchiveMapping(cmd, resolved.Root, args[0], opts.StripComponents, cmd.Flags().Changed("strip-components"))
			if err != nil {
				return err
			}
			_, _, err = applyBundle(cmd, resolved.Root, args[0], opts)
			return err
		},
	}
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "show what the archive would change without applying it")
	cmd.Flags().BoolVar(&opts.FullSnapshot, "full-snapshot", false, "treat archive as a complete source snapshot; prune only previously tracked unchanged files")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "apply without an interactive confirmation")
	cmd.Flags().IntVar(&opts.StripComponents, "strip-components", 0, "remove leading path components from archive entries")
	return cmd
}

func applyBundle(cmd *cobra.Command, root, archive string, opts applyOptions) (bundle.Record, bool, error) {
	if _, err := os.Stat(archive); err != nil {
		return bundle.Record{}, false, fmt.Errorf("update archive: %w", err)
	}
	inspection, err := bundle.InspectArchive(archive, opts.StripComponents)
	if err != nil {
		return bundle.Record{}, false, err
	}
	manifest := inspection.Manifest
	if manifest != nil {
		cfg, cfgErr := config.LoadProject(root)
		if cfgErr != nil {
			return bundle.Record{}, false, cfgErr
		}
		projectNameDiffers := manifest.Project.Name != "" && manifest.Project.Name != cfg.Name

		payloadPaths := make([]string, 0, len(inspection.Entries))
		for _, entry := range inspection.Entries {
			payloadPaths = append(payloadPaths, entry.Path)
		}
		scopedMatch := false
		if len(manifest.Base.Files) > 0 {
			var mismatches []string
			scopedMatch, mismatches, err = bundle.CheckBaseFiles(root, payloadPaths, manifest.Base.Files)
			if err != nil {
				return bundle.Record{}, false, err
			}
			if !scopedMatch {
				return bundle.Record{}, false, fmt.Errorf("update target files do not match the bundle base state\n  %s", strings.Join(mismatches, "\n  "))
			}
		}

		exactMatch := false
		if manifest.Base.Fingerprint != "" {
			scan, scanErr := syncer.Scan(root, cfg.Sync.Exclude)
			if scanErr != nil {
				return bundle.Record{}, false, fmt.Errorf("fingerprint project: %w", scanErr)
			}
			rootAbs, _ := filepath.Abs(root)
			archiveAbs, _ := filepath.Abs(archive)
			if rel, relErr := filepath.Rel(rootAbs, archiveAbs); relErr == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				rel = filepath.ToSlash(rel)
				filtered := scan.Files[:0]
				for _, file := range scan.Files {
					if file.Path != rel {
						filtered = append(filtered, file)
					}
				}
				scan.Files = filtered
			}
			current := syncer.Fingerprint(scan)
			exactMatch = current == manifest.Base.Fingerprint
			if !exactMatch && !scopedMatch {
				return bundle.Record{}, false, fmt.Errorf("update was created for a different project state\nexpected: %s\ncurrent:  %s", manifest.Base.Fingerprint, current)
			}
		}

		if projectNameDiffers {
			if !exactMatch && !scopedMatch {
				return bundle.Record{}, false, fmt.Errorf("update declares project %q, current project is %q, and no matching base state verifies that it is safe here", manifest.Project.Name, cfg.Name)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "! Bundle project name %q differs from current project %q; verified base state matches, continuing safely\n", manifest.Project.Name, cfg.Name)
		}
	}
	plan, err := bundle.BuildPlan(root, archive, opts.StripComponents)
	if err != nil {
		return bundle.Record{}, false, err
	}
	if opts.FullSnapshot {
		cfg, err := config.LoadProject(root)
		if err != nil {
			return bundle.Record{}, false, err
		}
		if err := bundle.PrepareFullSnapshot(root, &plan, cfg.Sync.Exclude); err != nil {
			return bundle.Record{}, false, err
		}
		if !plan.SnapshotExisted {
			fmt.Fprintln(cmd.OutOrStdout(), "! First full snapshot: recording managed files; unknown existing files are preserved")
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Update   %s\n", plan.Archive)
	fmt.Fprintf(cmd.OutOrStdout(), "Added    %d\n", len(plan.Added))
	fmt.Fprintf(cmd.OutOrStdout(), "Modified %d\n", len(plan.Modified))
	fmt.Fprintf(cmd.OutOrStdout(), "Unchanged %d\n", len(plan.Unchanged))
	if opts.FullSnapshot {
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted  %d\n", len(plan.Deleted))
	}
	for _, path := range plan.Added {
		fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", path)
	}
	for _, path := range plan.Modified {
		fmt.Fprintf(cmd.OutOrStdout(), "  ~ %s\n", path)
	}
	for _, path := range plan.Deleted {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", path)
	}

	if len(plan.Added) == 0 && len(plan.Modified) == 0 && len(plan.Deleted) == 0 && !opts.FullSnapshot {
		fmt.Fprintln(cmd.OutOrStdout(), "✓ No local changes required")
		return bundle.Record{}, true, nil
	}
	if opts.DryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "Dry run only; nothing was changed.")
		return bundle.Record{}, false, nil
	}
	if !opts.Yes {
		ok, err := confirmApply(cmd)
		if err != nil {
			return bundle.Record{}, false, err
		}
		if !ok {
			fmt.Fprintln(cmd.OutOrStdout(), "Cancelled; nothing was changed.")
			return bundle.Record{}, false, nil
		}
	}

	record, err := bundle.Apply(root, plan)
	if err != nil {
		return bundle.Record{}, false, err
	}
	if opts.ManagedHash != "" || opts.ManagedName != "" {
		updated, metaErr := bundle.SetManagedInfo(root, record.ID, opts.ManagedHash, opts.ManagedName)
		if metaErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "! update applied, but managed metadata could not be recorded: %v\n", metaErr)
		} else {
			record = updated
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), "✓ Update applied")
	if record.Backup != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Backup   %s\n", record.Backup)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Rollback routurn rollback previous")
	if verbose {
		fmt.Fprintf(cmd.OutOrStdout(), "Update ID %s\n", record.ID)
	}
	return record, true, nil
}

func confirmApply(cmd *cobra.Command) (bool, error) {
	fmt.Fprint(cmd.OutOrStdout(), "Apply this update? [y/N] ")
	reader := bufio.NewReader(cmd.InOrStdin())
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
