package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/selimserbes/routurn/internal/config"
)

func TestTargetJumpCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Setenv("APPDATA", filepath.Join(home, "appdata"))
	t.Setenv("HOME", home)
	run := func(args ...string) (string, error) {
		t.Helper()
		cmd := newTargetCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		err := cmd.Execute()
		return out.String(), err
	}
	_, err := run("add", "gpu", "--host", "private.internal", "--user", "mss", "--jump", "ops@bastion:2222")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Targets["gpu"].Jump != "ops@bastion:2222" {
		t.Fatalf("jump not persisted: %+v", cfg.Targets["gpu"])
	}

	// Updating without --jump should preserve the prior route.
	if _, err := run("add", "gpu", "--host", "private2.internal"); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Targets["gpu"].Jump != "ops@bastion:2222" {
		t.Fatal("unrelated target update cleared jump")
	}

	// Conversion to named endpoints must preserve the legacy jump.
	if _, err := run("endpoint", "add", "gpu", "backup", "--host", "backup.internal", "--jump", "gateway1,gateway2"); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Targets["gpu"].Endpoints["primary"].Jump != "ops@bastion:2222" || cfg.Targets["gpu"].Endpoints["backup"].Jump != "gateway1,gateway2" {
		t.Fatalf("named endpoints lost their jump configuration: %+v", cfg.Targets["gpu"].Endpoints)
	}
	output, err := run("endpoint", "list", "gpu")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "jump=gateway1,gateway2") || !strings.Contains(output, "jump=ops@bastion:2222") {
		t.Fatalf("list omitted jump details: %s", output)
	}

	// Omitted --jump should not clear an existing endpoint jump.
	if _, err := run("endpoint", "add", "gpu", "backup", "--host", "new.internal"); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Targets["gpu"].Endpoints["backup"].Jump != "gateway1,gateway2" {
		t.Fatal("updating endpoint cleared jump")
	}

	// Explicit empty value is an intentional reset.
	if _, err := run("endpoint", "add", "gpu", "backup", "--host", "new.internal", "--jump", ""); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Targets["gpu"].Endpoints["backup"].Jump != "" {
		t.Fatal("cannot clear a configured jump")
	}

	// Reject malicious/malformed jump syntax without saving any target.
	if _, err := run("endpoint", "add", "gpu", "bad", "--host", "somewhere", "--jump", "-oProxyCommand=evil"); err == nil {
		t.Fatal("unsafe jump was accepted")
	}
	cfg, err = config.LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Targets["gpu"].Endpoints["bad"]; ok {
		t.Fatal("unsafe endpoint was saved")
	}
}
