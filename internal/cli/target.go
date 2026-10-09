package cli

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/selimserbes/routurn/internal/remote"
	"github.com/spf13/cobra"
)

func newTargetCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "target", Short: "Manage remote SSH targets and connection routes"}
	cmd.AddCommand(
		newTargetAddCmd(),
		newTargetListCmd(),
		newTargetRemoveCmd(),
		newTargetEndpointCmd(),
		newTargetRouteCmd(),
		newTargetMergeCmd(),
		newTargetTestCmd(),
	)
	return cmd
}

func newTargetAddCmd() *cobra.Command {
	var host, user, jump string
	var port int
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add or update a single-route SSH target",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if host == "" {
				return fmt.Errorf("--host is required")
			}
			if err := config.ValidateJump(jump); err != nil {
				return err
			}
			cfg, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			if existing, ok := cfg.Targets[args[0]]; ok && len(existing.Endpoints) > 0 {
				return fmt.Errorf("target %q already has named endpoints; use 'routurn target endpoint add %s <endpoint> ...' instead", args[0], args[0])
			}
			// Updating an existing record without --jump preserves its jump route.
			if !cmd.Flags().Changed("jump") {
				if current, ok := cfg.Targets[args[0]]; ok {
					jump = current.Jump
				}
			}
			cfg.Targets[args[0]] = config.Target{Host: host, User: user, Port: port, Jump: jump}
			if err := config.SaveGlobal(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved target %q -> %s\n", args[0], remote.Destination(config.NormalizeEndpoint(config.Endpoint{Host: host, User: user, Port: port})))
			return nil
		},
	}
	cmd.Flags().StringVar(&host, "host", "", "SSH host or ~/.ssh/config alias")
	cmd.Flags().StringVar(&user, "user", "", "SSH user (optional when defined by SSH config)")
	cmd.Flags().IntVar(&port, "port", 22, "SSH port")
	cmd.Flags().StringVar(&jump, "jump", "", "SSH ProxyJump chain, e.g. user@bastion:2222,second-bastion")
	return cmd
}

func newTargetListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured logical targets and their endpoints",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			names := make([]string, 0, len(cfg.Targets))
			for name := range cfg.Targets {
				names = append(names, name)
			}
			sort.Strings(names)
			if len(names) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No targets configured.")
				return nil
			}
			for _, name := range names {
				target := cfg.Targets[name]
				endpoints := config.EndpointList(target)
				if len(endpoints) == 1 && len(target.Endpoints) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "%-16s %-32s port=%d%s\n", name, remote.Destination(endpoints[0].Endpoint), endpoints[0].Endpoint.Port, jumpLabel(endpoints[0].Endpoint.Jump))
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s  route=%s  endpoints=%d\n", name, config.EffectiveRoute(target), len(endpoints))
				for _, endpoint := range endpoints {
					marker := " "
					if config.EffectiveRoute(target) == endpoint.Name {
						marker = "*"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %s %-12s %-32s port=%d priority=%d%s\n", marker, endpoint.Name, remote.Destination(endpoint.Endpoint), endpoint.Endpoint.Port, displayPriority(endpoint.Endpoint.Priority), jumpLabel(endpoint.Endpoint.Jump))
				}
			}
			return nil
		},
	}
}

func newTargetRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a target",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			if _, ok := cfg.Targets[args[0]]; !ok {
				return fmt.Errorf("target %q does not exist", args[0])
			}
			if users := projectsUsingTarget(cfg, args[0]); len(users) > 0 {
				return fmt.Errorf("target %q is still used by registered project(s): %s", args[0], strings.Join(users, ", "))
			}
			delete(cfg.Targets, args[0])
			if err := config.SaveGlobal(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed target %q\n", args[0])
			return nil
		},
	}
}

func newTargetEndpointCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "endpoint", Short: "Manage named SSH endpoints for a logical target"}
	cmd.AddCommand(newTargetEndpointAddCmd(), newTargetEndpointListCmd(), newTargetEndpointRemoveCmd())
	return cmd
}

