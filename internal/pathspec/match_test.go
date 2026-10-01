package pathspec

import "testing"

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{".git/**", ".git", true},
		{".git/**", ".git/config", true},
		{"**/__pycache__/**", "pkg/__pycache__", true},
		{"**/__pycache__/**", "pkg/__pycache__/x.pyc", true},
		{"outputs/**", "outputs/a/b/result.zip", true},
		{"outputs/*.zip", "outputs/result.zip", true},
		{"outputs/*.zip", "outputs/a/result.zip", false},
		{"*.go", "main.go", true},
		{"*.go", "cmd/main.go", false},
	}
	for _, tt := range tests {
		if got := Match(tt.pattern, tt.value); got != tt.want {
			t.Fatalf("Match(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
		}
	}
}
