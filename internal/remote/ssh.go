package remote

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/selimserbes/routurn/internal/config"
)

func Destination(target config.Target) string {
	if target.User == "" {
		return target.Host
	}
	return target.User + "@" + target.Host
}

func Run(target config.Target, remotePath, command string, interactive bool) error {
	args := []string{"-o", "ServerAliveInterval=30", "-o", "ServerAliveCountMax=3"}
	if target.Port > 0 && target.Port != 22 {
		args = append(args, "-p", strconv.Itoa(target.Port))
	}
	if interactive {
		args = append(args, "-t")
	}

	remoteCommand := fmt.Sprintf("cd %s && exec sh -lc %s", shellQuote(remotePath), shellQuote(command))
	args = append(args, Destination(target), remoteCommand)

	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("remote task failed with exit code %d", exitErr.ExitCode())
		}
		return fmt.Errorf("run ssh: %w", err)
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
