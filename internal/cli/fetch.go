package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/selimserbes/routurn/internal/runstate"
	"github.com/spf13/cobra"
)

func newFetchCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "fetch [task|run-id|latest]",
		Short: "Fetch artifacts for a recorded run",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := resolveProjectContext()
			if err != nil {
				return err
			}

			selector := "latest"
			if len(args) == 1 {
				selector = args[0]
			}
			manifest, err := resolveFetchRun(ctx.Resolved.Root, selector)
			if err != nil {
				return err
			}
			if _, ok := ctx.Resolved.Config.Tasks[manifest.Task]; !ok {
				return fmt.Errorf("task %q from run %s is not defined in routurn.toml", manifest.Task, manifest.ID)
			}

			dest := output
			attachToRun := dest == ""
			if attachToRun {
				dest = filepath.Join(runstate.Dir(ctx.Resolved.Root, manifest.ID), "artifacts")
			}

			files, err := fetchTaskArtifacts(ctx, manifest.Task, dest)
			if err != nil {
				return err
			}
			if len(files) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching artifacts found.")
				return nil
			}

			if attachToRun {
				manifest.Artifacts = files
				if err := runstate.Save(ctx.Resolved.Root, manifest); err != nil {
					return err
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Fetched %d artifact(s) for run %s\n", len(files), manifest.ID)
			for _, file := range files {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", file)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved to %s\n", dest)
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "custom artifact destination directory")
	return cmd
}

func resolveFetchRun(root, selector string) (runstate.Manifest, error) {
	if selector == "latest" {
		id, err := runstate.ResolveID(root, "latest")
		if err != nil {
			return runstate.Manifest{}, err
		}
		return runstate.Load(root, id)
	}

	// Prefer an exact run ID when a matching run directory exists.
	if _, err := os.Stat(runstate.Dir(root, selector)); err == nil {
		return runstate.Load(root, selector)
	}

	// Otherwise treat the selector as a task name and resolve its latest run.
	return runstate.LatestByTask(root, selector)
}
