package cli

import (
	"fmt"

	"github.com/selimserbes/routurn/internal/config"
	"github.com/selimserbes/routurn/internal/project"
	"github.com/selimserbes/routurn/internal/remote"
	"github.com/selimserbes/routurn/internal/runstate"
)

type runRemoteContext struct {
	Resolved   *project.Resolved
	Global     *config.GlobalConfig
	Manifest   runstate.Manifest
	Target     config.Endpoint
	RemotePath string
}

func resolveRunRemote(runID string) (*runRemoteContext, error) {
	resolved, err := project.Resolve(projectName)
	if err != nil {
		return nil, err
	}
	id, err := runstate.ResolveSelector(resolved.Root, runID)
	if err != nil {
		return nil, err
	}
	manifest, err := runstate.Load(resolved.Root, id)
	if err != nil {
		return nil, err
	}
	if !manifest.Detached {
		return nil, fmt.Errorf("run %s was not started in detached mode", id)
	}
	global, err := config.LoadGlobal()
	if err != nil {
		return nil, err
	}
	targetName := manifest.Target
	if targetName == "" {
		targetName = resolved.Config.Remote.Target
	}
	resolvedEndpoint, err := remote.ResolveEndpoint(global, targetName, endpointOverride)
	if err != nil {
		return nil, err
	}
	target := resolvedEndpoint.Endpoint
	manifest.Endpoint = resolvedEndpoint.EndpointName
	remotePath := manifest.RemotePath
	if remotePath == "" {
		remotePath = resolved.Config.Remote.Path
	}
	if remotePath == "" {
		return nil, fmt.Errorf("remote path is missing for run %s", id)
	}
	manifest.ID = id
	return &runRemoteContext{Resolved: resolved, Global: global, Manifest: manifest, Target: target, RemotePath: remotePath}, nil
}

func refreshDetachedRun(ctx *runRemoteContext) (remote.DetachedStatus, runstate.Manifest, error) {
	state, err := remote.QueryDetached(ctx.Target, ctx.RemotePath, ctx.Manifest.ID)
	if err != nil {
		return remote.DetachedStatus{}, ctx.Manifest, err
	}
	m := ctx.Manifest
	m.Status = state.Status
	m.RemotePID = state.PID
	if state.StartedAt != "" {
		m.StartedAt = state.StartedAt
	}
	if state.FinishedAt != "" {
		m.FinishedAt = state.FinishedAt
	}
	m.ExitCode = state.ExitCode
	if err := runstate.Save(ctx.Resolved.Root, m); err != nil {
		return state, m, err
	}
	return state, m, nil
}
