package bundle

import (
	"archive/tar"
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestZipRejectsTraversal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.zip")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("../escape.txt")
	_, _ = w.Write([]byte("bad"))
	_ = zw.Close()
	_ = f.Close()
	if _, err := ReadArchive(path, 0); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestZipRejectsProtectedPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.zip")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	w, _ := zw.Create(".git/config")
	_, _ = w.Write([]byte("bad"))
	_ = zw.Close()
	_ = f.Close()
	if _, err := ReadArchive(path, 0); err == nil {
		t.Fatal("expected protected path rejection")
	}
}

func TestTarRejectsSymlink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.tar")
	f, _ := os.Create(path)
	tw := tar.NewWriter(f)
	_ = tw.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/tmp/x", Mode: 0o777})
	_ = tw.Close()
	_ = f.Close()
	if _, err := ReadArchive(path, 0); err == nil {
		t.Fatal("expected symlink rejection")
	}
}

func TestStripComponents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ok.zip")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("project/src/main.go")
	_, _ = w.Write([]byte("package main\n"))
	_ = zw.Close()
	_ = f.Close()
	entries, err := ReadArchive(path, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Path != "src/main.go" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}
