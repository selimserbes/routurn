package cli

import (
	"fmt"
	"path/filepath"

	"github.com/selimserbes/routurn/internal/bundle"
	"github.com/spf13/cobra"
)

// chooseRollbackTTY is read-only. The caller asks for confirmation before
// restoring anything. Explicit `routurn rollback previous` is unchanged.
func chooseRollbackTTY(cmd *cobra.Command, root, projectName string) (string, error) {
	records, err := bundle.List(root)
	if err != nil {
		return "", err
	}
	// A non-latest rollback can silently overwrite a newer update. Until
	// rollback chains have dependency-aware planning, show only the latest
	// eligible backup in the simple menu. Explicit IDs remain available.
	latestID, err := bundle.ResolveID(root, "previous")
	if err != nil {
		return "", err
	}
	available := make([]bundle.Record, 0, 1)
	menu := interactiveMenu{Heading: "Undo the latest update", ConfirmHint: "Review latest backup"}
	for _, record := range records {
		if record.RolledBack || record.ID != latestID {
			continue
		}
		name := record.BundleName
		if name == "" {
			name = filepath.Base(record.Archive)
		}
		menu.Items = append(menu.Items, interactiveMenuItem{Label: name, Detail: record.AppliedAt})
		available = append(available, record)
	}
	if len(available) == 0 {
		return "", fmt.Errorf("latest update backup not available")
	}
	chosen, err := chooseInteractiveMenu(cmd, projectName, menu)
	if err != nil {
		return "", err
	}
	if chosen.Index < 0 {
		return "", fmt.Errorf("rollback selection cancelled")
	}
	item := available[chosen.Index]
	fmt.Fprintln(cmd.OutOrStdout(), "This will restore the previous files and undo the selected update.")
	fmt.Fprintf(cmd.OutOrStdout(), "Undo update %s? [y/N] ", item.ID)
	ok, err := confirmYesNo(cmd.InOrStdin())
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("rollback cancelled; files unchanged")
	}
	return item.ID, nil
}
