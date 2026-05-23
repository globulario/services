package rules

// runtime_verification_test.go — Phase 9 wire-up tests.
//
// Pins the contract that the doctor rule translates verifier verdicts +
// cross-findings into rules.Finding shape on every EvaluateAll pass.
// We do NOT re-test the verifier's decision logic here — that lives in
// golang/verifier — only that the translation seam stays honest.

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/verifier"
)

// nodeRecordsWithPhase returns a single-element NodeRecord slice whose
// bootstrap_phase metadata is set to phase. Used to simulate a node in a
// specific bootstrap state for runtimeVerification tests.
func nodeRecordsWithPhase(nodeID, phase string) []*cluster_controllerpb.NodeRecord {
	meta := map[string]string{}
	if phase != "" {
		meta["bootstrap_phase"] = phase
	}
	return []*cluster_controllerpb.NodeRecord{
		{NodeId: nodeID, Metadata: meta},
	}
}

func TestRuntimeVerification_NilResult_NoFindings(t *testing.T) {
	snap := &collector.Snapshot{} // VerifierResult nil
	got := (runtimeVerification{}).Evaluate(snap, testConfig())
	if len(got) != 0 {
		t.Errorf("expected 0 findings when VerifierResult is nil; got %d", len(got))
	}
}

func TestRuntimeVerification_PerTargetFinding_TranslatesToDoctorFinding(t *testing.T) {
	snap := &collector.Snapshot{
		VerifierResult: &verifier.Result{
			Verdicts: []verifier.Verdict{
				{
					Target: verifier.Target{
						Service: "foo",
						NodeID:  "ryzen",
					},
					ProofStatus: verifier.ProofMismatch,
					Reason:      "mismatch: " + verifier.FindingRunningBinaryHashMismatch,
					Findings: []verifier.Finding{
						{
							ID:       verifier.FindingRunningBinaryHashMismatch,
							Severity: verifier.SeverityCritical,
							Service:  "foo",
							NodeID:   "ryzen",
							Evidence: map[string]string{
								"installed_sha256": "aaaa",
								"running_sha256":   "bbbb",
							},
						},
					},
				},
			},
		},
	}
	got := (runtimeVerification{}).Evaluate(snap, testConfig())
	if len(got) != 1 {
		t.Fatalf("expected 1 finding; got %d (%+v)", len(got), got)
	}
	f := got[0]
	if f.InvariantID != verifier.FindingRunningBinaryHashMismatch {
		t.Errorf("InvariantID=%q want=%q", f.InvariantID, verifier.FindingRunningBinaryHashMismatch)
	}
	if f.EntityRef != "ryzen/foo" {
		t.Errorf("EntityRef=%q want=ryzen/foo", f.EntityRef)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("Severity=%v want=SEVERITY_ERROR (critical maps to ERROR)", f.Severity)
	}
	if f.Category != "diagnostic.runtime" {
		t.Errorf("Category=%q want=diagnostic.runtime", f.Category)
	}
	// Evidence carries both the verifier's structured payload AND the
	// verdict's ProofStatus / Reason so operators see the full context.
	kv := f.Evidence[0].KeyValues
	if kv["installed_sha256"] != "aaaa" || kv["running_sha256"] != "bbbb" {
		t.Errorf("verifier evidence lost in translation: %v", kv)
	}
	if kv["proof_status"] != verifier.ProofMismatch {
		t.Errorf("proof_status=%q want=%q", kv["proof_status"], verifier.ProofMismatch)
	}
	if kv["service"] != "foo" {
		t.Errorf("service=%q want=foo", kv["service"])
	}
}

func TestRuntimeVerification_CrossFindings_TranslatesToDoctorFinding(t *testing.T) {
	snap := &collector.Snapshot{
		VerifierResult: &verifier.Result{
			CrossFindings: []verifier.Finding{
				{
					ID:       verifier.FindingSilentFallbackActive,
					Severity: verifier.SeverityDegraded,
					Service:  "repository",
					NodeID:   "ryzen",
					Evidence: map[string]string{
						"dependency": "scylladb",
						"mode":       "minio_read",
					},
				},
				{
					ID:       verifier.FindingCrossNodeFileDrift,
					Severity: verifier.SeverityDegraded,
					Service:  "webroot",
					Evidence: map[string]string{
						"path":   "index.html",
						"drifts": "ryzen: present; nuc: absent",
					},
				},
			},
		},
	}
	got := (runtimeVerification{}).Evaluate(snap, testConfig())
	if len(got) != 2 {
		t.Fatalf("expected 2 findings; got %d (%+v)", len(got), got)
	}
	ids := map[string]bool{}
	for _, f := range got {
		ids[f.InvariantID] = true
	}
	if !ids[verifier.FindingSilentFallbackActive] {
		t.Errorf("missing fallback finding; got %+v", got)
	}
	if !ids[verifier.FindingCrossNodeFileDrift] {
		t.Errorf("missing cross-node drift finding; got %+v", got)
	}
}

