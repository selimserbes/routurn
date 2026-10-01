package cli

import (
	"fmt"
	"path/filepath"

	"github.com/selimserbes/routurn/internal/runstate"
	"github.com/spf13/cobra"
)

func newFetchCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "fetch <task>",
		Short: "Fetch artifacts declared by a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := resolveProjectContext()
			if err != nil {
				return err
			}
			taskName := args[0]
			if _, ok := ctx.Resolved.Config.Tasks[taskName]; !ok {
				return fmt.Errorf("task %q is not defined in routurn.toml", taskName)
			}
			if output == "" {
				output = filepath.Join(ctx.Resolved.Root, ".routurn", "fetches", runstate.NewID()+"-"+taskName)
			}
			files, err := fetchTaskArtifacts(ctx, taskName, output)
			if err != nil {
				return err
			}
			if len(files) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching artifacts found.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Fetched %d artifact(s)\n", len(files))
			for _, file := range files {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", file)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved to %s\n", output)
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "artifact destination directory")
	return cmd
}
