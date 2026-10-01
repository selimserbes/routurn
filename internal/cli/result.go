package cli

import (
	"fmt"

	resultstore "github.com/selimserbes/routurn/internal/result"
	"github.com/spf13/cobra"
)

func newResultCmd() *cobra.Command {
	var pathOnly bool
	cmd := &cobra.Command{
		Use:   "result <task>",
		Short: "Show the latest successful materialized result for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := resolveLocalProjectContext()
			if err != nil {
				return err
			}
			meta, publicPath, err := resultstore.Read(ctx.Resolved.Root, args[0])
			if err != nil {
				return fmt.Errorf("no materialized result for task %q; run 'routurn exec %s' first", args[0], args[0])
			}
			if pathOnly {
				fmt.Fprintln(cmd.OutOrStdout(), publicPath)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Task    %s\n", meta.Task)
			fmt.Fprintf(cmd.OutOrStdout(), "Latest  %s\n", publicPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Files   %d\n", len(meta.Files))
			for _, file := range meta.Files {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", file)
			}
			if verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "Run     %s\n", meta.RunID)
				fmt.Fprintf(cmd.OutOrStdout(), "Store   %s\n", resultstore.Dir(ctx.Resolved.Root, args[0]))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&pathOnly, "path", false, "print only the stable user-facing result path")
	return cmd
}
