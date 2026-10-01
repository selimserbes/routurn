package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/selimserbes/routurn/internal/project"
	"github.com/selimserbes/routurn/internal/remote"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check local prerequisites, project configuration, and remote capabilities",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			failed := false
			if path, err := exec.LookPath("ssh"); err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "✗ ssh      missing (required)")
				failed = true
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ ssh      %s\n", path)
			}
			if path, err := exec.LookPath("git"); err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ git      %s (optional)\n", path)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "! git      missing (optional)")
			}

			resolved, err := project.Resolve(projectName)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "! project  %v\n", err)
				if failed {
					return fmt.Errorf("doctor found required issues")
				}
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ project  %s (%s)\n", resolved.Config.Name, resolved.Root)
			if resolved.Config.Remote.Target == "" || resolved.Config.Remote.Path == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "! remote   target/path not configured in routurn.toml")
				if failed {
					return fmt.Errorf("doctor found required issues")
				}
				return nil
			}

			global, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			target, ok := global.Targets[resolved.Config.Remote.Target]
			if !ok {
				fmt.Fprintf(cmd.OutOrStdout(), "✗ target   %s is not registered\n", resolved.Config.Remote.Target)
				failed = true
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ target   %s (%s)\n", resolved.Config.Remote.Target, remote.Destination(target))
				probe := "for c in sh tar find; do command -v \"$c\" || exit 42; done"
				data, probeErr := remote.Capture(target, probe)
				if probeErr != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "✗ remote   SSH/capability check failed: %v\n", probeErr)
					failed = true
				} else {
					lines := strings.Fields(string(data))
					fmt.Fprintln(cmd.OutOrStdout(), "✓ remote   SSH connection works")
					if len(lines) >= 3 {
						fmt.Fprintln(cmd.OutOrStdout(), "✓ tools    remote sh, tar, and find available")
					}
				}
			}

			if failed {
				return fmt.Errorf("doctor found required issues")
			}
			return nil
		},
	}
}
