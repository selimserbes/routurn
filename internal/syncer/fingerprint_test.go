package syncer

import "testing"

func TestFingerprintStableForOrderedScan(t *testing.T) {
	scan := &ScanResult{Files: []FileEntry{
		{Path: "a.txt", Hash: "aaa"},
		{Path: "b.txt", Hash: "bbb"},
	}}
	got := Fingerprint(scan)
	if got == "" || got[:7] != "sha256:" {
		t.Fatalf("unexpected fingerprint: %q", got)
	}
	if Fingerprint(scan) != got {
		t.Fatal("fingerprint changed for identical scan")
	}
}
