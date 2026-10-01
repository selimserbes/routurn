package remote

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/selimserbes/routurn/internal/config"
)

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type DetachedStatus struct {
	Status     string
	PID        int
	Alive      bool
	ExitCode   *int
	StartedAt  string
	FinishedAt string
	Mode       string
}

func validateRunID(runID string) error {
	if !runIDPattern.MatchString(runID) {
		return fmt.Errorf("invalid run id: %q", runID)
	}
	return nil
}

func StartDetached(target config.Target, remoteRoot, runID, command string) (int, error) {
	if err := validateRunID(runID); err != nil {
		return 0, err
	}

	runDir := ".routurn/runs/" + runID
	wrapper := fmt.Sprintf(`#!/bin/sh
set +e
cd %s || exit 125
run_dir=%s
finish_cancelled() {
  printf 'CANCELLED\n' > "$run_dir/status"
  printf '130\n' > "$run_dir/exit_code"
  date -u '+%%Y-%%m-%%dT%%H:%%M:%%SZ' > "$run_dir/finished_at"
  exit 130
}
trap finish_cancelled TERM INT HUP
sh -lc %s
code=$?
printf '%%s\n' "$code" > "$run_dir/exit_code"
if [ "$code" -eq 0 ]; then
  printf 'SUCCEEDED\n' > "$run_dir/status"
else
  printf 'FAILED\n' > "$run_dir/status"
fi
date -u '+%%Y-%%m-%%dT%%H:%%M:%%SZ' > "$run_dir/finished_at"
exit "$code"
`, shellQuote(remoteRoot), shellQuote(runDir), shellQuote(command))

	script := fmt.Sprintf(`set -eu
cd %s
run_dir=%s
mkdir -p "$run_dir"
: > "$run_dir/stdout.log"
: > "$run_dir/stderr.log"
printf 'RUNNING\n' > "$run_dir/status"
date -u '+%%Y-%%m-%%dT%%H:%%M:%%SZ' > "$run_dir/started_at"
printf '%%s\n' %s > "$run_dir/command"
printf '%%s' %s > "$run_dir/task.sh"
chmod 700 "$run_dir/task.sh"
if command -v setsid >/dev/null 2>&1; then
  nohup setsid sh "$run_dir/task.sh" > "$run_dir/stdout.log" 2> "$run_dir/stderr.log" < /dev/null &
  mode=group
else
  nohup sh "$run_dir/task.sh" > "$run_dir/stdout.log" 2> "$run_dir/stderr.log" < /dev/null &
  mode=pid
fi
pid=$!
printf '%%s\n' "$pid" > "$run_dir/pid"
printf '%%s\n' "$mode" > "$run_dir/mode"
printf '%%s\n' "$pid"
`, shellQuote(remoteRoot), shellQuote(runDir), shellQuote(command), shellQuote(wrapper))

	data, err := Capture(target, "sh -lc "+shellQuote(script))
	if err != nil {
		return 0, fmt.Errorf("start detached task: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("start detached task: invalid remote pid %q", strings.TrimSpace(string(data)))
	}
	return pid, nil
}

func QueryDetached(target config.Target, remoteRoot, runID string) (DetachedStatus, error) {
	if err := validateRunID(runID); err != nil {
		return DetachedStatus{}, err
	}
	runDir := ".routurn/runs/" + runID
	script := fmt.Sprintf(`set -eu
cd %s
run_dir=%s
[ -d "$run_dir" ] || exit 44
status="$(cat "$run_dir/status" 2>/dev/null || printf 'UNKNOWN')"
pid="$(cat "$run_dir/pid" 2>/dev/null || printf '0')"
mode="$(cat "$run_dir/mode" 2>/dev/null || printf 'pid')"
alive=false
if [ "$pid" -gt 0 ] 2>/dev/null && kill -0 "$pid" 2>/dev/null; then alive=true; fi
if [ "$status" = RUNNING ] && [ "$alive" = false ]; then status=LOST; fi
if [ "$status" = STOPPING ] && [ "$alive" = false ]; then status=CANCELLED; fi
case "$status" in SUCCEEDED|FAILED|CANCELLED|LOST) alive=false ;; esac
printf 'status=%%s\n' "$status"
printf 'pid=%%s\n' "$pid"
printf 'alive=%%s\n' "$alive"
printf 'mode=%%s\n' "$mode"
if [ -f "$run_dir/exit_code" ]; then printf 'exit_code=%%s\n' "$(cat "$run_dir/exit_code")"; fi
if [ -f "$run_dir/started_at" ]; then printf 'started_at=%%s\n' "$(cat "$run_dir/started_at")"; fi
if [ -f "$run_dir/finished_at" ]; then printf 'finished_at=%%s\n' "$(cat "$run_dir/finished_at")"; fi
`, shellQuote(remoteRoot), shellQuote(runDir))
	data, err := Capture(target, "sh -lc "+shellQuote(script))
	if err != nil {
		return DetachedStatus{}, fmt.Errorf("inspect detached run: %w", err)
	}
	return parseDetachedStatus(data)
}

func parseDetachedStatus(data []byte) (DetachedStatus, error) {
	values := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[key] = value
	}
	if values["status"] == "" {
		return DetachedStatus{}, fmt.Errorf("remote run status is missing")
	}
	out := DetachedStatus{
		Status:     values["status"],
		Alive:      values["alive"] == "true",
		StartedAt:  values["started_at"],
		FinishedAt: values["finished_at"],
		Mode:       values["mode"],
	}
	if values["pid"] != "" {
		pid, err := strconv.Atoi(values["pid"])
		if err != nil {
			return DetachedStatus{}, fmt.Errorf("invalid remote pid %q", values["pid"])
		}
		out.PID = pid
	}
	if values["exit_code"] != "" {
		code, err := strconv.Atoi(values["exit_code"])
		if err != nil {
			return DetachedStatus{}, fmt.Errorf("invalid remote exit code %q", values["exit_code"])
		}
		out.ExitCode = &code
	}
	return out, nil
}

