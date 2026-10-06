package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/selimserbes/routurn/internal/config"
)

type runnableCommand struct {
	Name         string
	Command      string
	Source       string
	Description  string
	Saved        bool
	ProjectPath  string
	Hidden       bool
	HiddenReason string
}

func discoverRunnableCommands(root string, tasks map[string]config.Task) []runnableCommand {
	var out []runnableCommand
	names := make([]string, 0, len(tasks))
	for name := range tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		task := tasks[name]
		out = append(out, runnableCommand{
			Name:        name,
			Command:     task.Command,
			Source:      "Saved",
			Description: "saved Routurn task",
			Saved:       true,
		})
	}

	out = append(out, discoverNodeCommands(root)...)
	out = append(out, discoverGoCommands(root)...)
	out = append(out, discoverRustCommands(root)...)
	out = append(out, discoverPythonCommands(root)...)
	out = append(out, discoverMakeCommands(root)...)
	out = append(out, discoverDockerCommands(root)...)
	out = append(out, discoverExecutableCommands(root)...)
	out = dedupeRunnableCommands(out)
	return annotateRunnableVisibility(out)
}

func dedupeRunnableCommands(in []runnableCommand) []runnableCommand {
	seenCommand := map[string]bool{}
	seenName := map[string]bool{}
	out := make([]runnableCommand, 0, len(in))
	for _, item := range in {
		command := strings.TrimSpace(item.Command)
		if command == "" || seenCommand[command] {
			continue
		}
		seenCommand[command] = true
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = commandLabel(command)
		}
		if seenName[name] {
			prefix := strings.ToLower(strings.ReplaceAll(item.Source, " ", "-"))
			if prefix == "" {
				prefix = "command"
			}
			name = prefix + "-" + name
			for seenName[name] {
				name += "-alt"
			}
		}
		item.Name = name
		seenName[name] = true
		out = append(out, item)
	}
	return out
}

func annotateRunnableVisibility(items []runnableCommand) []runnableCommand {
	saved := make([]runnableCommand, 0)
	for _, item := range items {
		if item.Saved {
			saved = append(saved, item)
		}
	}

	for i := range items {
		item := &items[i]
		if item.Saved {
			continue
		}
		if isMaintenanceHelper(*item) {
			item.Hidden = true
			item.HiddenReason = "maintenance helper"
			continue
		}
		if item.ProjectPath == "" {
			continue
		}
		for _, task := range saved {
			if savedCommandReferencesPath(task.Command, item.ProjectPath) {
				item.Hidden = true
				item.HiddenReason = "already available as saved task " + task.Name
				break
			}
		}
	}
	return items
}

func defaultRunnableCommands(items []runnableCommand) []runnableCommand {
	out := make([]runnableCommand, 0, len(items))
	for _, item := range items {
		if !item.Hidden {
			out = append(out, item)
		}
	}
	return out
}

func hasHiddenRunnableCommands(items []runnableCommand) bool {
	for _, item := range items {
		if item.Hidden {
			return true
		}
	}
	return false
}

func savedCommandReferencesPath(command, projectPath string) bool {
	rel := strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(projectPath)), "./")
	if rel == "" {
		return false
	}
	command = filepath.ToSlash(command)
	return strings.Contains(command, rel)
}

func isMaintenanceHelper(item runnableCommand) bool {
	name := strings.ToLower(strings.TrimSpace(item.Name))
	if item.ProjectPath != "" {
		base := strings.TrimSuffix(filepath.Base(item.ProjectPath), filepath.Ext(item.ProjectPath))
		name = strings.ToLower(base)
	}
	name = strings.TrimPrefix(name, "run_")
	prefixes := []string{
		"install_", "install-",
		"setup_", "setup-",
		"migrate_", "migrate-",
		"bootstrap_", "bootstrap-",
		"generate_", "generate-",
		"register_", "register-",
		"resolve_", "resolve-",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func discoverNodeCommands(root string) []runnableCommand {
	path := filepath.Join(root, "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &pkg) != nil || len(pkg.Scripts) == 0 {
		return nil
	}
	runner := "npm run"
	switch {
	case fileExists(filepath.Join(root, "pnpm-lock.yaml")):
		runner = "pnpm run"
	case fileExists(filepath.Join(root, "yarn.lock")):
		runner = "yarn run"
	case fileExists(filepath.Join(root, "bun.lock")) || fileExists(filepath.Join(root, "bun.lockb")):
		runner = "bun run"
	}
	names := make([]string, 0, len(pkg.Scripts))
	for name := range pkg.Scripts {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]runnableCommand, 0, len(names))
	for _, name := range names {
		out = append(out, runnableCommand{
			Name:        name,
			Command:     runner + " " + shellQuote(name),
			Source:      "Node",
			Description: pkg.Scripts[name],
		})
	}
	return out
}

func discoverGoCommands(root string) []runnableCommand {
	if !fileExists(filepath.Join(root, "go.mod")) {
		return nil
	}
	out := []runnableCommand{
		{Name: "test", Command: "go test ./...", Source: "Go", Description: "test all packages"},
		{Name: "build", Command: "go build ./...", Source: "Go", Description: "build all packages"},
	}
	if hasGoMain(root) {
		out = append([]runnableCommand{{Name: "run", Command: "go run .", Source: "Go", Description: "run root main package"}}, out...)
	}
	cmdRoot := filepath.Join(root, "cmd")
	entries, err := os.ReadDir(cmdRoot)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || !hasGoMain(filepath.Join(cmdRoot, entry.Name())) {
				continue
			}
			out = append(out, runnableCommand{
				Name:        entry.Name(),
				Command:     "go run ./cmd/" + shellQuote(entry.Name()),
				Source:      "Go",
				Description: "run cmd/" + entry.Name(),
			})
		}
	}
	return out
}

