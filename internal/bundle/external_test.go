package bundle

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseSevenZipListing(t *testing.T) {
	listing := `
Path = archive.7z
Type = 7z
Physical Size = 123

Path = src
Folder = +
Size = 0
Attributes = D drwxr-xr-x

Path = src/main.sh
Folder = -
Size = 8
Attributes = A -rwxr-xr-x
Encrypted = -

Path = README.md
Folder = -
Size = 5
Attributes = A -rw-r--r--
Encrypted = -
`
	records, err := parseSevenZipListing(listing)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d: %#v", len(records), records)
	}
	if records[1]["Path"] != "src/main.sh" {
		t.Fatalf("unexpected record: %#v", records[1])
	}
}

func TestExternalRecordRejectsSymlink(t *testing.T) {
	record := sevenZipRecord{
		"Path":          "link",
		"Folder":        "-",
		"Size":          "4",
		"Mode":          "lrwxrwxrwx",
		"Symbolic Link": "../target",
	}
	if err := validateExternalRecord("link", record, false); err == nil {
		t.Fatal("expected symlink rejection")
	}
}

func TestExternalRecordRejectsEncrypted(t *testing.T) {
	record := sevenZipRecord{
		"Path":      "secret.txt",
		"Folder":    "-",
		"Size":      "4",
		"Encrypted": "+",
	}
	if err := validateExternalRecord("secret.txt", record, false); err == nil {
		t.Fatal("expected encrypted entry rejection")
	}
}

func TestExternalArchiveUsesSafeStdoutImporter(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell importer test is Unix-only")
	}
	bin := t.TempDir()
	tool := filepath.Join(bin, "7zz")
	script := `#!/bin/sh
set -eu
case "$1" in
  l)
    cat <<'LIST'
Path = pkg
Folder = +
Size = 0
Attributes = D drwxr-xr-x

Path = pkg/run.sh
Folder = -
Size = 8
Attributes = A -rwxr-xr-x
Encrypted = -

Path = pkg/readme.txt
Folder = -
Size = 5
Attributes = A -rw-r--r--
Encrypted = -
LIST
    ;;
  x)
    last=""
    for arg in "$@"; do last="$arg"; done
    case "$last" in
      pkg/run.sh) printf '#!/bin/x' ;;
      pkg/readme.txt) printf 'hello' ;;
      *) echo "unexpected path: $last" >&2; exit 7 ;;
    esac
    ;;
  *) exit 8 ;;
esac
`
	if err := os.WriteFile(tool, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	archive := filepath.Join(t.TempDir(), "update.7z")
	if err := os.WriteFile(archive, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadArchive(archive, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %#v", entries)
	}
	if entries[0].Path != "run.sh" || entries[0].Mode != 0o755 || string(entries[0].Data) != "#!/bin/x" {
		t.Fatalf("unexpected first entry: %#v", entries[0])
	}
	if entries[1].Path != "readme.txt" || entries[1].Mode != 0o644 || string(entries[1].Data) != "hello" {
		t.Fatalf("unexpected second entry: %#v", entries[1])
	}
}

func TestExternalArchiveRejectsTraversalBeforeExtract(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell importer test is Unix-only")
	}
	bin := t.TempDir()
	tool := filepath.Join(bin, "7z")
	script := `#!/bin/sh
set -eu
if [ "$1" = "l" ]; then
cat <<'LIST'
Path = ../escape.txt
Folder = -
Size = 3
Attributes = A -rw-r--r--
Encrypted = -
LIST
exit 0
fi
echo EXTRACT_CALLED >&2
exit 99
`
	if err := os.WriteFile(tool, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	archive := filepath.Join(t.TempDir(), "bad.rar")
	_ = os.WriteFile(archive, []byte("fake"), 0o644)
	_, err := ReadArchive(archive, 0)
	if err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("expected traversal rejection, got %v", err)
	}
	if strings.Contains(err.Error(), "EXTRACT_CALLED") {
		t.Fatalf("archive extraction ran before traversal rejection: %v", err)
	}
}

func TestNormalizeRejectsDriveAndControlPaths(t *testing.T) {
	for _, name := range []string{
		"C:/Windows/file.txt",
		"dir/\x1b[31m.txt",
		"dir/tab\tname.txt",
		"file:stream",
		".git./config",
		"dir/NUL.txt",
		"dir/COM1",
		"dir/trailing. /file.txt",
	} {
		if _, _, err := normalizeArchivePath(name, 0); err == nil {
			t.Fatalf("expected unsafe path rejection for %q", name)
		}
	}
}

func TestExternalImporterMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	archive := filepath.Join(t.TempDir(), "update.7z")
	_ = os.WriteFile(archive, []byte("fake"), 0o644)
	_, err := ReadArchive(archive, 0)
	if err == nil || !strings.Contains(err.Error(), "7-Zip-compatible CLI") {
		t.Fatalf("unexpected error: %v", err)
	}
}
