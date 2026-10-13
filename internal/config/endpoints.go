package config

import (
	"fmt"
	"sort"
	"strings"
)

const LegacyEndpointName = "default"

func NormalizeEndpoint(endpoint Endpoint) Endpoint {
	if endpoint.Port == 0 {
		endpoint.Port = 22
	}
	return endpoint
}

func EndpointList(target Target) []NamedEndpoint {
	if len(target.Endpoints) == 0 {
		if strings.TrimSpace(target.Host) == "" {
			return nil
		}
		return []NamedEndpoint{{
			Name: LegacyEndpointName,
			Endpoint: NormalizeEndpoint(Endpoint{
				Host: target.Host,
				User: target.User,
				Port: target.Port,
				Jump: target.Jump,
			}),
		}}
	}

	out := make([]NamedEndpoint, 0, len(target.Endpoints))
	for name, endpoint := range target.Endpoints {
		out = append(out, NamedEndpoint{Name: name, Endpoint: NormalizeEndpoint(endpoint)})
	}
	sort.Slice(out, func(i, j int) bool {
		pi := endpointPriority(out[i].Endpoint)
		pj := endpointPriority(out[j].Endpoint)
		if pi != pj {
			return pi < pj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func FindEndpoint(target Target, name string) (Endpoint, bool) {
	for _, candidate := range EndpointList(target) {
		if candidate.Name == name {
			return candidate.Endpoint, true
		}
	}
	return Endpoint{}, false
}

func EffectiveRoute(target Target) string {
	route := strings.TrimSpace(target.Route)
	if route == "" {
		return "auto"
	}
	return route
}

// PromoteLegacyTarget converts the original host/user/port representation into
// a named endpoint. It is intentionally explicit so simply loading old config
// never rewrites user configuration.
func PromoteLegacyTarget(target Target, endpointName string) (Target, error) {
	if len(target.Endpoints) > 0 {
		return target, nil
	}
	if strings.TrimSpace(target.Host) == "" {
		if target.Endpoints == nil {
			target.Endpoints = map[string]Endpoint{}
		}
		return target, nil
	}
	endpointName = strings.TrimSpace(endpointName)
	if endpointName == "" || endpointName == "auto" {
		return Target{}, fmt.Errorf("endpoint name must not be empty or %q", endpointName)
	}
	target.Endpoints = map[string]Endpoint{
		endpointName: NormalizeEndpoint(Endpoint{Host: target.Host, User: target.User, Port: target.Port, Priority: 10, Jump: target.Jump}),
	}
	target.Host = ""
	target.User = ""
	target.Port = 0
	target.Jump = ""
	if target.Route == LegacyEndpointName {
		target.Route = endpointName
	}
	return target, nil
}

func endpointPriority(endpoint Endpoint) int {
	if endpoint.Priority <= 0 {
		return 100
	}
	return endpoint.Priority
}
