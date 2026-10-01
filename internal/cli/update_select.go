package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/selimserbes/routurn/internal/bundle"
	"github.com/selimserbes/routurn/internal/config"
	"github.com/selimserbes/routurn/internal/intake"
	"github.com/selimserbes/routurn/internal/syncer"
	"github.com/spf13/cobra"
)

type updateChoice struct {
	Path       string
	Inspection bundle.Inspection
	Hash       string
	ModTime    time.Time
}

func resolveUpdateInput(cmd *cobra.Command, ctx *projectContext, value string, strip int) (string, error) {
	switch strings.TrimSpace(value) {
	case "", "select":
		return chooseUpdateInteractive(cmd, ctx, strip)
	case "recent":
		return chooseRecentUpdate(cmd, ctx, strip)
	case "latest":
		managed, err := intake.LatestForProject(ctx.Resolved.Config.Name)
		if err != nil {
			return "", err
		}
		return managed.Path, nil
	default:
		return value, nil
	}
}

func chooseRecentUpdate(cmd *cobra.Command, ctx *projectContext, strip int) (string, error) {
	choices, err := recentCompatibleUpdates(ctx, strip)
	if err != nil {
		return "", err
	}
	if len(choices) == 0 {
		return "", fmt.Errorf("no recent compatible Routurn updates found; use 'routurn update' to browse or provide a path")
	}
	if len(choices) == 1 {
		fmt.Fprintf(cmd.OutOrStdout(), "Selected recent update: %s\n", choices[0].Path)
		return choices[0].Path, nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Multiple compatible updates found; choose one:")
	for i, choice := range choices {
		name := displayBundleName(choice.Inspection, filepath.Base(choice.Path))
		fmt.Fprintf(cmd.OutOrStdout(), "  %d) %-28s %s\n", i+1, name, choice.Path)
	}
	return promptChoice(cmd.InOrStdin(), cmd.OutOrStdout(), choices)
}

func chooseUpdateInteractive(cmd *cobra.Command, ctx *projectContext, strip int) (string, error) {
	choices, _ := recentCompatibleUpdates(ctx, strip)
	fmt.Fprintf(cmd.OutOrStdout(), "Routurn · %s\n\n", ctx.Resolved.Config.Name)
	fmt.Fprintln(cmd.OutOrStdout(), "Choose update source")
	idx := 1
	for _, choice := range choices {
		name := displayBundleName(choice.Inspection, filepath.Base(choice.Path))
		fmt.Fprintf(cmd.OutOrStdout(), "  %d) Recent: %-24s %s\n", idx, name, choice.Path)
		idx++
	}
	browseIndex := idx
	fmt.Fprintf(cmd.OutOrStdout(), "  %d) Browse files\n", browseIndex)
	idx++
	pathIndex := idx
	fmt.Fprintf(cmd.OutOrStdout(), "  %d) Enter a path\n", pathIndex)
	idx++
	managedIndex := idx
	fmt.Fprintf(cmd.OutOrStdout(), "  %d) Use latest managed update\n", managedIndex)
	fmt.Fprintln(cmd.OutOrStdout(), "  q) Cancel")

	reader := bufio.NewReader(cmd.InOrStdin())
	for {
		fmt.Fprint(cmd.OutOrStdout(), "> ")
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			if err == io.EOF {
				return "", fmt.Errorf("interactive update selection needs terminal input; provide an update path")
			}
			return "", err
		}
		line = strings.TrimSpace(line)
		if strings.EqualFold(line, "q") || strings.EqualFold(line, "quit") {
			return "", fmt.Errorf("update selection cancelled")
		}
		n, convErr := strconv.Atoi(line)
		if convErr != nil {
			fmt.Fprintln(cmd.OutOrStdout(), "Enter a menu number or q.")
			continue
		}
		if n >= 1 && n <= len(choices) {
			return choices[n-1].Path, nil
		}
		switch n {
		case browseIndex:
			return browseForArchive(cmd, ctx)
		case pathIndex:
			fmt.Fprint(cmd.OutOrStdout(), "Path: ")
			path, readErr := reader.ReadString('\n')
			if readErr != nil && len(path) == 0 {
				return "", readErr
			}
			return expandUserPath(strings.TrimSpace(path)), nil
		case managedIndex:
			managed, managedErr := intake.LatestForProject(ctx.Resolved.Config.Name)
			if managedErr != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "No managed update available: %v\n", managedErr)
				continue
			}
			return managed.Path, nil
		default:
			fmt.Fprintln(cmd.OutOrStdout(), "Unknown selection.")
		}
	}
}

