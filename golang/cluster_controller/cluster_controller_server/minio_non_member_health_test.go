package main

import (
	"testing"
	"time"
)

// TestEvaluateNodeStatus_MinioNonMember verifies that a node with
// MinioJoinPhase=non_member is NOT blocked from reaching "ready" status
// just because globular-minio.service and globular-sidekick.service are
// not active. When the topology contract holds a node out of the MinIO
// pool, those units are intentionally inactive — they must not prevent
// convergence of the node's workload services.
//
// Root cause (2026-05-22): minio_join_phase=non_member caused all service
// releases on nuc/dell to cycle FAILED indefinitely because:
//   evaluateNodeStatus → computeNodePlan → requiredUnitsFromPlan included
//   globular-minio.service and globular-sidekick.service → units not active →
//   node.Status="converging" → healthy=false in reconcileAvailable → DEGRADED.
func TestEvaluateNodeStatus_MinioNonMember(t *testing.T) {
	t.Helper()

	// A storage-profile node that is workload_ready but MinIO is held back
	// pending topology admission (non_member).
	node := &nodeState{
		NodeID:         "nuc",
		Status:         "converging",
		BootstrapPhase: BootstrapWorkloadReady,
		MinioJoinPhase: MinioJoinNonMember,
		LastSeen:       time.Now(),
		ReportedAt:     time.Now(),
		Profiles:       []string{"core", "storage", "control-plane", "gateway"},
		Identity:       storedIdentity{Hostname: "globule-nuc", Ips: []string{"10.0.0.8"}},
	}

	srv := newTestServer(t, &controllerState{
		Nodes: map[string]*nodeState{"nuc": node},
	})

	// Compute what the plan actually requires for this profile set, then
	// provide active units for all of them — except minio/sidekick which are
	// intentionally absent because of the non_member topology hold.
	plan, _ := srv.computeNodePlan(node)
	required := requiredUnitsFromPlan(plan)
	units := make([]unitStatusRecord, 0, len(required))
	for u := range required {
		if u == "globular-minio.service" || u == "globular-sidekick.service" {
			continue // intentionally absent — our fix should ignore these
		}
		units = append(units, unitStatusRecord{Name: u, State: "active"})
	}

	status, reason := srv.evaluateNodeStatus(node, units)
	if status != "ready" {
		t.Errorf("expected status=ready for MinioJoinNonMember node, got %q (reason: %s)", status, reason)
	}
}

// TestEvaluateNodeStatus_MinioMember_RequiresMinio verifies that a node
// that IS a MinIO pool member (not non_member) still requires minio to
// be active for "ready" status.
func TestEvaluateNodeStatus_MinioMember_RequiresMinio(t *testing.T) {
	t.Helper()

	node := &nodeState{
		NodeID:         "ryzen",
		Status:         "ready",
		BootstrapPhase: BootstrapWorkloadReady,
		MinioJoinPhase: MinioJoinVerified, // pool member
		LastSeen:       time.Now(),
		ReportedAt:     time.Now(),
		Profiles:       []string{"core", "storage", "control-plane", "gateway"},
		Identity:       storedIdentity{Hostname: "globule-ryzen", Ips: []string{"10.0.0.63"}},
	}

	// All services active EXCEPT minio — which should be required for a pool member.
	units := []unitStatusRecord{
		{Name: "globular-etcd.service", State: "active"},
		{Name: "globular-scylladb.service", State: "active"},
		{Name: "globular-gateway.service", State: "active"},
		{Name: "globular-xds.service", State: "active"},
		{Name: "globular-envoy.service", State: "active"},
		{Name: "globular-alertmanager.service", State: "active"},
		{Name: "globular-node-exporter.service", State: "active"},
		{Name: "globular-prometheus.service", State: "active"},
		{Name: "globular-scylla-manager.service", State: "active"},
		{Name: "globular-scylla-manager-agent.service", State: "active"},
		{Name: "globular-sidekick.service", State: "active"},
		{Name: "keepalived.service", State: "active"},
		// globular-minio.service absent — should cause non-ready for pool member.
	}

	srv := newTestServer(t, &controllerState{
		Nodes: map[string]*nodeState{"ryzen": node},
	})

	status, _ := srv.evaluateNodeStatus(node, units)
	if status == "ready" {
		t.Error("expected non-ready status for MinIO pool member with minio service absent")
	}
}
