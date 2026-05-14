package rules

// repository_dns_invariants_test.go — Targeted tests for the doctor rules
// that surface the 2026-05-14 composed-path failure.
//
// These tests exercise rule logic against constructed Snapshots; they do
// not touch live etcd. The repository.desired_build_ids_resolve rule does
// call readDesiredBuildIDs(), which reads etcd; in this unit-test context
// the etcd client is unconfigured so readDesiredBuildIDs returns an empty
// map and the rule emits no findings. We test the data-driven half: when
// a snapshot carries an installed build_id, no orphan is reported for it.
// The "active orphan emits finding" case is covered by integration tests
// that wire a real etcd; documented here for traceability.

import (
	"context"
	"strings"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ─────────────────────────────────────────────────────────────────────────
// dns.records_match_runtime_health
// ─────────────────────────────────────────────────────────────────────────

func TestDoctorRule_DNSRecordsMatchRuntimeHealth_PlannedNotInstalled(t *testing.T) {
	// hp-01 has profile=gateway but gateway is not installed → rule must
	// emit a finding marked dns.records_match_runtime_health.
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			{
				NodeId:   "hp-01",
				Status:   "ready",
				Profiles: []string{"gateway"},
				LastSeen: timestamppb.New(time.Now()),
			},
		},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"hp-01": {
				NodeId:            "hp-01",
				InstalledVersions: map[string]string{}, // nothing installed
			},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"hp-01": {Units: nil},
		},
	}
	findings := (dnsRecordsMatchRuntimeHealth{}).Evaluate(snap, testConfig())
	if len(findings) == 0 {
		t.Fatal("expected a finding when profile=gateway but service not installed")
	}
	found := false
	for _, f := range findings {
		if f.InvariantID == "dns.records_match_runtime_health" &&
			strings.Contains(f.Summary, "not installed") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected finding referencing 'not installed', got %d findings: %+v", len(findings), findings)
	}
}

func TestDoctorRule_DNSRecordsMatchRuntimeHealth_InstalledButInactive(t *testing.T) {
	// dell has gateway installed but unit inactive → rule must emit a finding.
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			{
				NodeId:   "dell",
				Status:   "ready",
				Profiles: []string{"gateway"},
				LastSeen: timestamppb.New(time.Now()),
			},
		},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"dell": {
				NodeId:            "dell",
				InstalledVersions: map[string]string{"gateway": "1.2.45"},
			},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"dell": {Units: []*node_agentpb.UnitStatus{
				{Name: "globular-gateway.service", State: "inactive"},
			}},
		},
	}
	findings := (dnsRecordsMatchRuntimeHealth{}).Evaluate(snap, testConfig())
	if len(findings) == 0 {
		t.Fatal("expected a finding when service installed but unit inactive")
	}
	found := false
	for _, f := range findings {
		if f.InvariantID == "dns.records_match_runtime_health" &&
			strings.Contains(f.Summary, "state=inactive") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected finding referencing 'state=inactive', got %d findings: %+v", len(findings), findings)
	}
}

