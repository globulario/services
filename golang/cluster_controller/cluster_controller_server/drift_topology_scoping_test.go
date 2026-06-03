package main

// drift_topology_scoping_test.go — Phase 35.
//
// Pins the topology gate's per-package scoping behaviour:
// driftActionSafe must block ONLY packages whose subsystem owns the
// violating topology dimension. Blanket-blocking all INFRASTRUCTURE
// on any violation is the bug Phase 35 fixes — it was caught live on
// globule-ryzen 2026-06-03 where storage_quorum (legitimately failing
// on a single-node cluster) was blocking node-agent rollouts that
// have nothing to do with MinIO erasure coding.

import (
	"testing"
)

// storageQuorum is the canonical "MinIO needs ≥3 nodes" violation —
// the exact condition observed live during Phase 35 onset.
var storageQuorum = []topologySafetyViolation{
	{Kind: "storage_quorum", Message: "only 1 active storage node — minimum 3 required"},
}

var ingressParticipant = []topologySafetyViolation{
	{Kind: "ingress_participant", Message: "fewer than required ingress participants healthy"},
}

var controllerPlacement = []topologySafetyViolation{
	{Kind: "controller_placement", Message: "controller would be removed from last placement-eligible node"},
}

var objectstoreTopology = []topologySafetyViolation{
	{Kind: "objectstore_topology_mismatch", Message: "applied topology does not match desired"},
}

// helper: build an infrastructure drift action for a named package.
func infraAction(name string) driftAction {
	return driftAction{
		NodeID:     "node-1",
		PackageKey: "INFRASTRUCTURE/" + name,
		Kind:       "INFRASTRUCTURE",
		ActionKind: classifyDriftAction("INFRASTRUCTURE"),
	}
}

func TestTopologyScoping_StorageQuorum_BlocksMinIO(t *testing.T) {
	// Regression of the original intent: MinIO upgrade MUST stay blocked
	// when MinIO's own storage_quorum is violated. Upgrading the storage
	// provider while erasure coding is degraded is dangerous.
	if driftActionSafe(infraAction("minio"), storageQuorum) {
		t.Fatal("expected MinIO to be blocked by storage_quorum")
	}
}

func TestTopologyScoping_StorageQuorum_BlocksScylla(t *testing.T) {
	// Scylla and backup-manager touch storage durability — block them
	// too. (The conservative side of the Phase 35 narrowing.)
	for _, name := range []string{"scylladb", "scylla", "scylla-manager", "backup-manager"} {
		if driftActionSafe(infraAction(name), storageQuorum) {
			t.Errorf("expected %s to be blocked by storage_quorum", name)
		}
	}
}

func TestTopologyScoping_StorageQuorum_DoesNotBlockNodeAgent(t *testing.T) {
	// THE Phase 35 fix: node-agent upgrade has nothing to do with MinIO
	// erasure coding. A storage_quorum violation on a single-node cluster
	// must NOT permanently block node-agent rollouts. This was the exact
	// live blocker before this commit.
	if !driftActionSafe(infraAction("node-agent"), storageQuorum) {
		t.Fatal("node-agent must NOT be blocked by storage_quorum (Phase 35 regression)")
	}
}

func TestTopologyScoping_StorageQuorum_DoesNotBlockControlPlaneServices(t *testing.T) {
	// Sweep across control-plane / observability / AI / service packages.
	// None of these mutate storage topology; storage_quorum must not
	// block their upgrades.
	for _, name := range []string{
		"node-agent", "cluster-controller", "repository", "authentication",
		"rbac", "resource", "event", "log", "monitoring", "prometheus",
		"alertmanager", "ai-executor", "ai-memory", "ai-router", "ai-watcher",
		"mcp", "media", "file", "search", "title", "persistence",
		"workflow", "torrent", "awareness-graph", "ldap", "mail",
		"globular-cli", "node-exporter", "scylla-manager-agent",
		"cluster-doctor", "etcd",
	} {
		if !driftActionSafe(infraAction(name), storageQuorum) {
			t.Errorf("package %q must NOT be blocked by storage_quorum (it does not touch storage topology)", name)
		}
	}
}

func TestTopologyScoping_IngressParticipant_BlocksMeshPackages(t *testing.T) {
	// Envoy / xds / keepalived / dns are the mesh-routing subsystem;
	// ingress_participant violation must continue to block them.
	for _, name := range []string{"envoy", "xds", "keepalived", "dns"} {
		if driftActionSafe(infraAction(name), ingressParticipant) {
			t.Errorf("expected %s to be blocked by ingress_participant", name)
		}
	}
}

func TestTopologyScoping_IngressParticipant_DoesNotBlockNonMesh(t *testing.T) {
	// MinIO/Scylla/node-agent are not mesh participants — they must NOT
	// be blocked by an ingress_participant violation.
	for _, name := range []string{"minio", "scylladb", "node-agent", "repository"} {
		if !driftActionSafe(infraAction(name), ingressParticipant) {
			t.Errorf("package %q must NOT be blocked by ingress_participant", name)
		}
	}
}

func TestTopologyScoping_ControllerPlacement_BlocksController(t *testing.T) {
	// cluster-controller upgrade must respect controller_placement.
	if driftActionSafe(infraAction("cluster-controller"), controllerPlacement) {
		t.Fatal("cluster-controller must be blocked by controller_placement")
	}
}

func TestTopologyScoping_ControllerPlacement_DoesNotBlockOthers(t *testing.T) {
	// controller_placement is controller-specific; other packages
	// must not be blocked by it.
	for _, name := range []string{"minio", "envoy", "node-agent", "repository"} {
		if !driftActionSafe(infraAction(name), controllerPlacement) {
			t.Errorf("package %q must NOT be blocked by controller_placement", name)
		}
	}
}

