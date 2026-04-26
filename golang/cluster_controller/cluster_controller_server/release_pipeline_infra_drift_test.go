package main

import (
	"context"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// infraRelWithNode builds an InfrastructureRelease whose per-node status marks
// nodeID as AVAILABLE.
func infraRelWithNode(component, nodeID string) *cluster_controllerpb.InfrastructureRelease {
	return &cluster_controllerpb.InfrastructureRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "infra/test/" + component},
		Spec: &cluster_controllerpb.InfrastructureReleaseSpec{Component: component},
		Status: &cluster_controllerpb.InfrastructureReleaseStatus{
			Phase:           cluster_controllerpb.ReleasePhaseAvailable,
			ResolvedVersion: "1.0.0",
			Nodes: []*cluster_controllerpb.NodeReleaseStatus{
				{NodeID: nodeID, Phase: cluster_controllerpb.ReleasePhaseAvailable},
			},
		},
	}
}

// captureHandle wraps infraReleaseHandle but overrides PatchStatus so the
// test can assert what patch was applied without writing to etcd.
type patchCapture struct {
	called bool
	patch  statusPatch
}

func handleWithCapture(rel *cluster_controllerpb.InfrastructureRelease, cap *patchCapture) *releaseHandle {
	h := &releaseHandle{
		Name:               rel.Meta.Name,
		ResourceType:       "InfrastructureRelease",
		Phase:              rel.Status.Phase,
		ResolvedVersion:    rel.Status.ResolvedVersion,
		InstalledStateName: rel.Spec.Component,
		InstalledStateKind: "INFRASTRUCTURE",
		Nodes:              rel.Status.Nodes,
		PatchStatus: func(_ context.Context, p statusPatch) error {
			cap.called = true
			cap.patch = p
			return nil
		},
	}
	return h
}

// ── C1: detectInfraDrift ─────────────────────────────────────────────────────

// TestDetectInfraDrift_UnitInactive_DowngradesToDegraded verifies that a node
// whose per-node release status is AVAILABLE but whose globular-xds.service is
// inactive is downgraded to DEGRADED and a patch is written.
//
// This is the exact live bug: nuc was stuck at etcd_ready because
// InfrastructureRelease had no DriftDetector — stale AVAILABLE was never
// re-evaluated against the dead runtime.
func TestDetectInfraDrift_UnitInactive_DowngradesToDegraded(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:   "n1",
				LastSeen: time.Now(),
				Units: []unitStatusRecord{
					{Name: "globular-xds.service", State: "inactive"},
				},
			},
		},
	}
	srv := newTestServer(t, state)
	rel := infraRelWithNode("xds", "n1")
	cap := &patchCapture{}
	h := handleWithCapture(rel, cap)

	detected := srv.detectInfraDrift(context.Background(), rel, h)

	if !detected {
		t.Fatal("expected drift detected for inactive globular-xds.service")
	}
	if !cap.called {
		t.Fatal("expected PatchStatus to be called")
	}
	if cap.patch.Phase != cluster_controllerpb.ReleasePhaseDegraded {
		t.Errorf("expected DEGRADED patch phase, got %q", cap.patch.Phase)
	}
	if len(cap.patch.Nodes) != 1 {
		t.Fatalf("expected 1 node in patch, got %d", len(cap.patch.Nodes))
	}
	if cap.patch.Nodes[0].Phase != cluster_controllerpb.ReleasePhaseDegraded {
		t.Errorf("expected node phase DEGRADED, got %q", cap.patch.Nodes[0].Phase)
	}
}

// TestDetectInfraDrift_UnitActive_NoDrift verifies that a node with an active
// globular-xds.service is NOT flagged as drifted.
func TestDetectInfraDrift_UnitActive_NoDrift(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:   "n1",
				LastSeen: time.Now(),
				Units: []unitStatusRecord{
					{Name: "globular-xds.service", State: "active"},
				},
			},
		},
	}
	srv := newTestServer(t, state)
	rel := infraRelWithNode("xds", "n1")
	cap := &patchCapture{}
	h := handleWithCapture(rel, cap)

	detected := srv.detectInfraDrift(context.Background(), rel, h)

	if detected {
		t.Fatal("expected no drift for active globular-xds.service")
	}
	if cap.called {
		t.Fatal("expected PatchStatus NOT to be called when runtime is healthy")
	}
}

// TestDetectInfraDrift_CommandLikeRestic_NoDrift verifies that command-like
// infrastructure components (restic, rclone, mc, etc.) are skipped entirely —
// they have no systemd unit, so runtime proof is not required.
func TestDetectInfraDrift_CommandLikeRestic_NoDrift(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:   "n1",
				LastSeen: time.Now(),
				Units:    []unitStatusRecord{}, // no units at all
			},
		},
	}
	srv := newTestServer(t, state)
	rel := infraRelWithNode("restic", "n1")
	cap := &patchCapture{}
	h := handleWithCapture(rel, cap)

	detected := srv.detectInfraDrift(context.Background(), rel, h)

	if detected {
		t.Fatal("expected no drift for command-like infra component restic (skipRuntimeCheck)")
	}
}

// TestInfraReleaseHandle_DriftDetectorWired verifies that infraReleaseHandle
// wires a non-nil DriftDetector. This is the compilation-time contract check.
func TestInfraReleaseHandle_DriftDetectorWired(t *testing.T) {
	srv := newTestServer(t, &controllerState{Nodes: map[string]*nodeState{}})
	rel := &cluster_controllerpb.InfrastructureRelease{
		Meta:   &cluster_controllerpb.ObjectMeta{Name: "infra/test/xds"},
		Spec:   &cluster_controllerpb.InfrastructureReleaseSpec{Component: "xds"},
		Status: &cluster_controllerpb.InfrastructureReleaseStatus{Phase: cluster_controllerpb.ReleasePhaseAvailable},
	}
	h := srv.infraReleaseHandle(rel)
	if h.DriftDetector == nil {
		t.Fatal("infraReleaseHandle must wire DriftDetector; stale AVAILABLE would never be re-evaluated otherwise")
	}
}

