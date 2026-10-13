package bundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeOrderTestRecord(t *testing.T, root string, record Record) {
	t.Helper()
	dir := filepath.Join(root, ".routurn", "updates", record.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeRecord(dir, record); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateHistoryUsesAppliedTimeNotRandomIDSuffix(t *testing.T) {
	root := t.TempDir()
	older := Record{ID: "20261013T193138Z-ffffff", AppliedAt: "2026-10-13T19:31:38.100Z"}
	newer := Record{ID: "20261013T193138Z-000000", AppliedAt: "2026-10-13T19:31:38.200Z"}
	writeOrderTestRecord(t, root, older)
	writeOrderTestRecord(t, root, newer)
	records, err := List(root)
	if err != nil || len(records) != 2 || records[0].ID != newer.ID {
		t.Fatalf("newest record selection: %+v, %v", records, err)
	}
	chosen, err := ResolveID(root, "previous")
	if err != nil || chosen != newer.ID {
		t.Fatalf("previous should point to newer update: %s, %v", chosen, err)
	}
	newer.RolledBack = true
	writeOrderTestRecord(t, root, newer)
	chosen, err = ResolveID(root, "latest")
	if err != nil || chosen != older.ID {
		t.Fatalf("latest non-rolled-back update: %s, %v", chosen, err)
	}
}

func TestUpdateHistoryFailsClosedOnAmbiguousLegacySeconds(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"20261013T193138Z-aabbcc", "20261013T193138Z-ddeeff"} {
		writeOrderTestRecord(t, root, Record{ID: id, AppliedAt: "2026-10-13T19:31:38Z"})
	}
	if _, err := ResolveID(root, "previous"); err == nil || !strings.Contains(err.Error(), "specify an update ID") {
		t.Fatalf("expected explicit ID on ambiguous legacy timestamps: %v", err)
	}
}

func TestUpdateHistoryMixedOldAndNewTimestampFormats(t *testing.T) {
	root := t.TempDir()
	writeOrderTestRecord(t, root, Record{ID: "20261013T193138Z-ffffff", AppliedAt: "2026-10-13T19:31:38Z"})
	writeOrderTestRecord(t, root, Record{ID: "20261013T193138Z-000000", AppliedAt: "2026-10-13T19:31:38.1Z"})
	id, err := ResolveID(root, "previous")
	if err != nil || id != "20261013T193138Z-000000" {
		t.Fatalf("mixed timestamp formats: %q, %v", id, err)
	}
}

func TestNewAppliedTimeHasSubsecondPrecision(t *testing.T) {
	seen := false
	for i := 0; i < 5; i++ {
		stamp := nowRFC3339()
		ts, err := time.Parse(time.RFC3339Nano, stamp)
		if err != nil {
			t.Fatal(err)
		}
		if ts.Nanosecond() != 0 {
			seen = true
			break
		}
	}
	if !seen {
		t.Fatal("applied time loses subsecond precision")
	}
}
