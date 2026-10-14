package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/selimserbes/routurn/internal/syncer"
)

// A marked full bundle must prove the exact current project state before
// obsolete-file deletion is enabled with the ordinary `routurn update` command.
func TestManifestAutoFullSnapshotWithExactBase(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("ROUTURN_DATA_DIR", filepath.Join(t.TempDir(), "managed"))
	mappingTestFile(t, root, "routurn.toml", "version = 1\nname = \"auto-project\"\n[execution]\nmode = \"local\"\n")
	mappingTestFile(t, root, "user-notes.txt", "unmanaged\n")

	previousProject := projectName
	projectName = root
	t.Cleanup(func() { projectName = previousProject })

	run := func(args ...string) (string, error) {
		t.Helper()
		cmd := newUpdateCmd()
		cmd.SetArgs(args)
		cmd.SetIn(strings.NewReader(""))
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		err := cmd.Execute()
		return output.String(), err
	}

	initial := filepath.Join(t.TempDir(), "initial.zip")
	mappingTestZip(t, initial, map[string]string{
		"obsolete.go": "managed old\n",
		"main.go":     "initial\n",
	})
	if out, err := run(initial, "--full-snapshot", "--yes", "--keep-source"); err != nil {
		t.Fatalf("initial ownership: %v\n%s", err, out)
	}

	scan, err := syncer.Scan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := syncer.Fingerprint(scan)
	manifest := "schema = 1\n[bundle]\nname = \"replacement\"\nmode = \"full\"\n[project]\nname = \"auto-project\"\n[base]\nfingerprint = \"" + base + "\"\n"
	archive := filepath.Join(t.TempDir(), "verified.zip")
	mappingTestZip(t, archive, map[string]string{
		"routurn-bundle.toml": manifest,
		"main.go":             "updated\n",
		"replacement.go":      "replacement\n",
	})

	out, err := run(archive, "--dry-run")
	if err != nil || !strings.Contains(out, "automatic obsolete-file cleanup enabled") || !strings.Contains(out, "Deleted  1") {
		t.Fatalf("dry run did not select verified full snapshot: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(root, "obsolete.go")); err != nil {
		t.Fatal("dry run removed a file:", err)
	}

	out, err = run(archive, "--yes", "--keep-source")
	if err != nil {
		t.Fatalf("automatic full update: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(root, "obsolete.go")); !os.IsNotExist(err) {
		t.Fatalf("obsolete managed file should have been removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "user-notes.txt")); err != nil {
		t.Fatal("unmanaged file removed:", err)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatal("archive not preserved:", err)
	}
}

func TestUnverifiedFullDeclarationDoesNotPrune(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("ROUTURN_DATA_DIR", filepath.Join(t.TempDir(), "managed"))
	mappingTestFile(t, root, "routurn.toml", "version = 1\nname = \"safe-project\"\n[execution]\nmode = \"local\"\n")
	previousProject := projectName
	projectName = root
	t.Cleanup(func() { projectName = previousProject })
	initial := filepath.Join(t.TempDir(), "initial.zip")
	mappingTestZip(t, initial, map[string]string{
		"obsolete.go": "old\n",
		"main.go":     "old\n",
	})
	cmd := newUpdateCmd()
	cmd.SetArgs([]string{initial, "--yes", "--keep-source", "--full-snapshot"})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	unverified := filepath.Join(t.TempDir(), "unverified.zip")
	mappingTestZip(t, unverified, map[string]string{
		"routurn-bundle.toml": "schema = 1\n[bundle]\nmode = \"full\"\n[project]\nname = \"safe-project\"\n",
		"main.go":             "new\n",
	})
	cmd = newUpdateCmd()
	cmd.SetArgs([]string{unverified, "--dry-run"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("safe fallback failed: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "preserving old files") || strings.Contains(out.String(), "Deleted  1") {
		t.Fatalf("unsafe deletion inferred: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, "obsolete.go")); err != nil {
		t.Fatal("unverified bundle deleted old file:", err)
	}
}
