// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.invariant_registry_harvest_downgrade_test
// @awareness file_role=tests_registry_downgrades_confident_findings_resting_on_errored_evidence
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

// evidenceStub is a configurable rule that emits a single finding with a chosen
// verdict and one evidence row pointing at a chosen (service, rpc) source. Used
// to drive the registry's reduced-harvest downgrade net.
type evidenceStub struct {
	id     string
	status cluster_doctorpb.InvariantStatus
	evSvc  string
	evRpc  string
}

func (s evidenceStub) ID() string       { return s.id }
func (s evidenceStub) Category() string { return "test" }
func (s evidenceStub) Scope() string    { return "cluster" }
func (s evidenceStub) Evaluate(*collector.Snapshot, Config) []Finding {
	return []Finding{{
		FindingID:       s.id + "-id",
		InvariantID:     s.id,
		Severity:        cluster_doctorpb.Severity_SEVERITY_ERROR,
		Category:        "test",
		EntityRef:       "cluster",
		Summary:         "stub " + s.id + " summary",
		InvariantStatus: s.status,
		Evidence:        []*cluster_doctorpb.Evidence{kvEvidence(s.evSvc, s.evRpc, map[string]string{"k": "v"})},
	}}
}

func findByInvariantID(findings []Finding, id string) (Finding, bool) {
	for _, f := range findings {
		if f.InvariantID == id {
			return f, true
		}
	}
	return Finding{}, false
}

// TestEvaluateAll_DowngradesConfidentFindingOnErroredEvidenceSource is the OT-2 #2
// headline: a confident verdict (FAIL or PASS) whose OWN evidence rests on a
// source that errored this sweep must be demoted to INVARIANT_UNKNOWN with a
// non-empty CheckError — it must never survive as a confident verdict on data its
// own evidence says was unavailable. The source error is recorded instance-qualified
// (node_agent@globule-nuc) while the rule stamps evidence on the base name
// (node_agent); the downgrade depends on Snapshot.HadError matching the two (gap #4).
func TestEvaluateAll_DowngradesConfidentFindingOnErroredEvidenceSource(t *testing.T) {
	snap := &collector.Snapshot{
		DataIncomplete: true,
		DataErrors: []collector.DataError{
			{Service: "node_agent@globule-nuc", RPC: "GetInventory", Err: errors.New("context deadline exceeded")},
		},
	}

	cases := []struct {
		name   string
		status cluster_doctorpb.InvariantStatus
	}{
		{"confident FAIL on errored source", cluster_doctorpb.InvariantStatus_INVARIANT_FAIL},
		{"confident PASS on errored source (false-green)", cluster_doctorpb.InvariantStatus_INVARIANT_PASS},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &Registry{
				invariants: []Invariant{evidenceStub{id: "compromised", status: tc.status, evSvc: "node_agent", evRpc: "GetInventory"}},
				cfg:        Config{},
			}
			rules, _ := splitSourceUnavailable(r.EvaluateAll(snap))
			f, ok := findByInvariantID(rules, "compromised")
			if !ok {
				t.Fatalf("expected the compromised rule finding to be present")
			}
			if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN {
				t.Errorf("verdict must be downgraded to INVARIANT_UNKNOWN, got %v", f.InvariantStatus)
			}
			if f.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
				t.Errorf("downgraded finding must be WARN, got %v", f.Severity)
			}
			if f.CheckError == "" {
				t.Errorf("downgraded finding must carry a non-empty CheckError so aggregators do not count it as a confident verdict")
			}
			if !strings.HasPrefix(f.Summary, "[harvest-degraded] ") {
				t.Errorf("downgraded Summary must be prefixed [harvest-degraded]; got %q", f.Summary)
			}
			// The compromised source is named by its base name (how the rule stamped it).
			if !strings.Contains(f.CheckError, "node_agent.GetInventory") {
				t.Errorf("CheckError must name the compromised source node_agent.GetInventory; got %q", f.CheckError)
			}
		})
	}
}

// TestEvaluateAll_DoesNotDowngradeFindingWithHealthySource proves the downgrade is
// PRECISE: when a confident finding's own evidence source did NOT error this sweep,
// the verdict is preserved (only the generic [reduced-harvest] label is added,
// since the snapshot was incomplete for an unrelated source). A blanket downgrade
// on any DataIncomplete would silence real, well-grounded findings.
func TestEvaluateAll_DoesNotDowngradeFindingWithHealthySource(t *testing.T) {
	snap := &collector.Snapshot{
		DataIncomplete: true,
		DataErrors: []collector.DataError{
			// An unrelated source errored — NOT the one this finding rests on.
			{Service: "node_agent@globule-nuc", RPC: "GetInventory", Err: errors.New("timeout")},
		},
	}
	r := &Registry{
		invariants: []Invariant{evidenceStub{id: "healthy", status: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL, evSvc: "etcd", evRpc: "GetDomains"}},
		cfg:        Config{},
	}
	rules, _ := splitSourceUnavailable(r.EvaluateAll(snap))
	f, ok := findByInvariantID(rules, "healthy")
	if !ok {
		t.Fatalf("expected the healthy-source rule finding to be present")
	}
	if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
		t.Errorf("finding whose own source did not error must keep its FAIL verdict, got %v", f.InvariantStatus)
	}
	if f.CheckError != "" {
		t.Errorf("finding with a healthy source must not be marked indeterminate; CheckError=%q", f.CheckError)
	}
	if !strings.HasPrefix(f.Summary, "[reduced-harvest] ") {
		t.Errorf("a non-compromised finding under an incomplete snapshot keeps the generic [reduced-harvest] label; got %q", f.Summary)
	}
}

// TestEvaluateAll_DoesNotDowngradeNonConclusiveFinding proves only CONFIDENT
// verdicts are downgraded. A finding that is already UNKNOWN/PENDING resting on an
// errored source is left as-is (it is already non-conclusive) and just labeled.
func TestEvaluateAll_DoesNotDowngradeNonConclusiveFinding(t *testing.T) {
	snap := &collector.Snapshot{
		DataIncomplete: true,
		DataErrors: []collector.DataError{
			{Service: "node_agent@globule-nuc", RPC: "GetInventory", Err: errors.New("timeout")},
		},
	}
	r := &Registry{
		invariants: []Invariant{evidenceStub{id: "pending", status: cluster_doctorpb.InvariantStatus_INVARIANT_PENDING, evSvc: "node_agent", evRpc: "GetInventory"}},
		cfg:        Config{},
	}
	rules, _ := splitSourceUnavailable(r.EvaluateAll(snap))
	f, ok := findByInvariantID(rules, "pending")
	if !ok {
		t.Fatalf("expected the pending rule finding to be present")
	}
	if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_PENDING {
		t.Errorf("a non-conclusive finding must be left unchanged, got %v", f.InvariantStatus)
	}
	if strings.HasPrefix(f.Summary, "[harvest-degraded] ") {
		t.Errorf("a non-conclusive finding must not be downgraded; got %q", f.Summary)
	}
}
