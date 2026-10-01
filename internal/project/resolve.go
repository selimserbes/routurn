package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/selimserbes/routurn/internal/config"
)

type Resolved struct {
	Root   string
	Config *config.ProjectConfig
}

func Resolve(name string) (*Resolved, error) {
	if name != "" {
		global, err := config.LoadGlobal()
		if err != nil {
			return nil, err
		}
		link, ok := global.Projects[name]
		if !ok {
			return nil, fmt.Errorf("project %q is not registered\n\nRegister an existing project:\n  routurn project add /path/to/project\n\nOr initialize a new project:\n  routurn init", name)
		}
		root, err := filepath.Abs(link.Root)
		if err != nil {
			return nil, err
		}
		cfg, err := config.LoadProject(root)
		if err != nil {
			return nil, err
		}
		return &Resolved{Root: root, Config: cfg}, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := config.FindProjectRoot(cwd)
	if err != nil {
		return nil, fmt.Errorf("no %s found from %s upward; use -p <project> or run 'routurn init'", config.ProjectFileName, cwd)
	}
	cfg, err := config.LoadProject(root)
	if err != nil {
		return nil, err
	}
	return &Resolved{Root: root, Config: cfg}, nil
}
