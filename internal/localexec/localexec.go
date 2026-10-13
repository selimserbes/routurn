// Package localexec implements explicit SSH-free task execution and artifact
// collection for local Routurn projects.
package localexec

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/selimserbes/routurn/internal/pathspec"
	"github.com/selimserbes/routurn/internal/result"
)

func Run(root, command string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	shell, args := "sh", []string{"-c", command}
	if runtime.GOOS == "windows" {
		shell, args = "cmd", []string{"/C", command}
	}
	cmd := exec.Command(shell, args...)
	cmd.Dir, cmd.Stdin, cmd.Stdout, cmd.Stderr = root, stdin, stdout, stderr
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), fmt.Errorf("local command failed (exit %d): %w", exit.ExitCode(), err)
	}
	return -1, fmt.Errorf("start local command: %w", err)
}

// Collect matches task artifact patterns relative to the project root. It never
// follows symlinks, reads Routurn's private history, or accepts traversal paths.
func Collect(root string, patterns []string, dest string) ([]string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	dest, err = filepath.Abs(dest)
	if err != nil {
		return nil, err
	}
	for _, pattern := range patterns {
		pattern = strings.ReplaceAll(strings.TrimSpace(pattern), "\\", "/")
		if pattern == "" {
			continue
		}
		if strings.HasPrefix(pattern, "/") || (len(pattern) >= 2 && pattern[1] == ':') || strings.HasPrefix(pattern, "../") || pattern == ".." || strings.Contains("/"+pattern+"/", "/../") || filepath.IsAbs(pattern) {
			return nil, fmt.Errorf("invalid local artifact pattern %q: paths must be project-relative", pattern)
		}
	}
	// The destination must never be the project root or its ancestor; this
	// prevents a fetch from overwriting source files with collected artifacts.
	relDest, err := filepath.Rel(root, dest)
	if err != nil {
		return nil, err
	}
	if relDest == "." || relDest == ".." || strings.HasPrefix(relDest, ".."+string(filepath.Separator)) {
		// A destination outside the project is deliberately allowed, except
		// when it is an ancestor of the project itself.
		relRoot, relErr := filepath.Rel(dest, root)
		if relErr == nil && (relRoot == "." || (relRoot != ".." && !strings.HasPrefix(relRoot, ".."+string(filepath.Separator)))) {
			return nil, fmt.Errorf("artifact destination must not contain the project root: %s", dest)
		}
	}
	// Reject a destination nested in a symlink, including pre-existing
	// destination files. This avoids traversing outside an expected directory.
	if err := rejectSymlinkComponents(dest, root); err != nil {
		return nil, err
	}
	selected := []string{}
	err = filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == root {
			return nil
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		slash := filepath.ToSlash(rel)
		if entry.IsDir() {
			if slash == ".git" || slash == ".routurn" || slash == "node_modules" || slash == ".venv" {
				return filepath.SkipDir
			}
			// Broad patterns such as **/*.txt must not re-collect Routurn's
			// published results from a previous run. A user-owned results/
			// directory is still eligible: only directories bearing Routurn's
			// exact managed-results marker are excluded.
			if (slash == "results" || slash == "routurn-results") && result.IsManagedPublicRoot(name) {
				return filepath.SkipDir
			}
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return nil
		}
		for _, pattern := range patterns {
			if pathspec.Match(filepath.ToSlash(strings.TrimSpace(pattern)), slash) {
				selected = append(selected, slash)
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(selected)
	for _, file := range selected {
		src := filepath.Join(root, filepath.FromSlash(file))
		target := filepath.Join(dest, filepath.FromSlash(file))
		if src == target {
			return nil, fmt.Errorf("artifact destination overlaps source: %s", file)
		}
		if err := rejectSymlinkComponents(target, root); err != nil {
			return nil, err
		}
		info, err := os.Lstat(src)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("artifact source changed while collecting: %s", file)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		input, err := os.Open(src)
		if err != nil {
			return nil, err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			input.Close()
			return nil, err
		}
		_, copyErr := io.Copy(output, input)
		inErr, outErr := input.Close(), output.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if inErr != nil {
			return nil, inErr
		}
		if outErr != nil {
			return nil, outErr
		}
	}
	return selected, nil
}

// Trust the user-selected project root, even when an OS-controlled ancestor is
// a symlink (e.g. /var on macOS). Inspect every writable path component below
// that root; destinations outside the root get a full component check.
func rejectSymlinkComponents(p, root string) error {
	p, err := filepath.Abs(p)
	if err != nil {
		return err
	}
	stop := filepath.VolumeName(p) + string(filepath.Separator)
	if rel, relErr := filepath.Rel(root, p); relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		stop = root
	}
	for path := p; path != stop; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in artifact destination: %s", path)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if filepath.Dir(path) == path {
			break
		}
	}
	return nil
}
