package config

type GlobalConfig struct {
	Version  int                    `toml:"version"`
	Targets  map[string]Target      `toml:"targets"`
	Projects map[string]ProjectLink `toml:"projects"`
}

type Target struct {
	Host string `toml:"host"`
	User string `toml:"user,omitempty"`
	Port int    `toml:"port,omitempty"`
}

type ProjectLink struct {
	Root string `toml:"root"`
}

type ProjectConfig struct {
	Version int             `toml:"version"`
	Name    string          `toml:"name"`
	Remote  ProjectRemote   `toml:"remote"`
	Sync    SyncConfig      `toml:"sync"`
	Tasks   map[string]Task `toml:"tasks"`
}

type ProjectRemote struct {
	Target string `toml:"target"`
	Path   string `toml:"path"`
}

type SyncConfig struct {
	Exclude []string `toml:"exclude"`
}

type Task struct {
	Command     string   `toml:"command"`
	Interactive bool     `toml:"interactive,omitempty"`
	Artifacts   []string `toml:"artifacts,omitempty"`
}
