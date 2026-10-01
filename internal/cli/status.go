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
	var check bool
	cmd := &cobra.Command{
		Use:   "status [task|run-id|latest]",
		Short: "Show project status or inspect a detached remote run",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return showRunStatus(cmd, args[0])
			}
			return showProjectStatus(cmd, check)
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "check endpoint reachability and show the route Routurn would use")
	return cmd
}

func showProjectStatus(cmd *cobra.Command, check bool) error {
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
			endpoints := config.EndpointList(target)
			fmt.Fprintf(cmd.OutOrStdout(), "Route        %s\n", config.EffectiveRoute(target))
			fmt.Fprintf(cmd.OutOrStdout(), "Endpoints    %d\n", len(endpoints))
			if len(endpoints) == 1 {
				fmt.Fprintf(cmd.OutOrStdout(), "SSH          %s\n", remote.Destination(endpoints[0].Endpoint))
			}
			if check {
				fmt.Fprintln(cmd.OutOrStdout(), "Connectivity")
				reachable := make(map[string]bool, len(endpoints))
				for _, endpoint := range endpoints {
					if probeErr := remote.Probe(endpoint.Endpoint, 2); probeErr != nil {
						fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %-12s %s\n", endpoint.Name, remote.Destination(endpoint.Endpoint))
						if verbose {
							fmt.Fprintf(cmd.OutOrStdout(), "    %v\n", probeErr)
						}
					} else {
						reachable[endpoint.Name] = true
						fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %-12s %s\n", endpoint.Name, remote.Destination(endpoint.Endpoint))
					}
				}
				selected, selectErr := remote.ResolveEndpoint(global, resolved.Config.Remote.Target, endpointOverride)
				if selectErr != nil {
					fmt.Fprintln(cmd.OutOrStdout(), "Selected     none")
					return selectErr
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Selected     %s (%s)\n", selected.EndpointName, remote.Destination(selected.Endpoint))
				if !reachable[selected.EndpointName] {
					return fmt.Errorf("selected endpoint %q is unreachable", selected.EndpointName)
				}
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Connectivity not checked (use --check)")
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Target state unregistered\n")
		}
	}
	runs, err := runstate.List(resolved.Root)
	if err == nil && len(runs) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Latest       %s (%s)\n", runs[0].Task, runs[0].Status)
		if verbose {
			fmt.Fprintf(cmd.OutOrStdout(), "Latest run   %s\n", runs[0].ID)
		}
	}
	return nil
}

func showRunStatus(cmd *cobra.Command, runID string) error {
	resolved, err := project.Resolve(projectName)
	if err != nil {
		return err
	}
	id, err := runstate.ResolveSelector(resolved.Root, runID)
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
	if verbose {
		fmt.Fprintf(cmd.OutOrStdout(), "Run        %s\n", m.ID)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Project    %s\n", m.Project)
	fmt.Fprintf(cmd.OutOrStdout(), "Target     %s\n", m.Target)
	if m.Endpoint != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Endpoint   %s\n", m.Endpoint)
	}
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
