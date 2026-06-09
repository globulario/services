// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.invariant_registry_harvest_test
// @awareness file_role=tests_registry_level_reduced_harvest_finding_annotation
// @awareness enforces=globular.platform:invariant.doctor_rule_evaluate_must_consult_snap_errors
// @awareness risk=high
package rules

import (
	"errors"
	"strings"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// stubInvariant is a no-op rule that returns a single hard-coded
// finding. Used to test the registry wrapper's annotation behavior
// without depending on any real rule's data model.
type stubInvariant struct {
	scope string
}

func (s stubInvariant) ID() string       { return "stub" }
func (s stubInvariant) Category() string { return "test" }
func (s stubInvariant) Scope() string    { return s.scope }
func (s stubInvariant) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	return []Finding{
		{
			FindingID:   "stub-id",
			InvariantID: "stub",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "test",
			EntityRef:   "cluster",
			Summary:     "stub finding summary",
		},
	}
}

// TestEvaluateAll_CompleteSnapshot_NoAnnotation is the baseline.
// When the snapshot has no DataErrors, findings come back unmodified.
// The wrapper exists but is a no-op.
func TestEvaluateAll_CompleteSnapshot_NoAnnotation(t *testing.T) {
	r := &Registry{
		invariants: []Invariant{stubInvariant{scope: "cluster"}},
		cfg:        Config{},
	}
	snap := &collector.Snapshot{} // empty = complete (no errors)

	findings := r.EvaluateAll(snap)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if strings.Contains(findings[0].Summary, "reduced-harvest") {
		t.Errorf("complete snapshot must NOT produce reduced-harvest annotation; got Summary=%q", findings[0].Summary)
	}
	if findings[0].Summary != "stub finding summary" {
		t.Errorf("Summary modified unexpectedly: %q", findings[0].Summary)
	}
	for _, ev := range findings[0].Evidence {
		if ev != nil && ev.GetSourceRpc() == "reduced_harvest" {
			t.Errorf("complete snapshot must NOT add reduced_harvest evidence")
		}
	}
}

// TestEvaluateAll_PartialSnapshot_FindingAnnotated is the headline
// regression. When at least one collector sub-fetch errored, the
// snapshot carries DataIncomplete=true and the registry MUST
// annotate every finding produced this cycle. INC-2026-0004 was
// the absence of this behavior at a single rule; the registry-level
// wrapper closes it for all rules at once.
func TestEvaluateAll_PartialSnapshot_FindingAnnotated(t *testing.T) {
	r := &Registry{
		invariants: []Invariant{stubInvariant{scope: "cluster"}},
		cfg:        Config{},
	}
	snap := &collector.Snapshot{
		DataIncomplete: true,
		DataErrors: []collector.DataError{
			{Service: "cluster_controller", RPC: "ListNodes", Err: errors.New("timeout")},
		},
	}

	// EvaluateAll now returns the annotated rule finding PLUS one
	// INVARIANT_UNKNOWN "source unavailable" finding per missing source
	// (the structural fix for the silent-false-negative class). Split them.
	stub, srcUnavailable := splitSourceUnavailable(r.EvaluateAll(snap))
	if len(stub) != 1 {
		t.Fatalf("expected 1 rule finding, got %d", len(stub))
	}
	if len(srcUnavailable) != 1 {
		t.Fatalf("expected 1 source-unavailable finding, got %d", len(srcUnavailable))
	}
	if srcUnavailable[0].InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN {
		t.Errorf("source-unavailable finding must be INVARIANT_UNKNOWN, got %v", srcUnavailable[0].InvariantStatus)
	}
	if srcUnavailable[0].EntityRef != "cluster_controller.ListNodes" {
		t.Errorf("source-unavailable EntityRef = %q, want cluster_controller.ListNodes", srcUnavailable[0].EntityRef)
	}
	findings := stub
	if !strings.HasPrefix(findings[0].Summary, "[reduced-harvest] ") {
		t.Errorf("partial snapshot did not annotate Summary; got %q", findings[0].Summary)
	}
	if !strings.Contains(findings[0].Summary, "stub finding summary") {
		t.Errorf("annotation replaced rather than prepended; got %q", findings[0].Summary)
	}

	foundHarvestEvidence := false
	for _, ev := range findings[0].Evidence {
		if ev == nil {
			continue
		}
		if ev.GetSourceRpc() == "reduced_harvest" {
			foundHarvestEvidence = true
			items := ev.GetKeyValues()
			if items["missing_sources"] != "cluster_controller.ListNodes" {
				t.Errorf("missing_sources = %q, want cluster_controller.ListNodes", items["missing_sources"])
			}
			if items["missing_sources_count"] != "1" {
				t.Errorf("missing_sources_count = %q, want 1", items["missing_sources_count"])
			}
			if items["explanation"] == "" {
				t.Errorf("explanation must not be empty")
			}
			break
		}
	}
	if !foundHarvestEvidence {
		t.Errorf("partial snapshot did not add reduced_harvest evidence")
	}
}

