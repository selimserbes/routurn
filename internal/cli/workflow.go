package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/selimserbes/routurn/internal/artifact"
	"github.com/selimserbes/routurn/internal/config"
	"github.com/selimserbes/routurn/internal/project"
	"github.com/selimserbes/routurn/internal/remote"
	"github.com/selimserbes/routurn/internal/runstate"
	"github.com/selimserbes/routurn/internal/syncer"
)

type projectContext struct {
	Resolved *project.Resolved
	Global   *config.GlobalConfig
	Target   config.Target
}

func resolveProjectContext() (*projectContext, error) {
	resolved, err := project.Resolve(projectName)
	if err != nil {
		return nil, err
	}
	if resolved.Config.Remote.Target == "" || resolved.Config.Remote.Path == "" {
		return nil, fmt.Errorf("project remote target/path is not configured in %s", config.ProjectFileName)
	}
	global, err := config.LoadGlobal()
	if err != nil {
		return nil, err
	}
	target, ok := global.Targets[resolved.Config.Remote.Target]
	if !ok {
		return nil, fmt.Errorf("target %q is not registered; use 'routurn target add ...'", resolved.Config.Remote.Target)
	}
	return &projectContext{Resolved: resolved, Global: global, Target: target}, nil
}

type syncResult struct {
	Changed  []string
	Deleted  []string
	Snapshot string
}

func performSync(ctx *projectContext, out io.Writer, dryRun bool, snapshotID string) (syncResult, error) {
	scan, err := syncer.Scan(ctx.Resolved.Root, ctx.Resolved.Config.Sync.Exclude)
	if err != nil {
		return syncResult{}, fmt.Errorf("scan local project: %w", err)
	}
	previous, err := syncer.LoadManifest(ctx.Resolved.Root)
	if err != nil {
		return syncResult{}, err
	}
	plan := syncer.Diff(previous, scan)

	changed := make([]string, 0, len(plan.Changed))
	for _, entry := range plan.Changed {
		changed = append(changed, entry.Path)
	}

	fmt.Fprintf(out, "Changes  %d modified/new, %d deleted\n", len(changed), len(plan.Deleted))
	if len(changed) == 0 && len(plan.Deleted) == 0 {
		fmt.Fprintln(out, "✓ Remote project is already in sync with the last Routurn state")
		return syncResult{}, nil
	}

	if dryRun {
		for _, path := range changed {
			fmt.Fprintf(out, "  + %s\n", path)
		}
		for _, path := range plan.Deleted {
			fmt.Fprintf(out, "  - %s\n", path)
		}
		return syncResult{Changed: changed, Deleted: plan.Deleted}, nil
	}

	if err := remote.EnsureDir(ctx.Target, ctx.Resolved.Config.Remote.Path); err != nil {
		return syncResult{}, fmt.Errorf("ensure remote project path: %w", err)
	}

	if snapshotID == "" {
		snapshotID = runstate.NewID()
	}
	snapshotPaths := append(append([]string{}, changed...), plan.Deleted...)
	snapshot, err := remote.CreateSnapshot(ctx.Target, ctx.Resolved.Config.Remote.Path, snapshotID, snapshotPaths)
	if err != nil {
		return syncResult{}, err
	}
	if snapshot != "" {
		fmt.Fprintf(out, "Snapshot %s\n", snapshot)
	}

	if len(changed) > 0 {
		fmt.Fprintf(out, "Upload   %d file(s)\n", len(changed))
		if err := remote.UploadTar(ctx.Target, ctx.Resolved.Root, ctx.Resolved.Config.Remote.Path, changed); err != nil {
			return syncResult{}, err
		}
	}
	if len(plan.Deleted) > 0 {
		fmt.Fprintf(out, "Delete   %d remote file(s)\n", len(plan.Deleted))
		if err := remote.RemovePaths(ctx.Target, ctx.Resolved.Config.Remote.Path, plan.Deleted); err != nil {
			return syncResult{}, err
		}
	}

	if err := syncer.SaveManifest(ctx.Resolved.Root, syncer.ToManifest(scan)); err != nil {
		return syncResult{}, err
	}
	fmt.Fprintln(out, "✓ Sync complete")
	return syncResult{Changed: changed, Deleted: plan.Deleted, Snapshot: snapshot}, nil
}

type taskRunResult struct {
	Manifest runstate.Manifest
	RunDir   string
	Err      error
}

func runTask(ctx *projectContext, taskName string, manifest runstate.Manifest, stdin io.Reader, stdout, stderr io.Writer) taskRunResult {
	task, ok := ctx.Resolved.Config.Tasks[taskName]
	if !ok {
		return taskRunResult{Err: fmt.Errorf("task %q is not defined in %s", taskName, config.ProjectFileName)}
	}

	if manifest.ID == "" {
		manifest = runstate.Manifest{
			ID:        runstate.NewID(),
			Project:   ctx.Resolved.Config.Name,
			Target:    ctx.Resolved.Config.Remote.Target,
			Task:      taskName,
			Status:    "RUNNING",
			StartedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if _, err := runstate.Create(ctx.Resolved.Root, manifest); err != nil {
			return taskRunResult{Err: err}
		}
	} else {
		manifest.Status = "RUNNING"
		if manifest.StartedAt == "" {
			manifest.StartedAt = time.Now().UTC().Format(time.RFC3339)
		}
		if err := runstate.Save(ctx.Resolved.Root, manifest); err != nil {
			return taskRunResult{Manifest: manifest, Err: err}
		}
	}
	runDir := runstate.Dir(ctx.Resolved.Root, manifest.ID)
	stdoutLog, stderrLog, err := runstate.OpenLogs(ctx.Resolved.Root, manifest.ID)
	if err != nil {
		return taskRunResult{Manifest: manifest, RunDir: runDir, Err: err}
	}
	defer stdoutLog.Close()
	defer stderrLog.Close()

	fmt.Fprintf(stdout, "Run      %s\n", manifest.ID)
	fmt.Fprintln(stdout, "────────────────────────────────────────")
	exitCode, runErr := remote.RunWithIO(
		ctx.Target,
		ctx.Resolved.Config.Remote.Path,
		task.Command,
		task.Interactive,
		stdin,
		io.MultiWriter(stdout, stdoutLog),
		io.MultiWriter(stderr, stderrLog),
	)
	fmt.Fprintln(stdout, "────────────────────────────────────────")

	manifest.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	manifest.ExitCode = &exitCode
	if runErr != nil {
		manifest.Status = "FAILED"
	} else {
		manifest.Status = "SUCCEEDED"
	}
	_ = runstate.Save(ctx.Resolved.Root, manifest)
	return taskRunResult{Manifest: manifest, RunDir: runDir, Err: runErr}
}

func fetchTaskArtifacts(ctx *projectContext, taskName, dest string) ([]string, error) {
	task, ok := ctx.Resolved.Config.Tasks[taskName]
	if !ok {
		return nil, fmt.Errorf("task %q is not defined in %s", taskName, config.ProjectFileName)
	}
	if len(task.Artifacts) == 0 {
		return nil, nil
	}
	return artifact.Fetch(ctx.Target, ctx.Resolved.Config.Remote.Path, task.Artifacts, dest)
}

func relativePaths(root string, paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if rel, err := filepath.Rel(root, path); err == nil {
			out = append(out, filepath.ToSlash(rel))
		} else {
			out = append(out, path)
		}
	}
	return out
}

func stdinForTask() io.Reader {
	return os.Stdin
}
