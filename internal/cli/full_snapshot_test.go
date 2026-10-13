package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateFullSnapshotLifecycleAndRollback(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("ROUTURN_DATA_DIR", filepath.Join(t.TempDir(), "managed"))
	mappingTestFile(t, root, "routurn.toml", "version = 1\nname = \"snapshot-demo\"\n[execution]\nmode = \"local\"\n")
	mappingTestFile(t, root, "legacy.py", "older source\n")
	mappingTestFile(t, root, "user-notes.txt", "private notes\n")
	previous := projectName
	projectName = root
	t.Cleanup(func() { projectName = previous })
	makeZip := func(files map[string]string) string {
		t.Helper()
		filename := filepath.Join(t.TempDir(), "update.zip")
		mappingTestZip(t, filename, files)
		return filename
	}
	run := func(stdin string, args ...string) (string, error) {
		t.Helper()
		cmd := newUpdateCmd()
		cmd.SetArgs(args)
		cmd.SetIn(strings.NewReader(stdin))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		err := cmd.Execute()
		return out.String(), err
	}
	first := makeZip(map[string]string{"legacy.py": "managed\n", "current.py": "current\n"})
	output, err := run("", first, "--full-snapshot", "--yes", "--keep-source", "--strip-components", "0")
	if err != nil {
		t.Fatalf("first full snapshot: %v\n%s", err, output)
	}
	if !strings.Contains(output, "First full snapshot") {
		t.Fatalf("no ownership bootstrap explanation: %s", output)
	}
	if _, err := os.Stat(first); err != nil {
		t.Fatalf("keep-source failed: %v", err)
	}
	second := makeZip(map[string]string{"current.py": "current v2\n", "next.py": "next\n"})
	output, err = run("", second, "--full-snapshot", "--dry-run", "--strip-components", "0")
	if err != nil {
		t.Fatalf("dry-run: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Deleted  1") || !strings.Contains(output, "- legacy.py") {
		t.Fatalf("missing deletion preview: %s", output)
	}
	data, err := os.ReadFile(filepath.Join(root, "legacy.py"))
	if err != nil || string(data) != "managed\n" {
		t.Fatalf("dry-run deleted file: %q %v", data, err)
	}
	output, err = run("", second, "--full-snapshot", "--yes", "--keep-source", "--strip-components", "0")
	if err != nil {
		t.Fatalf("apply: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(root, "legacy.py")); !os.IsNotExist(err) {
		t.Fatalf("obsolete path not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "user-notes.txt")); err != nil {
		t.Fatalf("unknown user file removed: %v", err)
	}
	rollback := newRollbackCmd()
	rollback.SetArgs([]string{"previous"})
	rollback.SetOut(new(bytes.Buffer))
	rollback.SetErr(new(bytes.Buffer))
	if err := rollback.Execute(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	data, err = os.ReadFile(filepath.Join(root, "legacy.py"))
	if err != nil || string(data) != "managed\n" {
		t.Fatalf("rollback did not restore old path: %q, %v", data, err)
	}
}

func TestUpdateFullSnapshotNoOpKeepsOriginalArchive(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("ROUTURN_DATA_DIR", filepath.Join(t.TempDir(), "managed"))
	mappingTestFile(t, root, "routurn.toml", "version = 1\nname = \"nochange\"\n[execution]\nmode = \"local\"\n")
	mappingTestFile(t, root, "README.md", "same\n")
	zipFile := filepath.Join(t.TempDir(), "same.zip")
	mappingTestZip(t, zipFile, map[string]string{"README.md": "same\n"})
	previous := projectName
	projectName = root
	t.Cleanup(func() { projectName = previous })
	cmd := newUpdateCmd()
	cmd.SetArgs([]string{zipFile, "--full-snapshot", "--yes", "--strip-components", "0"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("tracking only: %v\n%s", err, out.String())
	}
	if _, err := os.Stat(zipFile); err != nil {
		t.Fatalf("tracking-only update consumed archive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".routurn", "full-snapshot.json")); err != nil {
		t.Fatalf("snapshot state not saved: %v", err)
	}
}

func TestUpdateFullSnapshotDeclinedKeepsFileAndArchive(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("ROUTURN_DATA_DIR", filepath.Join(t.TempDir(), "managed"))
	mappingTestFile(t, root, "routurn.toml", "version = 1\nname = \"declined\"\n[execution]\nmode = \"local\"\n")
	previous := projectName
	projectName = root
	t.Cleanup(func() { projectName = previous })
	makeZip := func(files map[string]string) string {
		t.Helper()
		filename := filepath.Join(t.TempDir(), "update.zip")
		mappingTestZip(t, filename, files)
		return filename
	}
	run := func(input string, archive string, args ...string) (string, error) {
		t.Helper()
		cmd := newUpdateCmd()
		cmd.SetArgs(append([]string{archive}, args...))
		cmd.SetIn(strings.NewReader(input))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		err := cmd.Execute()
		return out.String(), err
	}
	first := makeZip(map[string]string{"obsolete.py": "old\n", "keep.py": "same\n"})
	if output, err := run("", first, "--full-snapshot", "--yes", "--keep-source", "--strip-components", "0"); err != nil {
		t.Fatalf("bootstrap: %v\n%s", err, output)
	}
	second := makeZip(map[string]string{"keep.py": "changed\n"})
	output, err := run("n\n", second, "--full-snapshot", "--strip-components", "0")
	if err != nil {
		t.Fatalf("declined: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Cancelled") || !strings.Contains(output, "Deleted  1") {
		t.Fatalf("expected deletion plan and cancel: %s", output)
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("declined source was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "obsolete.py")); err != nil {
		t.Fatalf("declined file was removed: %v", err)
	}
}