func hasGoMain(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err == nil && regexp.MustCompile(`(?m)^\s*package\s+main\s*$`).Match(data) {
			return true
		}
	}
	return false
}

func discoverRustCommands(root string) []runnableCommand {
	path := filepath.Join(root, "Cargo.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cargo struct {
		Bin []struct {
			Name string `toml:"name"`
		} `toml:"bin"`
	}
	_ = toml.Unmarshal(data, &cargo)
	out := []runnableCommand{
		{Name: "test", Command: "cargo test", Source: "Rust", Description: "run Cargo tests"},
		{Name: "build-release", Command: "cargo build --release", Source: "Rust", Description: "release build"},
	}
	if fileExists(filepath.Join(root, "src", "main.rs")) {
		out = append([]runnableCommand{{Name: "run", Command: "cargo run", Source: "Rust", Description: "run default binary"}}, out...)
	}
	for _, bin := range cargo.Bin {
		if strings.TrimSpace(bin.Name) == "" {
			continue
		}
		out = append(out, runnableCommand{
			Name:        bin.Name,
			Command:     "cargo run --bin " + shellQuote(bin.Name),
			Source:      "Rust",
			Description: "run Cargo binary",
		})
	}
	return out
}

func discoverPythonCommands(root string) []runnableCommand {
	var out []runnableCommand
	if fileExists(filepath.Join(root, "pyproject.toml")) {
		data, err := os.ReadFile(filepath.Join(root, "pyproject.toml"))
		if err == nil {
			var pyproject struct {
				Project struct {
					Scripts map[string]string `toml:"scripts"`
				} `toml:"project"`
			}
			if toml.Unmarshal(data, &pyproject) == nil {
				names := make([]string, 0, len(pyproject.Project.Scripts))
				for name := range pyproject.Project.Scripts {
					names = append(names, name)
				}
				sort.Strings(names)
				prefix := ""
				if fileExists(filepath.Join(root, "uv.lock")) {
					prefix = "uv run "
				}
				for _, name := range names {
					out = append(out, runnableCommand{
						Name:        name,
						Command:     prefix + shellQuote(name),
						Source:      "Python",
						Description: pyproject.Project.Scripts[name],
					})
				}
			}
		}
	}
	switch {
	case fileExists(filepath.Join(root, "manage.py")):
		out = append(out, runnableCommand{Name: "django-dev", Command: "python manage.py runserver", Source: "Python", Description: "Django development server"})
	case fileExists(filepath.Join(root, "main.py")):
		out = append(out, runnableCommand{Name: "run", Command: "python main.py", Source: "Python", Description: "run main.py"})
	case fileExists(filepath.Join(root, "app.py")):
		out = append(out, runnableCommand{Name: "run", Command: "python app.py", Source: "Python", Description: "run app.py"})
	}
	if dirExists(filepath.Join(root, "tests")) || fileExists(filepath.Join(root, "pytest.ini")) {
		out = append(out, runnableCommand{Name: "test", Command: "pytest", Source: "Python", Description: "run pytest"})
	}
	return out
}

