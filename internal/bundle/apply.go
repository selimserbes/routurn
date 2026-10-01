package bundle

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/selimserbes/routurn/internal/runstate"
)

func Apply(root string, plan Plan) (Record, error) {
	id := runstate.NewID()
	updateDir := filepath.Join(root, ".routurn", "updates", id)
	if err := os.MkdirAll(updateDir, 0o755); err != nil {
		return Record{}, err
	}

	record := Record{
		ID: id, Archive: plan.Archive, AppliedAt: nowRFC3339(),
		Added: append([]string(nil), plan.Added...), Modified: append([]string(nil), plan.Modified...),
	}
	if len(plan.Modified) > 0 {
		backup := filepath.Join(updateDir, "backup.tar")
		if err := createBackup(root, backup, plan.Modified); err != nil {
			return Record{}, err
		}
		record.Backup = filepath.ToSlash(filepath.Join(".routurn", "updates", id, "backup.tar"))
	}

	if err := writeRecord(updateDir, record); err != nil {
		return Record{}, err
	}

	applied := make([]string, 0)
	for _, entry := range plan.Entries {
		if contains(plan.Unchanged, entry.Path) {
			continue
		}
		if err := writeEntryAtomic(root, entry); err != nil {
			_ = restore(root, updateDir, record, applied)
			return Record{}, fmt.Errorf("apply %s: %w", entry.Path, err)
		}
		applied = append(applied, entry.Path)
	}
	return record, nil
}

func Rollback(root, id string) (Record, error) {
	updateDir := filepath.Join(root, ".routurn", "updates", id)
	record, err := loadRecord(updateDir)
	if err != nil {
		return Record{}, err
	}
	if record.RolledBack {
		return record, fmt.Errorf("update %s is already rolled back", id)
	}
	paths := append(append([]string{}, record.Added...), record.Modified...)
	if err := restore(root, updateDir, record, paths); err != nil {
		return Record{}, err
	}
	record.RolledBack = true
	record.RollbackAt = nowRFC3339()
	if err := writeRecord(updateDir, record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func createBackup(root, path string, files []string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(f)
	for _, rel := range files {
		full, err := safeLocalTarget(root, rel)
		if err != nil {
			tw.Close()
			f.Close()
			return err
		}
		info, err := os.Lstat(full)
		if err != nil {
			tw.Close()
			f.Close()
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			tw.Close()
			f.Close()
			return fmt.Errorf("cannot back up non-regular file: %s", rel)
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			tw.Close()
			f.Close()
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			tw.Close()
			f.Close()
			return err
		}
		src, err := os.Open(full)
		if err != nil {
			tw.Close()
			f.Close()
			return err
		}
		_, cpErr := io.Copy(tw, src)
		clErr := src.Close()
		if cpErr != nil {
			tw.Close()
			f.Close()
			return cpErr
		}
		if clErr != nil {
			tw.Close()
			f.Close()
			return clErr
		}
	}
	if err := tw.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func writeEntryAtomic(root string, entry Entry) error {
	targetAbs, err := safeLocalTarget(root, entry.Path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(targetAbs), ".routurn-apply-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(entry.Data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(os.FileMode(entry.Mode) & 0o777); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, targetAbs)
}

func restore(root, updateDir string, record Record, applied []string) error {
	// Remove files that were introduced by this update.
	added := map[string]struct{}{}
	for _, p := range record.Added {
		added[p] = struct{}{}
	}
	for _, p := range applied {
		if _, ok := added[p]; ok {
			_ = os.Remove(filepath.Join(root, filepath.FromSlash(p)))
		}
	}
	if record.Backup == "" {
		return nil
	}
	f, err := os.Open(filepath.Join(updateDir, "backup.tar"))
	if err != nil {
		return err
	}
	defer f.Close()
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			return fmt.Errorf("invalid backup entry: %s", hdr.Name)
		}
		entry := Entry{Path: filepath.ToSlash(hdr.Name), Mode: uint32(os.FileMode(hdr.Mode).Perm())}
		entry.Data, err = io.ReadAll(tr)
		if err != nil {
			return err
		}
		if err := writeEntryAtomic(root, entry); err != nil {
			return err
		}
	}
	return nil
}

func writeRecord(dir string, record Record) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(dir, "update.json"), data, 0o644)
}

func loadRecord(dir string) (Record, error) {
	data, err := os.ReadFile(filepath.Join(dir, "update.json"))
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
