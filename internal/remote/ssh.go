package remote

import (
	"bytes"
	"fmt"
	"io"
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

func sshArgs(target config.Target, interactive bool) []string {
	args := []string{"-o", "ServerAliveInterval=30", "-o", "ServerAliveCountMax=3"}
	if target.Port > 0 && target.Port != 22 {
		args = append(args, "-p", strconv.Itoa(target.Port))
	}
	if interactive {
		args = append(args, "-t")
	}
	return args
}

func Run(target config.Target, remotePath, command string, interactive bool) error {
	_, err := RunWithIO(target, remotePath, command, interactive, os.Stdin, os.Stdout, os.Stderr)
	return err
}

func RunWithIO(target config.Target, remotePath, command string, interactive bool, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	args := sshArgs(target, interactive)
	remoteCommand := fmt.Sprintf("cd %s && exec sh -lc %s", shellQuote(remotePath), shellQuote(command))
	args = append(args, Destination(target), remoteCommand)

	cmd := exec.Command("ssh", args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), fmt.Errorf("remote task failed with exit code %d", exitErr.ExitCode())
		}
		return -1, fmt.Errorf("run ssh: %w", err)
	}
	return 0, nil
}

func RunCommand(target config.Target, command string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	args := sshArgs(target, false)
	args = append(args, Destination(target), command)
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), fmt.Errorf("remote command failed with exit code %d", exitErr.ExitCode())
		}
		return -1, fmt.Errorf("run ssh: %w", err)
	}
	return 0, nil
}

func Capture(target config.Target, command string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	_, err := RunCommand(target, command, nil, &stdout, &stderr)
	if err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

func ShellQuote(value string) string {
	return shellQuote(value)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
