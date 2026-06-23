package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// canonicalLanes / freshnessModes mirror AWG core's validateRuntimeSnapshot
// vocabulary (cmd/awg/cmd_runtime.go). The collector is the platform adapter, so
// these tests enforce the target schema locally; the cross-boundary guarantee is
// the live `awg runtime-snapshot validate` step.
var rtCanonicalLanes = map[string]bool{
	"desired_state": true, "observed_state": true, "runtime_identity": true,
	"diagnosis": true, "health": true, "topology": true, "quorum": true,
	"release_state": true, "action_trace": true, "performance": true,
}
var rtFreshnessModes = map[string]bool{
	"fresh": true, "stale": true, "cache_only": true, "unknown": true, "unavailable": true,
}

// provenEvidence models the live `event` case: release boundary PROVEN end to
// end (desired == installed == running), node converged.
func provenEvidence() collectedEvidence {
	return collectedEvidence{
		SubjectType: "service", SubjectID: "event", Node: "nodeA",
		ClusterID: "c1", Platform: "globular", GeneratedAt: "2026-06-23T00:00:00Z",
		DesiredReachable: true, DesiredFound: true, DesiredVersion: "1.2.233", DesiredBuildNumber: 1,
		BoundaryReachable: true, BoundaryVerdict: "PROVEN", BoundaryBuildID: "bid-1",
		DesiredBuildID: "bid-1", InstalledBuildID: "bid-1", InstalledPresent: true,
		RunningExeSHA: "sha-1", Running: true, Checksum: "sha-1",
		DoctorReachable: true, DoctorFreshMode: "FRESHNESS_CACHED", DoctorAgeSeconds: 10,
		HealthReachable: true, NodeConverged: true, NodeStatus: "healthy",
		TopologyReachable: true, NodeCount: 1, StorageNodeCount: 1,
	}
}

// indeterminateEvidence models the live `sidekick` case: release boundary
// INDETERMINATE (installed-package evidence missing / build_id unpinned) even
// though a process runs. Identity cannot be proven.
func indeterminateEvidence() collectedEvidence {
	return collectedEvidence{
		SubjectType: "service", SubjectID: "sidekick", Node: "nodeA",
		ClusterID: "c1", Platform: "globular", GeneratedAt: "2026-06-23T00:00:00Z",
		DesiredReachable: true, DesiredFound: true, DesiredVersion: "7.0.0", DesiredBuildNumber: 1,
		BoundaryReachable: true, BoundaryVerdict: "INDETERMINATE",
		InstalledPresent: false, RunningExeSHA: "sha-x", Running: true,
		DoctorReachable: true, DoctorFreshMode: "FRESHNESS_CACHED", DoctorAgeSeconds: 10,
		HealthReachable: true, NodeConverged: true, NodeStatus: "healthy",
		TopologyReachable: true, NodeCount: 1, StorageNodeCount: 1,
	}
}

// assertWellFormed enforces the runtime-evidence/v1 shape rules AWG core checks:
// known lanes, valid freshness enum, owner required, schema_version pinned.
func assertWellFormed(t *testing.T, snap rtSnapshot) {
	t.Helper()
	if snap.SchemaVersion != runtimeEvidenceSchemaVersion {
		t.Fatalf("schema_version=%q want %q", snap.SchemaVersion, runtimeEvidenceSchemaVersion)
	}
	if snap.Platform == "" || snap.GeneratedAt == "" || snap.Subject.ID == "" {
		t.Fatalf("missing required top-level field: %+v", snap)
	}
	if len(snap.Lanes) == 0 {
		t.Fatal("snapshot has no lanes")
	}
	for name, lane := range snap.Lanes {
		if !rtCanonicalLanes[name] {
			t.Errorf("lane %q is not a canonical AWG lane", name)
		}
		if !rtFreshnessModes[lane.Freshness] {
			t.Errorf("lane %q freshness=%q is not a valid mode", name, lane.Freshness)
		}
		if strings.TrimSpace(lane.Owner) == "" {
			t.Errorf("lane %q has no owner (authority anchor required)", name)
		}
	}
}

