package bundle

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const maxImporterListBytes = int64(32 << 20) // 32 MiB of technical listing output

// ExternalImporter returns the local 7-Zip-compatible CLI used for .7z/.rar
// archives. Routurn never extracts these archives directly into the project;
// files are streamed through stdout and then passed through the same safety
// checks used by native archive readers.
func ExternalImporter() (string, bool) {
	for _, name := range []string{"7zz", "7z", "7za"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, true
		}
	}
	return "", false
}

type sevenZipRecord map[string]string

func readExternalArchive(path string, strip int) ([]Entry, error) {
	tool, ok := ExternalImporter()
	if !ok {
		return nil, fmt.Errorf("%s import requires a local 7-Zip-compatible CLI (7zz, 7z, or 7za); ZIP/TAR formats remain native", archiveFormat(path))
	}

	listing, err := runImporter(tool, maxImporterListBytes, "l", "-slt", "-ba", "-sccUTF-8", "--", path)
	if err != nil {
		return nil, fmt.Errorf("inspect %s with %s: %w", filepath.Base(path), filepath.Base(tool), err)
	}
	records, err := parseSevenZipListing(string(listing))
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(records))
	seen := map[string]struct{}{}
	var total int64
	for _, record := range records {
		rawName := record["Path"]
		if rawName == "" {
			continue
		}

		folder := record["Folder"] == "+"
		if err := validateExternalRecord(rawName, record, folder); err != nil {
			return nil, err
		}
		if folder {
			continue
		}

		name, skip, err := normalizeArchivePath(rawName, strip)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("duplicate archive path after normalization: %s", name)
		}
		seen[name] = struct{}{}

		size, err := parseExternalSize(record["Size"], name)
		if err != nil {
			return nil, err
		}
		if size > MaxBytes {
			return nil, fmt.Errorf("archive entry is too large: %s", name)
		}
		total += size
		if total > MaxBytes {
			return nil, fmt.Errorf("archive exceeds %d bytes uncompressed", MaxBytes)
		}
		if len(entries) >= MaxEntries {
			return nil, fmt.Errorf("archive has too many entries: > %d", MaxEntries)
		}

		// -spd disables wildcard matching so an archive-controlled filename is
		// selected literally. -so keeps extraction out of the filesystem.
		data, err := runImporter(tool, size+1, "x", "-so", "-bd", "-y", "-spd", "-sccUTF-8", "--", path, rawName)
		if err != nil {
			return nil, fmt.Errorf("read archive entry %s with %s: %w", name, filepath.Base(tool), err)
		}
		if int64(len(data)) != size {
			return nil, fmt.Errorf("archive entry size mismatch for %s: listed %d bytes, read %d", name, size, len(data))
		}

		entries = append(entries, Entry{
			Path: name,
			Mode: externalMode(record),
			Size: size,
			Data: data,
		})
	}
	return entries, nil
}

func parseSevenZipListing(output string) ([]sevenZipRecord, error) {
	output = strings.ReplaceAll(output, "\r\n", "\n")
	blocks := strings.Split(output, "\n\n")
	records := make([]sevenZipRecord, 0, len(blocks))
	for _, block := range blocks {
		record := sevenZipRecord{}
		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "----------") {
				continue
			}
			key, value, ok := strings.Cut(line, " = ")
			if !ok {
				continue
			}
			record[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
		// Archive-level technical records have Path/Type/Physical Size, but
		// file records have Folder and/or Size. Skip the former.
		if record["Path"] != "" && (record["Folder"] != "" || record["Size"] != "") {
			records = append(records, record)
		}
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("7-Zip listing contained no readable file records")
	}
	return records, nil
}

func validateExternalRecord(name string, record sevenZipRecord, folder bool) error {
	if record["Encrypted"] == "+" {
		return fmt.Errorf("encrypted archive entry is not supported: %s", name)
	}
	if record["Alternate Stream"] == "+" {
		return fmt.Errorf("archive alternate stream is not allowed: %s", name)
	}
	for _, field := range []string{"Symbolic Link", "Hard Link", "Copy Link"} {
		if strings.TrimSpace(record[field]) != "" {
			return fmt.Errorf("archive link is not allowed: %s", name)
		}
	}

	mode := strings.TrimSpace(record["Mode"])
	if mode == "" {
		mode = strings.TrimSpace(record["Attributes"])
	}
	kind := externalFileKind(mode)
	if folder {
		if kind == 'l' {
			return fmt.Errorf("archive symlink is not allowed: %s", name)
		}
		return nil
	}
	switch kind {
	case 'l':
		return fmt.Errorf("archive symlink is not allowed: %s", name)
	case 'b', 'c', 'p', 's':
		return fmt.Errorf("unsupported special archive entry: %s", name)
	case 'd':
		return fmt.Errorf("archive entry has inconsistent directory metadata: %s", name)
	}
	return nil
}

func parseExternalSize(raw, name string) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, fmt.Errorf("archive entry has no size metadata: %s", name)
	}
	size, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || size < 0 {
		return 0, fmt.Errorf("invalid archive entry size for %s: %q", name, raw)
	}
	return size, nil
}

func externalMode(record sevenZipRecord) uint32 {
	mode := strings.TrimSpace(record["Mode"])
	if mode == "" {
		mode = strings.TrimSpace(record["Attributes"])
	}
	if perm, ok := parsePermissionString(mode); ok {
		return perm
	}
	return 0o644
}

func externalFileKind(mode string) byte {
	for _, field := range strings.Fields(mode) {
		if len(field) >= 10 {
			switch field[0] {
			case '-', 'd', 'l', 'b', 'c', 'p', 's':
				return field[0]
			}
		}
	}
	return 0
}

func parsePermissionString(mode string) (uint32, bool) {
	for _, field := range strings.Fields(mode) {
		if len(field) < 9 {
			continue
		}
		permText := field[len(field)-9:]
		valid := true
		var perm uint32
		for i, ch := range permText {
			bit := uint32(1 << (8 - i))
			switch i % 3 {
			case 0:
				if ch == 'r' {
					perm |= bit
				} else if ch != '-' {
					valid = false
				}
			case 1:
				if ch == 'w' {
					perm |= bit
				} else if ch != '-' {
					valid = false
				}
			case 2:
				if ch == 'x' || ch == 's' || ch == 't' {
					perm |= bit
				} else if ch != '-' && ch != 'S' && ch != 'T' {
					valid = false
				}
			}
		}
		if valid {
			return perm, true
		}
	}
	return 0, false
}

type cappedBuffer struct {
	buf   bytes.Buffer
	limit int64
	n     int64
}

func (w *cappedBuffer) Write(p []byte) (int, error) {
	if w.n+int64(len(p)) > w.limit {
		return 0, fmt.Errorf("command output exceeded %d bytes", w.limit)
	}
	n, err := w.buf.Write(p)
	w.n += int64(n)
	return n, err
}

func runImporter(tool string, limit int64, args ...string) ([]byte, error) {
	if limit < 1 {
		limit = 1
	}
	stdout := &cappedBuffer{limit: limit}
	stderr := &cappedBuffer{limit: 1 << 20}
	cmd := exec.Command(tool, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		msg := sanitizeImporterMessage(stderr.buf.String())
		if msg != "" {
			return nil, fmt.Errorf("%v: %s", err, msg)
		}
		return nil, err
	}
	return stdout.buf.Bytes(), nil
}

func sanitizeImporterMessage(message string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(message) {
		if r == '\n' || r == '\t' || r >= 0x20 {
			if r != 0x7f && r != 0x1b {
				b.WriteRune(r)
			}
		}
	}
	return strings.TrimSpace(b.String())
}