func TestTopologyScoping_ObjectstoreMismatch_BlocksMinIO(t *testing.T) {
	// objectstore_topology_mismatch is a MinIO-specific failure mode.
	if driftActionSafe(infraAction("minio"), objectstoreTopology) {
		t.Fatal("MinIO must be blocked by objectstore_topology_mismatch")
	}
	// And does not block non-storage packages.
	if !driftActionSafe(infraAction("node-agent"), objectstoreTopology) {
		t.Error("node-agent must NOT be blocked by objectstore_topology_mismatch")
	}
}

func TestTopologyScoping_UnknownPackage_DefaultsConservative(t *testing.T) {
	// New, unclassified package names must default to "any violation
	// blocks" — the conservative branch. This catches misnames and
	// new packages that haven't been classified yet.
	if driftActionSafe(infraAction("brand-new-package-name"), storageQuorum) {
		t.Fatal("unknown package must default conservative (any violation blocks) — got allowed")
	}
}

func TestTopologyScoping_KnownControlPlane_AllowedThroughAnyViolation(t *testing.T) {
	// A known control-plane package (sensitivities=nil) is allowed
	// through regardless of which violation kind fired.
	for _, kind := range []string{
		"storage_quorum", "ingress_participant",
		"controller_placement", "objectstore_topology_mismatch",
	} {
		v := []topologySafetyViolation{{Kind: kind, Message: "test"}}
		if !driftActionSafe(infraAction("node-agent"), v) {
			t.Errorf("node-agent blocked by violation kind %q — control-plane must be exempt", kind)
		}
	}
}

func TestTopologyScoping_KnownControlPlane_AllowedThroughMultipleViolations(t *testing.T) {
	// Multiple simultaneous violations — control-plane stays allowed.
	v := []topologySafetyViolation{
		{Kind: "storage_quorum", Message: "test"},
		{Kind: "ingress_participant", Message: "test"},
		{Kind: "controller_placement", Message: "test"},
	}
	if !driftActionSafe(infraAction("node-agent"), v) {
		t.Fatal("node-agent must remain allowed even under multiple violations")
	}
}

func TestTopologyScoping_StorageSensitive_OnlyBlockedByOwnViolation(t *testing.T) {
	// MinIO must NOT be blocked by ingress_participant — it doesn't
	// participate in mesh routing. Symmetry check for the per-violation
	// scoping.
	if !driftActionSafe(infraAction("minio"), ingressParticipant) {
		t.Fatal("MinIO must not be blocked by ingress_participant — not a mesh participant")
	}
	if !driftActionSafe(infraAction("minio"), controllerPlacement) {
		t.Fatal("MinIO must not be blocked by controller_placement — not the controller")
	}
}

func TestTopologyScoping_SafeKindIgnoresAllViolations(t *testing.T) {
	// SERVICE and COMMAND drift actions must continue to bypass the
	// gate entirely — this is the pre-Phase-35 invariant Phase 32
	// already pinned, re-asserted here for completeness.
	v := []topologySafetyViolation{
		{Kind: "storage_quorum", Message: "x"},
		{Kind: "ingress_participant", Message: "y"},
	}
	for _, kind := range []string{"SERVICE", "COMMAND"} {
		a := driftAction{
			NodeID:     "node-1",
			PackageKey: kind + "/authentication",
			Kind:       kind,
			ActionKind: classifyDriftAction(kind),
		}
		if !driftActionSafe(a, v) {
			t.Errorf("%s drift action must always be safe", kind)
		}
	}
}

func TestPackageTopologySensitivities_CaseInsensitive(t *testing.T) {
	// Package names from the workflow / desired state can arrive in
	// mixed case (proto enums, config files). Lookup must be case-
	// insensitive.
	for _, name := range []string{"MinIO", "MINIO", "minio", "Minio"} {
		s, known := packageTopologySensitivities(name)
		if !known {
			t.Errorf("packageTopologySensitivities(%q) reported unknown", name)
		}
		if !s["storage_quorum"] {
			t.Errorf("packageTopologySensitivities(%q) did not include storage_quorum", name)
		}
	}
}

func TestPackageTopologySensitivities_UnknownKnownStateMatrix(t *testing.T) {
	// Pin the (known, sensitivities) tri-state contract.
	cases := []struct {
		name      string
		known     bool
		hasGate   bool
		hasKind   string
	}{
		{"minio", true, true, "storage_quorum"},
		{"envoy", true, true, "ingress_participant"},
		{"cluster-controller", true, true, "controller_placement"},
		{"node-agent", true, false, ""}, // control-plane, no gates
		{"repository", true, false, ""}, // control-plane, no gates
		{"etcd", true, false, ""},       // own quorum, no storage_quorum
		{"definitely-not-a-package", false, false, ""}, // unknown
	}
	for _, tc := range cases {
		s, known := packageTopologySensitivities(tc.name)
		if known != tc.known {
			t.Errorf("%s: known=%v want %v", tc.name, known, tc.known)
		}
		if tc.hasGate {
			if s == nil {
				t.Errorf("%s: expected sensitivities map, got nil", tc.name)
			} else if !s[tc.hasKind] {
				t.Errorf("%s: expected gate on %q", tc.name, tc.hasKind)
			}
		} else {
			if s != nil {
				t.Errorf("%s: expected nil sensitivities (no gate), got %v", tc.name, s)
			}
		}
	}
}