func browseForArchive(cmd *cobra.Command, ctx *projectContext) (string, error) {
	start := initialBrowseDir(ctx.Global)
	reader := bufio.NewReader(cmd.InOrStdin())
	current := start
	for {
		entries, err := os.ReadDir(current)
		if err != nil {
			return "", err
		}
		type item struct {
			name string
			path string
			dir  bool
		}
		var items []item
		for _, entry := range entries {
			path := filepath.Join(current, entry.Name())
			if entry.IsDir() {
				items = append(items, item{name: entry.Name() + string(filepath.Separator), path: path, dir: true})
				continue
			}
			if supportedArchive(path) {
				items = append(items, item{name: entry.Name(), path: path})
			}
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].dir != items[j].dir {
				return items[i].dir
			}
			return strings.ToLower(items[i].name) < strings.ToLower(items[j].name)
		})

		fmt.Fprintf(cmd.OutOrStdout(), "\nLocation: %s\n", current)
		fmt.Fprintln(cmd.OutOrStdout(), "  0) ../")
		for i, item := range items {
			fmt.Fprintf(cmd.OutOrStdout(), "  %d) %s\n", i+1, item.name)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "  p) Enter a path")
		fmt.Fprintln(cmd.OutOrStdout(), "  q) Cancel")
		fmt.Fprint(cmd.OutOrStdout(), "> ")
		line, readErr := reader.ReadString('\n')
		if readErr != nil && len(line) == 0 {
			return "", readErr
		}
		line = strings.TrimSpace(line)
		switch strings.ToLower(line) {
		case "q", "quit":
			return "", fmt.Errorf("update selection cancelled")
		case "p":
			fmt.Fprint(cmd.OutOrStdout(), "Path: ")
			path, err := reader.ReadString('\n')
			if err != nil && len(path) == 0 {
				return "", err
			}
			path = expandUserPath(strings.TrimSpace(path))
			info, err := os.Stat(path)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Cannot open path: %v\n", err)
				continue
			}
			if info.IsDir() {
				current, _ = filepath.Abs(path)
				continue
			}
			if !supportedArchive(path) {
				fmt.Fprintln(cmd.OutOrStdout(), "That file is not a supported update archive.")
				continue
			}
			rememberUpdateDir(ctx.Global, filepath.Dir(path))
			return path, nil
		}
		n, err := strconv.Atoi(line)
		if err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), "Enter a number, p, or q.")
			continue
		}
		if n == 0 {
			parent := filepath.Dir(current)
			if parent != current {
				current = parent
			}
			continue
		}
		if n < 1 || n > len(items) {
			fmt.Fprintln(cmd.OutOrStdout(), "Unknown selection.")
			continue
		}
		selected := items[n-1]
		if selected.dir {
			current = selected.path
			continue
		}
		rememberUpdateDir(ctx.Global, filepath.Dir(selected.path))
		return selected.path, nil
	}
}

func recentCompatibleUpdates(ctx *projectContext, strip int) ([]updateChoice, error) {
	dirs := candidateUpdateDirs(ctx.Global)
	type candidate struct {
		path string
		mod  time.Time
	}
	seenPath := map[string]struct{}{}
	var candidates []candidate
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			if !supportedArchive(path) {
				continue
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				continue
			}
			if _, ok := seenPath[abs]; ok {
				continue
			}
			seenPath[abs] = struct{}{}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			candidates = append(candidates, candidate{path: abs, mod: info.ModTime()})
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].mod.After(candidates[j].mod) })
	if len(candidates) > 30 {
		candidates = candidates[:30]
	}

	seenHash := map[string]struct{}{}
	var out []updateChoice
	for _, candidate := range candidates {
		inspection, err := bundle.InspectArchive(candidate.path, strip)
		if err != nil || inspection.Manifest == nil {
			continue
		}
		fingerprint, fpErr := currentProjectFingerprint(ctx, candidate.path)
		if fpErr != nil || !manifestCompatible(inspection.Manifest, ctx.Resolved.Config.Name, fingerprint) {
			continue
		}
		hash, err := intake.HashFile(candidate.path)
		if err != nil {
			continue
		}
		if _, ok := seenHash[hash]; ok {
			continue
		}
		seenHash[hash] = struct{}{}
		out = append(out, updateChoice{Path: candidate.path, Inspection: inspection, Hash: hash, ModTime: candidate.mod})
	}
	return out, nil
}

func validateUpdateCompatibility(ctx *projectContext, archive string, strip int) (*bundle.Manifest, error) {
	manifest, err := bundle.ManifestFromArchive(archive, strip)
	if err != nil {
		return nil, err
	}
	if manifest == nil {
		return nil, nil
	}
	fingerprint, err := currentProjectFingerprint(ctx, archive)
	if err != nil {
		return nil, err
	}
	if manifest.Project.Name != "" && manifest.Project.Name != ctx.Resolved.Config.Name {
		return nil, fmt.Errorf("update belongs to project %q; current project is %q", manifest.Project.Name, ctx.Resolved.Config.Name)
	}
	if manifest.Base.Fingerprint != "" && manifest.Base.Fingerprint != fingerprint {
		return nil, fmt.Errorf("update was created for a different project state\nexpected: %s\ncurrent:  %s", manifest.Base.Fingerprint, fingerprint)
	}
	return manifest, nil
}

