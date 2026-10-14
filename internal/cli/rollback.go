package cli

import (
	"fmt"

	"github.com/selimserbes/routurn/internal/bundle"
	"github.com/selimserbes/routurn/internal/project"
	"github.com/spf13/cobra"
)

func newRollbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rollback [update-id|latest|previous]",
		Short: "Restore local files from a previous Routurn update backup",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var resolved *project.Resolved
			var err error
			if len(args) == 0 {
				ctx, ctxErr := resolveLocalProjectContextForPicker(cmd)
				if ctxErr != nil {
					return ctxErr
				}
				resolved = ctx.Resolved
			} else {
				resolved, err = project.Resolve(projectName)
				if err != nil {
					return err
				}
			}
			selection := ""
			if len(args) > 0 {
				selection = args[0]
			} else {
				if !pickerIsInteractiveTerminal(cmd) {
					return fmt.Errorf("interactive rollback requires a terminal; specify an update ID or 'previous'")
				}
				selection, err = chooseRollbackTTY(cmd, resolved.Root, resolved.Config.Name)
				if err != nil {
					return err
				}
			}
			id, err := bundle.ResolveID(resolved.Root, selection)
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
			if !resolved.Config.IsLocal() {
				fmt.Fprintln(cmd.OutOrStdout(), "Run 'routurn sync' to send the restored local state to the remote target.")
			}
			return nil
		},
	}
}