func newTargetEndpointAddCmd() *cobra.Command {
	var host, user, primaryName, jump string
	var port, priority int
	cmd := &cobra.Command{
		Use:   "add <target> <endpoint>",
		Short: "Add or update a connection endpoint",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if host == "" {
				return fmt.Errorf("--host is required")
			}
			if err := config.ValidateJump(jump); err != nil {
				return err
			}
			if err := validateEndpointName(args[1]); err != nil {
				return err
			}
			cfg, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			target, ok := cfg.Targets[args[0]]
			if !ok {
				return fmt.Errorf("target %q does not exist", args[0])
			}
			if len(target.Endpoints) == 0 && target.Host != "" {
				target, err = config.PromoteLegacyTarget(target, primaryName)
				if err != nil {
					return err
				}
			}
			if target.Endpoints == nil {
				target.Endpoints = map[string]config.Endpoint{}
			}
			if !cmd.Flags().Changed("jump") {
				if current, ok := target.Endpoints[args[1]]; ok {
					jump = current.Jump
				}
			}
			target.Endpoints[args[1]] = config.NormalizeEndpoint(config.Endpoint{Host: host, User: user, Port: port, Priority: priority, Jump: jump})
			if target.Route == "" {
				target.Route = "auto"
			}
			cfg.Targets[args[0]] = target
			if err := config.SaveGlobal(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved endpoint %s/%s -> %s\n", args[0], args[1], remote.Destination(target.Endpoints[args[1]]))
			fmt.Fprintln(cmd.OutOrStdout(), "This endpoint is treated as another route to the same logical remote machine.")
			return nil
		},
	}
	cmd.Flags().StringVar(&host, "host", "", "SSH host or ~/.ssh/config alias")
	cmd.Flags().StringVar(&user, "user", "", "SSH user")
	cmd.Flags().IntVar(&port, "port", 22, "SSH port")
	cmd.Flags().IntVar(&priority, "priority", 100, "auto-route priority; lower values are preferred")
	cmd.Flags().StringVar(&jump, "jump", "", "SSH ProxyJump chain, e.g. user@bastion:2222,second-bastion")
	cmd.Flags().StringVar(&primaryName, "primary-name", "primary", "name to give an existing single route when converting the target")
	return cmd
}

func newTargetEndpointListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <target>",
		Short: "List endpoints for a logical target",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			target, ok := cfg.Targets[args[0]]
			if !ok {
				return fmt.Errorf("target %q does not exist", args[0])
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Target %s\nRoute  %s\n", args[0], config.EffectiveRoute(target))
			for _, endpoint := range config.EndpointList(target) {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-32s port=%d priority=%d%s\n", endpoint.Name, remote.Destination(endpoint.Endpoint), endpoint.Endpoint.Port, displayPriority(endpoint.Endpoint.Priority), jumpLabel(endpoint.Endpoint.Jump))
			}
			return nil
		},
	}
}

func newTargetEndpointRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <target> <endpoint>",
		Short: "Remove an endpoint from a logical target",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			target, ok := cfg.Targets[args[0]]
			if !ok {
				return fmt.Errorf("target %q does not exist", args[0])
			}
			if target.Endpoints == nil {
				return fmt.Errorf("target %q uses the legacy single-route format; add another endpoint first or replace it with 'target add'", args[0])
			}
			if _, ok := target.Endpoints[args[1]]; !ok {
				return fmt.Errorf("target %q has no endpoint %q", args[0], args[1])
			}
			if len(target.Endpoints) <= 1 {
				return fmt.Errorf("cannot remove the only endpoint from target %q", args[0])
			}
			delete(target.Endpoints, args[1])
			if target.Route == args[1] {
				target.Route = "auto"
			}
			cfg.Targets[args[0]] = target
			if err := config.SaveGlobal(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed endpoint %s/%s\n", args[0], args[1])
			return nil
		},
	}
}

func newTargetRouteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "route <target> [auto|endpoint]",
		Short: "Show or persist the preferred connection route",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			target, ok := cfg.Targets[args[0]]
			if !ok {
				return fmt.Errorf("target %q does not exist", args[0])
			}
			if len(args) == 1 {
				route, err := chooseTargetRoute(cmd, args[0], target)
				if err != nil {
					return err
				}
				if route == "" {
					return nil
				}
				target.Route = route
			} else {
				route := args[1]
				if route != "auto" {
					if _, ok := config.FindEndpoint(target, route); !ok {
						return fmt.Errorf("target %q has no endpoint %q", args[0], route)
					}
				}
				target.Route = route
			}
			cfg.Targets[args[0]] = target
			if err := config.SaveGlobal(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Target %s route set to %s\n", args[0], config.EffectiveRoute(target))
			return nil
		},
	}
}

func chooseTargetRoute(cmd *cobra.Command, targetName string, target config.Target) (string, error) {
	endpoints := config.EndpointList(target)
	fmt.Fprintf(cmd.OutOrStdout(), "Target %s\nCurrent route: %s\n\nChoose route\n", targetName, config.EffectiveRoute(target))
	fmt.Fprintln(cmd.OutOrStdout(), "  1) auto")
	for i, endpoint := range endpoints {
		fmt.Fprintf(cmd.OutOrStdout(), "  %d) %-12s %s\n", i+2, endpoint.Name, remote.Destination(endpoint.Endpoint))
	}
	fmt.Fprintln(cmd.OutOrStdout(), "  q) Cancel")
	reader := bufio.NewReader(cmd.InOrStdin())
	for {
		fmt.Fprint(cmd.OutOrStdout(), "> ")
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			if err == io.EOF {
				return "", fmt.Errorf("interactive route selection needs terminal input; provide auto or an endpoint name")
			}
			return "", err
		}
		line = strings.TrimSpace(line)
		if strings.EqualFold(line, "q") || strings.EqualFold(line, "quit") {
			return "", nil
		}
		n, err := strconv.Atoi(line)
		if err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), "Enter a menu number or q.")
			continue
		}
		if n == 1 {
			return "auto", nil
		}
		if n >= 2 && n <= len(endpoints)+1 {
			return endpoints[n-2].Name, nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Unknown selection.")
	}
}

