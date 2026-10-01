package cli

import (
	"fmt"

	"github.com/selimserbes/routurn/internal/bundle"
	"github.com/selimserbes/routurn/internal/project"
	"github.com/spf13/cobra"
)

func newRollbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rollback <update-id|latest|previous>",
		Short: "Restore local files from a previous Routurn update backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := project.Resolve(projectName)
			if err != nil {
				return err
			}
			id, err := bundle.ResolveID(resolved.Root, args[0])
			if err != nil {
				return err
			}
			record, err := bundle.Rollback(resolved.Root, id)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "✓ Rolled back to the previous local state")
			if verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "Update ID %s\n", record.ID)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Run 'routurn sync' to send the restored local state to the remote target.")
			return nil
		},
	}
}
