package main

import (
	"os"
	"strings"
	"testing"
)

// invariantRepairMinioStorage repairs nodes the objectstore contract ALREADY
// places in the pool. It must never decide pool membership.
//
// It used to reset MinioJoinPhase for every storage-profile node absent from
// the pool, which grows the pool on its own. That was dormant only because
// cluster.invariant.enforcement was never dispatched; the moment the workflow
// started running, a healthy standalone cluster was driven toward distributed
// and produced 7 CRITICAL findings in a single pass (minio.not_stalled x3,
// minio.topology_matches_desired x3 with format.json risk, and
// objectstore.minio.unapproved_path) — fm:objectstore.minio.
// standalone_to_distributed_grow_deadlock, self-inflicted.
//
// Growing standalone -> distributed rewrites .minio.sys, so it belongs to an
// operator with a matching generation
// (intent:objectstore.destructive_changes_require_approval), never to an
// automatic repair pass.

func TestMinioRepairDoesNotGrowThePool(t *testing.T) {
	src, err := os.ReadFile("workflow_invariant.go")
	if err != nil {
		t.Fatalf("read workflow_invariant.go: %v", err)
	}

	start := strings.Index(string(src), "func (srv *server) invariantRepairMinioStorage")
	if start < 0 {
		t.Fatal("invariantRepairMinioStorage not found")
	}
	body := string(src)[start:]
	if end := strings.Index(body[1:], "\nfunc "); end > 0 {
		body = body[:end]
	}

	// The non-member branch must not reset join state to pull a node in.
	for _, line := range strings.Split(body, "\n") {
		code := strings.TrimSpace(line)
		if strings.HasPrefix(code, "//") {
			continue
		}
		if strings.Contains(code, "reset_join_phase_for_pool_join") {
			t.Error("repair still resets the join phase to add a non-member node to " +
				"the pool — that is a destructive topology change, not a repair")
		}
	}

	// And it must still repair genuine members (config re-render + restart),
	// otherwise the guard would have removed the step's actual purpose.
	if !strings.Contains(body, "clear_config_hash_and_restart") {
		t.Error("repair no longer fixes in-pool members; the step exists to converge " +
			"nodes the contract already placed in the pool")
	}
}
