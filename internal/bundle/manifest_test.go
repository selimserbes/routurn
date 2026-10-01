package bundle

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestIsNotAppliedAsPayload(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "update.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	mf, _ := zw.Create(ManifestPath)
	_, _ = mf.Write([]byte("schema = 1\n[bundle]\nname = \"demo\"\n[project]\nname = \"example\"\n"))
	payload, _ := zw.Create("hello.txt")
	_, _ = payload.Write([]byte("hello\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	manifest, err := ManifestFromArchive(archive, 0)
	if err != nil {
		t.Fatal(err)
	}
	if manifest == nil || manifest.Bundle.Name != "demo" || manifest.Project.Name != "example" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	inspection, err := InspectArchive(archive, 0)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Manifest == nil || inspection.Files != 1 || len(inspection.Entries) != 1 {
		t.Fatalf("unexpected inspection: %#v", inspection)
	}

	project := filepath.Join(dir, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(project, archive, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Added) != 1 || plan.Added[0] != "hello.txt" {
		t.Fatalf("manifest leaked into payload plan: %#v", plan.Added)
	}
}
