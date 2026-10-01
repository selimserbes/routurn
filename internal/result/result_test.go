package result

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterializePublishesStableSingleArtifact(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "artifacts")
	artifact := filepath.Join(source, "outputs", "gait", "telemetry.zip")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	publicPath, files, err := Materialize(root, "telemetry", "run-1", source)
	if err != nil {
		t.Fatal(err)
	}
	wantPublic := filepath.Join(root, "results", "telemetry", "latest.zip")
	if publicPath != wantPublic {
		t.Fatalf("public path = %q, want %q", publicPath, wantPublic)
	}
	if len(files) != 1 || files[0] != "outputs/gait/telemetry.zip" {
		t.Fatalf("unexpected files: %#v", files)
	}
	data, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "one\n" {
		t.Fatalf("unexpected public content: %q", data)
	}
	if !IsManagedPublicRoot(filepath.Join(root, "results")) {
		t.Fatal("results root is not marked as Routurn-managed")
	}

	meta, readPath, err := Read(root, "telemetry")
	if err != nil {
		t.Fatal(err)
	}
	if readPath != wantPublic {
		t.Fatalf("read path = %q, want %q", readPath, wantPublic)
	}
	if meta.PublicPath != "results/telemetry/latest.zip" {
		t.Fatalf("metadata public path = %q", meta.PublicPath)
	}
}

func TestMaterializeReplacesLatestResult(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(t.TempDir(), "one")
	if err := os.MkdirAll(filepath.Join(one, "reports"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(one, "reports", "result.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	publicOne, files, err := Materialize(root, "telemetry", "run-1", one)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "reports/result.txt" {
		t.Fatalf("unexpected files: %#v", files)
	}
	if filepath.Base(publicOne) != "latest.txt" {
		t.Fatalf("unexpected first public path: %s", publicOne)
	}

	two := filepath.Join(t.TempDir(), "two")
	if err := os.MkdirAll(two, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(two, "summary.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	publicTwo, _, err := Materialize(root, "telemetry", "run-2", two)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(publicTwo) != "latest.json" {
		t.Fatalf("unexpected second public path: %s", publicTwo)
	}
	if _, err := os.Stat(publicOne); !os.IsNotExist(err) {
		t.Fatalf("old public result survived replacement: %v", err)
	}
	internal := Dir(root, "telemetry")
	if _, err := os.Stat(filepath.Join(internal, "reports", "result.txt")); !os.IsNotExist(err) {
		t.Fatalf("old internal result survived replacement: %v", err)
	}
	if _, err := os.Stat(filepath.Join(internal, "summary.json")); err != nil {
		t.Fatalf("new internal result missing: %v", err)
	}
	meta, readPath, err := Read(root, "telemetry")
	if err != nil {
		t.Fatal(err)
	}
	if meta.RunID != "run-2" {
		t.Fatalf("unexpected latest run: %s", meta.RunID)
	}
	if readPath != publicTwo {
		t.Fatalf("unexpected read path: %s", readPath)
	}
}

func TestMaterializeMultipleArtifactsStripsCommonParent(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "artifacts")
	base := filepath.Join(source, "outputs", "telemetry")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"summary.json":  "{}\n",
		"telemetry.csv": "step,time\n",
	} {
		if err := os.WriteFile(filepath.Join(base, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	publicPath, _, err := Materialize(root, "telemetry", "run-1", source)
	if err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(root, "results", "telemetry")
	if publicPath != wantDir {
		t.Fatalf("public path = %q, want %q", publicPath, wantDir)
	}
	for _, name := range []string{"summary.json", "telemetry.csv"} {
		if _, err := os.Stat(filepath.Join(wantDir, name)); err != nil {
			t.Fatalf("public artifact %s missing: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(wantDir, "outputs")); !os.IsNotExist(err) {
		t.Fatalf("common artifact parent was not stripped: %v", err)
	}
}

func TestMaterializeDoesNotClaimExistingUserResultsDirectory(t *testing.T) {
	root := t.TempDir()
	userResults := filepath.Join(root, "results")
	if err := os.Mkdir(userResults, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userResults, "mine.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "artifact.zip"), []byte("zip\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	publicPath, _, err := Materialize(root, "task", "run-1", source)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "routurn-results", "task", "latest.zip")
	if publicPath != want {
		t.Fatalf("public path = %q, want fallback %q", publicPath, want)
	}
	data, err := os.ReadFile(filepath.Join(userResults, "mine.txt"))
	if err != nil || string(data) != "keep\n" {
		t.Fatalf("user results directory was modified: data=%q err=%v", data, err)
	}
}

func TestReadUpgradesLegacyResultToPublicView(t *testing.T) {
	root := t.TempDir()
	dir := Dir(root, "telemetry")
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "artifact.tgz"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	legacy := Metadata{Task: "telemetry", RunID: "legacy", UpdatedAt: "2026-10-01T00:00:00Z", Files: []string{"nested/artifact.tgz"}}
	if err := writeMetadata(filepath.Join(dir, "result.json"), legacy); err != nil {
		t.Fatal(err)
	}

	meta, publicPath, err := Read(root, "telemetry")
	if err != nil {
		t.Fatal(err)
	}
	if publicPath != filepath.Join(root, "results", "telemetry", "latest.tgz") {
		t.Fatalf("unexpected upgraded public path: %s", publicPath)
	}
	if meta.PublicPath != "results/telemetry/latest.tgz" {
		t.Fatalf("legacy metadata was not upgraded: %#v", meta)
	}
}

func TestMaterializeAddsGitLocalExclude(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "artifact.zip"), []byte("zip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Materialize(root, "task", "run-1", source); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "/results/") {
		t.Fatalf("git local exclude missing Routurn results path: %q", data)
	}
}

func TestStableSuffixPreservesCompoundTarExtension(t *testing.T) {
	if got := stableSuffix("bundle.tar.gz"); got != ".tar.gz" {
		t.Fatalf("stableSuffix = %q", got)
	}
}
