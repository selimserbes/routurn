package syncer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func manifestPath(root string) string {
	return filepath.Join(root, ".routurn", "state", "sync-manifest.json")
}

func LoadManifest(root string) (Manifest, error) {
	path := manifestPath(root)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Manifest{Files: map[string]string{}}, nil
	}
	if err != nil {
		return Manifest{}, fmt.Errorf("read sync manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse sync manifest: %w", err)
	}
	if m.Files == nil {
		m.Files = map[string]string{}
	}
	return m, nil
}

func SaveManifest(root string, manifest Manifest) error {
	path := manifestPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write sync manifest: %w", err)
	}
	return nil
}
