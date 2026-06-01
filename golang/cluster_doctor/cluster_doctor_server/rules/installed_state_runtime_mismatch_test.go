package rules

import (
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func freshNodeRecord(id string) *cluster_controllerpb.NodeRecord {
	return &cluster_controllerpb.NodeRecord{
		NodeId:   id,
		Status:   "ready",
		LastSeen: timestamppb.New(time.Now()),
	}
}

func freshNodeHealth(id string, pkgs map[string]string) *cluster_controllerpb.NodeHealth {
	return &cluster_controllerpb.NodeHealth{
		NodeId:            id,
		InstalledVersions: pkgs,
	}
}

func inventoryWithUnits(units ...*node_agentpb.UnitStatus) *node_agentpb.Inventory {
	return &node_agentpb.Inventory{Units: units}
}

func unit(name, state string) *node_agentpb.UnitStatus {
	return &node_agentpb.UnitStatus{Name: name, State: state}
}

// W03: COMMAND packages must not require systemd units.

func TestInstalledStateRuntimeMismatch_CommandPackage_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"rclone": "1.65.0"})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits()}, // no units
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("COMMAND package rclone must not require a systemd unit, got %d findings", len(findings))
	}
}

func TestInstalledStateRuntimeMismatch_DaemonMissingUnit_FindingFired(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"keepalived": "0.0.1"})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits()}, // no units at all
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for missing keepalived unit, got %d", len(findings))
	}
	if findings[0].InvariantID != "installed_state_runtime_mismatch" {
		t.Errorf("wrong invariant_id: %s", findings[0].InvariantID)
	}
}

func TestInstalledStateRuntimeMismatch_DaemonActiveUnit_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"keepalived": "0.0.1"})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits(unit("keepalived.service", "active"))},
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected no finding for active keepalived unit, got %d", len(findings))
	}
}

func TestInstalledStateRuntimeMismatch_DaemonFailedUnit_FindingFired(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"keepalived": "0.0.1"})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits(unit("keepalived.service", "failed"))},
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for failed keepalived unit, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("expected WARN for failed unit on ready node, got %v", findings[0].Severity)
	}
}

func TestInstalledStateRuntimeMismatch_UnhealthyNode_IsError(t *testing.T) {
	node := &cluster_controllerpb.NodeRecord{
		NodeId:   "n1",
		Status:   "degraded",
		LastSeen: timestamppb.New(time.Now()),
	}
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{node},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"mcp": "1.0.0"})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits()},
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding on degraded node, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("expected ERROR for unhealthy node, got %v", findings[0].Severity)
	}
}

func TestInstalledStateRuntimeMismatch_StaleHeartbeat_FindingFired(t *testing.T) {
	staleNode := &cluster_controllerpb.NodeRecord{
		NodeId:   "n1",
		Status:   "ready",
		LastSeen: timestamppb.New(time.Now().Add(-10 * time.Minute)), // 10 min stale
	}
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{staleNode},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"mcp": "1.0.0"})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits(unit("globular-mcp.service", "active"))},
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for stale heartbeat, got %d", len(findings))
	}
	if findings[0].Summary == "" {
		t.Error("expected non-empty summary for stale finding")
	}
}

func TestInstalledStateRuntimeMismatch_EmptyVersion_NoFinding(t *testing.T) {
	// Packages with empty version should be ignored (no false positives).
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"mcp": ""})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits()},
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected no finding for empty version, got %d", len(findings))
	}
}

func TestInstalledStateRuntimeMismatch_NoInventory_NoFinding(t *testing.T) {
	// Node without inventory entry: rule should skip gracefully.
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"mcp": "1.0.0"})},
		Inventories: map[string]*node_agentpb.Inventory{}, // no inventory for n1
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected no finding when inventory is absent, got %d", len(findings))
	}
}

func TestPackageIsCommand_FallbackList(t *testing.T) {
	known := []string{
		"rclone", "restic", "mc", "sctool", "etcdctl", "ffmpeg", "globular-cli",
		"cli", "sha256sum", "yt-dlp", "claude",
	}
	for _, name := range known {
		if !packageIsCommand(name, nil) {
			t.Errorf("expected %q to be classified as a command package via fallback", name)
		}
	}
}

func TestPackageIsCommand_EtcdKindWins(t *testing.T) {
	// A package recorded as COMMAND in etcd is skipped even if absent from static list.
	kinds := map[string]string{"new-tool": "COMMAND"}
	if !packageIsCommand("new-tool", kinds) {
		t.Error("etcd COMMAND kind must mark package as command")
	}

	// Static list is authoritative for known command packages — a stale INFRASTRUCTURE
	// or SERVICE etcd entry (from a pre-kind-sidecar install) must NOT cause the
	// rule to fire a spurious incident for a definitionally command-only package.
	// rclone has no systemd unit by design; no etcd kind can override that.
	kinds2 := map[string]string{"rclone": "SERVICE"}
	if !packageIsCommand("rclone", kinds2) {
		t.Error("static list must win over stale non-COMMAND etcd kind for known command packages")
	}
	kinds3 := map[string]string{"mc": "INFRASTRUCTURE"}
	if !packageIsCommand("mc", kinds3) {
		t.Error("mc must not fire when etcd has stale INFRASTRUCTURE kind from pre-kind-sidecar install")
	}
}

