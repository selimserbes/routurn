package cli

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"
)

const pickerLoadingDelay = 450 * time.Millisecond

type pickerTiming struct {
	name    string
	elapsed time.Duration
}

// withPickerLoading gives feedback only if interactive preparation takes longer
// than pickerLoadingDelay. It does not change when work starts or add a wait.
// The progress line is always cleared before the caller draws the menu.
// Non-TTY callers never emit ANSI control characters or use a goroutine.
func withPickerLoading(out io.Writer, interactive bool, label string, work func()) {
	if !interactive || runtime.GOOS == "windows" {
		work()
		return
	}
	withPickerLoadingAfter(out, label, pickerLoadingDelay, work)
}

func withPickerLoadingAfter(out io.Writer, label string, delay time.Duration, work func()) {
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-done:
			return
		case <-timer.C:
		}

		ticker := time.NewTicker(120 * time.Millisecond)
		defer ticker.Stop()
		frames := []string{"|", "/", "-", "\\"}
		frame := 0
		for {
			fmt.Fprintf(out, "\r\x1b[2K%s %s", frames[frame%len(frames)], label)
			frame++
			select {
			case <-done:
				fmt.Fprint(out, "\r\x1b[2K")
				return
			case <-ticker.C:
			}
		}
	}()
	defer func() {
		close(done)
		<-stopped
	}()
	work()
}

// Opt-in startup diagnostics are printed after leaving the picker, so its
// screen redraw cannot hide the timings. No logs or filesystem writes occur
// unless ROUTURN_STARTUP_TIMING=1 is explicitly set by the user.
func pickerReportTiming(out io.Writer, scope string, stages ...pickerTiming) {
	if os.Getenv("ROUTURN_STARTUP_TIMING") != "1" {
		return
	}
	parts := make([]string, 0, len(stages))
	for _, stage := range stages {
		parts = append(parts, fmt.Sprintf("%s=%s", stage.name, stage.elapsed.Round(time.Millisecond)))
	}
	fmt.Fprintf(out, "[routurn startup] %s: %s\n", scope, strings.Join(parts, ", "))
}
