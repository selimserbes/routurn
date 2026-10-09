package remote

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/selimserbes/routurn/internal/config"
)

var (
	verboseMu     sync.RWMutex
	verbose       bool
	verboseWriter io.Writer = io.Discard
)

// SetVerbose enables or disables diagnostic output for remote commands.
func SetVerbose(enabled bool, writer io.Writer) {
	verboseMu.Lock()
	defer verboseMu.Unlock()
	verbose = enabled
	if writer == nil {
		writer = io.Discard
	}
	verboseWriter = writer
}

func debugCommand(name string, args []string) {
	verboseMu.RLock()
	defer verboseMu.RUnlock()
	if !verbose {
		return
	}
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, name)
	for _, arg := range args {
		parts = append(parts, shellDisplayQuote(arg))
	}
	fmt.Fprintf(verboseWriter, "+ %s\n", strings.Join(parts, " "))
}

func shellDisplayQuote(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return !(r == '-' || r == '_' || r == '.' || r == '/' || r == ':' || r == '@' || r == '%' || r == '=' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	}) == -1 {
		return value
	}
	return shellQuote(value)
}

func Destination(target config.Endpoint) string {
	if target.User == "" {
		return target.Host
	}
	return target.User + "@" + target.Host
}

func sshArgs(target config.Endpoint, interactive bool) []string {
	args := []string{
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
	}
	if controlPath := sshControlPath(target.Jump); controlPath != "" {
		args = append(args,
			"-o", "ControlMaster=auto",
			"-o", "ControlPersist=120",
			"-o", "ControlPath="+controlPath,
		)
	}
	if target.Jump != "" {
		// -J is routed through OpenSSH directly; no ProxyCommand shell and no
		// SSH agent forwarding through the destination session.
		args = append(args, "-o", "ForwardAgent=no", "-J", target.Jump)
	}
	if target.Port > 0 && target.Port != 22 {
		args = append(args, "-p", strconv.Itoa(target.Port))
	}
	if interactive {
		args = append(args, "-t")
	}
	return args
}

func sshControlPath(jump string) string {
	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		return ""
	}
	dir := filepath.Join(cacheDir, "routurn", "ssh")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	// Keep direct and bastion-routed connections in distinct multiplex pools.
	// A previously established direct socket must never bypass a selected jump.
	if jump != "" {
		fingerprint := sha256.Sum256([]byte(jump))
		return filepath.Join(dir, "j"+hex.EncodeToString(fingerprint[:6])+"-%C")
	}
	return filepath.Join(dir, "%C")
}

func Run(target config.Endpoint, remotePath, command string, interactive bool) error {
	_, err := RunWithIO(target, remotePath, command, interactive, os.Stdin, os.Stdout, os.Stderr)
	return err
}

func RunWithIO(target config.Endpoint, remotePath, command string, interactive bool, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	args := sshArgs(target, interactive)
	remoteCommand := fmt.Sprintf("cd %s && exec sh -lc %s", shellQuote(remotePath), shellQuote(command))
	args = append(args, Destination(target), remoteCommand)
	debugCommand("ssh", args)

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

func RunCommand(target config.Endpoint, command string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	args := sshArgs(target, false)
	args = append(args, Destination(target), command)
	debugCommand("ssh", args)
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

func Capture(target config.Endpoint, command string) ([]byte, error) {
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

// Probe checks whether an SSH endpoint can be reached non-interactively. It is
// used only for automatic route selection, so it deliberately avoids password
// prompts and uses a short connection timeout.
func Probe(target config.Endpoint, timeoutSeconds int) error {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 2
	}
	args := sshArgs(target, false)
	args = append(args,
		"-o", "BatchMode=yes",
		"-o", "ConnectionAttempts=1",
		"-o", "ConnectTimeout="+strconv.Itoa(timeoutSeconds),
		Destination(target),
		"true",
	)
	debugCommand("ssh", args)
	cmd := exec.Command("ssh", args...)
	var stderr bytes.Buffer
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return err
	}
	return nil
}
