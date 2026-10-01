package bundle

import (
	"fmt"
	"path/filepath"
)

type InspectionEntry struct {
	Path string `json:"path"`
	Mode uint32 `json:"mode"`
	Size int64  `json:"size"`
}

type Inspection struct {
	Archive    string            `json:"archive"`
	Format     string            `json:"format"`
	Backend    string            `json:"backend"`
	Files      int               `json:"files"`
	Bytes      int64             `json:"bytes"`
	Executable int               `json:"executable"`
	Entries    []InspectionEntry `json:"entries"`
}

func InspectArchive(path string, strip int) (Inspection, error) {
	entries, err := ReadArchive(path, strip)
	if err != nil {
		return Inspection{}, err
	}

	backend := "native"
	format := archiveFormat(path)
	if format == "7z" || format == "rar" {
		tool, ok := ExternalImporter()
		if !ok {
			return Inspection{}, fmt.Errorf("%s importer became unavailable", format)
		}
		backend = filepath.Base(tool)
	}

	result := Inspection{
		Archive: filepath.Base(path),
		Format:  format,
		Backend: backend,
		Files:   len(entries),
		Entries: make([]InspectionEntry, 0, len(entries)),
	}
	for _, entry := range entries {
		result.Bytes += entry.Size
		if entry.Mode&0o111 != 0 {
			result.Executable++
		}
		result.Entries = append(result.Entries, InspectionEntry{
			Path: entry.Path,
			Mode: entry.Mode,
			Size: entry.Size,
		})
	}
	return result, nil
}
