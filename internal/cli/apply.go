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
			_, _, err = applyBundle(cmd, resolved.Root, args[0], opts)
			return err
		},
	}
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "show what the archive would change without applying it")
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
		if manifest.Project.Name != "" && manifest.Project.Name != cfg.Name {
			return bundle.Record{}, false, fmt.Errorf("update belongs to project %q; current project is %q", manifest.Project.Name, cfg.Name)
		}

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
			if current != manifest.Base.Fingerprint && !scopedMatch {
				return bundle.Record{}, false, fmt.Errorf("update was created for a different project state\nexpected: %s\ncurrent:  %s", manifest.Base.Fingerprint, current)
			}
		}
	}
	plan, err := bundle.BuildPlan(root, archive, opts.StripComponents)
	if err != nil {
		return bundle.Record{}, false, err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Update   %s\n", plan.Archive)
	fmt.Fprintf(cmd.OutOrStdout(), "Added    %d\n", len(plan.Added))
	fmt.Fprintf(cmd.OutOrStdout(), "Modified %d\n", len(plan.Modified))
	fmt.Fprintf(cmd.OutOrStdout(), "Unchanged %d\n", len(plan.Unchanged))
	for _, path := range plan.Added {
		fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", path)
	}
	for _, path := range plan.Modified {
		fmt.Fprintf(cmd.OutOrStdout(), "  ~ %s\n", path)
	}

	if len(plan.Added) == 0 && len(plan.Modified) == 0 {
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
