package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const ProjectFileName = "routurn.toml"

func LoadProject(root string) (*ProjectConfig, error) {
	path := filepath.Join(root, ProjectFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg ProjectConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	switch cfg.Execution.Mode {
	case "", "remote", "local":
	default:
		return nil, fmt.Errorf("invalid [execution].mode %q (expected local or remote)", cfg.Execution.Mode)
	}
	if cfg.Tasks == nil {
		cfg.Tasks = map[string]Task{}
	}
	localTasks, err := LoadLocalTasks(root)
	if err != nil {
		return nil, err
	}
	for name, task := range localTasks {
		// Routurn-managed local shortcuts intentionally override a same-named
		// project task without rewriting the user's routurn.toml.
		cfg.Tasks[name] = task
	}
	return &cfg, nil
}

func FindProjectRoot(start string) (string, error) {
	root, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(root, ProjectFileName)
		if _, err := os.Stat(candidate); err == nil {
			return root, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		parent := filepath.Dir(root)
		if parent == root {
			break
		}
		root = parent
	}
	return "", os.ErrNotExist
}
