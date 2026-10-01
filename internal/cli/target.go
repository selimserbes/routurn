package cli

import (
	"fmt"
	"sort"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/spf13/cobra"
)

func newTargetCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "target", Short: "Manage remote SSH targets"}
	cmd.AddCommand(newTargetAddCmd(), newTargetListCmd(), newTargetRemoveCmd())
	return cmd
}

func newTargetAddCmd() *cobra.Command {
	var host, user string
	var port int
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add or update an SSH target",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if host == "" {
				return fmt.Errorf("--host is required")
			}
			cfg, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			cfg.Targets[args[0]] = config.Target{Host: host, User: user, Port: port}
			if err := config.SaveGlobal(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved target %q -> %s\n", args[0], host)
			return nil
		},
	}
	cmd.Flags().StringVar(&host, "host", "", "SSH host or ~/.ssh/config alias")
	cmd.Flags().StringVar(&user, "user", "", "SSH user (optional when defined by SSH config)")
	cmd.Flags().IntVar(&port, "port", 22, "SSH port")
	return cmd
}

func newTargetListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured targets",
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
				t := cfg.Targets[name]
				dest := t.Host
				if t.User != "" {
					dest = t.User + "@" + t.Host
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-16s %-32s port=%d\n", name, dest, t.Port)
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
			delete(cfg.Targets, args[0])
			if err := config.SaveGlobal(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed target %q\n", args[0])
			return nil
		},
	}
}
