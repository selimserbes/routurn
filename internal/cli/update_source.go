package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/selimserbes/routurn/internal/intake"
	"github.com/spf13/cobra"
)

// finishUpdateSource runs only after a durable successful apply. In particular,
// declining review, failing a precondition, and no-op updates keep the source
// ZIP even when Routurn has imported a verified content-addressed copy.
func finishUpdateSource(cmd *cobra.Command, original string, imported intake.Result, keep bool) {
	if keep || imported.Path == "" {
		return
	}
	originalAbs, aErr := filepath.Abs(original)
	managedAbs, mErr := filepath.Abs(imported.Path)
	if aErr != nil || mErr != nil || originalAbs == managedAbs {
		return
	}
	// Avoid deleting symlinks or directories (including a path swapped by
	// another process after initial inspection).
	info, err := os.Lstat(originalAbs)
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	actual, err := intake.HashFile(originalAbs)
	if err != nil || actual != imported.Hash {
		fmt.Fprintln(cmd.ErrOrStderr(), "! Original update changed during import; source was kept")
		return
	}
	if err := os.Remove(originalAbs); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "! Could not remove original update: %v\n", err)
		return
	}
	fmt.Fprintln(cmd.OutOrStdout(), "✓ Original source removed after successful update")
}
