package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Routurn in the current project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			cwd, err = filepath.Abs(cwd)
			if err != nil {
				return err
			}
			if name == "" {
				name = filepath.Base(cwd)
			}

			path := filepath.Join(cwd, config.ProjectFileName)
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("%s already exists", path)
			}

			template := fmt.Sprintf(`version = 1
name = %q

[remote]
target = ""
path = ""

[sync]
exclude = [
  ".git/**",
  ".routurn/**",
  ".venv/**",
  "venv/**",
  "**/__pycache__/**",
  "node_modules/**",
  "target/**",
]

# Example task:
# [tasks.test]
# command = "go test ./..."
# artifacts = []
`, name)
			if err := os.WriteFile(path, []byte(template), 0o644); err != nil {
				return err
			}

			global, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			global.Projects[name] = config.ProjectLink{Root: cwd}
			if err := config.SaveGlobal(global); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Initialized Routurn project %q\n", name)
			fmt.Fprintf(cmd.OutOrStdout(), "Config: %s\n", path)
			fmt.Fprintln(cmd.OutOrStdout(), "Next: configure [remote] and at least one [tasks.<name>] entry.")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "project name (defaults to current directory name)")
	return cmd
}
