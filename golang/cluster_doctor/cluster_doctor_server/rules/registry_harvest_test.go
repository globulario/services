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

	findings := r.EvaluateAll(snap)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
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

	findings := r.EvaluateAll(snap)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding")
	}
	for _, ev := range findings[0].Evidence {
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

// TestEvaluateAll_EmptyFindings_NoCrash covers the edge case where
// a rule produces no findings during a partial-snapshot evaluation.
// The wrapper must not panic.
func TestEvaluateAll_EmptyFindings_NoCrash(t *testing.T) {
	r := &Registry{
		invariants: []Invariant{stubInvariantEmpty{}},
		cfg:        Config{},
	}
	snap := &collector.Snapshot{
		DataIncomplete: true,
		DataErrors:     []collector.DataError{{Service: "x", RPC: "y", Err: errors.New("z")}},
	}
	findings := r.EvaluateAll(snap)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

type stubInvariantEmpty struct{}

func (stubInvariantEmpty) ID() string                                     { return "empty" }
func (stubInvariantEmpty) Category() string                               { return "test" }
func (stubInvariantEmpty) Scope() string                                  { return "cluster" }
func (stubInvariantEmpty) Evaluate(*collector.Snapshot, Config) []Finding { return nil }
