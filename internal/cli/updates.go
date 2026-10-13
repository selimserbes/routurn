package cli

import (
	"fmt"

	"github.com/selimserbes/routurn/internal/bundle"
	"github.com/spf13/cobra"
)

func newUpdatesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "updates",
		Short: "Show local update history",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := resolveLocalProjectContext()
			if err != nil {
				return err
			}
			records, err := bundle.List(ctx.Resolved.Root)
			if err != nil {
				return err
			}
			if len(records) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No updates recorded.")
				return nil
			}
			currentMarked := false
			fmt.Fprintln(cmd.OutOrStdout(), "STATE      UPDATE                           APPLIED")
			for _, record := range records {
				state := "history"
				if record.RolledBack {
					state = "rolledback"
				} else if !currentMarked {
					state = "current"
					currentMarked = true
				}
				name := record.BundleName
				if name == "" {
					name = record.Archive
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-10s %-32s %s\n", state, name, record.AppliedAt)
				if verbose {
					fmt.Fprintf(cmd.OutOrStdout(), "           id=%s hash=%s\n", record.ID, record.BundleHash)
				}
			}
			return nil
		},
	}
}
