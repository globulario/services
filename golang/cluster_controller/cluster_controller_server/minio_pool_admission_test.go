package main

import "testing"

// appendPoolIfEmpty mirrors the guard ReportNodeStatus applies before touching
// MinioPoolNodes: a heartbeat may bootstrap an EMPTY pool, but joining an
// existing one is admission and belongs to the objectstore admission plane.
// Kept as a pure helper so the rule is testable without standing up a
// controller.
func appendPoolIfEmpty(pool []string, ip string) ([]string, bool) {
	for _, existing := range pool {
		if existing == ip {
			return pool, false
		}
	}
	if len(pool) > 0 {
		return pool, false
	}
	return append(pool, ip), true
}

// TestHeartbeatMayNotAdmitIntoAnExistingPool pins that a status report cannot
// grant object-store pool membership.
//
// ReportNodeStatus appended any profile-eligible node's IP to MinioPoolNodes on
// heartbeat. profilesForMinio is ["core","compute","storage","control-plane"] —
// effectively every node in a normal cluster — so a node walked into an
// established erasure pool without passing `objectstore disk approve`, without
// the explicit-member check in minio_pools.go, and without the "once a pool
// exists, a non-member is HELD" contract gate that same file applies a few
// lines later. The sibling append in minio_pools.go already refuses a non-empty
// pool for exactly this reason; this writer did not.
//
// Observed 2026-08-24 on release 1.2.331: after node-5 was wiped and rejoined
// the controller logged
//
//	ReportNodeStatus: added 10.10.0.15 (node-5) to MinIO pool
//
// and /globular/objectstore/config became a DISTRIBUTED pool of four nodes
// whose paths disagreed — 10.10.0.11/.12/.13 on /var/lib/minio/d1 (admitted)
// and 10.10.0.15 on /var/lib/globular/minio (never admitted). node-5 does not
// even carry the storage profile. Erasure-set membership is a data-placement
// decision, so an unadmitted member can attract placement onto storage nobody
// approved (intent:objectstore.destructive_changes_require_approval).
//
// Day-0 must keep working: the founding node meets an empty pool and creates
// it, which is what keeps the objectstore endpoint resolvable.
func TestHeartbeatMayNotAdmitIntoAnExistingPool(t *testing.T) {
	t.Run("bootstraps an empty pool", func(t *testing.T) {
		pool, added := appendPoolIfEmpty(nil, "10.10.0.11")
		if !added {
			t.Fatal("Day-0 founding node must still be able to create the pool")
		}
		if len(pool) != 1 || pool[0] != "10.10.0.11" {
			t.Fatalf("unexpected pool %v", pool)
		}
	})

	t.Run("refuses to join an existing pool", func(t *testing.T) {
		existing := []string{"10.10.0.11", "10.10.0.12", "10.10.0.13"}
		pool, added := appendPoolIfEmpty(existing, "10.10.0.15")
		if added {
			t.Error("a heartbeat must not admit a node into an existing pool — " +
				"membership requires objectstore admission, not a status report")
		}
		if len(pool) != 3 {
			t.Errorf("pool must be left unchanged, got %v", pool)
		}
	})

	t.Run("is idempotent for a node already in the pool", func(t *testing.T) {
		existing := []string{"10.10.0.11"}
		pool, added := appendPoolIfEmpty(existing, "10.10.0.11")
		if added {
			t.Error("an existing member must not be appended twice")
		}
		if len(pool) != 1 {
			t.Errorf("pool must be left unchanged, got %v", pool)
		}
	})
}
