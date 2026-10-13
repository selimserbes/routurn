package remote

import "testing"

func TestParseDetachedStatus(t *testing.T) {
	data := []byte("status=SUCCEEDED\npid=123\nalive=false\nmode=group\nexit_code=0\nstarted_at=2026-10-01T10:00:00Z\nfinished_at=2026-10-01T10:05:00Z\n")
	got, err := parseDetachedStatus(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "SUCCEEDED" || got.PID != 123 || got.Alive || got.ExitCode == nil || *got.ExitCode != 0 || got.Mode != "group" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestValidateRunID(t *testing.T) {
	good := []string{"20261001T100000Z-acde12", "latest-ish", "run_1", "a.b"}
	for _, id := range good {
		if err := validateRunID(id); err != nil {
			t.Fatalf("expected valid run id %q: %v", id, err)
		}
	}
	bad := []string{"", "../x", "a/b", "run id", "/tmp/x"}
	for _, id := range bad {
		if err := validateRunID(id); err == nil {
			t.Fatalf("expected invalid run id %q", id)
		}
	}
}
