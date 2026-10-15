package cli

import (
	"sort"
	"strings"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/selimserbes/routurn/internal/project"
	"github.com/spf13/cobra"
)

// configureDynamicCompletion reads only local Routurn state. Tab completion
// must never open SSH connections, run tasks or modify projects.
func configureDynamicCompletion(root *cobra.Command) {
	bind := func(path []string, fn func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)) {
		node := root
		for _, part := range path {
			var next *cobra.Command
			for _, child := range node.Commands() {
				if child.Name() == part {
					next = child
					break
				}
			}
			if next == nil {
				panic("missing completion command: " + strings.Join(path, " "))
			}
			node = next
		}
		node.ValidArgsFunction = fn
	}

	targetNames := func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		cfg, err := config.LoadGlobal()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completionKeys(cfg.Targets, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
	projectNames := func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		cfg, err := config.LoadGlobal()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completionKeys(cfg.Projects, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
	taskNames := func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		resolved, err := project.Resolve(projectName)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completionKeys(resolved.Config.Tasks, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
	targetThenEndpoint := func(includeAuto bool) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			cfg, err := config.LoadGlobal()
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			switch len(args) {
			case 0:
				return completionKeys(cfg.Targets, toComplete), cobra.ShellCompDirectiveNoFileComp
			case 1:
				target, found := cfg.Targets[args[0]]
				if !found {
					return nil, cobra.ShellCompDirectiveNoFileComp
				}
				values := make([]string, 0, len(target.Endpoints)+1)
				if includeAuto {
					values = append(values, "auto")
				}
				for _, ep := range config.EndpointList(target) {
					values = append(values, ep.Name)
				}
				return completionFilter(values, toComplete), cobra.ShellCompDirectiveNoFileComp
			default:
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
		}
	}
	twoTargets := func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 1 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		cfg, err := config.LoadGlobal()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		candidates := completionKeys(cfg.Targets, toComplete)
		if len(args) == 0 {
			return candidates, cobra.ShellCompDirectiveNoFileComp
		}
		filtered := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate != args[0] {
				filtered = append(filtered, candidate)
			}
		}
		return filtered, cobra.ShellCompDirectiveNoFileComp
	}

	for _, path := range [][]string{
		{"target", "test"}, {"target", "remove"},
		{"target", "endpoint", "list"},
	} {
		bind(path, targetNames)
	}
	for _, path := range [][]string{
		{"target", "endpoint", "add"}, {"target", "endpoint", "remove"},
	} {
		bind(path, targetThenEndpoint(false))
	}
	bind([]string{"target", "route"}, targetThenEndpoint(true))
	bind([]string{"target", "merge"}, twoTargets)

	for _, path := range [][]string{
		{"project", "show"}, {"project", "remove"},
	} {
		bind(path, projectNames)
	}
	// 'project check' also takes paths, so leave filename completion available.
	bind([]string{"project", "check"}, func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		names, _ := projectNames(cmd, args, prefix)
		return names, cobra.ShellCompDirectiveDefault
	})
	for _, path := range [][]string{
		{"exec"}, {"run"}, {"result"}, {"task", "remove"},
	} {
		bind(path, taskNames)
	}

	_ = root.RegisterFlagCompletionFunc("project", projectNames)
	_ = root.RegisterFlagCompletionFunc("endpoint", func(_ *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		resolved, err := project.Resolve(projectName)
		if err != nil || resolved.Config.IsLocal() {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		cfg, err := config.LoadGlobal()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		target, found := cfg.Targets[resolved.Config.Remote.Target]
		if !found {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names := make([]string, 0, len(target.Endpoints))
		for _, endpoint := range config.EndpointList(target) {
			names = append(names, endpoint.Name)
		}
		return completionFilter(names, prefix), cobra.ShellCompDirectiveNoFileComp
	})
}

func completionKeys[T any](values map[string]T, prefix string) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	return completionFilter(names, prefix)
}

func completionFilter(names []string, prefix string) []string {
	found := make([]string, 0, len(names))
	for _, name := range names {
		if strings.HasPrefix(name, prefix) {
			found = append(found, name)
		}
	}
	sort.Strings(found)
	return found
}
