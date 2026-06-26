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

// ── reconcileAvailable: spec↔resolved coherence guard ────────────────────────

// TestReconcileAvailable_SpecResolvedDrift_ReentersPending reproduces the stuck
// xds infra-release: phase AVAILABLE, generation fully observed
// (Generation == ObservedGeneration), but ResolvedVersion (1.2.235) lags
// spec.Version (1.2.237). The coherence guard
// (intent:reconciler.resolution_must_match_spec) must re-enter PENDING with
// reason spec_resolved_drift so reconcilePending re-resolves the spec version.
// The guard returns before any etcd-backed call, so this exercises the real
// reconcileAvailable path hermetically.
func TestReconcileAvailable_SpecResolvedDrift_ReentersPending(t *testing.T) {
	srv := newTestServer(t, &controllerState{Nodes: map[string]*nodeState{}})
	cap := &patchCapture{}
	h := &releaseHandle{
		Name:               "infra/core@globular.io/xds",
		ResourceType:       "InfrastructureRelease",
		Phase:              cluster_controllerpb.ReleasePhaseAvailable,
		Generation:         3,
		ObservedGeneration: 3, // fully observed — generation guard must NOT fire
		ResolvedVersion:    "1.2.235",
		InstalledStateName: "xds",
		InstalledStateKind: "INFRASTRUCTURE",
		Nodes:              []*cluster_controllerpb.NodeReleaseStatus{},
		ResolverSpec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID: "core@globular.io",
			ServiceName: "xds",
			Version:     "1.2.237", // spec ahead of resolved
		},
		PatchStatus: func(_ context.Context, p statusPatch) error {
			cap.called = true
			cap.patch = p
			return nil
		},
	}

	srv.reconcileAvailable(context.Background(), h)

	if !cap.called {
		t.Fatal("expected PatchStatus for AVAILABLE release with resolved != spec")
	}
	if cap.patch.Phase != cluster_controllerpb.ReleasePhasePending {
		t.Errorf("expected PENDING, got %q", cap.patch.Phase)
	}
	if cap.patch.TransitionReason != "spec_resolved_drift" {
		t.Errorf("expected reason spec_resolved_drift, got %q", cap.patch.TransitionReason)
	}
}

// TestVersionForDriftCompare covers the decision logic of the coherence guard
// without driving the etcd-backed tail of reconcileAvailable: semver versions
// canonicalize (so "v1.2.237" == "1.2.237" → no false-positive churn), blank
// specs are skipped (channel/latest releases), non-semver infra versions fall
// back to raw equality, and a genuine version lag is detected as unequal.
func TestVersionForDriftCompare(t *testing.T) {
	cases := []struct {
		name      string
		spec      string
		resolved  string
		wantDrift bool // true => guard would fire (both non-empty and unequal)
	}{
		{"semver_lag_is_drift", "1.2.237", "1.2.235", true},
		{"semver_v_prefix_equal", "1.2.237", "v1.2.237", false},
		{"empty_spec_skipped", "", "1.35.3", false},
		{"empty_resolved_skipped", "1.2.237", "", false},
		{"nonsemver_equal", "RELEASE.2025-09-07T16-13-09Z", "RELEASE.2025-09-07T16-13-09Z", false},
		{"nonsemver_lag_is_drift", "2025.3.8", "2025.3.7", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			specV := versionForDriftCompare(c.spec)
			resolvedV := versionForDriftCompare(c.resolved)
			gotDrift := specV != "" && resolvedV != "" && specV != resolvedV
			if gotDrift != c.wantDrift {
				t.Errorf("spec=%q resolved=%q: gotDrift=%v want=%v (canon spec=%q resolved=%q)",
					c.spec, c.resolved, gotDrift, c.wantDrift, specV, resolvedV)
			}
		})
	}
}

