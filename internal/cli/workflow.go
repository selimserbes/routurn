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
	Resolved     *project.Resolved
	Global       *config.GlobalConfig
	TargetName   string
	EndpointName string
	Endpoint     config.Endpoint
	Route        string
}

func resolveLocalProjectContext() (*projectContext, error) {
	resolved, err := project.Resolve(projectName)
	if err != nil {
		return nil, err
	}
	global, err := config.LoadGlobal()
	if err != nil {
		return nil, err
	}
	return &projectContext{Resolved: resolved, Global: global}, nil
}

func resolveProjectContext() (*projectContext, error) {
	ctx, err := resolveLocalProjectContext()
	if err != nil {
		return nil, err
	}
	if ctx.Resolved.Config.Remote.Target == "" || ctx.Resolved.Config.Remote.Path == "" {
		return nil, fmt.Errorf("project remote target/path is not configured in %s", config.ProjectFileName)
	}
	resolvedEndpoint, err := remote.ResolveEndpoint(ctx.Global, ctx.Resolved.Config.Remote.Target, endpointOverride)
	if err != nil {
		return nil, err
	}
	ctx.TargetName = resolvedEndpoint.TargetName
	ctx.EndpointName = resolvedEndpoint.EndpointName
	ctx.Endpoint = resolvedEndpoint.Endpoint
	ctx.Route = resolvedEndpoint.Route
	return ctx, nil
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

	if err := remote.EnsureDir(ctx.Endpoint, ctx.Resolved.Config.Remote.Path); err != nil {
		return syncResult{}, fmt.Errorf("ensure remote project path: %w", err)
	}

	if snapshotID == "" {
		snapshotID = runstate.NewID()
	}
	snapshotPaths := append(append([]string{}, changed...), plan.Deleted...)
	snapshot, err := remote.CreateSnapshot(ctx.Endpoint, ctx.Resolved.Config.Remote.Path, snapshotID, snapshotPaths)
	if err != nil {
		return syncResult{}, err
	}
	if snapshot != "" {
		fmt.Fprintf(out, "Snapshot %s\n", snapshot)
	}

	if len(changed) > 0 {
		fmt.Fprintf(out, "Upload   %d file(s)\n", len(changed))
		if err := remote.UploadTar(ctx.Endpoint, ctx.Resolved.Root, ctx.Resolved.Config.Remote.Path, changed); err != nil {
			return syncResult{}, err
		}
	}
	if len(plan.Deleted) > 0 {
		fmt.Fprintf(out, "Delete   %d remote file(s)\n", len(plan.Deleted))
		if err := remote.RemovePaths(ctx.Endpoint, ctx.Resolved.Config.Remote.Path, plan.Deleted); err != nil {
			return syncResult{}, err
		}
	}

	if err := syncer.SaveManifest(ctx.Resolved.Root, syncer.ToManifest(scan)); err != nil {
		return syncResult{}, err
	}
	if snapshot != "" {
		if _, pruneErr := remote.PruneSnapshots(ctx.Endpoint, ctx.Resolved.Config.Remote.Path, remote.DefaultSnapshotRetention); pruneErr != nil && verbose {
			fmt.Fprintf(out, "! remote snapshot cleanup: %v\n", pruneErr)
		}
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
			ID:         runstate.NewID(),
			Project:    ctx.Resolved.Config.Name,
			Target:     ctx.Resolved.Config.Remote.Target,
			Endpoint:   ctx.EndpointName,
			Task:       taskName,
			Status:     "RUNNING",
			StartedAt:  time.Now().UTC().Format(time.RFC3339),
			RemotePath: ctx.Resolved.Config.Remote.Path,
		}
		if _, err := runstate.Create(ctx.Resolved.Root, manifest); err != nil {
			return taskRunResult{Err: err}
		}
	} else {
		manifest.Status = "RUNNING"
		if manifest.Endpoint == "" {
			manifest.Endpoint = ctx.EndpointName
		}
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

	if verbose {
		fmt.Fprintf(stdout, "Run      %s\n", manifest.ID)
	}
	fmt.Fprintln(stdout, "────────────────────────────────────────")
	exitCode, runErr := remote.RunWithIO(
		ctx.Endpoint,
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

func runTaskDetached(ctx *projectContext, taskName string, manifest runstate.Manifest) taskRunResult {
	task, ok := ctx.Resolved.Config.Tasks[taskName]
	if !ok {
		return taskRunResult{Err: fmt.Errorf("task %q is not defined in %s", taskName, config.ProjectFileName)}
	}
	if task.Interactive {
		return taskRunResult{Err: fmt.Errorf("task %q is interactive and cannot be detached", taskName)}
	}

	if manifest.ID == "" {
		manifest = runstate.Manifest{
			ID:         runstate.NewID(),
			Project:    ctx.Resolved.Config.Name,
			Target:     ctx.Resolved.Config.Remote.Target,
			Endpoint:   ctx.EndpointName,
			Task:       taskName,
			Status:     "RUNNING",
			StartedAt:  time.Now().UTC().Format(time.RFC3339),
			Detached:   true,
			RemotePath: ctx.Resolved.Config.Remote.Path,
		}
		if _, err := runstate.Create(ctx.Resolved.Root, manifest); err != nil {
			return taskRunResult{Err: err}
		}
	} else {
		manifest.Status = "RUNNING"
		if manifest.Endpoint == "" {
			manifest.Endpoint = ctx.EndpointName
		}
		manifest.Detached = true
		manifest.RemotePath = ctx.Resolved.Config.Remote.Path
		if manifest.StartedAt == "" {
			manifest.StartedAt = time.Now().UTC().Format(time.RFC3339)
		}
		if err := runstate.Save(ctx.Resolved.Root, manifest); err != nil {
			return taskRunResult{Manifest: manifest, Err: err}
		}
	}

	pid, err := remote.StartDetached(ctx.Endpoint, ctx.Resolved.Config.Remote.Path, manifest.ID, task.Command)
	if err != nil {
		manifest.Status = "FAILED"
		manifest.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		_ = runstate.Save(ctx.Resolved.Root, manifest)
		return taskRunResult{Manifest: manifest, RunDir: runstate.Dir(ctx.Resolved.Root, manifest.ID), Err: err}
	}
	manifest.RemotePID = pid
	_ = runstate.Save(ctx.Resolved.Root, manifest)
	_, _ = remote.PruneDetachedRuns(ctx.Endpoint, ctx.Resolved.Config.Remote.Path, remote.DefaultDetachedRetention)
	return taskRunResult{Manifest: manifest, RunDir: runstate.Dir(ctx.Resolved.Root, manifest.ID)}
}

func fetchTaskArtifacts(ctx *projectContext, taskName, dest string) ([]string, error) {
	task, ok := ctx.Resolved.Config.Tasks[taskName]
	if !ok {
		return nil, fmt.Errorf("task %q is not defined in %s", taskName, config.ProjectFileName)
	}
	if len(task.Artifacts) == 0 {
		return nil, nil
	}
	return artifact.Fetch(ctx.Endpoint, ctx.Resolved.Config.Remote.Path, task.Artifacts, dest)
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

func printResolvedTarget(out io.Writer, ctx *projectContext) {
	fmt.Fprintf(out, "Target   %s\n", ctx.TargetName)
	fmt.Fprintf(out, "Route    %s\n", ctx.Route)
	fmt.Fprintf(out, "Endpoint %s (%s)\n", ctx.EndpointName, remote.Destination(ctx.Endpoint))
}

func stdinForTask() io.Reader {
	return os.Stdin
}