func newTargetMergeCmd() *cobra.Command {
	var primaryName, secondaryName string
	var secondaryPriority int
	var removeSource bool
	cmd := &cobra.Command{
		Use:   "merge <target> <other-target>",
		Short: "Treat two target records as connection routes to the same logical machine",
		Long:  "Merge explicitly declares that both target records reach the same remote machine. Routurn will then allow safe route selection/fallback within that logical target.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == args[1] {
				return fmt.Errorf("targets must be different")
			}
			if err := validateEndpointName(primaryName); err != nil {
				return err
			}
			if err := validateEndpointName(secondaryName); err != nil {
				return err
			}
			cfg, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			primary, ok := cfg.Targets[args[0]]
			if !ok {
				return fmt.Errorf("target %q does not exist", args[0])
			}
			secondary, ok := cfg.Targets[args[1]]
			if !ok {
				return fmt.Errorf("target %q does not exist", args[1])
			}

			primary, err = config.PromoteLegacyTarget(primary, primaryName)
			if err != nil {
				return err
			}
			secondaryEndpoints := config.EndpointList(secondary)
			if len(secondaryEndpoints) != 1 {
				return fmt.Errorf("source target %q has %d endpoints; merge currently requires exactly one source endpoint", args[1], len(secondaryEndpoints))
			}
			if _, exists := primary.Endpoints[secondaryName]; exists {
				return fmt.Errorf("target %q already has endpoint %q", args[0], secondaryName)
			}
			candidate := secondaryEndpoints[0].Endpoint
			candidate.Priority = secondaryPriority
			primary.Endpoints[secondaryName] = config.NormalizeEndpoint(candidate)
			if primary.Route == "" || primary.Route == config.LegacyEndpointName {
				primary.Route = "auto"
			}
			cfg.Targets[args[0]] = primary
			if removeSource {
				if users := projectsUsingTarget(cfg, args[1]); len(users) > 0 {
					return fmt.Errorf("cannot remove source target %q; it is still used by registered project(s): %s", args[1], strings.Join(users, ", "))
				}
				delete(cfg.Targets, args[1])
			}
			if err := config.SaveGlobal(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Merged %q into logical target %q as endpoint %q\n", args[1], args[0], secondaryName)
			fmt.Fprintln(cmd.OutOrStdout(), "This merge declares that the endpoints reach the same remote machine.")
			fmt.Fprintf(cmd.OutOrStdout(), "Route   %s\n", config.EffectiveRoute(primary))
			if removeSource {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed standalone target %q\n", args[1])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&primaryName, "primary-name", "primary", "endpoint name for the first target's existing route")
	cmd.Flags().StringVar(&secondaryName, "as", "secondary", "endpoint name for the merged route")
	cmd.Flags().IntVar(&secondaryPriority, "priority", 20, "auto-route priority for the merged endpoint")
	cmd.Flags().BoolVar(&removeSource, "remove-source", false, "remove the standalone source target after merging")
	return cmd
}

func newTargetTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test [target]",
		Short: "Check SSH reachability for target endpoints",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			var names []string
			if len(args) == 1 {
				if _, ok := cfg.Targets[args[0]]; !ok {
					return fmt.Errorf("target %q does not exist", args[0])
				}
				names = []string{args[0]}
			} else {
				for name := range cfg.Targets {
					names = append(names, name)
				}
				sort.Strings(names)
			}
			failed := false
			for _, name := range names {
				target := cfg.Targets[name]
				fmt.Fprintf(cmd.OutOrStdout(), "%s  route=%s\n", name, config.EffectiveRoute(target))
				for _, endpoint := range config.EndpointList(target) {
					err := remote.Probe(endpoint.Endpoint, 2)
					if err != nil {
						failed = true
						fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %-12s %-32s unreachable\n", endpoint.Name, remote.Destination(endpoint.Endpoint))
						if verbose {
							fmt.Fprintf(cmd.OutOrStdout(), "    %v\n", err)
						}
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %-12s %-32s reachable\n", endpoint.Name, remote.Destination(endpoint.Endpoint))
					}
				}
			}
			if failed {
				return fmt.Errorf("one or more endpoints are unreachable")
			}
			return nil
		},
	}
}

func validateEndpointName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("endpoint name must not be empty")
	}
	if name == "auto" {
		return fmt.Errorf("endpoint name %q is reserved", name)
	}
	if strings.ContainsAny(name, " \t\r\n/") {
		return fmt.Errorf("endpoint name %q contains unsupported characters", name)
	}
	return nil
}

func projectsUsingTarget(cfg *config.GlobalConfig, targetName string) []string {
	var projects []string
	for name, link := range cfg.Projects {
		projectCfg, err := config.LoadProject(link.Root)
		if err != nil {
			continue
		}
		if projectCfg.Remote.Target == targetName {
			projects = append(projects, name)
		}
	}
	sort.Strings(projects)
	return projects
}

func jumpLabel(jump string) string {
	if jump == "" {
		return ""
	}
	return " jump=" + jump
}

func displayPriority(priority int) int {
	if priority <= 0 {
		return 100
	}
	return priority
}
