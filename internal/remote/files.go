package remote

import (
	"archive/tar"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/selimserbes/routurn/internal/config"
)

func EnsureDir(target config.Target, remotePath string) error {
	cmd := "mkdir -p " + shellQuote(remotePath)
	_, err := RunCommand(target, cmd, nil, io.Discard, os.Stderr)
	return err
}

func CreateSnapshot(target config.Target, remoteRoot, snapshotID string, paths []string) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}
	snapshotRel := ".routurn/snapshots/" + snapshotID + ".tar"
	script := fmt.Sprintf(`set -eu
cd %s
mkdir -p .routurn/snapshots
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
while IFS= read -r p; do
  case "$p" in
    ''|/*|..|../*|*/../*) exit 41 ;;
  esac
  if [ -e "$p" ] || [ -L "$p" ]; then
    printf './%%s\n' "$p" >> "$tmp"
  fi
done
if [ -s "$tmp" ]; then
  tar -cf %s -T "$tmp"
else
  tar -cf %s -T /dev/null
fi
`, shellQuote(remoteRoot), shellQuote(snapshotRel), shellQuote(snapshotRel))
	input := strings.NewReader(strings.Join(paths, "\n") + "\n")
	var stderr bytes.Buffer
	_, err := RunCommand(target, "sh -lc "+shellQuote(script), input, io.Discard, &stderr)
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("create remote snapshot: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("create remote snapshot: %w", err)
	}
	return snapshotRel, nil
}

func UploadTar(target config.Target, localRoot, remoteRoot string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := sshArgs(target, false)
	remoteCommand := fmt.Sprintf("mkdir -p %s && cd %s && tar -xf -", shellQuote(remoteRoot), shellQuote(remoteRoot))
	args = append(args, Destination(target), remoteCommand)
	debugCommand("ssh", args)

	cmd := exec.Command("ssh", args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ssh upload: %w", err)
	}

	writeErr := writeTar(stdin, localRoot, paths)
	closeErr := stdin.Close()
	waitErr := cmd.Wait()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			return fmt.Errorf("remote tar extraction failed with exit code %d", exitErr.ExitCode())
		}
		return fmt.Errorf("wait ssh upload: %w", waitErr)
	}
	return nil
}

func writeTar(w io.Writer, root string, paths []string) error {
	tw := tar.NewWriter(w)
	for _, rel := range paths {
		if err := validateRelativePath(rel); err != nil {
			return err
		}
		full := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(full)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("only regular files are supported: %s", rel)
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(full)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return tw.Close()
}

func RemovePaths(target config.Target, remoteRoot string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	script := fmt.Sprintf(`set -eu
cd %s
while IFS= read -r p; do
  case "$p" in
    ''|/*|..|../*|*/../*) exit 41 ;;
  esac
  rm -f -- "$p"
done
`, shellQuote(remoteRoot))
	input := strings.NewReader(strings.Join(paths, "\n") + "\n")
	_, err := RunCommand(target, "sh -lc "+shellQuote(script), input, io.Discard, os.Stderr)
	return err
}

func ListFiles(target config.Target, remoteRoot string) ([]string, error) {
	command := fmt.Sprintf("cd %s && find . -type f -print0", shellQuote(remoteRoot))
	data, err := Capture(target, command)
	if err != nil {
		return nil, err
	}
	parts := bytes.Split(data, []byte{0})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		path := strings.TrimPrefix(string(part), "./")
		if path == "" || strings.ContainsRune(path, '\n') || strings.ContainsRune(path, '\r') {
			continue
		}
		if path == ".routurn" || strings.HasPrefix(path, ".routurn/") {
			continue
		}
		if err := validateRelativePath(path); err != nil {
			continue
		}
		out = append(out, filepath.ToSlash(path))
	}
	return out, nil
}

func ReadTar(target config.Target, remoteRoot string, paths []string) (io.Reader, func() error, error) {
	args := sshArgs(target, false)
	remoteCommand := fmt.Sprintf("cd %s && tar -cf - -T -", shellQuote(remoteRoot))
	args = append(args, Destination(target), remoteCommand)
	debugCommand("ssh", args)
	cmd := exec.Command("ssh", args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	go func() {
		bw := bufio.NewWriter(stdin)
		for _, path := range paths {
			if validateRelativePath(path) == nil {
				fmt.Fprintln(bw, "./"+filepath.ToSlash(path))
			}
		}
		bw.Flush()
		stdin.Close()
	}()

	wait := func() error {
		if err := cmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return fmt.Errorf("remote artifact tar failed with exit code %d", exitErr.ExitCode())
			}
			return err
		}
		return nil
	}
	return stdout, wait, nil
}

func validateRelativePath(path string) error {
	path = filepath.ToSlash(path)
	if path == "" || strings.HasPrefix(path, "/") || path == ".." || strings.HasPrefix(path, "../") || strings.Contains(path, "/../") {
		return fmt.Errorf("unsafe relative path: %q", path)
	}
	if strings.ContainsRune(path, '\n') || strings.ContainsRune(path, '\r') {
		return fmt.Errorf("unsupported newline in path: %q", path)
	}
	return nil
}
