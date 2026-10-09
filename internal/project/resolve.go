package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/selimserbes/routurn/internal/config"
)

type Resolved struct {
	Root   string
	Config *config.ProjectConfig
}

// Resolve chooses a registered project by name, an explicitly specified
// filesystem path, or the nearest project ancestor of the working directory.
// Registered names take priority over relative paths with the same spelling.
func Resolve(name string) (*Resolved, error) {
	if name != "" {
		global, err := config.LoadGlobal()
		if err != nil {
			return nil, err
		}
		if link, ok := global.Projects[name]; ok {
			return resolveRoot(link.Root)
		}
		if !isProjectPath(name) {
			return nil, fmt.Errorf("project %q is not registered\n\nRegister an existing project:\n  routurn project add /path/to/project\n\nOr use a project directory:\n  routurn -p /path/to/project exec", name)
		}
		path := name
		if path == "~" || strings.HasPrefix(path, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			if path == "~" {
				path = home
			} else {
				path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
			}
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("project path %q: %w", path, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("project path %q is not a directory", path)
		}
		root, err := config.FindProjectRoot(path)
		if err != nil {
			return nil, fmt.Errorf("no %s found from %s upward: %w", config.ProjectFileName, path, err)
		}
		return resolveRoot(root)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := config.FindProjectRoot(cwd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("no %s found from %s upward; use -p <project> or run 'routurn init': %w", config.ProjectFileName, cwd, err)
		}
		return nil, err
	}
	return resolveRoot(root)
}

func isProjectPath(name string) bool {
	return filepath.IsAbs(name) || name == "." || name == ".." || name == "~" ||
		strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") ||
		strings.HasPrefix(name, "~/") || strings.ContainsRune(name, filepath.Separator)
}

func resolveRoot(dir string) (*Resolved, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadProject(root)
	if err != nil {
		return nil, err
	}
	return &Resolved{Root: root, Config: cfg}, nil
}