// TestEvaluateAll_MultipleErrors_AllListedInEvidence pins the
// behavior where the evidence row carries every missing source,
// sorted alphabetically and deduplicated.
func TestEvaluateAll_MultipleErrors_AllListedInEvidence(t *testing.T) {
	r := &Registry{
		invariants: []Invariant{stubInvariant{scope: "cluster"}},
		cfg:        Config{},
	}
	snap := &collector.Snapshot{
		DataIncomplete: true,
		DataErrors: []collector.DataError{
			{Service: "cluster_controller", RPC: "ListNodes", Err: errors.New("timeout")},
			{Service: "etcd", RPC: "LoadObjectStoreDesiredState", Err: errors.New("not found")},
			{Service: "cluster_controller", RPC: "ListNodes", Err: errors.New("timeout")}, // duplicate
		},
	}

	stub, srcUnavailable := splitSourceUnavailable(r.EvaluateAll(snap))
	if len(stub) != 1 {
		t.Fatalf("expected 1 rule finding, got %d", len(stub))
	}
	// One source-unavailable finding per distinct missing source (deduped).
	if len(srcUnavailable) != 2 {
		t.Fatalf("expected 2 source-unavailable findings (one per distinct source), got %d", len(srcUnavailable))
	}
	for _, ev := range stub[0].Evidence {
		if ev == nil || ev.GetSourceRpc() != "reduced_harvest" {
			continue
		}
		items := ev.GetKeyValues()
		want := "cluster_controller.ListNodes, etcd.LoadObjectStoreDesiredState"
		if items["missing_sources"] != want {
			t.Errorf("missing_sources = %q, want %q", items["missing_sources"], want)
		}
		if items["missing_sources_count"] != "2" {
			t.Errorf("missing_sources_count = %q, want 2 (after dedup)", items["missing_sources_count"])
		}
		return
	}
	t.Errorf("no reduced_harvest evidence in finding")
}

// TestClusterScopedRulesRefuseOnSourceError verifies the per-rule guards added
// 2026-06-09: when the cluster_controller source errored, the etcd/services
// rules whose entire basis is that source return nil (refuse to answer) rather
// than reading an empty snap.Nodes / snap.NodeHealths as "nothing wrong". An
// errored source with NON-empty stale data must still be refused — the guard
// keys off snap.HadError, not emptiness.
func TestClusterScopedRulesRefuseOnSourceError(t *testing.T) {
	snap := &collector.Snapshot{
		DataIncomplete: true,
		DataErrors: []collector.DataError{
			{Service: "cluster_controller", RPC: "ListNodes", Err: errors.New("timeout")},
			{Service: "cluster_controller", RPC: "GetClusterHealthV1", Err: errors.New("timeout")},
		},
	}
	cfg := Config{}
	guarded := []Invariant{
		etcdQuorumHealth{}, staleNodeDetection{}, bootstrapPhaseStuck{}, nodeAgentCrash{},
		clusterServicesDrift{},
	}
	for _, inv := range guarded {
		if got := inv.Evaluate(snap, cfg); got != nil {
			t.Errorf("%s.Evaluate must return nil when its source errored, got %d finding(s)", inv.ID(), len(got))
		}
	}
}