func TestRuntimeVerification_SeverityMapping(t *testing.T) {
	cases := []struct {
		in   string
		want cluster_doctorpb.Severity
	}{
		{verifier.SeverityCritical, cluster_doctorpb.Severity_SEVERITY_ERROR},
		{verifier.SeverityHigh, cluster_doctorpb.Severity_SEVERITY_WARN},
		{verifier.SeverityDegraded, cluster_doctorpb.Severity_SEVERITY_WARN},
		{verifier.SeverityInfo, cluster_doctorpb.Severity_SEVERITY_INFO},
		{"unknown_value", cluster_doctorpb.Severity_SEVERITY_WARN}, // safe default
	}
	for _, tc := range cases {
		got := severityFromVerifier(tc.in)
		if got != tc.want {
			t.Errorf("severityFromVerifier(%q)=%v want=%v", tc.in, got, tc.want)
		}
	}
}

// Info-severity verifier findings (bootstrap_ordering_skew on first
// install, etc.) MUST surface as INVARIANT_PASS so the workflow
// incident scanner — which opens an incident for every non-PASS finding
// — doesn't fill the queue with one informational marker per service.
func TestRuntimeVerification_InfoSeverity_IsInvariantPass(t *testing.T) {
	snap := &collector.Snapshot{
		VerifierResult: &verifier.Result{
			Verdicts: []verifier.Verdict{
				{
					Target:      verifier.Target{Service: "rbac", NodeID: "ryzen"},
					ProofStatus: verifier.ProofRuntimeVerified,
					Reason:      "all proofs agree",
					Findings: []verifier.Finding{
						{
							ID:       verifier.FindingBootstrapOrderingSkew,
							Severity: verifier.SeverityInfo,
							Service:  "rbac",
							NodeID:   "ryzen",
						},
					},
				},
			},
		},
	}
	got := (runtimeVerification{}).Evaluate(snap, testConfig())
	if len(got) != 1 {
		t.Fatalf("expected 1 finding; got %d", len(got))
	}
	if got[0].InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_PASS {
		t.Errorf("info-severity finding must be INVARIANT_PASS; got %v (incident scanner will create one OPEN incident per service-day for what is just a normal first-install marker)",
			got[0].InvariantStatus)
	}
	if got[0].Severity != cluster_doctorpb.Severity_SEVERITY_INFO {
		t.Errorf("info severity must map to SEVERITY_INFO; got %v", got[0].Severity)
	}
}

// Critical / high / degraded verifier findings MUST stay INVARIANT_FAIL
// so they surface as incidents (the whole point of the verifier).
func TestRuntimeVerification_NonInfoSeverity_StaysInvariantFail(t *testing.T) {
	for _, sev := range []string{
		verifier.SeverityCritical,
		verifier.SeverityHigh,
		verifier.SeverityDegraded,
	} {
		snap := &collector.Snapshot{
			VerifierResult: &verifier.Result{
				Verdicts: []verifier.Verdict{
					{
						Target:      verifier.Target{Service: "x", NodeID: "n"},
						ProofStatus: verifier.ProofMismatch,
						Findings: []verifier.Finding{
							{ID: "x.drift", Severity: sev, Service: "x", NodeID: "n"},
						},
					},
				},
			},
		}
		got := (runtimeVerification{}).Evaluate(snap, testConfig())
		if len(got) != 1 || got[0].InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
			t.Errorf("severity %q must stay INVARIANT_FAIL; got %+v", sev, got)
		}
	}
}

