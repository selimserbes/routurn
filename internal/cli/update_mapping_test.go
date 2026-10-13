package cli

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func mappingTestZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, value := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(value)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
func mappingTestFile(t *testing.T, root, path, data string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}
func mappingTestCmd() (*cobra.Command, *bytes.Buffer) {
	out := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	return cmd, out
}
func TestAutoMapFourArbitraryWrapperLevels(t *testing.T) {
	root := t.TempDir()
	mappingTestFile(t, root, "README.md", "old")
	mappingTestFile(t, root, "pyproject.toml", "old")
	mappingTestFile(t, root, "src/app.py", "old")
	zipPath := filepath.Join(t.TempDir(), "update.zip")
	mappingTestZip(t, zipPath, map[string]string{
		"foo/abc/bar/blabla/README.md":      "new",
		"foo/abc/bar/blabla/pyproject.toml": "new",
		"foo/abc/bar/blabla/src/app.py":     "new",
		"foo/abc/bar/blabla/new.txt":        "new",
	})
	cmd, out := mappingTestCmd()
	got, err := resolveArchiveMapping(cmd, root, zipPath, 0, false)
	if err != nil || got != 4 {
		t.Fatalf("strip=%d err=%v", got, err)
	}
	if !strings.Contains(out.String(), "4 wrapper folder(s) skipped") {
		t.Fatalf("message=%q", out.String())
	}
}
func TestAutoMapKeepsFlatArchive(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(t.TempDir(), "simple.zip")
	mappingTestZip(t, zipPath, map[string]string{"README.md": "hi", "src/app.py": "hi"})
	cmd, _ := mappingTestCmd()
	got, err := resolveArchiveMapping(cmd, root, zipPath, 0, false)
	if err != nil || got != 0 {
		t.Fatalf("strip=%d err=%v", got, err)
	}
}
func TestUnknownNestedMappingFailsClosedAndPreservesArchive(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(t.TempDir(), "unknown.zip")
	mappingTestZip(t, zipPath, map[string]string{
		"weird/dir/README.md": "abc", "weird/dir/thing.txt": "def",
	})
	cmd, _ := mappingTestCmd()
	_, err := resolveArchiveMapping(cmd, root, zipPath, 0, false)
	if err == nil || !strings.Contains(err.Error(), "--strip-components") {
		t.Fatalf("unexpected err: %v", err)
	}
	if _, err := os.Stat(zipPath); err != nil {
		t.Fatalf("source lost: %v", err)
	}
	got, err := resolveArchiveMapping(cmd, root, zipPath, 0, true)
	if err != nil || got != 0 {
		t.Fatalf("explicit keep-strip got=%d err=%v", got, err)
	}
}
func TestAmbiguousNestedMappingFailsClosed(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"README.md", "pyproject.toml", "src/app.py", "foo/README.md", "foo/pyproject.toml", "foo/src/app.py"} {
		mappingTestFile(t, root, p, "unchanged")
	}
	zipPath := filepath.Join(t.TempDir(), "ambiguous.zip")
	mappingTestZip(t, zipPath, map[string]string{
		"foo/README.md": "next", "foo/pyproject.toml": "next", "foo/src/app.py": "next",
	})
	cmd, _ := mappingTestCmd()
	_, err := resolveArchiveMapping(cmd, root, zipPath, 0, false)
	if err == nil || !strings.Contains(err.Error(), "--strip-components") {
		t.Fatalf("ambiguous mapping accepted: %v", err)
	}
}
func TestLegitimateNestedProjectFilesNotFlattened(t *testing.T) {
	root := t.TempDir()
	mappingTestFile(t, root, "data/README.md", "old")
	mappingTestFile(t, root, "data/schema.json", "old")
	mappingTestFile(t, root, "data/input.txt", "old")
	zipPath := filepath.Join(t.TempDir(), "data.zip")
	mappingTestZip(t, zipPath, map[string]string{
		"data/README.md": "new", "data/schema.json": "new", "data/input.txt": "new",
	})
	cmd, out := mappingTestCmd()
	strip, err := resolveArchiveMapping(cmd, root, zipPath, 0, false)
	if err != nil || strip != 0 {
		t.Fatalf("strip=%d err=%v output=%q", strip, err, out.String())
	}
}
func TestManifestArchiveUsesExplicitPaths(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(t.TempDir(), "manifest.zip")
	mappingTestZip(t, zipPath, map[string]string{
		"routurn-bundle.toml": "version = 1\n[bundle]\nname = \"a\"\n[project]\nname = \"demo\"\n",
		"x/README.md":         "a",
	})
	cmd, _ := mappingTestCmd()
	strip, err := resolveArchiveMapping(cmd, root, zipPath, 0, false)
	if err != nil || strip != 0 {
		t.Fatalf("strip=%d err=%v", strip, err)
	}
}
func TestAutoMappingCannotExposeProtectedDotGit(t *testing.T) {
	root := t.TempDir()
	mappingTestFile(t, root, "README.md", "old")
	mappingTestFile(t, root, "pyproject.toml", "old")
	zipPath := filepath.Join(t.TempDir(), "protected.zip")
	mappingTestZip(t, zipPath, map[string]string{
		"wrapper/README.md": "new", "wrapper/pyproject.toml": "new", "wrapper/.git/config": "bad",
	})
	cmd, _ := mappingTestCmd()
	_, err := resolveArchiveMapping(cmd, root, zipPath, 0, false)
	if err == nil {
		t.Fatal("protected nested payload was auto-selected")
	}
}
