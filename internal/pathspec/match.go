package pathspec

import (
	"path"
	"strings"
)

// Match matches slash-separated paths with shell-style segment wildcards.
// A segment equal to ** matches zero or more path segments.
func Match(pattern, value string) bool {
	pattern = clean(pattern)
	value = clean(value)
	if pattern == "" {
		return value == ""
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(value, "/"))
}

func matchSegments(pattern, value []string) bool {
	if len(pattern) == 0 {
		return len(value) == 0
	}
	if pattern[0] == "**" {
		if matchSegments(pattern[1:], value) {
			return true
		}
		return len(value) > 0 && matchSegments(pattern, value[1:])
	}
	if len(value) == 0 {
		return false
	}
	ok, err := path.Match(pattern[0], value[0])
	if err != nil || !ok {
		return false
	}
	return matchSegments(pattern[1:], value[1:])
}

func clean(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\\", "/"))
	s = strings.TrimPrefix(s, "./")
	s = strings.Trim(s, "/")
	return s
}