func TestInstalledStateRuntimeMismatch_CommandPackages_NoFinding(t *testing.T) {
	pkgs := map[string]string{
		"cli":       "1.2.21",
		"sha256sum": "9.4.0",
		"yt-dlp":    "2026.2.21",
		"claude":    "2.1.80",
	}
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", pkgs)},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits()},
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("CLI tool packages must not require systemd units, got %d findings", len(findings))
	}
}

func TestInstalledStateRuntimeMismatch_EtcdCommandKind_NoFinding(t *testing.T) {
	// Packages recorded as COMMAND in etcd must never fire, even if not in fallback list.
	pkgs := map[string]string{"brand-new-tool": "1.0.0"}
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", pkgs)},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits()},
		NodePackageKinds: map[string]map[string]string{
			"n1": {"brand-new-tool": "COMMAND"},
		},
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("COMMAND package from etcd must not require a systemd unit, got %d findings", len(findings))
	}
}

func TestPackageIsCommand_DaemonIsNotCommand(t *testing.T) {
	daemons := []string{"keepalived", "mcp", "cluster-controller", "node-agent", "scylladb"}
	for _, name := range daemons {
		if packageIsCommand(name, nil) {
			t.Errorf("expected %q NOT to be classified as a command package", name)
		}
	}
}

// ── Ingress-gated keepalived suppression ──────────────────────────────────
//
// keepalived is opt-in: it runs only when the cluster's ingress spec at
// /globular/ingress/v1/spec declares a VIP. The Day-0 bootstrap writes
// `mode: "disabled"` until an operator runs `globular cluster network ...`.
// In that disabled state, keepalived MUST be installed (so it can be
// enabled later without a re-install) but MUST NOT be running. Firing
// installed_state_runtime_mismatch in that state is a false positive that
// fills the dashboard with ERROR severity on a healthy-by-design cluster.

func TestInstalledStateRuntimeMismatch_KeepalivedSilentWhenIngressDisabled(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"keepalived": "0.0.1"})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits(unit("keepalived.service", "inactive"))},
		// Day-0 bootstrap default — mode disabled, explicit_disabled true.
		IngressSpecPresent: true,
		IngressSpecRaw:     `{"version":"v1","mode":"disabled","explicit_disabled":true,"reason":"day0 bootstrap default"}`,
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("keepalived inactive must be silent when ingress is disabled; got %d findings: %+v", len(findings), findings)
	}
}

func TestInstalledStateRuntimeMismatch_KeepalivedFiresWhenIngressActive(t *testing.T) {
	// Mirror image of the disabled case: when ingress is configured (mode is
	// anything other than disabled, and explicit_disabled is false), an
	// inactive keepalived IS a real convergence problem and the rule MUST fire.
	snap := &collector.Snapshot{
		Nodes:              []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths:        map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"keepalived": "0.0.1"})},
		Inventories:        map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits(unit("keepalived.service", "inactive"))},
		IngressSpecPresent: true,
		IngressSpecRaw:     `{"version":"v1","mode":"vip","explicit_disabled":false,"vip":"10.0.0.100"}`,
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) == 0 {
		t.Fatal("keepalived inactive with ingress active is a real mismatch — rule MUST fire")
	}
	if findings[0].InvariantID != "installed_state_runtime_mismatch" {
		t.Errorf("wrong invariant id: %q", findings[0].InvariantID)
	}
}

func TestInstalledStateRuntimeMismatch_KeepalivedFiresWhenIngressSpecMissing(t *testing.T) {
	// Fail-open: when the ingress spec is missing or unreadable, do NOT
	// silently suppress the keepalived finding. Missing spec is a SEPARATE
	// problem (collector should have written it during Day-0), and pretending
	// keepalived is fine because we couldn't read the spec would mask both bugs.
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"keepalived": "0.0.1"})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits(unit("keepalived.service", "inactive"))},
		// IngressSpecPresent intentionally false — spec wasn't read.
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) == 0 {
		t.Error("missing ingress spec must NOT suppress the finding; fail-open is the contract")
	}
}

func TestInstalledStateRuntimeMismatch_ActivatingSuppressedDuringFreshDrift(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"search": "1.2.131"})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits(unit("globular-search.service", "activating"))},
		NodeDriftAge: map[string]time.Duration{
			"n1": 30 * time.Second,
		},
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Fatalf("activating during fresh drift should be suppressed, got %d findings: %+v", len(findings), findings)
	}
}

func TestInstalledStateRuntimeMismatch_ActivatingFiresWhenDriftPersists(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"search": "1.2.131"})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits(unit("globular-search.service", "activating"))},
		NodeDriftAge: map[string]time.Duration{
			"n1": 3 * time.Minute,
		},
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) == 0 {
		t.Fatal("activating beyond grace window must fire mismatch finding")
	}
}
