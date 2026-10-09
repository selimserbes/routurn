package cli

import (
	"fmt"

	resultstore "github.com/selimserbes/routurn/internal/result"
	"github.com/spf13/cobra"
)

func newResultCmd() *cobra.Command {
	var pathOnly bool
	cmd := &cobra.Command{
		Use:   "result [task]",
		Short: "Show the latest successful materialized result for a task",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var ctx *projectContext
			var err error
			if len(args) == 0 && !pathOnly {
				ctx, err = resolveLocalProjectContextForPicker(cmd)
			} else {
				ctx, err = resolveLocalProjectContext()
			}
			if err != nil {
				return err
			}
			taskName := ""
			if len(args) > 0 {
				taskName = args[0]
			} else if pickerIsInteractiveTerminal(cmd) {
				taskName, err = chooseResultTTY(cmd, ctx.Resolved.Root, ctx.Resolved.Config.Name)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("result requires a task name in noninteractive mode: routurn result <task>")
			}
			meta, publicPath, err := resultstore.Read(ctx.Resolved.Root, taskName)
			if err != nil {
				return fmt.Errorf("no materialized result for task %q; run 'routurn exec %s' first", taskName, taskName)
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
				fmt.Fprintf(cmd.OutOrStdout(), "Store   %s\n", resultstore.Dir(ctx.Resolved.Root, taskName))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&pathOnly, "path", false, "print only the stable user-facing result path")
	return cmd
}
