package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
)

const MissingBaseState = "missing"

// CheckBaseFiles verifies that every payload path has an explicit base-state
// precondition and that the current local file state matches it. Expected
// values are either "missing" or "sha256:<hex>".
func CheckBaseFiles(root string, payloadPaths []string, expected map[string]string) (bool, []string, error) {
	if len(expected) == 0 {
		return false, nil, nil
	}

	payload := make(map[string]struct{}, len(payloadPaths))
	for _, path := range payloadPaths {
		payload[path] = struct{}{}
	}

	var mismatches []string
	for path := range payload {
		want, ok := expected[path]
		if !ok {
			mismatches = append(mismatches, fmt.Sprintf("%s: missing base-file precondition", path))
			continue
		}
		if !validBaseState(want) {
			return false, nil, fmt.Errorf("invalid base-file precondition for %s: %q", path, want)
		}
		got, err := localBaseState(root, path)
		if err != nil {
			return false, nil, err
		}
		if got != want {
			mismatches = append(mismatches, fmt.Sprintf("%s: expected %s, current %s", path, want, got))
		}
	}

	for path, want := range expected {
		if _, ok := payload[path]; ok {
			continue
		}
		if !validBaseState(want) {
			return false, nil, fmt.Errorf("invalid base-file precondition for %s: %q", path, want)
		}
		mismatches = append(mismatches, fmt.Sprintf("%s: base-file precondition has no matching payload entry", path))
	}

	sort.Strings(mismatches)
	return len(mismatches) == 0, mismatches, nil
}

func validBaseState(value string) bool {
	if value == MissingBaseState {
		return true
	}
	if !strings.HasPrefix(value, "sha256:") {
		return false
	}
	hexPart := strings.TrimPrefix(value, "sha256:")
	if len(hexPart) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(hexPart)
	return err == nil
}

func localBaseState(root, rel string) (string, error) {
	target, err := safeLocalTarget(root, rel)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		return MissingBaseState, nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect local %s: %w", rel, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("refusing base-file precondition through symlink: %s", rel)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("base-file precondition target is not a regular file: %s", rel)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return "", fmt.Errorf("read local %s: %w", rel, err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
