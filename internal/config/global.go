package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const appDirName = "routurn"

func ConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, appDirName), nil
}

func GlobalConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

func LoadGlobal() (*GlobalConfig, error) {
	path, err := GlobalConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &GlobalConfig{
			Version:  1,
			Targets:  map[string]Target{},
			Projects: map[string]ProjectLink{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read global config: %w", err)
	}

	var cfg GlobalConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse global config %s: %w", path, err)
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Targets == nil {
		cfg.Targets = map[string]Target{}
	}
	if cfg.Projects == nil {
		cfg.Projects = map[string]ProjectLink{}
	}
	for name, target := range cfg.Targets {
		if err := ValidateJump(target.Jump); err != nil {
			return nil, fmt.Errorf("target %q: %w", name, err)
		}
		for endpointName, endpoint := range target.Endpoints {
			if err := ValidateJump(endpoint.Jump); err != nil {
				return nil, fmt.Errorf("target %q endpoint %q: %w", name, endpointName, err)
			}
		}
	}
	return &cfg, nil
}

func SaveGlobal(cfg *GlobalConfig) error {
	path, err := GlobalConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode global config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write global config: %w", err)
	}
	return nil
}
