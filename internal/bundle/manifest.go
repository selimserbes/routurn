package bundle

import (
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const ManifestPath = "routurn-bundle.toml"

type Manifest struct {
	Schema  int             `toml:"schema" json:"schema"`
	Bundle  ManifestBundle  `toml:"bundle" json:"bundle"`
	Project ManifestProject `toml:"project" json:"project"`
	Base    ManifestBase    `toml:"base" json:"base"`
}

type ManifestBundle struct {
	Name string `toml:"name" json:"name"`
	Mode string `toml:"mode,omitempty" json:"mode,omitempty"` // optional: "full" or "partial"
}

type ManifestProject struct {
	Name string `toml:"name" json:"name"`
}

type ManifestBase struct {
	Fingerprint string            `toml:"fingerprint,omitempty" json:"fingerprint,omitempty"`
	Files       map[string]string `toml:"files,omitempty" json:"files,omitempty"`
}

func ManifestFromArchive(path string, strip int) (*Manifest, error) {
	entries, err := ReadArchive(path, strip)
	if err != nil {
		return nil, err
	}
	manifest, _, err := SplitManifest(entries)
	return manifest, err
}

func SplitManifest(entries []Entry) (*Manifest, []Entry, error) {
	payload := make([]Entry, 0, len(entries))
	var manifest *Manifest
	for _, entry := range entries {
		if entry.Path != ManifestPath {
			payload = append(payload, entry)
			continue
		}
		if manifest != nil {
			return nil, nil, fmt.Errorf("duplicate %s", ManifestPath)
		}
		var parsed Manifest
		if err := toml.Unmarshal(entry.Data, &parsed); err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", ManifestPath, err)
		}
		if parsed.Schema == 0 {
			parsed.Schema = 1
		}
		if parsed.Schema != 1 {
			return nil, nil, fmt.Errorf("unsupported bundle manifest schema %d", parsed.Schema)
		}
		parsed.Bundle.Name = strings.TrimSpace(parsed.Bundle.Name)
		parsed.Bundle.Mode = strings.ToLower(strings.TrimSpace(parsed.Bundle.Mode))
		switch parsed.Bundle.Mode {
		case "", "partial", "full":
		default:
			return nil, nil, fmt.Errorf("invalid bundle mode %q: expected full or partial", parsed.Bundle.Mode)
		}
		parsed.Project.Name = strings.TrimSpace(parsed.Project.Name)
		parsed.Base.Fingerprint = strings.TrimSpace(parsed.Base.Fingerprint)
		manifest = &parsed
	}
	return manifest, payload, nil
}
