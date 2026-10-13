package cli

import (
	"fmt"
	"os"

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
		Short:     "Print the recommended command for installing completion",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := args[0]
			home, _ := os.UserHomeDir()
			switch shell {
			case "bash":
				fmt.Fprintln(cmd.OutOrStdout(), "routurn completion bash > ~/.local/share/bash-completion/completions/routurn")
			case "zsh":
				fmt.Fprintf(cmd.OutOrStdout(), "mkdir -p %s/.zfunc && routurn completion zsh > %s/.zfunc/_routurn\n", home, home)
			case "fish":
				fmt.Fprintln(cmd.OutOrStdout(), "routurn completion fish > ~/.config/fish/completions/routurn.fish")
			case "powershell":
				fmt.Fprintln(cmd.OutOrStdout(), "routurn completion powershell | Out-String | Invoke-Expression")
			default:
				return fmt.Errorf("unsupported shell %q", shell)
			}
			return nil
		},
	})

	return cmd
}
