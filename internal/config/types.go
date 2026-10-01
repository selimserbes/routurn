package config

type GlobalConfig struct {
	Version  int                    `toml:"version"`
	Targets  map[string]Target      `toml:"targets"`
	Projects map[string]ProjectLink `toml:"projects"`
	UI       UIConfig               `toml:"ui,omitempty"`
}

type UIConfig struct {
	LastUpdateDir string `toml:"last_update_dir,omitempty"`
}

// Target is a logical remote machine. Host/User/Port are retained for
// backwards compatibility with the original single-endpoint target format.
// Once Endpoints is populated, Routurn treats the target as one machine with
// multiple named connection routes.
type Target struct {
	Host      string              `toml:"host,omitempty"`
	User      string              `toml:"user,omitempty"`
	Port      int                 `toml:"port,omitempty"`
	Route     string              `toml:"route,omitempty"`
	Endpoints map[string]Endpoint `toml:"endpoints,omitempty"`
}

// Endpoint is one SSH route to a logical Target.
type Endpoint struct {
	Host     string `toml:"host"`
	User     string `toml:"user,omitempty"`
	Port     int    `toml:"port,omitempty"`
	Priority int    `toml:"priority,omitempty"`
}

type NamedEndpoint struct {
	Name     string
	Endpoint Endpoint
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