func TestBuildRuntimeSnapshot_ProvenIsWellFormedAndFresh(t *testing.T) {
	snap := buildRuntimeSnapshot(provenEvidence())
	assertWellFormed(t, snap)

	id := snap.Lanes["runtime_identity"]
	if id.Freshness != "fresh" {
		t.Fatalf("PROVEN identity freshness=%q, want fresh", id.Freshness)
	}
	if got := id.Facts["identity_proven"]; got != true {
		t.Fatalf("PROVEN identity_proven=%v, want true", got)
	}
	// All required lanes must be fresh for AWG to be able to certify convergence.
	for _, name := range snap.VerdictInputs.RequiredLanes {
		if snap.Lanes[name].Freshness != "fresh" {
			t.Fatalf("required lane %q freshness=%q, want fresh", name, snap.Lanes[name].Freshness)
		}
	}
}

// THE honesty invariant: an INDETERMINATE release boundary must NEVER produce a
// fresh runtime_identity lane, and runtime_identity must be a required lane — so
// AWG's diagnoser refuses to green a subject whose identity cannot be proven.
func TestBuildRuntimeSnapshot_IndeterminateNeverFresh(t *testing.T) {
	snap := buildRuntimeSnapshot(indeterminateEvidence())
	assertWellFormed(t, snap)

	id := snap.Lanes["runtime_identity"]
	if id.Freshness == "fresh" {
		t.Fatal("INDETERMINATE identity must not be fresh — would let AWG falsely certify convergence")
	}
	if id.Freshness != "unknown" {
		t.Fatalf("INDETERMINATE identity freshness=%q, want unknown", id.Freshness)
	}
	if got := id.Facts["identity_proven"]; got != false {
		t.Fatalf("INDETERMINATE identity_proven=%v, want false", got)
	}
	requiresIdentity := false
	for _, name := range snap.VerdictInputs.RequiredLanes {
		if name == "runtime_identity" {
			requiresIdentity = true
		}
	}
	if !requiresIdentity {
		t.Fatal("runtime_identity must be a required lane so unprovable identity blocks green")
	}
}

func TestIdentityFreshness_VerdictMapping(t *testing.T) {
	cases := map[string]string{
		"PROVEN": "fresh", "FAILED": "fresh",
		"INDETERMINATE": "unknown", "NOT_APPLICABLE": "unknown", "": "unknown",
	}
	for verdict, want := range cases {
		if got := identityFreshness(true, verdict); got != want {
			t.Errorf("identityFreshness(reachable, %q)=%q, want %q", verdict, got, want)
		}
	}
	if got := identityFreshness(false, "PROVEN"); got != "unavailable" {
		t.Errorf("unreachable boundary freshness=%q, want unavailable", got)
	}
}

// An unreachable owner source must become an unavailable lane — never silently
// dropped and never fresh. (meta: a tool failure is evidence, not silence.)
func TestBuildRuntimeSnapshot_UnreachableSourceIsUnavailableNotFresh(t *testing.T) {
	ev := provenEvidence()
	ev.DesiredReachable = false // controller unreachable
	ev.HealthReachable = false  // health unreachable
	snap := buildRuntimeSnapshot(ev)
	assertWellFormed(t, snap)

	if ds := snap.Lanes["desired_state"]; ds.Freshness != "unavailable" || ds.Status != "unavailable" {
		t.Fatalf("unreachable desired_state lane=%+v, want freshness/status unavailable", ds)
	}
	if h := snap.Lanes["health"]; h.Freshness != "unavailable" {
		t.Fatalf("unreachable health freshness=%q, want unavailable", h.Freshness)
	}
}