// TestSnapshot_HadError_Filters covers the opt-in API rules can call
// to consult errors directly. Empty service/rpc are wildcards.
func TestSnapshot_HadError_Filters(t *testing.T) {
	snap := &collector.Snapshot{
		DataIncomplete: true,
		DataErrors: []collector.DataError{
			{Service: "cluster_controller", RPC: "ListNodes", Err: errors.New("x")},
			{Service: "etcd", RPC: "LoadFoo", Err: errors.New("y")},
		},
	}
	if !snap.HadError("cluster_controller", "ListNodes") {
		t.Errorf("exact match should return true")
	}
	if !snap.HadError("cluster_controller", "") {
		t.Errorf("empty rpc should match any RPC under service")
	}
	if !snap.HadError("", "ListNodes") {
		t.Errorf("empty service should match any service")
	}
	if !snap.HadError("", "") {
		t.Errorf("both empty should match any error")
	}
	if snap.HadError("not_a_service", "") {
		t.Errorf("unknown service should return false")
	}
	if snap.HadError("cluster_controller", "WrongRPC") {
		t.Errorf("exact mismatch should return false")
	}
}

// TestEvaluateAll_EmptyFindings_SurfacesSourceUnavailable is the headline
// regression for the silent-false-negative class triaged 2026-06-09: a rule
// whose only source errored produces NO finding, so the [reduced-harvest]
// annotation has nothing to tag. Before the fix EvaluateAll returned zero
// findings here — a masked outage. Now it must surface the unavailable source
// as a first-class INVARIANT_UNKNOWN finding so "could not see" is never
// indistinguishable from "healthy". Must not panic on the empty-findings path.
func TestEvaluateAll_EmptyFindings_SurfacesSourceUnavailable(t *testing.T) {
	r := &Registry{
		invariants: []Invariant{stubInvariantEmpty{}},
		cfg:        Config{},
	}
	snap := &collector.Snapshot{
		DataIncomplete: true,
		DataErrors:     []collector.DataError{{Service: "x", RPC: "y", Err: errors.New("z")}},
	}
	stub, srcUnavailable := splitSourceUnavailable(r.EvaluateAll(snap))
	if len(stub) != 0 {
		t.Errorf("expected 0 rule findings, got %d", len(stub))
	}
	if len(srcUnavailable) != 1 {
		t.Fatalf("expected 1 source-unavailable finding for the errored source, got %d", len(srcUnavailable))
	}
	if srcUnavailable[0].InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN {
		t.Errorf("source-unavailable finding must be INVARIANT_UNKNOWN, got %v", srcUnavailable[0].InvariantStatus)
	}
	if srcUnavailable[0].CheckError == "" {
		t.Errorf("source-unavailable finding must carry a non-empty CheckError so aggregators do not count it as a FAIL")
	}
}

type stubInvariantEmpty struct{}

func (stubInvariantEmpty) ID() string                                     { return "empty" }
func (stubInvariantEmpty) Category() string                               { return "test" }
func (stubInvariantEmpty) Scope() string                                  { return "cluster" }
func (stubInvariantEmpty) Evaluate(*collector.Snapshot, Config) []Finding { return nil }

// splitSourceUnavailable partitions EvaluateAll output into ordinary rule
// findings and the registry's source-unavailable findings, keyed off the
// reserved InvariantID.
func splitSourceUnavailable(all []Finding) (rules, srcUnavailable []Finding) {
	for _, f := range all {
		if f.InvariantID == "cluster_doctor.snapshot_source_unavailable" {
			srcUnavailable = append(srcUnavailable, f)
		} else {
			rules = append(rules, f)
		}
	}
	return rules, srcUnavailable
}
