package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/selimserbes/routurn/internal/bundle"
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
			resolved, projectErr := project.Resolve(projectName)
			localMode := projectErr == nil && resolved.Config.IsLocal()
			if path, err := exec.LookPath("ssh"); err != nil {
				if localMode {
					fmt.Fprintln(cmd.OutOrStdout(), "! ssh      missing (not needed for local mode)")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "✗ ssh      missing (required)")
					failed = true
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ ssh      %s\n", path)
			}
			if path, err := exec.LookPath("git"); err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ git      %s (optional)\n", path)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "! git      missing (optional)")
			}

			if path, ok := bundle.ExternalImporter(); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ archive  %s (optional .7z/.rar importer)\n", path)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "! archive  7zz/7z/7za not found (optional; ZIP/TAR remain native)")
			}

			if projectErr != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "! project  %v\n", projectErr)
				if failed {
					return fmt.Errorf("doctor found required issues")
				}
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ project  %s (%s)\n", resolved.Config.Name, resolved.Root)
			if localMode {
				fmt.Fprintln(cmd.OutOrStdout(), "✓ mode     local (SSH not required)")
				if _, err := exec.LookPath("sh"); err != nil {
					if runtime.GOOS != "windows" {
						return fmt.Errorf("local shell sh not found: %w", err)
					}
				}
				return nil
			}
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
			if _, ok := global.Targets[resolved.Config.Remote.Target]; !ok {
				fmt.Fprintf(cmd.OutOrStdout(), "✗ target   %s is not registered\n", resolved.Config.Remote.Target)
				failed = true
			} else {
				selected, selectErr := remote.ResolveEndpoint(global, resolved.Config.Remote.Target, endpointOverride)
				if selectErr != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "✗ target   %s: %v\n", resolved.Config.Remote.Target, selectErr)
					failed = true
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "✓ target   %s route=%s endpoint=%s (%s)\n", resolved.Config.Remote.Target, selected.Route, selected.EndpointName, remote.Destination(selected.Endpoint))
					probe := "for c in sh tar find; do command -v \"$c\" || exit 42; done"
					data, probeErr := remote.Capture(selected.Endpoint, probe)
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
			}

			if failed {
				return fmt.Errorf("doctor found required issues")
			}
			return nil
		},
	}
}
