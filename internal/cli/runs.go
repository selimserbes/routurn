package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/selimserbes/routurn/internal/project"
	"github.com/selimserbes/routurn/internal/runstate"
	"github.com/spf13/cobra"
)

func newRunsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "Inspect local Routurn run history",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := project.Resolve(projectName)
			if err != nil {
				return err
			}
			runs, err := runstate.List(resolved.Root)
			if err != nil {
				return err
			}
			if len(runs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No runs recorded.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-26s %-10s %-16s %s\n", "RUN", "STATUS", "TASK", "STARTED")
			limit := len(runs)
			if limit > 20 {
				limit = 20
			}
			for _, r := range runs[:limit] {
				fmt.Fprintf(cmd.OutOrStdout(), "%-26s %-10s %-16s %s\n", r.ID, r.Status, r.Task, r.StartedAt)
			}
			return nil
		},
	}
	cmd.AddCommand(newRunsShowCmd())
	return cmd
}

func newRunsShowCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show <run-id|latest>",
		Short: "Show one recorded run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := project.Resolve(projectName)
			if err != nil {
				return err
			}
			id := args[0]
			if id == "latest" {
				runs, err := runstate.List(resolved.Root)
				if err != nil {
					return err
				}
				if len(runs) == 0 {
					return fmt.Errorf("no runs recorded")
				}
				id = runs[0].ID
			}
			m, err := runstate.Load(resolved.Root, id)
			if err != nil {
				return err
			}
			if asJSON {
				data, _ := json.MarshalIndent(m, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Run        %s\n", m.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "Project    %s\n", m.Project)
			fmt.Fprintf(cmd.OutOrStdout(), "Target     %s\n", m.Target)
			fmt.Fprintf(cmd.OutOrStdout(), "Task       %s\n", m.Task)
			fmt.Fprintf(cmd.OutOrStdout(), "Status     %s\n", m.Status)
			fmt.Fprintf(cmd.OutOrStdout(), "Started    %s\n", m.StartedAt)
			if m.FinishedAt != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Finished   %s\n", m.FinishedAt)
			}
			if m.ExitCode != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Exit code  %d\n", *m.ExitCode)
			}
			if m.Snapshot != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Snapshot   %s\n", m.Snapshot)
			}
			if len(m.Changed) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Changed    %s\n", strings.Join(m.Changed, ", "))
			}
			if len(m.Deleted) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Deleted    %s\n", strings.Join(m.Deleted, ", "))
			}
			if len(m.Artifacts) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Artifacts  %s\n", strings.Join(m.Artifacts, ", "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the run manifest as JSON")
	return cmd
}
