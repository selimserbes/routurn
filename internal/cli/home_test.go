package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestHomeMenuNonInteractiveShowsHelp(t *testing.T) {
	cmd := &cobra.Command{Use: "routurn", Short: "CLI"}
	// Cobra only includes command usage when runnable subcommands exist.
	// The real Routurn root has runnable subcommands; mirror that here.
	cmd.AddCommand(&cobra.Command{Use: "exec", Short: "Run a task", Run: func(*cobra.Command, []string) {}})
	cmd.SetIn(strings.NewReader(""))
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runHomeMenu(cmd); err != nil {
		t.Fatalf("noninteractive home: %v", err)
	}
	if !strings.Contains(out.String(), "routurn") {
		t.Fatalf("expected CLI help: %q", out.String())
	}
}
