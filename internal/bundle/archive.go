package bundle

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ReadArchive(path string, strip int) ([]Entry, error) {
	if strip < 0 {
		return nil, fmt.Errorf("strip-components cannot be negative")
	}
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return readZip(path, strip)
	case strings.HasSuffix(lower, ".tar"):
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return readTar(tar.NewReader(f), strip)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("open gzip: %w", err)
		}
		defer gz.Close()
		return readTar(tar.NewReader(gz), strip)
	case strings.HasSuffix(lower, ".7z"), strings.HasSuffix(lower, ".rar"):
		return readExternalArchive(path, strip)
	default:
		return nil, fmt.Errorf("unsupported update archive %q; supported: .zip, .tar, .tar.gz, .tgz, .7z, .rar", filepath.Base(path))
	}
}

func archiveFormat(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return "tar.gz"
	case strings.HasSuffix(lower, ".zip"):
		return "zip"
	case strings.HasSuffix(lower, ".tar"):
		return "tar"
	case strings.HasSuffix(lower, ".7z"):
		return "7z"
	case strings.HasSuffix(lower, ".rar"):
		return "rar"
	default:
		return "unknown"
	}
}

func readZip(path string, strip int) ([]Entry, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()
	if len(zr.File) > MaxEntries {
		return nil, fmt.Errorf("archive has too many entries: %d > %d", len(zr.File), MaxEntries)
	}
	entries := make([]Entry, 0, len(zr.File))
	seen := map[string]struct{}{}
	var total int64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("archive symlink is not allowed: %s", f.Name)
		}
		if !f.Mode().IsRegular() {
			return nil, fmt.Errorf("unsupported archive entry type: %s", f.Name)
		}
		name, skip, err := normalizeArchivePath(f.Name, strip)
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
		if f.UncompressedSize64 > uint64(MaxBytes) {
			return nil, fmt.Errorf("archive entry is too large: %s", name)
		}
		total += int64(f.UncompressedSize64)
		if total > MaxBytes {
			return nil, fmt.Errorf("archive exceeds %d bytes uncompressed", MaxBytes)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(rc, MaxBytes+1))
		closeErr := rc.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if int64(len(data)) > MaxBytes {
			return nil, fmt.Errorf("archive entry is too large: %s", name)
		}
		mode := uint32(f.Mode().Perm())
		if mode == 0 {
			mode = 0o644
		}
		entries = append(entries, Entry{Path: name, Mode: mode, Size: int64(len(data)), Data: data})
	}
	return entries, nil
}

func readTar(tr *tar.Reader, strip int) ([]Entry, error) {
	entries := make([]Entry, 0)
	seen := map[string]struct{}{}
	var total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}
		if len(entries) >= MaxEntries {
			return nil, fmt.Errorf("archive has too many entries: > %d", MaxEntries)
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			return nil, fmt.Errorf("unsupported archive entry type for %s; links and special files are not allowed", hdr.Name)
		}
		name, skip, err := normalizeArchivePath(hdr.Name, strip)
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
		if hdr.Size < 0 || hdr.Size > MaxBytes {
			return nil, fmt.Errorf("archive entry is too large: %s", name)
		}
		total += hdr.Size
		if total > MaxBytes {
			return nil, fmt.Errorf("archive exceeds %d bytes uncompressed", MaxBytes)
		}
		data, err := io.ReadAll(io.LimitReader(tr, hdr.Size+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) != hdr.Size {
			return nil, fmt.Errorf("short archive entry: %s", name)
		}
		mode := uint32(os.FileMode(hdr.Mode).Perm())
		if mode == 0 {
			mode = 0o644
		}
		entries = append(entries, Entry{Path: name, Mode: mode, Size: hdr.Size, Data: data})
	}
	return entries, nil
}

func normalizeArchivePath(name string, strip int) (string, bool, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "./")
	if name == "" {
		return "", true, nil
	}
	if strings.HasPrefix(name, "/") || hasUnsafePathControl(name) || hasWindowsDrivePrefix(name) {
		return "", false, fmt.Errorf("unsafe archive path: %q", name)
	}
	parts := strings.Split(name, "/")
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		if p == ".." || strings.Contains(p, ":") || strings.HasSuffix(p, ".") || strings.HasSuffix(p, " ") || isWindowsReservedSegment(p) {
			return "", false, fmt.Errorf("unsafe archive path: %q", name)
		}
	}
	if strip >= len(parts) {
		return "", true, nil
	}
	parts = parts[strip:]
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.Join(parts, "/"))))
	if clean == "." || clean == "" {
		return "", true, nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return "", false, fmt.Errorf("unsafe archive path: %q", name)
	}
	if clean == ".git" || strings.HasPrefix(clean, ".git/") || clean == ".routurn" || strings.HasPrefix(clean, ".routurn/") {
		return "", false, fmt.Errorf("protected project path in archive: %s", clean)
	}
	return clean, false, nil
}

func hasUnsafePathControl(name string) bool {
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func hasWindowsDrivePrefix(name string) bool {
	if len(name) < 2 || name[1] != ':' {
		return false
	}
	c := name[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isWindowsReservedSegment(segment string) bool {
	base := segment
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	base = strings.ToUpper(base)
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CLOCK$":
		return true
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		return true
	}
	return false
}