// ── hasUnservedNodes: H1 guard and degraded node re-dispatch ─────────────────

// TestDetectInfraDrift_MinioNonMember_NoDrift verifies that a node whose
// MinioJoinPhase is NonMember is NOT flagged as drifted when
// globular-minio.service is inactive. The topology contract intentionally
// stops MinIO on non-member nodes; treating this as drift would create a
// restart fight-loop between detectInfraDrift and enforceMinioRuntimeMembership.
func TestDetectInfraDrift_MinioNonMember_NoDrift(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:         "n1",
				LastSeen:       time.Now(),
				MinioJoinPhase: MinioJoinNonMember,
				Units: []unitStatusRecord{
					{Name: "globular-minio.service", State: "inactive"},
				},
			},
		},
		MinioPoolNodes: []string{"10.0.0.63", "10.0.0.20"}, // n1 not in pool
	}
	srv := newTestServer(t, state)
	rel := infraRelWithNode("minio", "n1")
	cap := &patchCapture{}
	h := handleWithCapture(rel, cap)

	detected := srv.detectInfraDrift(context.Background(), rel, h)

	if detected {
		t.Fatal("expected no drift for MinioJoinNonMember node with inactive minio (topology contract)")
	}
}

// TestDetectInfraDrift_MinioMember_InactiveTriggersDrift verifies that a
// confirmed pool-member node (MinioJoinVerified) with inactive minio IS
// flagged as drifted. A member that drops out of MinIO must be re-dispatched.
func TestDetectInfraDrift_MinioMember_InactiveTriggersDrift(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:         "n1",
				Identity:       storedIdentity{Ips: []string{"10.0.0.8"}},
				LastSeen:       time.Now(),
				MinioJoinPhase: MinioJoinVerified,
				Units: []unitStatusRecord{
					{Name: "globular-minio.service", State: "inactive"},
				},
			},
		},
		MinioPoolNodes: []string{"10.0.0.8", "10.0.0.63"},
	}
	srv := newTestServer(t, state)
	rel := infraRelWithNode("minio", "n1")
	cap := &patchCapture{}
	h := handleWithCapture(rel, cap)

	detected := srv.detectInfraDrift(context.Background(), rel, h)

	if !detected {
		t.Fatal("expected drift for confirmed MinIO pool member with inactive service")
	}
	if cap.patch.Phase != cluster_controllerpb.ReleasePhaseDegraded {
		t.Errorf("expected DEGRADED patch, got %q", cap.patch.Phase)
	}
}

// TestHasUnservedNodes_DegradedNode_IsUnserved verifies that a node whose
// per-node release status is DEGRADED (after detectInfraDrift downgrades it)
// is treated as unserved on the next cycle, triggering a re-dispatch.
func TestHasUnservedNodes_DegradedNode_IsUnserved(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:         "n1",
				Status:         "ready",
				LastSeen:       time.Now(),
				BootstrapPhase: BootstrapWorkloadReady,
			},
		},
	}
	srv := newTestServer(t, state)

	// Per-node status is DEGRADED (just downgraded by detectInfraDrift).
	h := &releaseHandle{
		Name:               "infra/test/xds",
		ResourceType:       "InfrastructureRelease",
		Phase:              cluster_controllerpb.ReleasePhaseDegraded,
		ResolvedVersion:    "1.0.0",
		InstalledStateName: "xds",
		InstalledStateKind: "INFRASTRUCTURE",
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "n1", Phase: cluster_controllerpb.ReleasePhaseDegraded},
		},
		PatchStatus: func(_ context.Context, _ statusPatch) error { return nil },
	}

	if !srv.hasUnservedNodes(h) {
		t.Fatal("expected hasUnservedNodes=true for DEGRADED per-node status (must trigger re-dispatch)")
	}
}

// TestHasUnservedNodes_EmptyResolvedVersion_SkipsSignal2 verifies that
// convergence signal #2 (version match path) is not entered when
// ResolvedVersion is empty. An empty ResolvedVersion must not match an
// empty InstalledVersions entry — that would silently skip nodes whose
// release has never been resolved.
func TestHasUnservedNodes_EmptyResolvedVersion_SkipsSignal2(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:           "n1",
				Status:           "ready",
				LastSeen:         time.Now(),
				BootstrapPhase:   BootstrapWorkloadReady,
				InstalledVersions: map[string]string{
					"scylladb": "", // empty installed version
				},
			},
		},
	}
	srv := newTestServer(t, state)

	// Release not yet resolved — ResolvedVersion is empty.
	h := &releaseHandle{
		Name:               "infra/test/scylladb",
		ResourceType:       "InfrastructureRelease",
		Phase:              cluster_controllerpb.ReleasePhaseAvailable,
		ResolvedVersion:    "", // unresolved
		InstalledStateName: "scylladb",
		InstalledStateKind: "INFRASTRUCTURE",
		Nodes:              []*cluster_controllerpb.NodeReleaseStatus{}, // no AVAILABLE entry
		PatchStatus:        func(_ context.Context, _ statusPatch) error { return nil },
	}

	// The node is not in served (no AVAILABLE entry), and signal #2 must NOT
	// treat "" == "" as a version match. The node must be reported as unserved.
	if !srv.hasUnservedNodes(h) {
		t.Fatal("expected hasUnservedNodes=true: empty ResolvedVersion must not match empty installed version")
	}
}