func TestDoctorRule_DNSRecordsMatchRuntimeHealth_HealthyNode_NoFinding(t *testing.T) {
	// Healthy node: installed + unit active → no finding.
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			{
				NodeId:   "ryzen",
				Status:   "ready",
				Profiles: []string{"gateway"},
				LastSeen: timestamppb.New(time.Now()),
			},
		},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"ryzen": {
				NodeId:            "ryzen",
				InstalledVersions: map[string]string{"gateway": "1.2.45"},
			},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"ryzen": {Units: []*node_agentpb.UnitStatus{
				{Name: "globular-gateway.service", State: "active"},
			}},
		},
	}
	findings := (dnsRecordsMatchRuntimeHealth{}).Evaluate(snap, testConfig())
	for _, f := range findings {
		if f.InvariantID == "dns.records_match_runtime_health" {
			t.Errorf("healthy node should not produce a dns.records_match_runtime_health finding, got: %+v", f)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────
// repository.desired_build_ids_resolve
//
// These tests inject the desired-state reader so behavior is deterministic
// regardless of whether the host has live etcd. Each test restores the
// original reader on cleanup.
// ─────────────────────────────────────────────────────────────────────────

func withDesiredBuildIDs(t *testing.T, fn func(context.Context) map[string]string) {
	t.Helper()
	prev := desiredBuildIDsReader
	desiredBuildIDsReader = fn
	t.Cleanup(func() { desiredBuildIDsReader = prev })
}

func TestDoctorRule_RepositoryDesiredBuildIDsResolve_EmptyDesired_NoFinding(t *testing.T) {
	withDesiredBuildIDs(t, func(context.Context) map[string]string { return nil })

	snap := &collector.Snapshot{
		RepositoryBuildIDIndex: map[string]bool{"bid-A": true},
	}
	findings := (repositoryDesiredBuildIDsResolve{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("with no desired-state, rule must emit no findings; got %+v", findings)
	}
}

func TestDoctorRule_RepositoryDesiredBuildIDsResolve_NoRepoIndex_NoFinding(t *testing.T) {
	// When the collector couldn't get a repository build_id index (nil), the
	// rule degrades to silent — false positives are worse than false silence
	// for a CRITICAL-severity rule. This guards against the v1.2.47 false-
	// positive storm where the rule fired on legitimate Day-1 pins.
	withDesiredBuildIDs(t, func(context.Context) map[string]string {
		return map[string]string{"bid-X": "/globular/resources/ServiceRelease/core/echo"}
	})

	snap := &collector.Snapshot{
		// RepositoryBuildIDIndex is intentionally nil — collector had no signal.
	}
	findings := (repositoryDesiredBuildIDsResolve{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("nil RepositoryBuildIDIndex must produce no findings; got %+v", findings)
	}
}

func TestDoctorRule_RepositoryDesiredBuildIDsResolve_OrphanFires(t *testing.T) {
	// Desired pins bid-X. Repository's authoritative index does NOT contain it.
	// → orphan → CRITICAL finding.
	withDesiredBuildIDs(t, func(context.Context) map[string]string {
		return map[string]string{"bid-X": "/globular/resources/ServiceRelease/core/echo"}
	})

	snap := &collector.Snapshot{
		// Repository has artifacts, but bid-X is NOT among them.
		RepositoryBuildIDIndex: map[string]bool{"bid-OTHER": true},
	}
	findings := (repositoryDesiredBuildIDsResolve{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 orphan finding, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.InvariantID != "repository.desired_build_ids_resolve" {
		t.Errorf("wrong invariant_id: %q", f.InvariantID)
	}
	if !strings.Contains(f.Summary, "DesiredBuildIdOrphaned") {
		t.Errorf("expected summary to carry DesiredBuildIdOrphaned, got %q", f.Summary)
	}
	if !strings.Contains(f.Summary, "bid-X") {
		t.Errorf("expected summary to mention orphaned build_id, got %q", f.Summary)
	}
}

func TestDoctorRule_RepositoryDesiredBuildIDsResolve_RepositoryHasIt_NoFinding(t *testing.T) {
	// Desired pins bid-X. Repository CAN resolve bid-X. → no finding even if
	// no node has installed it yet (legitimate Day-1 / first-publish window).
	withDesiredBuildIDs(t, func(context.Context) map[string]string {
		return map[string]string{"bid-X": "/globular/resources/ServiceRelease/core/echo"}
	})

	snap := &collector.Snapshot{
		RepositoryBuildIDIndex: map[string]bool{"bid-X": true},
		// Critically: NodeHealths has no installs yet. This used to false-
		// positive on every Day-1 bootstrap.
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"nuc": {NodeId: "nuc", InstalledBuildIds: map[string]string{}},
		},
	}
	findings := (repositoryDesiredBuildIDsResolve{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("repository can resolve bid-X — rule must not fire even with zero installs. Got %+v", findings)
	}
}

