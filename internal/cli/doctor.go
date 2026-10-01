package cli

import (
	"fmt"
	"os/exec"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/selimserbes/routurn/internal/project"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check local Routurn prerequisites and project configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			checks := []struct {
				name     string
				required bool
			}{
				{"ssh", true},
				{"tar", true},
				{"rsync", false},
				{"git", false},
			}

			failed := false
			for _, check := range checks {
				path, err := exec.LookPath(check.name)
				if err != nil {
					if check.required {
						fmt.Fprintf(cmd.OutOrStdout(), "✗ %-8s missing (required)\n", check.name)
						failed = true
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "! %-8s missing (optional)\n", check.name)
					}
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "✓ %-8s %s\n", check.name, path)
			}

			if resolved, err := project.Resolve(projectName); err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ project  %s (%s)\n", resolved.Config.Name, resolved.Root)
				if resolved.Config.Remote.Target == "" || resolved.Config.Remote.Path == "" {
					fmt.Fprintln(cmd.OutOrStdout(), "! remote   target/path not configured in routurn.toml")
				} else {
					global, err := config.LoadGlobal()
					if err != nil {
						return err
					}
					if _, ok := global.Targets[resolved.Config.Remote.Target]; ok {
						fmt.Fprintf(cmd.OutOrStdout(), "✓ target   %s\n", resolved.Config.Remote.Target)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "✗ target   %s is not registered\n", resolved.Config.Remote.Target)
						failed = true
					}
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "! project  %v\n", err)
			}

			if failed {
				return fmt.Errorf("doctor found required issues")
			}
			return nil
		},
	}
}
