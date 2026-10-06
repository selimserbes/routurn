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
			name: "state classification ignores project display name",
			manifest: &bundle.Manifest{
				Project: bundle.ManifestProject{Name: "other-project"},
				Base:    bundle.ManifestBase{Fingerprint: current},
			},
			want: updateCompatibilityExact,
		},
		{
			name: "missing manifest",
			want: updateCompatibilityUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyUpdateCompatibility(tt.manifest, current, tt.scopedPresent, tt.scopedMatch); got != tt.want {
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

func TestClassifyUpdateIdentity(t *testing.T) {
	const project = "example-project"
	tests := []struct {
		name     string
		manifest *bundle.Manifest
		want     updateIdentity
	}{
		{name: "matching project", manifest: &bundle.Manifest{Project: bundle.ManifestProject{Name: project}}, want: updateIdentityMatch},
		{name: "different project", manifest: &bundle.Manifest{Project: bundle.ManifestProject{Name: "typo-project"}}, want: updateIdentityDiffers},
		{name: "missing project name", manifest: &bundle.Manifest{}, want: updateIdentityUnknown},
		{name: "missing manifest", want: updateIdentityUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyUpdateIdentity(tt.manifest, project); got != tt.want {
				t.Fatalf("classifyUpdateIdentity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAutomaticRecentRequiresMatchingIdentityAndVerifiedState(t *testing.T) {
	tests := []struct {
		name   string
		choice updateChoice
		want   bool
	}{
		{name: "exact matching project", choice: updateChoice{Identity: updateIdentityMatch, Compatibility: updateCompatibilityExact}, want: true},
		{name: "scoped matching project", choice: updateChoice{Identity: updateIdentityMatch, Compatibility: updateCompatibilityScoped}, want: true},
		{name: "wrong project name even with scoped state", choice: updateChoice{Identity: updateIdentityDiffers, Compatibility: updateCompatibilityScoped}, want: false},
		{name: "matching project but unverified", choice: updateChoice{Identity: updateIdentityMatch, Compatibility: updateCompatibilityUnverified}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAutomaticRecentCandidate(tt.choice); got != tt.want {
				t.Fatalf("isAutomaticRecentCandidate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateChoiceLabelsIncludeIdentityMismatch(t *testing.T) {
	choice := updateChoice{Identity: updateIdentityDiffers, Compatibility: updateCompatibilityScoped}
	if got, want := updateChoiceLabels(choice), "[scoped ok] [project name differs]"; got != want {
		t.Fatalf("updateChoiceLabels() = %q, want %q", got, want)
	}
}
