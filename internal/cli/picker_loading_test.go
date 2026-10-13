package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestPickerLoadingFastNoFlash(t *testing.T) {
	var out bytes.Buffer
	called := false
	withPickerLoadingAfter(&out, "Discovering tasks...", 100*time.Millisecond, func() { called = true })
	if !called || out.Len() != 0 {
		t.Fatalf("fast task flashed loading: called=%v output=%q", called, out.String())
	}
}

func TestPickerLoadingSlowClearsBeforeMenu(t *testing.T) {
	var out bytes.Buffer
	withPickerLoadingAfter(&out, "Discovering tasks...", 5*time.Millisecond, func() {
		time.Sleep(35 * time.Millisecond)
	})
	got := out.String()
	if !strings.Contains(got, "Discovering tasks...") || !strings.HasSuffix(got, "\r\x1b[2K") {
		t.Fatalf("loading line not displayed and cleared: %q", got)
	}
}

func TestPickerLoadingNonInteractiveEmitsNoControls(t *testing.T) {
	var out bytes.Buffer
	withPickerLoading(&out, false, "Discovering tasks...", func() { out.WriteString("normal output") })
	if got := out.String(); got != "normal output" {
		t.Fatalf("non-interactive output unexpectedly changed: %q", got)
	}
}

func TestPickerTimingOptInOnly(t *testing.T) {
	var out bytes.Buffer
	t.Setenv("ROUTURN_STARTUP_TIMING", "")
	pickerReportTiming(&out, "exec", pickerTiming{"discovery", time.Second})
	if out.Len() != 0 {
		t.Fatalf("unexpected timing output: %q", out.String())
	}
	t.Setenv("ROUTURN_STARTUP_TIMING", "1")
	pickerReportTiming(&out, "exec", pickerTiming{"discovery", time.Second})
	if !strings.Contains(out.String(), "discovery=1s") {
		t.Fatalf("missing timing output: %q", out.String())
	}
}