func TestDoctorSeverityToLane_ErrorBecomesBlocking(t *testing.T) {
	cases := map[string]string{
		"critical": "critical", "error": "blocking",
		"warn": "warning", "warning": "warning", "info": "info", "noise": "info",
	}
	for in, want := range cases {
		if got := doctorSeverityToLane(in); got != want {
			t.Errorf("doctorSeverityToLane(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestDoctorFindingsCarryIntoDiagnosisLane(t *testing.T) {
	ev := provenEvidence()
	ev.DoctorFindings = []collectedFinding{
		{ID: "f1", Severity: "error", Category: "artifact", Summary: "event boundary problem"},
	}
	snap := buildRuntimeSnapshot(ev)
	diag := snap.Lanes["diagnosis"]
	if len(diag.Findings) != 1 {
		t.Fatalf("diagnosis findings=%d, want 1", len(diag.Findings))
	}
	if diag.Findings[0].Severity != "blocking" {
		t.Fatalf("doctor error finding mapped to %q, want blocking", diag.Findings[0].Severity)
	}
}

// The snapshot must marshal to YAML and round-trip (it is written to a file the
// AWG CLI consumes).
func TestBuildRuntimeSnapshot_YAMLRoundTrips(t *testing.T) {
	snap := buildRuntimeSnapshot(provenEvidence())
	out, err := yaml.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back rtSnapshot
	if err := yaml.Unmarshal(out, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.SchemaVersion != snap.SchemaVersion || back.Subject.ID != snap.Subject.ID {
		t.Fatalf("round-trip mismatch: %+v", back)
	}
	if !strings.Contains(string(out), "runtime-evidence/v1") {
		t.Fatal("marshaled snapshot missing schema_version literal")
	}
}

// TestLiveProof_RuntimeSnapshotCollect runs the REAL collector handler against a
// LIVE Globular cluster and writes the resulting runtime-evidence/v1 snapshots to
// disk for the AWG spine to diagnose. Double-gated: requires AWG_LIVE_PROOF=1 AND
// a reachable cluster + node PKI (mTLS via the gateway client pool). Never runs in
// CI — a comment is not a gate; the env var is.
func TestLiveProof_RuntimeSnapshotCollect(t *testing.T) {
	if os.Getenv("AWG_LIVE_PROOF") != "1" {
		t.Skip("set AWG_LIVE_PROOF=1 to run the live collector proof (needs a live cluster + PKI)")
	}
	s := newServer(&MCPConfig{})
	registerRuntimeEvidenceTools(s)
	ctx := context.Background()
	for _, subj := range []string{"event", "sidekick"} {
		res, err := s.callTool(ctx, "runtime_snapshot_collect", map[string]interface{}{
			"subject_id": subj, "freshness": "fresh",
		})
		if err != nil {
			t.Fatalf("collect %s: %v", subj, err)
		}
		m, ok := res.(map[string]interface{})
		if !ok {
			t.Fatalf("collect %s: unexpected result type %T", subj, res)
		}
		yamlOut, _ := m["snapshot_yaml"].(string)
		if yamlOut == "" {
			t.Fatalf("collect %s: empty snapshot_yaml", subj)
		}
		path := filepath.Join(os.TempDir(), "awg-live-"+subj+".yaml")
		if err := os.WriteFile(path, []byte(yamlOut), 0o644); err != nil {
			t.Fatalf("write %s: %v", subj, err)
		}
		t.Logf("subject=%s boundary_verdict=%v lanes=%v -> %s", subj, m["boundary_verdict"], m["lane_count"], path)
	}
}

func TestFindingMentionsSubject(t *testing.T) {
	if !findingMentionsSubject("Release boundary for EVENT is INDETERMINATE", "event") {
		t.Error("expected case-insensitive subject match")
	}
	if findingMentionsSubject("unrelated finding", "event") {
		t.Error("did not expect a match")
	}
	if findingMentionsSubject("anything", "") {
		t.Error("empty subject must never match")
	}
}