func TestRuntimeVerification_PerTargetAndCrossSurfaceTogether(t *testing.T) {
	// A realistic sweep: one target is mismatched and a fallback is
	// active. Both surface as separate findings (counts add).
	snap := &collector.Snapshot{
		VerifierResult: &verifier.Result{
			Verdicts: []verifier.Verdict{
				{
					Target:      verifier.Target{Service: "foo", NodeID: "n1"},
					ProofStatus: verifier.ProofMismatch,
					Findings: []verifier.Finding{
						{ID: verifier.FindingRunningBinaryHashMismatch, Severity: verifier.SeverityCritical},
					},
				},
			},
			CrossFindings: []verifier.Finding{
				{ID: verifier.FindingSilentFallbackActive, Severity: verifier.SeverityDegraded, Service: "repository", NodeID: "n1"},
			},
		},
	}
	got := (runtimeVerification{}).Evaluate(snap, testConfig())
	if len(got) != 2 {
		t.Fatalf("expected 2 findings (1 per-target + 1 cross); got %d", len(got))
	}
}

// TestRuntimeVerification_UnprovenBootstrappingNode pins that
// service.runtime_identity_unproven is downgraded to INFO+PASS when the
// node that emitted it is in an active bootstrap phase. Services on
// bootstrapping nodes have not been installed via the pipeline yet, so no
// entrypoint_checksum exists to compare against — the finding is expected and
// must not open an incident.
func TestRuntimeVerification_UnprovenBootstrappingNode(t *testing.T) {
	bootstrapPhases := []string{
		"admitted",
		"infra_preparing",
		"etcd_joining",
		"etcd_ready",
		"xds_ready",
		"envoy_ready",
		"awareness_ready",
		"bootstrap_failed",
	}
	for _, phase := range bootstrapPhases {
		t.Run(phase, func(t *testing.T) {
			snap := &collector.Snapshot{
				Nodes: nodeRecordsWithPhase("dell-node", phase),
				VerifierResult: &verifier.Result{
					Verdicts: []verifier.Verdict{
						{
							Target:      verifier.Target{Service: "rbac", NodeID: "dell-node"},
							ProofStatus: verifier.ProofUnknown,
							Findings: []verifier.Finding{
								{
									ID:       verifier.FindingRuntimeIdentityUnproven,
									Severity: verifier.SeverityDegraded,
									Service:  "rbac",
									NodeID:   "dell-node",
								},
							},
						},
					},
				},
			}
			got := (runtimeVerification{}).Evaluate(snap, testConfig())
			if len(got) != 1 {
				t.Fatalf("phase=%s: expected 1 finding; got %d", phase, len(got))
			}
			f := got[0]
			if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_PASS {
				t.Errorf("phase=%s: runtime_identity_unproven on bootstrapping node must be INVARIANT_PASS; got %v",
					phase, f.InvariantStatus)
			}
			if f.Severity != cluster_doctorpb.Severity_SEVERITY_INFO {
				t.Errorf("phase=%s: runtime_identity_unproven on bootstrapping node must be SEVERITY_INFO; got %v",
					phase, f.Severity)
			}
		})
	}
}

// TestRuntimeVerification_UnprovenWorkloadReadyNode ensures that
// runtime_identity_unproven is NOT suppressed once the node reaches
// workload_ready — at that point services should be installed and proofs
// should exist, so the finding is a real signal.
func TestRuntimeVerification_UnprovenWorkloadReadyNode(t *testing.T) {
	for _, phase := range []string{"workload_ready", "storage_joining", ""} {
		t.Run("phase="+phase, func(t *testing.T) {
			snap := &collector.Snapshot{
				Nodes: nodeRecordsWithPhase("n1", phase),
				VerifierResult: &verifier.Result{
					Verdicts: []verifier.Verdict{
						{
							Target:      verifier.Target{Service: "repo", NodeID: "n1"},
							ProofStatus: verifier.ProofUnknown,
							Findings: []verifier.Finding{
								{
									ID:       verifier.FindingRuntimeIdentityUnproven,
									Severity: verifier.SeverityDegraded,
									Service:  "repo",
									NodeID:   "n1",
								},
							},
						},
					},
				},
			}
			got := (runtimeVerification{}).Evaluate(snap, testConfig())
			if len(got) != 1 {
				t.Fatalf("phase=%q: expected 1 finding; got %d", phase, len(got))
			}
			f := got[0]
			if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
				t.Errorf("phase=%q: runtime_identity_unproven on ready node must remain INVARIANT_FAIL; got %v",
					phase, f.InvariantStatus)
			}
		})
	}
}
