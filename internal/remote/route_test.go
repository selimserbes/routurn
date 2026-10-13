package remote

import (
	"errors"
	"testing"

	"github.com/selimserbes/routurn/internal/config"
)

func testGlobalTarget(target config.Target) *config.GlobalConfig {
	return &config.GlobalConfig{Targets: map[string]config.Target{"lab": target}}
}

func TestResolveEndpointExplicitRouteDoesNotProbe(t *testing.T) {
	global := testGlobalTarget(config.Target{
		Route: "remote",
		Endpoints: map[string]config.Endpoint{
			"lan":    {Host: "lan.test", Priority: 10},
			"remote": {Host: "remote.test", Priority: 20},
		},
	})
	probes := 0
	got, err := resolveEndpointWithProbe(global, "lab", "", func(config.Endpoint, int) error {
		probes++
		return nil
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.EndpointName != "remote" || got.Endpoint.Host != "remote.test" {
		t.Fatalf("unexpected endpoint: %#v", got)
	}
	if probes != 0 {
		t.Fatalf("explicit route should not probe, got %d probe(s)", probes)
	}
}

func TestResolveEndpointOverrideWins(t *testing.T) {
	global := testGlobalTarget(config.Target{
		Route: "remote",
		Endpoints: map[string]config.Endpoint{
			"lan":    {Host: "lan.test", Priority: 10},
			"remote": {Host: "remote.test", Priority: 20},
		},
	})
	got, err := resolveEndpointWithProbe(global, "lab", "lan", func(config.Endpoint, int) error {
		return errors.New("should not probe explicit override")
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.EndpointName != "lan" || got.Route != "lan" {
		t.Fatalf("unexpected override resolution: %#v", got)
	}
}

func TestResolveEndpointAutoFallsBack(t *testing.T) {
	global := testGlobalTarget(config.Target{
		Route: "auto",
		Endpoints: map[string]config.Endpoint{
			"lan":    {Host: "lan.test", Priority: 10},
			"remote": {Host: "remote.test", Priority: 20},
		},
	})
	var attempted []string
	got, err := resolveEndpointWithProbe(global, "lab", "", func(endpoint config.Endpoint, _ int) error {
		attempted = append(attempted, endpoint.Host)
		if endpoint.Host == "lan.test" {
			return errors.New("unreachable")
		}
		return nil
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.EndpointName != "remote" {
		t.Fatalf("expected remote fallback, got %#v", got)
	}
	if len(attempted) != 2 || attempted[0] != "lan.test" || attempted[1] != "remote.test" {
		t.Fatalf("unexpected probe order: %#v", attempted)
	}
}

func TestResolveEndpointLegacySingleRoute(t *testing.T) {
	global := testGlobalTarget(config.Target{Host: "legacy.test", User: "dev", Port: 22})
	got, err := resolveEndpointWithProbe(global, "lab", "", func(config.Endpoint, int) error {
		return errors.New("single endpoint should not need a preflight probe")
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.EndpointName != config.LegacyEndpointName || got.Endpoint.Host != "legacy.test" {
		t.Fatalf("unexpected legacy resolution: %#v", got)
	}
}
