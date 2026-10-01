package intake

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/selimserbes/routurn/internal/bundle"
)

const defaultKeepBundles = 20

type Metadata struct {
	Hash            string           `json:"hash"`
	Path            string           `json:"path"`
	OriginalName    string           `json:"original_name"`
	ImportedAt      string           `json:"imported_at"`
	LastImportedAt  string           `json:"last_imported_at"`
	Manifest        *bundle.Manifest `json:"manifest,omitempty"`
	StripComponents int              `json:"strip_components,omitempty"`
}

type Result struct {
	Metadata
	Duplicate     bool   `json:"duplicate"`
	SourceRemoved bool   `json:"source_removed"`
	Warning       string `json:"warning,omitempty"`
}

type Options struct {
	Consume         bool
	StripComponents int
}

func DataDir() (string, error) {
	if value := strings.TrimSpace(os.Getenv("ROUTURN_DATA_DIR")); value != "" {
		return filepath.Abs(value)
	}
	if runtime.GOOS == "windows" {
		if value := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); value != "" {
			return filepath.Join(value, "Routurn"), nil
		}
	}
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "routurn"), nil
	}
	if value := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); value != "" {
		return filepath.Join(value, "routurn"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "routurn"), nil
}

func Import(source string, opts Options) (Result, error) {
	abs, err := filepath.Abs(source)
	if err != nil {
		return Result{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Result{}, fmt.Errorf("update archive: %w", err)
	}
	if !info.Mode().IsRegular() {
		return Result{}, fmt.Errorf("update archive is not a regular file: %s", abs)
	}

	// Safety validation happens before Routurn takes ownership of the source.
	if _, err := bundle.InspectArchive(abs, opts.StripComponents); err != nil {
		return Result{}, err
	}
	manifest, err := bundle.ManifestFromArchive(abs, opts.StripComponents)
	if err != nil {
		return Result{}, err
	}

	hash, err := HashFile(abs)
	if err != nil {
		return Result{}, err
	}
	dataDir, err := DataDir()
	if err != nil {
		return Result{}, err
	}
	dir := filepath.Join(dataDir, "bundles", "sha256", hash[:2], hash)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Result{}, err
	}
	suffix := archiveSuffix(abs)
	managedPath := filepath.Join(dir, "bundle"+suffix)
	metaPath := filepath.Join(dir, "metadata.json")

	duplicate := false
	if existing, err := os.Stat(managedPath); err == nil && existing.Mode().IsRegular() {
		managedHash, hashErr := HashFile(managedPath)
		if hashErr != nil {
			return Result{}, hashErr
		}
		if managedHash != hash {
			return Result{}, fmt.Errorf("managed bundle hash mismatch for %s", managedPath)
		}
		duplicate = true
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	} else {
		if err := copyVerified(abs, managedPath, hash); err != nil {
			return Result{}, err
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	metadata := Metadata{
		Hash:            hash,
		Path:            managedPath,
		OriginalName:    filepath.Base(abs),
		ImportedAt:      now,
		LastImportedAt:  now,
		Manifest:        manifest,
		StripComponents: opts.StripComponents,
	}
	if old, err := loadMetadata(metaPath); err == nil {
		metadata.ImportedAt = old.ImportedAt
		if metadata.ImportedAt == "" {
			metadata.ImportedAt = now
		}
	}
	if err := writeMetadata(metaPath, metadata); err != nil {
		return Result{}, err
	}

	result := Result{Metadata: metadata, Duplicate: duplicate}
	if opts.Consume && !sameFilePath(abs, managedPath) {
		if err := os.Remove(abs); err != nil {
			result.Warning = fmt.Sprintf("managed copy is safe, but source could not be removed: %v", err)
		} else {
			result.SourceRemoved = true
		}
	}
	return result, nil
}

func LatestForProject(project string) (Metadata, error) {
	items, err := List()
	if err != nil {
		return Metadata{}, err
	}
	for _, item := range items {
		if item.Manifest != nil && item.Manifest.Project.Name == project {
			return item, nil
		}
	}
	return Metadata{}, fmt.Errorf("no managed updates found for project %q", project)
}

func FindByPath(path string) (Metadata, bool, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Metadata{}, false, err
	}
	items, err := List()
	if err != nil {
		return Metadata{}, false, err
	}
	for _, item := range items {
		itemAbs, itemErr := filepath.Abs(item.Path)
		if itemErr == nil && itemAbs == abs {
			return item, true, nil
		}
	}
	return Metadata{}, false, nil
}

func List() ([]Metadata, error) {
	dataDir, err := DataDir()
	if err != nil {
		return nil, err
	}
	base := filepath.Join(dataDir, "bundles", "sha256")
	var out []Metadata
	err = filepath.WalkDir(base, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if d.IsDir() || d.Name() != "metadata.json" {
			return nil
		}
		meta, err := loadMetadata(path)
		if err == nil {
			out = append(out, meta)
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastImportedAt > out[j].LastImportedAt
	})
	return out, nil
}

func Prune(keep int, dryRun bool) ([]string, error) {
	if keep < 0 {
		keep = 0
	}
	items, err := List()
	if err != nil {
		return nil, err
	}
	if len(items) <= keep {
		return nil, nil
	}
	var removed []string
	for _, item := range items[keep:] {
		dir := filepath.Dir(item.Path)
		removed = append(removed, dir)
		if !dryRun {
			if err := os.RemoveAll(dir); err != nil {
				return removed, err
			}
		}
	}
	return removed, nil
}

func PruneDefault() error {
	_, err := Prune(defaultKeepBundles, false)
	return err
}

func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyVerified(source, dest, expectedHash string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".bundle-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	src, err := os.Open(source)
	if err != nil {
		tmp.Close()
		return err
	}
	_, copyErr := io.Copy(tmp, src)
	closeSrcErr := src.Close()
	if copyErr != nil {
		tmp.Close()
		return copyErr
	}
	if closeSrcErr != nil {
		tmp.Close()
		return closeSrcErr
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	actual, err := HashFile(tmpName)
	if err != nil {
		return err
	}
	if actual != expectedHash {
		return fmt.Errorf("copied update hash mismatch")
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpName, dest)
}

func loadMetadata(path string) (Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Metadata{}, err
	}
	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return Metadata{}, err
	}
	return meta, nil
}

func writeMetadata(path string, meta Metadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".metadata-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func archiveSuffix(path string) string {
	lower := strings.ToLower(path)
	for _, suffix := range []string{".tar.gz", ".tgz", ".zip", ".tar", ".7z", ".rar"} {
		if strings.HasSuffix(lower, suffix) {
			return suffix
		}
	}
	return filepath.Ext(path)
}

func sameFilePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	return errA == nil && errB == nil && aa == bb
}
