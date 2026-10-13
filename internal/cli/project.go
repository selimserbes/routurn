package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/selimserbes/routurn/internal/project"
	"github.com/spf13/cobra"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "project", Short: "Manage registered local projects"}
	cmd.AddCommand(newProjectAddCmd(), newProjectListCmd(), newProjectShowCmd(), newProjectRemoveCmd(), newProjectSelectCmd(), newProjectCheckCmd())
	return cmd
}

func newProjectAddCmd() *cobra.Command {
	var name string
	var replace bool
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
			if old, ok := global.Projects[name]; ok && filepath.Clean(old.Root) != filepath.Clean(root) && !replace {
				return fmt.Errorf("project name %q is already registered to %s; use a different --name or explicitly pass --replace", name, old.Root)
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
	cmd.Flags().BoolVar(&replace, "replace", false, "explicitly replace an existing name pointing at a different directory")
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
				} else if _, err := config.LoadProject(link.Root); err != nil {
					state = "invalid"
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
			projectCfg, err := config.LoadProject(link.Root)
			if err != nil {
				return err
			}
			mode := "remote"
			if projectCfg.IsLocal() {
				mode = "local"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Mode  %s\n", mode)
			if !projectCfg.IsLocal() {
				fmt.Fprintf(cmd.OutOrStdout(), "Target %s\n", projectCfg.Remote.Target)
				fmt.Fprintf(cmd.OutOrStdout(), "Remote %s\n", projectCfg.Remote.Path)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Tasks  %d\n", len(projectCfg.Tasks))
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

// Project selection prints an explicit command rather than modifying a global
// default or attempting to change the parent shell's working directory.
func newProjectSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select",
		Short: "Choose a registered project without changing any defaults",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !pickerIsInteractiveTerminal(cmd) {
				return fmt.Errorf("project select requires an interactive terminal; use 'routurn project list' or 'routurn -p NAME exec'")
			}
			global, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			selected, err := chooseRegisteredProject(cmd, global)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Project %s\nRoot    %s\n", selected.Name, selected.Root)
			fmt.Fprintf(cmd.OutOrStdout(), "Next: routurn -p %s exec\n", shellQuote(selected.Name))
			return nil
		},
	}
}

// Project check validates local configuration only. It must not make an SSH
// connection, alter a project or silently resolve an automatic remote route.
func newProjectCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check [name-or-path]",
		Short: "Validate a project configuration without connecting to a server",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			selection := projectName
			if len(args) == 1 {
				if projectName != "" {
					return fmt.Errorf("use either --project or an argument, not both")
				}
				selection = args[0]
			}
			resolved, err := project.Resolve(selection)
			if err != nil {
				return err
			}
			global, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Project %s\nRoot    %s\n", resolved.Config.Name, resolved.Root)
			fmt.Fprintf(cmd.OutOrStdout(), "Tasks   %d\n", len(resolved.Config.Tasks))
			if resolved.Config.IsLocal() {
				fmt.Fprintln(cmd.OutOrStdout(), "Mode    local (no SSH required)")
				fmt.Fprintln(cmd.OutOrStdout(), "✓ Project configuration valid")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Mode    remote")
			target := resolved.Config.Remote.Target
			remotePath := resolved.Config.Remote.Path
			if target == "" || remotePath == "" {
				return fmt.Errorf("missing [remote].target or [remote].path in %s", config.ProjectFileName)
			}
			if _, ok := global.Targets[target]; !ok {
				return fmt.Errorf("remote target %q is not registered; use 'routurn target add'", target)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Target  %s\nRemote  %s\n", target, remotePath)
			fmt.Fprintln(cmd.OutOrStdout(), "✓ Basic project configuration valid (SSH connectivity and endpoint route not tested)")
			return nil
		},
	}
}
