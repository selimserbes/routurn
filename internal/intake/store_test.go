package intake

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func makeZip(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("hello\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestImportConsumesSourceAfterVerifiedCopy(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	t.Setenv("ROUTURN_DATA_DIR", dataDir)
	source := filepath.Join(t.TempDir(), "update.zip")
	makeZip(t, source)

	result, err := Import(source, Options{Consume: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.SourceRemoved {
		t.Fatal("expected source to be removed")
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source still exists: %v", err)
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("managed bundle missing: %v", err)
	}

	copySource := filepath.Join(t.TempDir(), "update (1).zip")
	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copySource, data, 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := Import(copySource, Options{Consume: true})
	if err != nil {
		t.Fatal(err)
	}
	if !second.Duplicate {
		t.Fatal("expected duplicate bundle detection")
	}
	if second.Hash != result.Hash || second.Path != result.Path {
		t.Fatalf("duplicate did not reuse managed object: %#v %#v", result, second)
	}
}
