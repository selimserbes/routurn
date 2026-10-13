package bundle

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
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
	// Update IDs contain a second-resolution timestamp followed by a random
	// suffix. Sorting by ID can select the wrong update when multiple updates
	// happen within one second. New records have nanosecond timestamps; older
	// RFC3339 records remain readable.
	sort.Slice(records, func(i, j int) bool {
		a, aErr := time.Parse(time.RFC3339Nano, records[i].AppliedAt)
		b, bErr := time.Parse(time.RFC3339Nano, records[j].AppliedAt)
		if aErr == nil && bErr == nil && !a.Equal(b) {
			return a.After(b)
		}
		if aErr == nil && bErr != nil {
			return true
		}
		if aErr != nil && bErr == nil {
			return false
		}
		return records[i].ID > records[j].ID
	})
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
	var newest *Record
	var newestAt time.Time
	for i := range records {
		record := &records[i]
		if record.RolledBack {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, record.AppliedAt)
		if err != nil {
			return "", fmt.Errorf("update %s has an invalid applied time; specify an update ID explicitly", record.ID)
		}
		if newest == nil {
			newest, newestAt = record, at
			continue
		}
		if at.Equal(newestAt) {
			// Legacy records can share a second-resolution applied time.
			// Do not select one using the random part of its ID.
			return "", fmt.Errorf("multiple updates share the latest applied time; specify an update ID explicitly")
		}
		break
	}
	if newest != nil {
		return newest.ID, nil
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
