package cli

import (
	"fmt"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/selimserbes/routurn/internal/project"
	"github.com/selimserbes/routurn/internal/remote"
	"github.com/selimserbes/routurn/internal/runstate"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [run-id|latest]",
		Short: "Show project status or inspect a detached remote run",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return showRunStatus(cmd, args[0])
			}
			return showProjectStatus(cmd)
		},
	}
}

func showProjectStatus(cmd *cobra.Command) error {
	resolved, err := project.Resolve(projectName)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Project      %s\n", resolved.Config.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "Local root   %s\n", resolved.Root)
	fmt.Fprintf(cmd.OutOrStdout(), "Target       %s\n", valueOrDash(resolved.Config.Remote.Target))
	fmt.Fprintf(cmd.OutOrStdout(), "Remote path  %s\n", valueOrDash(resolved.Config.Remote.Path))
	fmt.Fprintf(cmd.OutOrStdout(), "Tasks        %d\n", len(resolved.Config.Tasks))

	if resolved.Config.Remote.Target != "" {
		global, err := config.LoadGlobal()
		if err != nil {
			return err
		}
		if target, ok := global.Targets[resolved.Config.Remote.Target]; ok {
			fmt.Fprintf(cmd.OutOrStdout(), "SSH          %s\n", remote.Destination(target))
		}
	}
	runs, err := runstate.List(resolved.Root)
	if err == nil && len(runs) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Latest run   %s (%s, %s)\n", runs[0].ID, runs[0].Status, runs[0].Task)
	}
	return nil
}

func showRunStatus(cmd *cobra.Command, runID string) error {
	resolved, err := project.Resolve(projectName)
	if err != nil {
		return err
	}
	id, err := runstate.ResolveID(resolved.Root, runID)
	if err != nil {
		return err
	}
	m, err := runstate.Load(resolved.Root, id)
	if err != nil {
		return err
	}
	alive := false
	if m.Detached {
		ctx, err := resolveRunRemote(id)
		if err != nil {
			return err
		}
		state, refreshed, err := refreshDetachedRun(ctx)
		if err != nil {
			return err
		}
		m = refreshed
		alive = state.Alive
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Run        %s\n", m.ID)
	fmt.Fprintf(cmd.OutOrStdout(), "Project    %s\n", m.Project)
	fmt.Fprintf(cmd.OutOrStdout(), "Target     %s\n", m.Target)
	fmt.Fprintf(cmd.OutOrStdout(), "Task       %s\n", m.Task)
	fmt.Fprintf(cmd.OutOrStdout(), "State      %s\n", m.Status)
	if m.Detached {
		fmt.Fprintf(cmd.OutOrStdout(), "Detached   true\n")
		fmt.Fprintf(cmd.OutOrStdout(), "PID        %d\n", m.RemotePID)
		fmt.Fprintf(cmd.OutOrStdout(), "Alive      %t\n", alive)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Started    %s\n", valueOrDash(m.StartedAt))
	if m.FinishedAt != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Finished   %s\n", m.FinishedAt)
	}
	if m.ExitCode != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Exit code  %d\n", *m.ExitCode)
	}
	return nil
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
