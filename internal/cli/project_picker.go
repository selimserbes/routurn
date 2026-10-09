package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/selimserbes/routurn/internal/project"
	"github.com/spf13/cobra"
)

type registeredProject struct {
	Name string
	Root string
	Mode string
}

// availableProjects contains only valid registered projects. Bad or removed
// entries remain visible through `routurn project list` but cannot be selected.
func availableProjects(cfg *config.GlobalConfig) []registeredProject {
	list := make([]registeredProject, 0, len(cfg.Projects))
	for name, link := range cfg.Projects {
		projectCfg, err := config.LoadProject(link.Root)
		if err != nil {
			continue
		}
		mode := "remote"
		if projectCfg.IsLocal() {
			mode = "local"
		}
		list = append(list, registeredProject{Name: name, Root: link.Root, Mode: mode})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}

func chooseRegisteredProject(cmd *cobra.Command, cfg *config.GlobalConfig) (registeredProject, error) {
	var available []registeredProject
	withPickerLoading(cmd.OutOrStdout(), pickerIsInteractiveTerminal(cmd), "Finding registered projects...", func() {
		available = availableProjects(cfg)
	})
	if len(available) == 0 {
		return registeredProject{}, fmt.Errorf("no usable registered projects; run 'routurn project add /path/to/project' or 'routurn project list' to inspect entries")
	}
	menu := interactiveMenu{Heading: "Choose a project", ConfirmHint: "Use selected project"}
	for _, p := range available {
		menu.Items = append(menu.Items, interactiveMenuItem{Label: p.Name, Detail: p.Mode + " · " + p.Root})
	}
	selection, err := chooseInteractiveMenu(cmd, "Projects", menu)
	if err != nil {
		return registeredProject{}, err
	}
	if selection.Index < 0 {
		return registeredProject{}, fmt.Errorf("project selection cancelled")
	}
	if selection.Index >= len(available) {
		return registeredProject{}, fmt.Errorf("invalid project selection")
	}
	return available[selection.Index], nil
}

// resolveLocalProjectContextForPicker is used only when opening a menu without
// explicit arguments. It never changes the registered projects or saves a
// preferred project. Noninteractive calls always require cwd or -p context.
func resolveLocalProjectContextForPicker(cmd *cobra.Command) (*projectContext, error) {
	if projectName != "" || !pickerIsInteractiveTerminal(cmd) {
		return resolveLocalProjectContext()
	}
	resolved, err := project.Resolve("")
	if err == nil {
		global, loadErr := config.LoadGlobal()
		if loadErr != nil {
			return nil, loadErr
		}
		return &projectContext{Resolved: resolved, Global: global}, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err // malformed local project: never silently pick another
	}
	global, loadErr := config.LoadGlobal()
	if loadErr != nil {
		return nil, loadErr
	}
	chosen, selectErr := chooseRegisteredProject(cmd, global)
	if selectErr != nil {
		return nil, selectErr
	}
	resolved, err = project.Resolve(chosen.Name)
	if err != nil { // e.g. registered project was deleted while menu was open
		return nil, err
	}
	return &projectContext{Resolved: resolved, Global: global}, nil
}