// TestIsStaleResolvedGhost is the enforcement ratchet for
// invariant:reconciler.resolution_must_match_spec. It locks the decision the
// status-write choke points (patchReleaseStatus / patchAppReleaseStatus /
// patchInfraReleaseStatus) use to REFUSE persisting a converged release whose
// resolved_version disagrees with spec.version. If this logic regresses, the
// stale-resolved ghost — the system reporting a desired/resolved version it
// never actually resolved (the xds 1.2.235-vs-1.2.237 incident) — can be
// persisted again. That state must remain impossible to persist.
func TestIsStaleResolvedGhost(t *testing.T) {
	cases := []struct {
		name     string
		phase    string
		spec     string
		resolved string
		want     bool
	}{
		// The xds incident: a CONVERGED phase whose resolved lags spec → ghost.
		{"available_resolved_lags_spec", cluster_controllerpb.ReleasePhaseAvailable, "1.2.237", "1.2.235", true},
		{"degraded_resolved_lags_spec", cluster_controllerpb.ReleasePhaseDegraded, "1.2.237", "1.2.235", true},
		// Coherent converged release — never a ghost.
		{"available_resolved_matches_spec", cluster_controllerpb.ReleasePhaseAvailable, "1.2.237", "1.2.237", false},
		{"available_v_prefix_canonical_equal", cluster_controllerpb.ReleasePhaseAvailable, "1.2.237", "v1.2.237", false},
		// Non-converged phases are mid-flight; a spec/resolved mismatch is expected
		// there (the release is on its way to resolving) and must NOT be forced.
		{"pending_mismatch_not_ghost", cluster_controllerpb.ReleasePhasePending, "1.2.237", "1.2.235", false},
		{"resolved_mismatch_not_ghost", cluster_controllerpb.ReleasePhaseResolved, "1.2.237", "1.2.235", false},
		{"applying_mismatch_not_ghost", cluster_controllerpb.ReleasePhaseApplying, "1.2.237", "1.2.235", false},
		{"failed_mismatch_not_ghost", cluster_controllerpb.ReleasePhaseFailed, "1.2.237", "1.2.235", false},
		// Channel/latest releases carry no pinned spec version — never a ghost.
		{"empty_spec_skipped", cluster_controllerpb.ReleasePhaseAvailable, "", "1.35.3", false},
		{"empty_resolved_skipped", cluster_controllerpb.ReleasePhaseAvailable, "1.2.237", "", false},
		// Non-semver infra versions: equal raw → not a ghost; differing → ghost.
		{"nonsemver_equal", cluster_controllerpb.ReleasePhaseAvailable, "RELEASE.2025-09-07T16-13-09Z", "RELEASE.2025-09-07T16-13-09Z", false},
		{"nonsemver_lag", cluster_controllerpb.ReleasePhaseAvailable, "2025.3.8", "2025.3.7", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isStaleResolvedGhost(c.phase, c.spec, c.resolved); got != c.want {
				t.Errorf("isStaleResolvedGhost(%q, %q, %q) = %v, want %v",
					c.phase, c.spec, c.resolved, got, c.want)
			}
		})
	}

	// The enforcement reason token is part of the contract (doctor/log scrapers
	// and the ratchet key off it). Lock it so a rename is a deliberate decision.
	if staleResolvedGhostReason != "stale_resolved_ghost_blocked" {
		t.Errorf("staleResolvedGhostReason changed to %q — update scrapers/docs deliberately", staleResolvedGhostReason)
	}
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

// TestDetectInfraDrift_ServiceLikeComponents_InactiveIsDrift verifies that all
// service-like infrastructure components are detected as drifted when their
// systemd unit is inactive. Guardrail 3 coverage for repository, workflow, envoy, etcd.
func TestDetectInfraDrift_ServiceLikeComponents_InactiveIsDrift(t *testing.T) {
	components := []struct {
		name string
		unit string
	}{
		{"repository", "globular-repository.service"},
		{"workflow", "globular-workflow.service"},
		{"envoy", "globular-envoy.service"},
		{"etcd", "globular-etcd.service"},
		{"prometheus", "globular-prometheus.service"},
		{"alertmanager", "globular-alertmanager.service"},
		{"cluster-controller", "globular-cluster-controller.service"},
		{"cluster-doctor", "globular-cluster-doctor.service"},
		{"scylladb", "scylla-server.service"},             // packageUnitOverrides
		{"xds", "globular-xds.service"},                  // control-plane mesh layer
		{"sidekick", "globular-sidekick.service"},         // MinIO metrics proxy
		{"node-exporter", "globular-node-exporter.service"}, // host metrics
		{"scylla-manager", "globular-scylla-manager.service"},             // packageUnitOverrides
		{"scylla-manager-agent", "globular-scylla-manager-agent.service"}, // packageUnitOverrides
	}

	for _, c := range components {
		t.Run(c.name+"_inactive", func(t *testing.T) {
			state := &controllerState{
				Nodes: map[string]*nodeState{
					"n1": {
						NodeID:   "n1",
						LastSeen: time.Now(),
						Units: []unitStatusRecord{
							{Name: c.unit, State: "inactive"},
						},
					},
				},
			}
			srv := newTestServer(t, state)
			rel := infraRelWithNode(c.name, "n1")
			cap := &patchCapture{}
			h := handleWithCapture(rel, cap)

			detected := srv.detectInfraDrift(context.Background(), rel, h)

			if !detected {
				t.Fatalf("expected drift for %s with inactive %s", c.name, c.unit)
			}
			if cap.patch.Phase != cluster_controllerpb.ReleasePhaseDegraded {
				t.Errorf("expected DEGRADED, got %q", cap.patch.Phase)
			}
		})
	}
}

