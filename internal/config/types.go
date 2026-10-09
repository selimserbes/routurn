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
	Jump      string              `toml:"jump,omitempty"`
	Route     string              `toml:"route,omitempty"`
	Endpoints map[string]Endpoint `toml:"endpoints,omitempty"`
}

// Endpoint is one SSH route to a logical Target.
type Endpoint struct {
	Host     string `toml:"host"`
	User     string `toml:"user,omitempty"`
	Port     int    `toml:"port,omitempty"`
	Priority int    `toml:"priority,omitempty"`
	Jump     string `toml:"jump,omitempty"`
}

type NamedEndpoint struct {
	Name     string
	Endpoint Endpoint
}

type ProjectLink struct {
	Root string `toml:"root"`
}

type ProjectConfig struct {
	Execution ExecutionConfig `toml:"execution,omitempty"`
	Version   int             `toml:"version"`
	Name      string          `toml:"name"`
	Remote    ProjectRemote   `toml:"remote"`
	Sync      SyncConfig      `toml:"sync"`
	Tasks     map[string]Task `toml:"tasks"`
	Picker    PickerConfig    `toml:"picker,omitempty"`
}

// ExecutionConfig selects the task execution environment. An omitted mode
// preserves the v0.3.0 SSH behavior. Local mode must be explicitly enabled.
type ExecutionConfig struct {
	Mode string `toml:"mode,omitempty"`
}

func (p ProjectConfig) IsLocal() bool { return p.Execution.Mode == "local" }

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

// PickerConfig is optional, display-only project metadata. Task discovery and
// execution do not depend on picker configuration.
type PickerConfig struct {
	DefaultGroup string            `toml:"default_group,omitempty"`
	Groups       []PickerGroup     `toml:"groups,omitempty"`
	Labels       map[string]string `toml:"labels,omitempty"`
}

// PickerGroup uses task names or shell-style glob patterns over discovered tasks.
// It intentionally carries no technology- or workflow-specific semantics.
type PickerGroup struct {
	Name  string   `toml:"name"`
	Tasks []string `toml:"tasks"`
}
