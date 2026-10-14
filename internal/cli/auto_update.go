package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/selimserbes/routurn/internal/bundle"
	"github.com/spf13/cobra"
)

func confirmYesNo(in io.Reader) (bool, error) {
	line, err := bufio.NewReader(in).ReadString('\n')
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

// alreadyAppliedUpdate prevents the task menu from offering the same imported
// bundle every time it is opened. Rolled-back updates may be offered again.
func alreadyAppliedUpdate(root, hash string) (bool, error) {
	records, err := bundle.List(root)
	if err != nil {
		return false, err
	}
	for _, record := range records {
		if !record.RolledBack && record.BundleHash == hash {
			return true, nil
		}
	}
	return false, nil
}

// offerUpdateBeforeTask is invoked ONLY by the interactive task picker. It does
// not guess between multiple archives, and it never applies changes itself.
// The selected archive still goes through the normal plan and confirmation.
func offerUpdateBeforeTask(cmd *cobra.Command, ctx *projectContext) (string, error) {
	choices, err := recentCompatibleUpdates(ctx, 0)
	if err != nil {
		return "", nil // task execution should not depend on Downloads indexing
	}
	var pending []updateChoice
	for _, item := range choices {
		applied, err := alreadyAppliedUpdate(ctx.Resolved.Root, item.Hash)
		if err != nil {
			return "", err
		}
		if !applied {
			pending = append(pending, item)
		}
	}
	if len(pending) != 1 {
		if len(pending) > 1 {
			fmt.Fprintln(cmd.OutOrStdout(), "Multiple compatible updates found. Use 'routurn update' to choose one; task runs unchanged.")
		}
		return "", nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), "A verified compatible update is available for this project.")
	fmt.Fprint(cmd.OutOrStdout(), "Review this update before running the task? [y/N] ")
	ok, err := confirmYesNo(cmd.InOrStdin())
	if err != nil {
		return "", err
	}
	if ok {
		return pending[0].Path, nil
	}
	return "", nil
}
