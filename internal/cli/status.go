package cli

import (
	"fmt"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/selimserbes/routurn/internal/project"
	"github.com/selimserbes/routurn/internal/remote"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the resolved project and remote target",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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
			return nil
		},
	}
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
