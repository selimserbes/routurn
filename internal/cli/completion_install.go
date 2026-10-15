package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// installShellCompletion installs only user-level shell integration.
// It does not require administrator rights or affect other completions.
func installShellCompletion(root *cobra.Command, shell string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	var script bytes.Buffer
	var dest string
	var hookPath, hook string
	switch shell {
	case "zsh":
		if err = root.GenZshCompletion(&script); err != nil {
			return "", err
		}
		custom := os.Getenv("ZSH_CUSTOM")
		if custom == "" {
			base := os.Getenv("ZSH")
			if base == "" {
				base = filepath.Join(home, ".oh-my-zsh")
			}
			if info, statErr := os.Stat(filepath.Join(base, "oh-my-zsh.sh")); statErr == nil && !info.IsDir() {
				custom = filepath.Join(base, "custom")
			}
		}
		if custom != "" {
			dest = filepath.Join(custom, "completions", "_routurn")
		} else {
			dest = filepath.Join(home, ".local", "share", "zsh", "site-functions", "_routurn")
			hookPath = filepath.Join(home, ".zshrc")
			hook = `if [[ -r "$HOME/.local/share/zsh/site-functions/_routurn" ]]; then
  autoload -Uz compinit
  (( $+functions[compdef] )) || compinit
  source "$HOME/.local/share/zsh/site-functions/_routurn"
fi`
		}
	case "bash":
		if err = root.GenBashCompletion(&script); err != nil {
			return "", err
		}
		dest = filepath.Join(home, ".local", "share", "bash-completion", "completions", "routurn")
		// Also load from ~/.bashrc so Bash users without bash-completion get Tab support.
		hookPath = filepath.Join(home, ".bashrc")
		hook = `[ -r "$HOME/.local/share/bash-completion/completions/routurn" ] && . "$HOME/.local/share/bash-completion/completions/routurn"`
	case "fish":
		if err = root.GenFishCompletion(&script, true); err != nil {
			return "", err
		}
		dest = filepath.Join(home, ".config", "fish", "completions", "routurn.fish")
	case "powershell":
		if err = root.GenPowerShellCompletion(&script); err != nil {
			return "", err
		}
		if runtime.GOOS == "windows" {
			dest = filepath.Join(home, "Documents", "PowerShell", "Completions", "routurn.ps1")
			hookPath = filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
		} else {
			dest = filepath.Join(home, ".config", "powershell", "Completions", "routurn.ps1")
			hookPath = filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1")
		}
		// Single quotes must be doubled for PowerShell literal strings.
		quoted := strings.ReplaceAll(dest, "'", "''")
		hook = `. '` + quoted + `'`
	default:
		return "", fmt.Errorf("unsupported completion shell %q", shell)
	}

	if err := writeCompletionFile(dest, script.Bytes()); err != nil {
		return "", err
	}
	if hookPath != "" {
		if err := appendCompletionHook(hookPath, hook); err != nil {
			return dest, fmt.Errorf("completion created but shell profile could not be updated: %w", err)
		}
	}
	if shell == "powershell" && runtime.GOOS == "windows" {
		legacyProfile := filepath.Join(home, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")
		if err := appendCompletionHook(legacyProfile, hook); err != nil {
			return dest, fmt.Errorf("completion created but legacy PowerShell profile could not be updated: %w", err)
		}
	}
	return dest, nil
}

func writeCompletionFile(dest string, contents []byte) error {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if current, err := os.Lstat(dest); err == nil {
		if !current.Mode().IsRegular() {
			return fmt.Errorf("completion destination is not a regular file: %s", dest)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	f, err := os.CreateTemp(dir, ".routurn-completion-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(contents); err != nil {
		f.Close()
		return err
	}
	if err = f.Chmod(0o644); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, dest)
}

func appendCompletionHook(path, hook string) error {
	const marker = "# Routurn shell completion (managed)"
	old, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(old), marker) {
		return nil
	}
	if current, statErr := os.Lstat(path); statErr == nil && !current.Mode().IsRegular() {
		return fmt.Errorf("refusing to change non-regular profile: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n%s\n%s\n", marker, hook)
	return err
}
