package cli

import (
	"testing"

	"github.com/selimserbes/routurn/internal/bundle"
)

func TestClassifyUpdateCompatibility(t *testing.T) {
	const project = "example-project"
	const current = "sha256:current"

	tests := []struct {
		name          string
		manifest      *bundle.Manifest
		scopedPresent bool
		scopedMatch   bool
		want          updateCompatibility
	}{
		{
			name: "exact base",
			manifest: &bundle.Manifest{
				Project: bundle.ManifestProject{Name: project},
				Base:    bundle.ManifestBase{Fingerprint: current},
			},
			want: updateCompatibilityExact,
		},
		{
			name: "scoped files match despite global drift",
			manifest: &bundle.Manifest{
				Project: bundle.ManifestProject{Name: project},
				Base:    bundle.ManifestBase{Fingerprint: "sha256:other"},
			},
			scopedPresent: true,
			scopedMatch:   true,
			want:          updateCompatibilityScoped,
		},
		{
			name: "state differs",
			manifest: &bundle.Manifest{
				Project: bundle.ManifestProject{Name: project},
				Base:    bundle.ManifestBase{Fingerprint: "sha256:other"},
			},
			want: updateCompatibilityStateDiffers,
		},
		{
			name: "unverified base",
			manifest: &bundle.Manifest{
				Project: bundle.ManifestProject{Name: project},
			},
			want: updateCompatibilityUnverified,
		},
		{
			name: "different project",
			manifest: &bundle.Manifest{
				Project: bundle.ManifestProject{Name: "other-project"},
				Base:    bundle.ManifestBase{Fingerprint: current},
			},
			want: updateCompatibilityUnknown,
		},
		{
			name: "missing manifest",
			want: updateCompatibilityUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyUpdateCompatibility(tt.manifest, project, current, tt.scopedPresent, tt.scopedMatch); got != tt.want {
				t.Fatalf("classifyUpdateCompatibility() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateCompatibilityLabel(t *testing.T) {
	tests := map[updateCompatibility]string{
		updateCompatibilityExact:        "[compatible]",
		updateCompatibilityScoped:       "[scoped ok]",
		updateCompatibilityStateDiffers: "[state differs]",
		updateCompatibilityUnverified:   "[unverified]",
		updateCompatibilityUnknown:      "[unknown]",
	}
	for compatibility, want := range tests {
		if got := updateCompatibilityLabel(compatibility); got != want {
			t.Fatalf("updateCompatibilityLabel(%v) = %q, want %q", compatibility, got, want)
		}
	}
}