func StreamDetachedLogs(target config.Target, remoteRoot, runID string, lines int, follow bool, stdout, stderr io.Writer) error {
	if err := validateRunID(runID); err != nil {
		return err
	}
	if lines < 0 {
		lines = 0
	}
	runDir := ".routurn/runs/" + runID
	if !follow {
		command := fmt.Sprintf("cd %s && run_dir=%s && [ -d \"$run_dir\" ] && tail -n %d \"$run_dir/stdout.log\" \"$run_dir/stderr.log\"", shellQuote(remoteRoot), shellQuote(runDir), lines)
		_, err := RunCommand(target, command, nil, stdout, stderr)
		return err
	}

	script := fmt.Sprintf(`set -eu
cd %s
run_dir=%s
[ -d "$run_dir" ] || exit 44
tail -n %d -f "$run_dir/stdout.log" "$run_dir/stderr.log" &
tail_pid=$!
cleanup() { kill "$tail_pid" 2>/dev/null || true; wait "$tail_pid" 2>/dev/null || true; }
trap cleanup EXIT INT TERM HUP
while :; do
  status="$(cat "$run_dir/status" 2>/dev/null || printf 'UNKNOWN')"
  case "$status" in
    RUNNING|STOPPING) sleep 1 ;;
    *) break ;;
  esac
done
sleep 1
`, shellQuote(remoteRoot), shellQuote(runDir), lines)
	_, err := RunCommand(target, "sh -lc "+shellQuote(script), nil, stdout, stderr)
	return err
}

func StopDetached(target config.Target, remoteRoot, runID string, force bool) error {
	if err := validateRunID(runID); err != nil {
		return err
	}
	runDir := ".routurn/runs/" + runID
	signal := "TERM"
	finalStatus := "STOPPING"
	if force {
		signal = "KILL"
		finalStatus = "CANCELLED"
	}
	script := fmt.Sprintf(`set -eu
cd %s
run_dir=%s
[ -d "$run_dir" ] || exit 44
pid="$(cat "$run_dir/pid" 2>/dev/null || printf '0')"
mode="$(cat "$run_dir/mode" 2>/dev/null || printf 'pid')"
[ "$pid" -gt 0 ] 2>/dev/null || exit 45
if ! kill -0 "$pid" 2>/dev/null; then
  exit 0
fi
printf '%s\n' > "$run_dir/status"
if [ "$mode" = group ]; then
  kill -%s -"$pid" 2>/dev/null || kill -%s "$pid" 2>/dev/null || true
else
  kill -%s "$pid" 2>/dev/null || true
fi
`, shellQuote(remoteRoot), shellQuote(runDir), finalStatus, signal, signal, signal)
	var stderr bytes.Buffer
	_, err := RunCommand(target, "sh -lc "+shellQuote(script), nil, io.Discard, &stderr)
	if err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("stop detached run: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return fmt.Errorf("stop detached run: %w", err)
	}
	return nil
}
