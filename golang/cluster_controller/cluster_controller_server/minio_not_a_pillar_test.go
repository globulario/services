package main

import (
	"testing"
	"time"
)

// storageJoiningNode builds a node parked in storage_joining long enough that
// the phase has already timed out — the moment the gate must decide whether a
// waiting dependency costs this node its membership.
//
// Profiles select the dependencies: "core" maps to MinIO only, "scylla" to
// ScyllaDB only, "storage" to both (profilesForMinio / profilesForScyllaDB).
func storageJoiningNode(hostname string, profiles []string) *nodeState {
	return &nodeState{
		NodeID:             "n-" + hostname,
		Identity:           storedIdentity{Hostname: hostname, Ips: []string{"10.0.0.5"}},
		Profiles:           profiles,
		BootstrapPhase:     BootstrapStorageJoining,
		BootstrapStartedAt: time.Now().Add(-bootstrapPhaseTimeout - time.Minute),
		// The node is alive and reporting — this phase's runtime gate needs a
		// heartbeat to classify infra at all. The scenario under test is a live
		// node whose object store will not converge, not a dead one.
		LastSeen: time.Now(),
	}
}

// TestMinioMayNotBlockNodeConvergence pins the difference between a pillar and a
// commodity tier at the bootstrap storage gate.
//
// storage_joining used to treat MinIO and ScyllaDB identically: either one not
// reaching its verified phase held the node, and on timeout the node's bootstrap
// FAILED. For ScyllaDB that is right — it holds cluster state and the workflow
// lease table, so a node that cannot join the ring has not joined the cluster.
//
// For MinIO it violated invariant minio.is_commodity_not_a_pillar — "a commodity
// object-store tier, never a pillar — it must not gate a primary service's health
// nor block node convergence". Blocking turned an object-store problem into a
// cluster-membership problem, which is the recorded failure mode
// bootstrap.held_minio_nonmember_stranded_at_none: a held non-member stranded at
// MinioJoinNone wedges this phase.
func TestMinioMayNotBlockNodeConvergence(t *testing.T) {
	t.Run("MinIO alone degrades, never fails", func(t *testing.T) {
		node := storageJoiningNode("minio-only", []string{"core"})
		node.MinioJoinPhase = MinioJoinNone
		if !nodeHasMinioProfile(node) || nodeHasScyllaProfile(node) {
			t.Fatalf("test setup: %q must select MinIO and not ScyllaDB", "core")
		}

		if !reconcileBootstrapPhases([]*nodeState{node}, nil, &mockEmitter{}) {
			t.Fatal("expected the phase decision to mark state dirty")
		}
		if node.BootstrapPhase == BootstrapFailed {
			t.Fatalf("a commodity tier cost the node its membership: phase=%s err=%q",
				node.BootstrapPhase, node.BootstrapError)
		}
		if node.BootstrapPhase != BootstrapWorkloadReady {
			t.Fatalf("expected advance to %s despite unconverged MinIO, got %s",
				BootstrapWorkloadReady, node.BootstrapPhase)
		}
		// Advancing degraded must not fabricate a converged objectstore: the
		// join phase stays as observed so cluster-doctor still sees the truth.
		if node.MinioJoinPhase != MinioJoinNone {
			t.Fatalf("degraded advance rewrote the observed MinIO phase to %s",
				node.MinioJoinPhase)
		}
	})

	t.Run("ScyllaDB alone still fails", func(t *testing.T) {
		node := storageJoiningNode("scylla-only", []string{"scylla"})
		node.ScyllaJoinPhase = ScyllaJoinStarted
		if !nodeHasScyllaProfile(node) || nodeHasMinioProfile(node) {
			t.Fatalf("test setup: %q must select ScyllaDB and not MinIO", "scylla")
		}

		if !reconcileBootstrapPhases([]*nodeState{node}, nil, &mockEmitter{}) {
			t.Fatal("expected the phase decision to mark state dirty")
		}
		if node.BootstrapPhase != BootstrapFailed {
			t.Fatalf("ScyllaDB is a pillar — a node that cannot join the ring has not "+
				"joined the cluster; expected %s, got %s",
				BootstrapFailed, node.BootstrapPhase)
		}
	})

	t.Run("both waiting fails, because ScyllaDB does", func(t *testing.T) {
		node := storageJoiningNode("both", []string{"storage"})
		node.MinioJoinPhase = MinioJoinNone
		node.ScyllaJoinPhase = ScyllaJoinStarted
		if !nodeHasMinioProfile(node) || !nodeHasScyllaProfile(node) {
			t.Fatalf("test setup: %q must select both tiers", "storage")
		}

		if !reconcileBootstrapPhases([]*nodeState{node}, nil, &mockEmitter{}) {
			t.Fatal("expected the phase decision to mark state dirty")
		}
		if node.BootstrapPhase != BootstrapFailed {
			t.Fatalf("the pillar decides when both are waiting; expected %s, got %s",
				BootstrapFailed, node.BootstrapPhase)
		}
	})

	t.Run("MinIO non-member advances without waiting for the timeout", func(t *testing.T) {
		// A node held out of the pool is a lawful terminal state, not a stall —
		// it must converge immediately, not sit out the 5-minute phase budget.
		node := storageJoiningNode("non-member", []string{"core"})
		node.BootstrapStartedAt = time.Now()
		node.MinioJoinPhase = MinioJoinNonMember

		if !reconcileBootstrapPhases([]*nodeState{node}, nil, &mockEmitter{}) {
			t.Fatal("expected the phase decision to mark state dirty")
		}
		if node.BootstrapPhase != BootstrapWorkloadReady {
			t.Fatalf("expected %s for a lawful non-member, got %s",
				BootstrapWorkloadReady, node.BootstrapPhase)
		}
	})
}
