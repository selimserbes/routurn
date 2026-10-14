package cli

import (
	"errors"
	"fmt"
	"io"

	resultstore "github.com/selimserbes/routurn/internal/result"
	"github.com/selimserbes/routurn/internal/runstate"
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
			var selected resultChoice
			if len(args) > 0 {
				taskName = args[0]
			} else if pickerIsInteractiveTerminal(cmd) {
				selected, err = chooseResultTTY(cmd, ctx.Resolved.Root, ctx.Resolved.Config.Name)
				if errors.Is(err, errNoResultEntries) {
					fmt.Fprintln(cmd.OutOrStdout(), "No runs or results recorded yet. Run 'routurn exec' to get started.")
					return nil
				}
				if err != nil {
					return err
				}
				taskName = selected.Task
				if !selected.HasArtifact && selected.HasRun {
					printRunOnlyResult(cmd.OutOrStdout(), selected.LatestRun)
					return nil
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
			if selected.HasRun && selected.LatestRun.ID != meta.RunID {
				fmt.Fprintf(cmd.OutOrStdout(), "Notice  Saved files are from run %s; newest run %s is %s\n", meta.RunID, selected.LatestRun.ID, selected.LatestRun.Status)
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

// printRunOnlyResult is for the interactive result picker. Script-facing
// `result TASK` and `result --path` retain their artifact-only semantics.
func printRunOnlyResult(out io.Writer, run runstate.Manifest) {
	fmt.Fprintf(out, "Task     %s\n", run.Task)
	fmt.Fprintf(out, "Run      %s\n", run.ID)
	fmt.Fprintf(out, "Status   %s\n", run.Status)
	fmt.Fprintf(out, "Started  %s\n", run.StartedAt)
	if run.FinishedAt != "" {
		fmt.Fprintf(out, "Finished %s\n", run.FinishedAt)
	}
	if run.ExitCode != nil {
		fmt.Fprintf(out, "Exit code %d\n", *run.ExitCode)
	}
	fmt.Fprintln(out, "Files    No artifacts collected for this task.")
}