// TestDetectInfraDrift_CommandLikeComponents_NoDrift verifies that command-like
// infrastructure packages are never flagged as drifted regardless of unit state.
func TestDetectInfraDrift_CommandLikeComponents_NoDrift(t *testing.T) {
	commands := []string{"restic", "rclone", "ffmpeg", "sctool", "mc", "etcdctl", "sha256sum", "yt-dlp"}

	for _, name := range commands {
		t.Run(name, func(t *testing.T) {
			state := &controllerState{
				Nodes: map[string]*nodeState{
					"n1": {
						NodeID:   "n1",
						LastSeen: time.Now(),
						Units:    []unitStatusRecord{}, // no units
					},
				},
			}
			srv := newTestServer(t, state)
			rel := infraRelWithNode(name, "n1")
			cap := &patchCapture{}
			h := handleWithCapture(rel, cap)

			detected := srv.detectInfraDrift(context.Background(), rel, h)

			if detected {
				t.Fatalf("expected no drift for command-like %s", name)
			}
		})
	}
}

// TestDetectInfraDrift_HashDrift_DowngradesToDegraded verifies that a node
// reporting unit state "hash_drift" (unit file content changed outside the
// package pipeline) is downgraded to DEGRADED. Guardrail 4: unit definition drift.
func TestDetectInfraDrift_HashDrift_DowngradesToDegraded(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:   "n1",
				LastSeen: time.Now(),
				Units: []unitStatusRecord{
					// heartbeat reports hash_drift (computed by checkUnitHashDrift)
					{Name: "globular-xds.service", State: "hash_drift", Details: "running (load=loaded) [unit_hash_drift]"},
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
		t.Fatalf("expected drift for unit with hash_drift state")
	}
	if cap.patch.Phase != cluster_controllerpb.ReleasePhaseDegraded {
		t.Errorf("expected DEGRADED, got %q", cap.patch.Phase)
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

	if !srv.hasUnservedNodes(h, map[string]struct{}{}) {
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
	if !srv.hasUnservedNodes(h, map[string]struct{}{}) {
		t.Fatal("expected hasUnservedNodes=true: empty ResolvedVersion must not match empty installed version")
	}
}

// TestHasUnservedNodes_InstalledAheadOfDesired_SkipsNode verifies that a
// node whose installed version is strictly newer than the resolved version
// is skipped (treated as served). Without this, a stale desired version
// creates an infinite FAILED→PENDING→FAILED loop: the downgrade guard
// blocks the install, hasUnservedNodes sees the mismatch, and the release
// auto-retries forever.
func TestHasUnservedNodes_InstalledAheadOfDesired_SkipsNode(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:         "n1",
				Status:         "ready",
				LastSeen:       time.Now(),
				BootstrapPhase: BootstrapWorkloadReady,
				InstalledVersions: map[string]string{
					"gateway": "1.2.197", // ahead of desired 1.2.193
				},
			},
		},
	}
	srv := newTestServer(t, state)

	h := &releaseHandle{
		Name:               "infra/core@globular.io/gateway",
		ResourceType:       "InfrastructureRelease",
		Phase:              cluster_controllerpb.ReleasePhaseAvailable,
		ResolvedVersion:    "1.2.193",
		InstalledStateName: "gateway",
		InstalledStateKind: "INFRASTRUCTURE",
		Nodes:              []*cluster_controllerpb.NodeReleaseStatus{},
		PatchStatus:        func(_ context.Context, _ statusPatch) error { return nil },
	}

	if srv.hasUnservedNodes(h, map[string]struct{}{}) {
		t.Fatal("hasUnservedNodes must return false when installed version (1.2.197) > desired (1.2.193); no downgrade allowed")
	}
}
