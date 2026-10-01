package config

import "testing"

func TestEndpointListLegacyTarget(t *testing.T) {
	target := Target{Host: "example.test", User: "dev", Port: 2200}
	got := EndpointList(target)
	if len(got) != 1 {
		t.Fatalf("expected one endpoint, got %d", len(got))
	}
	if got[0].Name != LegacyEndpointName || got[0].Endpoint.Host != "example.test" || got[0].Endpoint.Port != 2200 {
		t.Fatalf("unexpected legacy endpoint: %#v", got[0])
	}
}

func TestEndpointListPriorityOrder(t *testing.T) {
	target := Target{Endpoints: map[string]Endpoint{
		"remote": {Host: "remote.test", Priority: 20},
		"lan":    {Host: "lan.test", Priority: 10},
		"other":  {Host: "other.test"},
	}}
	got := EndpointList(target)
	if len(got) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(got))
	}
	if got[0].Name != "lan" || got[1].Name != "remote" || got[2].Name != "other" {
		t.Fatalf("unexpected order: %#v", got)
	}
	for _, endpoint := range got {
		if endpoint.Endpoint.Port != 22 {
			t.Fatalf("expected default port 22 for %s, got %d", endpoint.Name, endpoint.Endpoint.Port)
		}
	}
}

func TestPromoteLegacyTarget(t *testing.T) {
	target, err := PromoteLegacyTarget(Target{Host: "lan.test", User: "dev", Port: 22}, "lan")
	if err != nil {
		t.Fatal(err)
	}
	if target.Host != "" || target.User != "" || target.Port != 0 {
		t.Fatalf("legacy fields were not cleared: %#v", target)
	}
	endpoint, ok := target.Endpoints["lan"]
	if !ok || endpoint.Host != "lan.test" || endpoint.Priority != 10 {
		t.Fatalf("unexpected promoted endpoint: %#v", target.Endpoints)
	}
}
