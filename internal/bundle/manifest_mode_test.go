package bundle

import "testing"

func TestManifestUpdateModeParsing(t *testing.T) {
	cases := []struct {
		name, mode, want string
		bad              bool
	}{
		{"legacy", "", "", false},
		{"partial", "partial", "partial", false},
		{"full", "full", "full", false},
		{"normalized", " Full ", "full", false},
		{"unknown", "guess", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := "schema = 1\n[bundle]\nname = \"example\"\n"
			if tc.mode != "" {
				text += "mode = \"" + tc.mode + "\"\n"
			}
			manifest, _, err := SplitManifest([]Entry{{Path: ManifestPath, Data: []byte(text)}})
			if tc.bad {
				if err == nil {
					t.Fatal("unsupported manifest mode accepted")
				}
				return
			}
			if err != nil || manifest == nil || manifest.Bundle.Mode != tc.want {
				t.Fatalf("parsed mode = %#v, error = %v, expected %q", manifest, err, tc.want)
			}
		})
	}
}
