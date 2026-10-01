package remote

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/selimserbes/routurn/internal/config"
)

const (
	DefaultSnapshotRetention = 10
	DefaultDetachedRetention = 10
)

func PruneSnapshots(target config.Endpoint, remoteRoot string, keep int) ([]string, error) {
	if keep < 0 {
		keep = 0
	}
	command := fmt.Sprintf("cd %s && if [ -d .routurn/snapshots ]; then find .routurn/snapshots -type f -name '*.tar' -print0; fi", shellQuote(remoteRoot))
	data, err := Capture(target, command)
	if err != nil {
		return nil, err
	}
	parts := bytes.Split(data, []byte{0})
	var paths []string
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		path := filepathSlash(strings.TrimPrefix(string(part), "./"))
		if !strings.HasPrefix(path, ".routurn/snapshots/") || !strings.HasSuffix(path, ".tar") {
			continue
		}
		if err := validateRelativePath(path); err != nil {
			continue
		}
		paths = append(paths, path)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	if len(paths) <= keep {
		return nil, nil
	}
	removed := append([]string(nil), paths[keep:]...)
	if err := RemovePaths(target, remoteRoot, removed); err != nil {
		return nil, err
	}
	return removed, nil
}

func PruneDetachedRuns(target config.Endpoint, remoteRoot string, keep int) ([]string, error) {
	if keep < 0 {
		keep = 0
	}
	script := fmt.Sprintf(`set -eu
cd %s
if [ -d .routurn/runs ]; then
  for f in .routurn/runs/*/status; do
    [ -f "$f" ] || continue
    d="${f%%/status}"
    id="${d##*/}"
    status="$(cat "$f" 2>/dev/null || printf 'UNKNOWN')"
    printf '%%s\t%%s\n' "$id" "$status"
  done
fi
`, shellQuote(remoteRoot))
	data, err := Capture(target, "sh -lc "+shellQuote(script))
	if err != nil {
		return nil, err
	}
	type entry struct {
		id     string
		status string
	}
	var completed []entry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		id, status, ok := strings.Cut(line, "\t")
		if !ok || validateRunID(id) != nil {
			continue
		}
		status = strings.TrimSpace(status)
		if status == "RUNNING" || status == "STOPPING" {
			continue
		}
		completed = append(completed, entry{id: id, status: status})
	}
	sort.Slice(completed, func(i, j int) bool { return completed[i].id > completed[j].id })
	if len(completed) <= keep {
		return nil, nil
	}
	var ids []string
	for _, item := range completed[keep:] {
		ids = append(ids, item.id)
	}
	if err := removeDetachedRunDirs(target, remoteRoot, ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func removeDetachedRunDirs(target config.Endpoint, remoteRoot string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		if err := validateRunID(id); err != nil {
			return err
		}
	}
	script := fmt.Sprintf(`set -eu
cd %s
while IFS= read -r id; do
  case "$id" in ''|*[!A-Za-z0-9._-]*) exit 41 ;; esac
  rm -rf -- ".routurn/runs/$id"
done
`, shellQuote(remoteRoot))
	input := strings.NewReader(strings.Join(ids, "\n") + "\n")
	_, err := RunCommand(target, "sh -lc "+shellQuote(script), input, io.Discard, io.Discard)
	return err
}

func filepathSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}
