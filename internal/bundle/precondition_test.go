package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func testSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func TestCheckBaseFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dir", "existing.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths := []string{"dir/existing.txt", "dir/new.txt"}
	expected := map[string]string{
		"dir/existing.txt": testSHA256([]byte("base\n")),
		"dir/new.txt":      MissingBaseState,
	}
	ok, mismatches, err := CheckBaseFiles(root, paths, expected)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || len(mismatches) != 0 {
		t.Fatalf("expected preconditions to match, ok=%v mismatches=%v", ok, mismatches)
	}

	if err := os.WriteFile(filepath.Join(root, "dir", "existing.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, mismatches, err = CheckBaseFiles(root, paths, expected)
	if err != nil {
		t.Fatal(err)
	}
	if ok || len(mismatches) != 1 {
		t.Fatalf("expected one mismatch, ok=%v mismatches=%v", ok, mismatches)
	}
}

func TestCheckBaseFilesRequiresFullPayloadCoverage(t *testing.T) {
	root := t.TempDir()
	ok, mismatches, err := CheckBaseFiles(root, []string{"a.txt", "b.txt"}, map[string]string{
		"a.txt": MissingBaseState,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok || len(mismatches) != 1 {
		t.Fatalf("expected missing precondition mismatch, ok=%v mismatches=%v", ok, mismatches)
	}
}
