package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/spf13/cobra"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "project", Short: "Manage registered local projects"}
	cmd.AddCommand(newProjectAddCmd(), newProjectListCmd(), newProjectShowCmd(), newProjectRemoveCmd())
	return cmd
}

func newProjectAddCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "add [path]",
		Short: "Register an existing Routurn project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			start := "."
			if len(args) == 1 {
				start = args[0]
			}
			abs, err := filepath.Abs(start)
			if err != nil {
				return err
			}
			root, err := config.FindProjectRoot(abs)
			if err != nil {
				return fmt.Errorf("no %s found from %s upward", config.ProjectFileName, abs)
			}
			cfg, err := config.LoadProject(root)
			if err != nil {
				return err
			}
			if name == "" {
				name = cfg.Name
			}
			if name == "" {
				return fmt.Errorf("project name is empty; set name in %s or pass --name", config.ProjectFileName)
			}

			global, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			global.Projects[name] = config.ProjectLink{Root: root}
			if err := config.SaveGlobal(global); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Registered project %q -> %s\n", name, root)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "registered name (defaults to name in routurn.toml)")
	return cmd
}

func newProjectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered projects",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			names := make([]string, 0, len(cfg.Projects))
			for name := range cfg.Projects {
				names = append(names, name)
			}
			sort.Strings(names)
			if len(names) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No projects registered.")
				return nil
			}
			for _, name := range names {
				link := cfg.Projects[name]
				state := "ok"
				if _, err := os.Stat(filepath.Join(link.Root, config.ProjectFileName)); err != nil {
					state = "missing"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-8s %s\n", name, state, link.Root)
			}
			return nil
		},
	}
}

func newProjectShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show a registered project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			link, ok := cfg.Projects[args[0]]
			if !ok {
				return fmt.Errorf("project %q is not registered", args[0])
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Name  %s\n", args[0])
			fmt.Fprintf(cmd.OutOrStdout(), "Root  %s\n", link.Root)
			if projectCfg, err := config.LoadProject(link.Root); err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Target %s\n", projectCfg.Remote.Target)
				fmt.Fprintf(cmd.OutOrStdout(), "Remote %s\n", projectCfg.Remote.Path)
				fmt.Fprintf(cmd.OutOrStdout(), "Tasks  %d\n", len(projectCfg.Tasks))
			}
			return nil
		},
	}
}

func newProjectRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a project from the local registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			if _, ok := cfg.Projects[args[0]]; !ok {
				return fmt.Errorf("project %q is not registered", args[0])
			}
			delete(cfg.Projects, args[0])
			if err := config.SaveGlobal(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed project %q from the local registry\n", args[0])
			return nil
		},
	}
}
