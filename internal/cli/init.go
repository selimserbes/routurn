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
	var localMode bool
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

			// A second project can share a configured name. Never silently
			// repoint its existing registry entry to this new directory.
			global, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			if old, ok := global.Projects[name]; ok && filepath.Clean(old.Root) != filepath.Clean(cwd) {
				return fmt.Errorf("project name %q is already registered to %s; choose another --name", name, old.Root)
			}

			mode := "remote"
			if localMode {
				mode = "local"
			}
			template := fmt.Sprintf(`version = 1
name = %q

[execution]
mode = %q

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

# Tasks are optional. Run routurn exec to discover common project commands,
# or routurn exec -- <command> to run any command in the selected mode.
`, name, mode)
			if err := os.WriteFile(path, []byte(template), 0o644); err != nil {
				return err
			}

			global.Projects[name] = config.ProjectLink{Root: cwd}
			if err := config.SaveGlobal(global); err != nil {
				_ = os.Remove(path) // don't leave an initialized project unregistered
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Initialized Routurn project %q\n", name)
			fmt.Fprintf(cmd.OutOrStdout(), "Config: %s\n", path)
			if localMode {
				fmt.Fprintln(cmd.OutOrStdout(), "Next: run 'routurn exec' to discover and execute local commands.")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Next: configure [remote], then run 'routurn exec' to discover project commands.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "project name (defaults to current directory name)")
	cmd.Flags().BoolVar(&localMode, "local", false, "initialize an SSH-free local project")
	return cmd
}
