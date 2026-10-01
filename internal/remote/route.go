package remote

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/selimserbes/routurn/internal/config"
)

const routeCacheTTL = time.Minute

type ResolvedEndpoint struct {
	TargetName   string
	EndpointName string
	Endpoint     config.Endpoint
	Route        string
	Auto         bool
}

type routeCache struct {
	Targets map[string]routeCacheEntry `json:"targets"`
}

type routeCacheEntry struct {
	Endpoint string    `json:"endpoint"`
	At       time.Time `json:"at"`
}

// ResolveEndpoint pins one concrete endpoint for the lifetime of the caller's
// workflow. Explicit --endpoint overrides the saved route. A saved named route
// never performs fallback. Auto mode tries a recently successful route first,
// then the target's priority order, using short non-interactive SSH probes.
func ResolveEndpoint(global *config.GlobalConfig, targetName, override string) (ResolvedEndpoint, error) {
	return resolveEndpointWithProbe(global, targetName, override, Probe, true)
}

func resolveEndpointWithProbe(global *config.GlobalConfig, targetName, override string, probe func(config.Endpoint, int) error, useCache bool) (ResolvedEndpoint, error) {
	logical, ok := global.Targets[targetName]
	if !ok {
		return ResolvedEndpoint{}, fmt.Errorf("target %q is not registered; use 'routurn target add ...'", targetName)
	}
	endpoints := config.EndpointList(logical)
	if len(endpoints) == 0 {
		return ResolvedEndpoint{}, fmt.Errorf("target %q has no SSH endpoints", targetName)
	}

	route := strings.TrimSpace(override)
	if route == "" {
		route = config.EffectiveRoute(logical)
	}
	if route != "auto" {
		endpoint, ok := config.FindEndpoint(logical, route)
		if !ok {
			return ResolvedEndpoint{}, fmt.Errorf("target %q has no endpoint %q", targetName, route)
		}
		return ResolvedEndpoint{TargetName: targetName, EndpointName: route, Endpoint: endpoint, Route: route}, nil
	}

	if len(endpoints) == 1 {
		return ResolvedEndpoint{TargetName: targetName, EndpointName: endpoints[0].Name, Endpoint: endpoints[0].Endpoint, Route: "auto", Auto: true}, nil
	}

	if useCache {
		endpoints = preferCachedEndpoint(targetName, endpoints)
	}
	var failures []string
	for _, candidate := range endpoints {
		if err := probe(candidate.Endpoint, 2); err == nil {
			if useCache {
				_ = rememberEndpoint(targetName, candidate.Name)
			}
			return ResolvedEndpoint{TargetName: targetName, EndpointName: candidate.Name, Endpoint: candidate.Endpoint, Route: "auto", Auto: true}, nil
		} else {
			failures = append(failures, fmt.Sprintf("%s (%s): %v", candidate.Name, Destination(candidate.Endpoint), err))
		}
	}
	return ResolvedEndpoint{}, fmt.Errorf(
		"no reachable endpoint for target %q\n  %s\nChoose a route with 'routurn target route %s <endpoint>' or override one command with '--endpoint <endpoint>'",
		targetName, strings.Join(failures, "\n  "), targetName,
	)
}

func preferCachedEndpoint(targetName string, endpoints []config.NamedEndpoint) []config.NamedEndpoint {
	cache, err := loadRouteCache()
	if err != nil || cache.Targets == nil {
		return endpoints
	}
	entry, ok := cache.Targets[targetName]
	if !ok || entry.Endpoint == "" || time.Since(entry.At) > routeCacheTTL {
		return endpoints
	}
	for i, endpoint := range endpoints {
		if endpoint.Name != entry.Endpoint || i == 0 {
			continue
		}
		out := make([]config.NamedEndpoint, 0, len(endpoints))
		out = append(out, endpoint)
		out = append(out, endpoints[:i]...)
		out = append(out, endpoints[i+1:]...)
		return out
	}
	return endpoints
}

func rememberEndpoint(targetName, endpointName string) error {
	cache, _ := loadRouteCache()
	if cache.Targets == nil {
		cache.Targets = map[string]routeCacheEntry{}
	}
	cache.Targets[targetName] = routeCacheEntry{Endpoint: endpointName, At: time.Now().UTC()}
	path, err := routeCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func loadRouteCache() (routeCache, error) {
	path, err := routeCachePath()
	if err != nil {
		return routeCache{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return routeCache{Targets: map[string]routeCacheEntry{}}, nil
	}
	if err != nil {
		return routeCache{}, err
	}
	var cache routeCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return routeCache{}, err
	}
	if cache.Targets == nil {
		cache.Targets = map[string]routeCacheEntry{}
	}
	return cache, nil
}

func routeCachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "routurn", "routes.json"), nil
}
