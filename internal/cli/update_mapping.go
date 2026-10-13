package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/selimserbes/routurn/internal/bundle"
	"github.com/spf13/cobra"
)

// archiveMapping is determined before importing or mutating an update. Only a
// single, evidence-backed match can be selected automatically; never infer a
// project root from archive name or directory depth alone.
type archiveMapping struct {
	Strip   int
	Matches int
	Total   int
	Prefix  string
}

// commonArchiveDirectories returns the chain of folders wrapping EVERY file.
// This deliberately cannot strip an internal source directory when an archive
// includes files outside that directory.
func commonArchiveDirectories(entries []bundle.InspectionEntry) []string {
	if len(entries) == 0 {
		return nil
	}
	first := strings.Split(entries[0].Path, "/")
	prefix := append([]string(nil), first[:len(first)-1]...)
	for _, entry := range entries[1:] {
		parts := strings.Split(entry.Path, "/")
		parts = parts[:len(parts)-1]
		i := 0
		for i < len(prefix) && i < len(parts) && prefix[i] == parts[i] {
			i++
		}
		prefix = prefix[:i]
	}
	return prefix
}

func projectRootAnchor(path string) bool {
	switch strings.ToLower(path) {
	case "readme.md", "readme.rst", "readme.txt", "pyproject.toml", "go.mod", "package.json", "cargo.toml", "pom.xml", "build.gradle", "build.gradle.kts", "cmakelists.txt", "makefile", "setup.py", "requirements.txt":
		return true
	default:
		return false
	}
}

func evaluateArchiveMapping(root string, entries []bundle.InspectionEntry, strip int, prefix []string) archiveMapping {
	m := archiveMapping{Strip: strip, Total: len(entries), Prefix: strings.Join(prefix[:strip], "/")}
	anchors := 0
	for _, entry := range entries {
		parts := strings.Split(entry.Path, "/")
		if len(parts) <= strip {
			continue
		}
		rel := strings.Join(parts[strip:], "/")
		// These are evidence probes, never writes; even a malicious archive
		// must not escape the project root or follow symlinked ancestors.
		if rel == "" || filepath.IsAbs(rel) || strings.HasPrefix(rel, "../") {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		if !mappingPathIsRegular(root, path) {
			continue
		}
		m.Matches++
		if projectRootAnchor(rel) {
			anchors++
		}
	}
	// Require at least two independent real files and a recognizable root
	// marker, or three files without a marker. One README never proves a root.
	if m.Matches < 2 || (anchors == 0 && m.Matches < 3) {
		m.Matches = 0 // insufficient evidence
	}
	return m
}

func mappingPathIsRegular(root, path string) bool {
	abs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(abs, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	cur := abs
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

func mappingCandidates(root, archive string) ([]archiveMapping, bool, error) {
	inspection, err := bundle.InspectArchive(archive, 0)
	if err != nil {
		return nil, false, err
	}
	// Manifests carry explicit bundle identity and path semantics. Do not
	// second-guess them with legacy filename heuristics.
	if inspection.Manifest != nil || len(inspection.Entries) == 0 {
		return nil, false, nil
	}
	prefix := commonArchiveDirectories(inspection.Entries)
	if len(prefix) == 0 {
		return nil, false, nil
	}
	candidates := make([]archiveMapping, 0, len(prefix)+1)
	for strip := 0; strip <= len(prefix); strip++ {
		if strip > 0 {
			// Stripping can expose protected paths or path collisions that were
			// hidden behind a wrapper; exclude those choices entirely.
			if _, err := bundle.InspectArchive(archive, strip); err != nil {
				continue
			}
		}
		candidates = append(candidates, evaluateArchiveMapping(root, inspection.Entries, strip, prefix))
	}
	return candidates, true, nil
}

// resolveArchiveMapping selects a path mapping only when evidence is unique.
// With uncertain input, a real terminal gets a simple choice; automation must
// explicitly pass --strip-components N rather than silently guess.
func resolveArchiveMapping(cmd *cobra.Command, root, archive string, requested int, explicit bool) (int, error) {
	if explicit {
		return requested, nil
	}
	candidates, wrapped, err := mappingCandidates(root, archive)
	if err != nil {
		return 0, err
	}
	if !wrapped {
		return requested, nil
	}
	positive := make([]archiveMapping, 0)
	for _, candidate := range candidates {
		if candidate.Matches > 0 {
			positive = append(positive, candidate)
		}
	}
	sort.Slice(positive, func(i, j int) bool {
		if positive[i].Matches == positive[j].Matches {
			return positive[i].Strip < positive[j].Strip
		}
		return positive[i].Matches > positive[j].Matches
	})
	if len(positive) > 0 {
		best := positive[0]
		second := 0
		if len(positive) > 1 {
			second = positive[1].Matches
		}
		// Reject close scores; e.g. 3 vs 2 local matches is not proof.
		minGap := 2
		if best.Matches/10 > minGap {
			minGap = best.Matches / 10
		}
		if best.Matches-second >= minGap {
			if best.Strip > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ Project root detected · %d wrapper folder(s) skipped\n", best.Strip)
			}
			return best.Strip, nil
		}
	}

	if pickerIsInteractiveTerminal(cmd) {
		// Show only viable mappings plus the default (which may be an
		// intentional new nested directory). Confirmation of the update plan
		// still happens separately after this choice.
		menu := interactiveMenu{Heading: "Choose archive root", ConfirmHint: "Review this mapping"}
		options := make([]archiveMapping, 0, len(candidates))
		for _, candidate := range candidates {
			label := "Keep archive folders (strip 0)"
			if candidate.Strip > 0 {
				label = fmt.Sprintf("Skip %d folder(s): %s/", candidate.Strip, candidate.Prefix)
			}
			menu.Items = append(menu.Items, interactiveMenuItem{Label: label, Detail: fmt.Sprintf("%d existing file match(es)", candidate.Matches)})
			options = append(options, candidate)
		}
		if len(options) > 0 {
			sel, err := chooseInteractiveMenu(cmd, "Update mapping", menu)
			if err != nil {
				return 0, err
			}
			if sel.Index >= 0 && sel.Index < len(options) {
				choice := options[sel.Index]
				fmt.Fprintf(cmd.OutOrStdout(), "Mapping selected: strip %d folder(s)\n", choice.Strip)
				return choice.Strip, nil
			}
		}
		return 0, fmt.Errorf("update mapping cancelled; source archive was not imported or removed")
	}
	return 0, fmt.Errorf("cannot safely identify project root inside archive; review with 'routurn update' in a terminal, or specify --strip-components N explicitly (0 keeps archive folders)")
}
