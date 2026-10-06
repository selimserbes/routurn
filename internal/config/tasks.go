package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const LocalTasksFileName = "tasks.toml"

// LocalTasksPath is Routurn-managed, project-local task storage. It lives under
// .routurn so users do not need to hand-edit routurn.toml just to remember a
// command shortcut, and it is excluded from normal project sync by default.
func LocalTasksPath(root string) string {
	return filepath.Join(root, ".routurn", LocalTasksFileName)
}

type localTasksFile struct {
	Tasks map[string]Task `toml:"tasks"`
}

func LoadLocalTasks(root string) (map[string]Task, error) {
	path := LocalTasksPath(root)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Task{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var file localTasksFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if file.Tasks == nil {
		file.Tasks = map[string]Task{}
	}
	return file.Tasks, nil
}

func SaveLocalTasks(root string, tasks map[string]Task) error {
	path := LocalTasksPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := toml.Marshal(localTasksFile{Tasks: tasks})
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	data = append([]byte("# Managed by Routurn. Use 'routurn task' commands instead of editing by hand.\n"), data...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func SaveLocalTask(root, name string, task Task) error {
	tasks, err := LoadLocalTasks(root)
	if err != nil {
		return err
	}
	tasks[name] = task
	return SaveLocalTasks(root, tasks)
}

func RemoveLocalTask(root, name string) (bool, error) {
	tasks, err := LoadLocalTasks(root)
	if err != nil {
		return false, err
	}
	if _, ok := tasks[name]; !ok {
		return false, nil
	}
	delete(tasks, name)
	return true, SaveLocalTasks(root, tasks)
}
