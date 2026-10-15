package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCompletionCmd(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:       "completion <shell>",
		Short:     "Generate shell completion scripts",
		Long:      "Generate shell completion scripts for bash, zsh, fish, or PowerShell.",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := args[0]
			switch shell {
			case "bash":
				return root.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletion(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell %q", shell)
			}
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:       "install <shell>",
		Short:     "Install shell completion for the current user",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			installed, err := installShellCompletion(root, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Installed %s completion: %s\nOpen a new terminal to activate it.\n", args[0], installed)
			return nil
		},
	})
	return cmd
}
