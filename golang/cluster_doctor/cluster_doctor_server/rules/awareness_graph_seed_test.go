package rules

import (
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// TestAwarenessGraphSeedEmpty_NotReachable verifies that when the
// awareness-graph service is not reachable, no finding is emitted.
func TestAwarenessGraphSeedEmpty_NotReachable(t *testing.T) {
	snap := &collector.Snapshot{
		AwarenessGraphReachable:  false,
		AwarenessGraphQueryEmpty: false,
	}
	inv := awarenessGraphSeedEmpty{}
	if got := inv.Evaluate(snap, Config{}); len(got) != 0 {
		t.Errorf("not reachable → no findings, got %d", len(got))
	}
}

// TestAwarenessGraphSeedEmpty_ReachableWithData verifies that when the service
// is reachable and has data, no finding is emitted.
func TestAwarenessGraphSeedEmpty_ReachableWithData(t *testing.T) {
	snap := &collector.Snapshot{
		AwarenessGraphReachable:  true,
		AwarenessGraphQueryEmpty: false,
	}
	inv := awarenessGraphSeedEmpty{}
	if got := inv.Evaluate(snap, Config{}); len(got) != 0 {
		t.Errorf("reachable with data → no findings, got %d", len(got))
	}
}

// TestAwarenessGraphSeedEmpty_ReachableAndEmpty verifies that when the service
// is reachable but the RDF store is empty, a WARN finding is emitted.
func TestAwarenessGraphSeedEmpty_ReachableAndEmpty(t *testing.T) {
	snap := &collector.Snapshot{
		AwarenessGraphReachable:  true,
		AwarenessGraphQueryEmpty: true,
	}
	inv := awarenessGraphSeedEmpty{}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.InvariantID != "awareness_graph.seed_empty" {
		t.Errorf("InvariantID = %q, want awareness_graph.seed_empty", f.InvariantID)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("Severity = %v, want SEVERITY_WARN", f.Severity)
	}
	if f.Category != "ai" {
		t.Errorf("Category = %q, want ai", f.Category)
	}
}

// TestAwarenessGraphSeedEmpty_FindingHasRemediation verifies that the finding
// includes at least two remediation steps (restart + verify).
func TestAwarenessGraphSeedEmpty_FindingHasRemediation(t *testing.T) {
	snap := &collector.Snapshot{
		AwarenessGraphReachable:  true,
		AwarenessGraphQueryEmpty: true,
	}
	inv := awarenessGraphSeedEmpty{}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) == 0 {
		t.Fatal("expected finding")
	}
	if len(findings[0].Remediation) < 2 {
		t.Errorf("expected at least 2 remediation steps, got %d", len(findings[0].Remediation))
	}
}

// TestAwarenessGraphSeedEmpty_PolicyIsPropose verifies that the heal policy
// for awareness_graph.seed_empty is HealPropose (never auto-execute).
func TestAwarenessGraphSeedEmpty_PolicyIsPropose(t *testing.T) {
	rule := LookupPolicy("awareness_graph.seed_empty")
	if rule.Disposition != HealPropose {
		t.Errorf("disposition = %v, want HealPropose", rule.Disposition)
	}
	if rule.AutoAction != "" {
		t.Errorf("AutoAction = %q, want empty (no auto-action for propose-only)", rule.AutoAction)
	}
}

// TestOpsKnowledgeSeedDeferred_PolicyIsPropose — Patch C Milestone 2
// demoted ops_knowledge.seed_deferred from HealAuto to HealPropose.
// The ai-memory upsert is cross-service and has no existing ActionType
// representation; until a workflow-or-action representation exists, the
// operator drives this from the CLI. Re-promotion happens (if at all)
// in Milestone 3 along with the necessary infrastructure.
func TestOpsKnowledgeSeedDeferred_PolicyIsPropose(t *testing.T) {
	rule := LookupPolicy("ops_knowledge.seed_deferred")
	if rule.Disposition != HealPropose {
		t.Errorf("disposition = %v, want HealPropose (M2 demotion)", rule.Disposition)
	}
	if rule.AutoAction != "" {
		t.Errorf("AutoAction = %q, want empty (no direct dispatch from healer)", rule.AutoAction)
	}
}
