package main

import "testing"

func TestOrderRestartUnits(t *testing.T) {
	input := []string{
		"globular-gateway.service",
		"globular-minio.service",
		"globular-etcd.service",
		"custom.service",
		"globular-minio.service",
	}
	got := resolveUnits(input, func(string) bool { return true })
	want := []string{
		"globular-etcd.service",
		"globular-minio.service",
		"globular-gateway.service",
		"custom.service",
	}
	if len(got) != len(want) {
		t.Fatalf("len mismatch got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order mismatch at %d: %v != %v", i, got, want)
		}
	}
}

func TestResolveUnitAliases(t *testing.T) {
	exists := map[string]bool{
		"globular-envoy.service":   true,
		"globular-gateway.service": false,
		"gateway.service":          true,
	}
	list := []string{"envoy.service", "globular-gateway.service"}
	got := resolveUnits(list, func(u string) bool { return exists[u] })
	if got[0] != "globular-envoy.service" {
		t.Fatalf("expected envoy canonical, got %v", got)
	}
	if got[1] != "gateway.service" {
		t.Fatalf("expected gateway alias, got %v", got)
	}
}
