package bundle

import "time"

const (
	MaxEntries = 10000
	MaxBytes   = int64(256 << 20) // 256 MiB uncompressed; streaming bundles can raise this later
)

type Entry struct {
	Path string `json:"path"`
	Mode uint32 `json:"mode"`
	Size int64  `json:"size"`
	Data []byte `json:"-"`
}

type Plan struct {
	Archive   string   `json:"archive"`
	Added     []string `json:"added,omitempty"`
	Modified  []string `json:"modified,omitempty"`
	Unchanged []string `json:"unchanged,omitempty"`
	Deleted   []string `json:"deleted,omitempty"`
	Entries   []Entry  `json:"-"`

	FullSnapshot    bool              `json:"-"`
	SnapshotFiles   map[string]string `json:"-"`
	SnapshotBefore  []byte            `json:"-"`
	SnapshotExisted bool              `json:"-"`
	DeleteHashes    map[string]string `json:"-"`
}

type Record struct {
	ID           string   `json:"id"`
	Archive      string   `json:"archive"`
	AppliedAt    string   `json:"applied_at"`
	Added        []string `json:"added,omitempty"`
	Modified     []string `json:"modified,omitempty"`
	Deleted      []string `json:"deleted,omitempty"`
	FullSnapshot bool     `json:"full_snapshot,omitempty"`
	HadSnapshot  bool     `json:"had_snapshot,omitempty"`
	Backup       string   `json:"backup,omitempty"`
	BundleHash   string   `json:"bundle_hash,omitempty"`
	BundleName   string   `json:"bundle_name,omitempty"`
	RolledBack   bool     `json:"rolled_back,omitempty"`
	RollbackAt   string   `json:"rollback_at,omitempty"`
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339Nano) }