func currentProjectFingerprint(ctx *projectContext, ignorePath string) (string, error) {
	scan, err := syncer.Scan(ctx.Resolved.Root, ctx.Resolved.Config.Sync.Exclude)
	if err != nil {
		return "", fmt.Errorf("fingerprint project: %w", err)
	}
	if ignorePath != "" {
		rootAbs, _ := filepath.Abs(ctx.Resolved.Root)
		ignoreAbs, _ := filepath.Abs(ignorePath)
		if rel, relErr := filepath.Rel(rootAbs, ignoreAbs); relErr == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			rel = filepath.ToSlash(rel)
			filtered := scan.Files[:0]
			for _, file := range scan.Files {
				if file.Path != rel {
					filtered = append(filtered, file)
				}
			}
			scan.Files = filtered
		}
	}
	return syncer.Fingerprint(scan), nil
}

func manifestCompatible(manifest *bundle.Manifest, project, fingerprint string) bool {
	if manifest == nil || manifest.Project.Name == "" || manifest.Project.Name != project {
		return false
	}
	// Automatic discovery is deliberately stricter than manual selection.
	// A base fingerprint is required so Routurn never guesses that an older
	// project-specific download is safe merely because its filename is recent.
	return manifest.Base.Fingerprint != "" && manifest.Base.Fingerprint == fingerprint
}

func candidateUpdateDirs(global *config.GlobalConfig) []string {
	seen := map[string]struct{}{}
	var dirs []string
	add := func(path string) {
		path = expandUserPath(strings.TrimSpace(path))
		if path == "" {
			return
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return
		}
		if _, ok := seen[abs]; ok {
			return
		}
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			return
		}
		seen[abs] = struct{}{}
		dirs = append(dirs, abs)
	}
	if global != nil {
		add(global.UI.LastUpdateDir)
	}
	if cwd, err := os.Getwd(); err == nil {
		add(cwd)
	}
	for _, path := range xdgUserDirs() {
		add(path)
	}
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, "Downloads"))
		add(filepath.Join(home, "Desktop"))
		add(filepath.Join(home, "Documents"))
		add(home)
	}
	add(os.TempDir())
	return dirs
}

func initialBrowseDir(global *config.GlobalConfig) string {
	if global != nil && global.UI.LastUpdateDir != "" {
		if info, err := os.Stat(global.UI.LastUpdateDir); err == nil && info.IsDir() {
			return global.UI.LastUpdateDir
		}
	}
	for _, dir := range xdgUserDirs() {
		if strings.Contains(strings.ToLower(filepath.Base(dir)), "download") {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				return dir
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		downloads := filepath.Join(home, "Downloads")
		if info, err := os.Stat(downloads); err == nil && info.IsDir() {
			return downloads
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	home, _ := os.UserHomeDir()
	return home
}

func rememberUpdateDir(global *config.GlobalConfig, dir string) {
	if global == nil || dir == "" {
		return
	}
	global.UI.LastUpdateDir = dir
	_ = config.SaveGlobal(global)
}

func xdgUserDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(home, ".config", "user-dirs.dirs"))
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "XDG_") || !strings.Contains(line, "_DIR=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"")
		value = strings.ReplaceAll(value, "$HOME", home)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func supportedArchive(path string) bool {
	lower := strings.ToLower(path)
	for _, suffix := range []string{".tar.gz", ".tgz", ".zip", ".tar", ".7z", ".rar"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func displayBundleName(inspection bundle.Inspection, fallback string) string {
	if inspection.Manifest != nil && inspection.Manifest.Bundle.Name != "" {
		return inspection.Manifest.Bundle.Name
	}
	return fallback
}

func promptChoice(in io.Reader, out io.Writer, choices []updateChoice) (string, error) {
	reader := bufio.NewReader(in)
	for {
		fmt.Fprint(out, "> ")
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return "", err
		}
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || n < 1 || n > len(choices) {
			fmt.Fprintln(out, "Enter a valid number.")
			continue
		}
		return choices[n-1].Path, nil
	}
}

func expandUserPath(path string) string {
	path = strings.Trim(path, "\"'")
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func rememberArchiveDir(global *config.GlobalConfig, archive string) {
	abs, err := filepath.Abs(archive)
	if err != nil {
		return
	}
	if dataDir, err := intake.DataDir(); err == nil {
		dataAbs, _ := filepath.Abs(dataDir)
		if abs == dataAbs || strings.HasPrefix(abs, dataAbs+string(filepath.Separator)) {
			return
		}
	}
	rememberUpdateDir(global, filepath.Dir(abs))
}
