package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The original update source must survive a declined confirmation or a no-op.
// Only a fully applied update may consume the original (unless --keep-source).
func TestLegacyUpdateRetainsSourceUntilSuccessfullyApplied(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("ROUTURN_DATA_DIR", filepath.Join(t.TempDir(), "managed"))
	mappingTestFile(t, root, "routurn.toml", "version = 1\nname = \"source-lifecycle\"\n[execution]\nmode = \"local\"\n")
	mappingTestFile(t, root, "README.md", "old\n")
	mappingTestFile(t, root, "pyproject.toml", "old\n")
	archive := filepath.Join(t.TempDir(), "update.zip")
	mappingTestZip(t, archive, map[string]string{
		"wrapper/README.md":      "new\n",
		"wrapper/pyproject.toml": "new\n",
	})
	previous := projectName
	projectName = root
	t.Cleanup(func() { projectName = previous })

	run := func(stdin string, flags ...string) (string, error) {
		t.Helper()
		cmd := newUpdateCmd()
		args := append([]string{archive}, flags...)
		cmd.SetArgs(args)
		cmd.SetIn(strings.NewReader(stdin))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		err := cmd.Execute()
		return out.String(), err
	}
	output, err := run("n\n")
	if err != nil {
		t.Fatalf("cancel: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Project root detected") || !strings.Contains(output, "Cancelled") {
		t.Fatalf("expected mapping and cancellation:\n%s", output)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("cancel removed the ZIP: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(root, "README.md")); err != nil || string(b) != "old\n" {
		t.Fatalf("cancel changed project: %q %v", b, err)
	}
	output, err = run("", "--yes")
	if err != nil {
		t.Fatalf("apply: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Update applied") {
		t.Fatalf("not applied:\n%s", output)
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatalf("applied update should consume source: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(root, "README.md")); err != nil || string(b) != "new\n" {
		t.Fatalf("applied content incorrect: %q %v", b, err)
	}
}
func TestNoopUpdatePreservesSource(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("ROUTURN_DATA_DIR", filepath.Join(t.TempDir(), "managed"))
	mappingTestFile(t, root, "routurn.toml", "version = 1\nname = \"noop\"\n[execution]\nmode = \"local\"\n")
	mappingTestFile(t, root, "README.md", "same\n")
	mappingTestFile(t, root, "pyproject.toml", "same\n")
	archive := filepath.Join(t.TempDir(), "noop.zip")
	mappingTestZip(t, archive, map[string]string{"wrap/README.md": "same\n", "wrap/pyproject.toml": "same\n"})
	previous := projectName
	projectName = root
	t.Cleanup(func() { projectName = previous })
	cmd := newUpdateCmd()
	cmd.SetArgs([]string{archive, "--yes"})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("no-op removed original source: %v", err)
	}
}

func TestExecUpdateUsesMappingAndKeepsDeclinedSource(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("ROUTURN_DATA_DIR", filepath.Join(t.TempDir(), "managed"))
	mappingTestFile(t, root, "routurn.toml", "version = 1\nname = \"exec-update\"\n[execution]\nmode = \"local\"\n[tasks.smoke]\ncommand = \"echo must-not-run\"\n")
	mappingTestFile(t, root, "README.md", "old\n")
	mappingTestFile(t, root, "pyproject.toml", "old\n")
	archive := filepath.Join(t.TempDir(), "update.zip")
	mappingTestZip(t, archive, map[string]string{
		"outer/middle/README.md":      "new\n",
		"outer/middle/pyproject.toml": "new\n",
	})
	previous := projectName
	projectName = root
	t.Cleanup(func() { projectName = previous })
	cmd := newExecCmd()
	cmd.SetArgs([]string{"smoke", "--update=" + archive})
	cmd.SetIn(strings.NewReader("n\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec update: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "2 wrapper folder(s) skipped") || !strings.Contains(out.String(), "Cancelled") {
		t.Fatalf("expected mapping and cancelled update: %s", out.String())
	}
	if strings.Contains(out.String(), "must-not-run") {
		t.Fatalf("task ran after cancelled update: %s", out.String())
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("exec cancelled update removed source: %v", err)
	}
}
