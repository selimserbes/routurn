package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestLegacyFullPromptEligibility(t *testing.T) {
	tests := []struct {
		name                      string
		picker, dryRun, yes, full bool
		want                      bool
	}{
		{name: "interactive picker", picker: true, want: true},
		{name: "explicit archive", want: false},
		{name: "dry run", picker: true, dryRun: true, want: false},
		{name: "noninteractive approval", picker: true, yes: true, want: false},
		{name: "explicit full snapshot", picker: true, full: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldOfferLegacyFullSnapshot(tt.picker, tt.dryRun, tt.yes, tt.full)
			if got != tt.want {
				t.Fatalf("got prompt eligibility %t, want %t", got, tt.want)
			}
		})
	}
}

func TestLegacyFullPromptDefaultNoAndBufferedApplyAnswer(t *testing.T) {
	tests := []struct {
		name, input    string
		want           bool
		checkRemainder bool
	}{
		{name: "opt in", input: "2\ny\n", want: true, checkRemainder: true},
		{name: "typed full is ignored", input: "FULL\ny\n", want: false, checkRemainder: true},
		{name: "ordinary yes never authorizes full", input: "y\ny\n", want: false, checkRemainder: true},
		{name: "opt out", input: "1\ny\n", want: false, checkRemainder: true},
		{name: "empty", input: "\n", want: false},
		{name: "end of input", input: "", want: false},
		{name: "unknown", input: "maybe\n", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "update"}
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetIn(strings.NewReader(tt.input))
			full, err := confirmLegacyFullSnapshot(cmd)
			if err != nil {
				t.Fatal(err)
			}
			if full != tt.want {
				t.Fatalf("full=%t; want %t", full, tt.want)
			}
			if tt.checkRemainder {
				remaining, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					t.Fatal(err)
				}
				if string(remaining) != "y\n" {
					t.Fatalf("apply answer lost or changed: %q", remaining)
				}
			}
		})
	}
}