func discoverMakeCommands(root string) []runnableCommand {
	path := ""
	for _, name := range []string{"Makefile", "makefile", "GNUmakefile"} {
		candidate := filepath.Join(root, name)
		if fileExists(candidate) {
			path = candidate
			break
		}
	}
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	targetRE := regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9_.-]*):(?:\s|$)`)
	seen := map[string]bool{}
	var targets []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(strings.TrimSpace(line), "#") || strings.Contains(line, "%:") {
			continue
		}
		match := targetRE.FindStringSubmatch(line)
		if len(match) != 2 {
			continue
		}
		name := match[1]
		if strings.HasPrefix(name, ".") || seen[name] {
			continue
		}
		seen[name] = true
		targets = append(targets, name)
		if len(targets) >= 12 {
			break
		}
	}
	out := make([]runnableCommand, 0, len(targets))
	for _, target := range targets {
		out = append(out, runnableCommand{Name: target, Command: "make " + shellQuote(target), Source: "Make", Description: "Make target"})
	}
	return out
}

func discoverDockerCommands(root string) []runnableCommand {
	for _, name := range []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"} {
		if fileExists(filepath.Join(root, name)) {
			return []runnableCommand{
				{Name: "compose-up", Command: "docker compose up", Source: "Docker", Description: "start Compose services"},
				{Name: "compose-up-build", Command: "docker compose up --build", Source: "Docker", Description: "build and start Compose services"},
				{Name: "compose-build", Command: "docker compose build", Source: "Docker", Description: "build Compose services"},
			}
		}
	}
	return nil
}

func discoverExecutableCommands(root string) []runnableCommand {
	type candidate struct {
		rel        string
		name       string
		executable bool
	}
	var candidates []candidate
	skipDirs := map[string]bool{
		".git": true, ".routurn": true, "node_modules": true, "target": true,
		".venv": true, "venv": true, "__pycache__": true, "logs": true,
		"results": true, "outputs": true, "dist": true, "build": true,
	}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil || rel == "." {
			return nil
		}
		depth := len(strings.Split(filepath.ToSlash(rel), "/"))
		if entry.IsDir() {
			if skipDirs[entry.Name()] || strings.HasPrefix(entry.Name(), ".") || depth > 3 {
				return filepath.SkipDir
			}
			return nil
		}
		if depth > 3 {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() {
			return nil
		}
		executable := info.Mode().Perm()&0o111 != 0
		if !executable && !hasShebang(path) && !looksLikeNamedEntrypoint(path) {
			return nil
		}
		if !looksLikeRunnableText(path) {
			return nil
		}
		base := entry.Name()
		name := strings.TrimSuffix(base, filepath.Ext(base))
		name = strings.TrimPrefix(name, "run_")
		candidates = append(candidates, candidate{rel: filepath.ToSlash(rel), name: name, executable: executable})
		return nil
	})
	sort.Slice(candidates, func(i, j int) bool {
		si := executablePriority(candidates[i].name)
		sj := executablePriority(candidates[j].name)
		if si != sj {
			return si > sj
		}
		return candidates[i].rel < candidates[j].rel
	})
	if len(candidates) > 80 {
		candidates = candidates[:80]
	}
	out := make([]runnableCommand, 0, len(candidates))
	for _, item := range candidates {
		out = append(out, runnableCommand{
			Name:        item.name,
			Command:     commandForProjectEntrypoint(root, item.rel, item.executable),
			Source:      "Project commands",
			Description: item.rel,
			ProjectPath: item.rel,
		})
	}
	return out
}

func hasShebang(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 2)
	n, _ := f.Read(buf)
	return n == 2 && string(buf) == "#!"
}

func looksLikeNamedEntrypoint(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".sh", ".bash", ".zsh", ".py", ".js", ".mjs", ".cjs", ".rb", ".pl":
	default:
		return false
	}
	name := strings.ToLower(strings.TrimSuffix(filepath.Base(path), ext))
	prefixes := []string{
		"run_", "run-", "start_", "start-", "dev_", "dev-", "serve_", "serve-",
		"train_", "train-", "smoke_", "smoke-", "test_", "test-", "eval_", "eval-",
		"evaluate_", "evaluate-", "telemetry_", "telemetry-", "seed_", "seed-",
		"check_", "check-", "worker_", "worker-", "deploy_", "deploy-", "build_", "build-",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func commandForProjectEntrypoint(root, rel string, executable bool) string {
	quoted := shellQuote(rel)
	if executable {
		return "./" + quoted
	}
	ext := strings.ToLower(filepath.Ext(rel))
	switch ext {
	case ".sh", ".bash":
		return "bash " + quoted
	case ".zsh":
		return "zsh " + quoted
	case ".py":
		return "python3 " + quoted
	case ".js", ".mjs", ".cjs":
		return "node " + quoted
	case ".rb":
		return "ruby " + quoted
	case ".pl":
		return "perl " + quoted
	}
	if interpreter := shebangInterpreter(filepath.Join(root, filepath.FromSlash(rel))); interpreter != "" {
		return interpreter + " " + quoted
	}
	return "./" + quoted
}

func shebangInterpreter(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	reader := bufio.NewReader(f)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.TrimPrefix(line, "#!"))
	if line == "" {
		return ""
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	if filepath.Base(fields[0]) == "env" && len(fields) > 1 {
		return shellQuote(fields[1])
	}
	return shellQuote(fields[0])
}

func looksLikeRunnableText(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".sh", ".bash", ".zsh", ".py", ".js", ".mjs", ".cjs", ".ts", ".rb", ".pl":
		return true
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 128)
	n, _ := f.Read(buf)
	return strings.HasPrefix(string(buf[:n]), "#!")
}

func executablePriority(name string) int {
	lower := strings.ToLower(name)
	keywords := []string{"seed", "check", "train", "smoke", "test", "dev", "serve", "server", "start", "run", "telemetry", "eval", "build", "deploy", "worker"}
	for i, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return len(keywords) - i
		}
	}
	return 0
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`).MatchString(value) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func shellJoin(args []string) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = shellQuote(arg)
	}
	return strings.Join(parts, " ")
}

func runnableLabel(item runnableCommand) string {
	if item.Saved {
		return fmt.Sprintf("%s (saved)", item.Name)
	}
	return item.Name
}
