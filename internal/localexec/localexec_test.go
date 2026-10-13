package localexec

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/selimserbes/routurn/internal/result"
)

func TestRunLocalUsesProjectRootAndExitCode(t *testing.T) {
	root := t.TempDir()
	var out, stderr bytes.Buffer
	cmd := "pwd"
	if runtime.GOOS == "windows" {
		cmd = "cd"
	}
	code, err := Run(root, cmd, nil, &out, &stderr)
	if err != nil || code != 0 || !strings.Contains(strings.ToLower(out.String()), strings.ToLower(filepath.Base(root))) {
		t.Fatalf("code=%d err=%v stdout=%q stderr=%q", code, err, out.String(), stderr.String())
	}
	code, err = Run(root, "exit 7", nil, &out, &stderr)
	if err == nil || code != 7 {
		t.Fatalf("expected exit 7; code=%d err=%v", code, err)
	}
}

func TestCollectProjectArtifacts(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "outputs", "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outputs", "nested", "result.json"), []byte(`{"ok":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, ".routurn", "runs", "test", "artifacts")
	files, err := Collect(root, []string{"outputs/**/*.json"}, dest)
	if err != nil || len(files) != 1 || files[0] != "outputs/nested/result.json" {
		t.Fatalf("files=%q err=%v", files, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "outputs", "nested", "result.json")); err != nil {
		t.Fatal(err)
	}
	files, err = Collect(root, []string{".routurn/**"}, dest)
	if err != nil || len(files) != 0 {
		t.Fatalf("private data collected: %q %v", files, err)
	}
}

func TestCollectRejectsTraversalAndSymlink(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"../secret", "foo/../../secret", "/etc/passwd"} {
		if _, err := Collect(root, []string{p}, filepath.Join(root, ".routurn", "out")); err == nil {
			t.Fatalf("accepted %q", p)
		}
	}
	if _, err := Collect(root, []string{"**"}, root); err == nil {
		t.Fatal("accepted root as destination")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err == nil {
		if _, err := Collect(root, []string{"**"}, filepath.Join(root, "linked", "outputs")); err == nil {
			t.Fatal("accepted symlink destination")
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "outputs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secrets"), filepath.Join(root, "outputs", "secret")); err == nil {
		files, err := Collect(root, []string{"outputs/**"}, filepath.Join(root, ".routurn", "safe"))
		if err != nil || len(files) != 0 {
			t.Fatalf("followed symlink: %q %v", files, err)
		}
	}
}

func TestCollectAllowsProjectUnderSymlinkAncestor(t *testing.T) {
	realParent := t.TempDir()
	aliasParent := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(realParent, aliasParent); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	root := filepath.Join(aliasParent, "project")
	if err := os.MkdirAll(filepath.Join(root, "output"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "output", "ok.txt"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	files, err := Collect(root, []string{"output/*.txt"}, filepath.Join(root, ".routurn", "artifacts"))
	if err != nil || len(files) != 1 {
		t.Fatalf("files=%v err=%v", files, err)
	}
}

func TestCollectSkipsOnlyRouturnManagedPublishedResults(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		t.Run(map[bool]string{false: "results", true: "fallback"}[fallback], func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "output"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "output", "report.txt"), []byte("new"), 0644); err != nil {
				t.Fatal(err)
			}
			if fallback {
				// Existing user-owned results/ must remain eligible.
				if err := os.Mkdir(filepath.Join(root, "results"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "results", "user.txt"), []byte("user"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			source := t.TempDir()
			if err := os.WriteFile(filepath.Join(source, "old.txt"), []byte("old"), 0644); err != nil {
				t.Fatal(err)
			}
			if _, _, err := result.Materialize(root, "smoke", "run-old", source); err != nil {
				t.Fatal(err)
			}
			files, err := Collect(root, []string{"**/*.txt"}, filepath.Join(root, ".routurn", "runs", "run-new", "artifacts"))
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"output/report.txt"}
			if fallback {
				want = append(want, "results/user.txt")
			}
			if strings.Join(files, ",") != strings.Join(want, ",") {
				t.Fatalf("collected %q, want %q", files, want)
			}
		})
	}
}
