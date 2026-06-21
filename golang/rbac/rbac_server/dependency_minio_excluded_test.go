package main

import "testing"

// TestDependencyHealthDeps_ExcludesMinio pins HARD RULE #8 for RBAC: MinIO is a
// commodity for secondary user data and must NEVER be a health-gating dependency
// of RBAC, whose authority for permissions is ScyllaDB. Gating RBAC's RPCs on
// MinIO would let a commodity tier take cluster-wide authorization down with it
// and, below the 3-node object-store quorum (where the MinIO pool never forms),
// would permanently wedge a healthy cluster.
//
// Regression (globule-nuc, 2026-06-20): rbac registered minio in its dependency
// watchdog → minio absent on a 2-node cluster → rbac RPCs Unavailable → unit
// "failed" → node unhealthy → service convergence blocked.
func TestDependencyHealthDeps_ExcludesMinio(t *testing.T) {
	srv := &server{}
	deps := srv.dependencyHealthDeps()

	if len(deps) == 0 {
		t.Fatal("expected at least the scylladb dependency")
	}
	var hasScylla bool
	for _, d := range deps {
		if d.Name == "minio" {
			t.Errorf("MinIO must NOT be an RBAC health dependency (commodity, not a pillar): %+v", deps)
		}
		if d.Name == "scylladb" {
			hasScylla = true
		}
	}
	if !hasScylla {
		t.Errorf("ScyllaDB (RBAC's permission authority) must be a health dependency, got %+v", deps)
	}
}
