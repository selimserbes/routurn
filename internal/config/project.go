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
	if cfg.Tasks == nil {
		cfg.Tasks = map[string]Task{}
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
