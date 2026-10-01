package bundle

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func List(root string) ([]Record, error) {
	base := filepath.Join(root, ".routurn", "updates")
	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	records := make([]Record, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		record, err := loadRecord(filepath.Join(base, entry.Name()))
		if err == nil {
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID > records[j].ID })
	return records, nil
}

func ResolveID(root, value string) (string, error) {
	if value != "latest" && value != "previous" {
		return value, nil
	}
	records, err := List(root)
	if err != nil {
		return "", err
	}
	for _, record := range records {
		if !record.RolledBack {
			return record.ID, nil
		}
	}
	return "", fmt.Errorf("no applied updates found")
}

func SetManagedInfo(root, id, hash, name string) (Record, error) {
	updateDir := filepath.Join(root, ".routurn", "updates", id)
	record, err := loadRecord(updateDir)
	if err != nil {
		return Record{}, err
	}
	record.BundleHash = hash
	record.BundleName = name
	if err := writeRecord(updateDir, record); err != nil {
		return Record{}, err
	}
	return record, nil
}
