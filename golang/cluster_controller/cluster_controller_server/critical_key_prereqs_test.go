package main

import (
	"testing"
)

func TestKindCriticalKeyPrereqsService(t *testing.T) {
	keys := kindCriticalKeyPrereqs["SERVICE"]
	if len(keys) == 0 {
		t.Fatal("SERVICE kind should have at least one critical key prereq")
	}
	found := false
	for _, k := range keys {
		if k == "/globular/system/config" {
			found = true
		}
	}
	if !found {
		t.Errorf("SERVICE kind prereqs %v should include /globular/system/config", keys)
	}
}

func TestKindCriticalKeyPrereqsWorkload(t *testing.T) {
	keys := kindCriticalKeyPrereqs["WORKLOAD"]
	if len(keys) == 0 {
		t.Fatal("WORKLOAD kind should have at least one critical key prereq")
	}
}

func TestKindCriticalKeyPrereqsInfrastructure(t *testing.T) {
	keys := kindCriticalKeyPrereqs["INFRASTRUCTURE"]
	if len(keys) != 0 {
		t.Errorf("INFRASTRUCTURE kind should have no prereqs (it creates config); got %v", keys)
	}
}

func TestKindCriticalKeyPrereqsCommand(t *testing.T) {
	keys := kindCriticalKeyPrereqs["COMMAND"]
	if len(keys) != 0 {
		t.Errorf("COMMAND kind should have no prereqs; got %v", keys)
	}
}

func TestPackageCriticalKeyPrereqsKeepalived(t *testing.T) {
	keys := packageCriticalKeyPrereqs["keepalived"]
	found := false
	for _, k := range keys {
		if k == "/globular/ingress/v1/spec" {
			found = true
		}
	}
	if !found {
		t.Errorf("keepalived prereqs %v should include /globular/ingress/v1/spec", keys)
	}
}

func TestPackageCriticalKeyPrereqsEnvoy(t *testing.T) {
	keys := packageCriticalKeyPrereqs["envoy"]
	found := false
	for _, k := range keys {
		if k == "/globular/ingress/v1/spec" {
			found = true
		}
	}
	if !found {
		t.Errorf("envoy prereqs %v should include /globular/ingress/v1/spec", keys)
	}
}

func TestCriticalKeyBlockActionID(t *testing.T) {
	id := criticalKeyBlockActionID("node-abc", "SERVICE", "rbac")
	expected := "controller/node-abc/SERVICE/rbac/critical_key_block"
	if id != expected {
		t.Errorf("criticalKeyBlockActionID = %q, want %q", id, expected)
	}
}

func TestCriticalKeyPrereqsMissingNoPrereqs(t *testing.T) {
	// INFRASTRUCTURE packages have no prereqs — returns "" without hitting etcd.
	result := criticalKeyPrereqsMissing(nil, "etcd", "INFRASTRUCTURE")
	if result != "" {
		t.Errorf("INFRASTRUCTURE pkg should have no prereqs, got %q", result)
	}
}
