package artifact

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/selimserbes/routurn/internal/pathspec"
	"github.com/selimserbes/routurn/internal/remote"
)

func Fetch(target config.Target, remoteRoot string, patterns []string, dest string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	files, err := remote.ListFiles(target, remoteRoot)
	if err != nil {
		return nil, err
	}
	selected := make([]string, 0)
	for _, file := range files {
		if matchAny(file, patterns) {
			selected = append(selected, file)
		}
	}
	sort.Strings(selected)
	if len(selected) == 0 {
		return nil, nil
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, err
	}
	r, wait, err := remote.ReadTar(target, remoteRoot, selected)
	if err != nil {
		return nil, err
	}
	extractErr := extractTar(r, dest)
	waitErr := wait()
	if extractErr != nil {
		return nil, extractErr
	}
	if waitErr != nil {
		return nil, waitErr
	}
	return selected, nil
}

func matchAny(path string, patterns []string) bool {
	path = filepath.ToSlash(strings.TrimPrefix(path, "./"))
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if pathspec.Match(pattern, path) {
			return true
		}
	}
	return false
}

func extractTar(r io.Reader, dest string) error {
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read artifact tar: %w", err)
		}
		clean := filepath.Clean(filepath.FromSlash(hdr.Name))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe artifact path: %q", hdr.Name)
		}
		targetPath := filepath.Join(destAbs, clean)
		if targetPath != destAbs && !strings.HasPrefix(targetPath, destAbs+string(filepath.Separator)) {
			return fmt.Errorf("artifact escapes destination: %q", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(hdr.Mode)&0o777); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(f, tr)
			closeErr := f.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("unsupported artifact entry type for %q", hdr.Name)
		}
	}
	return nil
}
