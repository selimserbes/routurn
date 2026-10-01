package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/selimserbes/routurn/internal/bundle"
	"github.com/selimserbes/routurn/internal/project"
	"github.com/spf13/cobra"
)

type applyOptions struct {
	DryRun          bool
	Yes             bool
	StripComponents int
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
	fmt.Fprintf(cmd.OutOrStdout(), "✓ Update applied · %s\n", record.ID)
	if record.Backup != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Backup   %s\n", record.Backup)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Rollback routurn rollback %s\n", record.ID)
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
