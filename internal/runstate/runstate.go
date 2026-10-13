package runstate

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Manifest struct {
	ID            string   `json:"id"`
	Project       string   `json:"project"`
	Target        string   `json:"target"`
	Endpoint      string   `json:"endpoint,omitempty"`
	Task          string   `json:"task"`
	Status        string   `json:"status"`
	StartedAt     string   `json:"started_at"`
	FinishedAt    string   `json:"finished_at,omitempty"`
	ExitCode      *int     `json:"exit_code,omitempty"`
	Changed       []string `json:"changed,omitempty"`
	Deleted       []string `json:"deleted,omitempty"`
	Snapshot      string   `json:"snapshot,omitempty"`
	Artifacts     []string `json:"artifacts,omitempty"`
	UpdateID      string   `json:"update_id,omitempty"`
	UpdateArchive string   `json:"update_archive,omitempty"`
	Detached      bool     `json:"detached,omitempty"`
	RemotePID     int      `json:"remote_pid,omitempty"`
	RemotePath    string   `json:"remote_path,omitempty"`
}

func NewID() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return time.Now().UTC().Format("20060102T150405Z")
	}
	return time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b)
}

func Dir(root, id string) string {
	return filepath.Join(root, ".routurn", "runs", id)
}

func Create(root string, m Manifest) (string, error) {
	dir := Dir(root, m.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := Save(root, m); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(root, ".routurn"), 0o755); err == nil {
		_ = os.WriteFile(filepath.Join(root, ".routurn", "latest"), []byte(m.ID+"\n"), 0o644)
	}
	return dir, nil
}

func Save(root string, m Manifest) error {
	dir := Dir(root, m.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(dir, "run.json"), data, 0o644)
}

func Load(root, id string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(Dir(root, id), "run.json"))
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func List(root string) ([]Manifest, error) {
	base := filepath.Join(root, ".routurn", "runs")
	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Manifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		m, err := Load(root, entry.Name())
		if err == nil {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func ResolveID(root, id string) (string, error) {
	if id != "latest" {
		return id, nil
	}
	runs, err := List(root)
	if err != nil {
		return "", err
	}
	if len(runs) == 0 {
		return "", fmt.Errorf("no runs recorded")
	}
	return runs[0].ID, nil
}

func OpenLogs(root, id string) (*os.File, *os.File, error) {
	dir := Dir(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, err
	}
	stdout, err := os.Create(filepath.Join(dir, "stdout.log"))
	if err != nil {
		return nil, nil, fmt.Errorf("create stdout log: %w", err)
	}
	stderr, err := os.Create(filepath.Join(dir, "stderr.log"))
	if err != nil {
		stdout.Close()
		return nil, nil, fmt.Errorf("create stderr log: %w", err)
	}
	return stdout, stderr, nil
}

// LatestByTask returns the most recent recorded run for a task.
func LatestByTask(root, task string) (Manifest, error) {
	runs, err := List(root)
	if err != nil {
		return Manifest{}, err
	}
	for _, run := range runs {
		if run.Task == task {
			return run, nil
		}
	}
	return Manifest{}, fmt.Errorf("no runs recorded for task %q", task)
}

// ResolveSelector resolves "latest", an exact run ID, or a task name.
func ResolveSelector(root, value string) (string, error) {
	if value == "latest" {
		return ResolveID(root, value)
	}
	if _, err := os.Stat(Dir(root, value)); err == nil {
		return value, nil
	}
	m, err := LatestByTask(root, value)
	if err != nil {
		return "", err
	}
	return m.ID, nil
}
