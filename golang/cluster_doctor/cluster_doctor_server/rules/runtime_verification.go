// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules
// @awareness file_role=verifier_findings_to_doctor_findings_bridge
// @awareness implements=globular.platform:intent.runtime.identity_requires_verification
// @awareness enforces=globular.platform:invariant.state.runtime_not_desired
// @awareness risk=high
package rules

// runtime_verification.go — Phase 9 wire-up of the Diagnostic Honesty
// Refactor. The collector runs verifier.VerifyTarget for every desired
// (service, node) and aggregates the result into snap.VerifierResult.
// This invariant translates each verifier finding into a doctor Finding
// so they surface alongside every other rules invariant on the next
// EvaluateAll pass.
//
// We do NOT re-run any decision logic here — that lives in the verifier
// package and is the single source of truth for claim-vs-proof.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/verifier"
)

type runtimeVerification struct{}

func (runtimeVerification) ID() string       { return "diagnostic.runtime_verification" }
func (runtimeVerification) Category() string { return "diagnostic" }
func (runtimeVerification) Scope() string    { return "service" }

func (runtimeVerification) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if snap == nil || snap.VerifierResult == nil {
		return nil
	}
	r := snap.VerifierResult

	// Build the set of nodes that are currently in an active bootstrap phase
	// (i.e. not yet workload_ready / storage_joining). Services on these nodes
	// have not been installed via the pipeline yet, so runtime_identity_unproven
	// is expected and must not surface as an actionable finding.
	bootstrapNodes := bootstrappingNodeSet(snap)

	var out []Finding

	// Per-target verdicts → one doctor Finding per verifier.Finding.
	for _, v := range r.Verdicts {
		for _, f := range v.Findings {
			out = append(out, verifierFindingToDoctorFinding(v, f, bootstrapNodes))
		}
	}

	// Cross-cutting findings (Phase 6 fallbacks, Phase 7 cross-node drift)
	// are already in Finding shape on the verifier side. Re-shape into
	// rules.Finding.
	for _, f := range r.CrossFindings {
		out = append(out, verifierCrossFindingToDoctorFinding(f))
	}

	return out
}

// bootstrappingNodeSet returns a set of node IDs whose bootstrap_phase metadata
// indicates they have not yet reached workload_ready or storage_joining.
// Nodes with no bootstrap_phase (legacy / pre-join) are treated as ready.
func bootstrappingNodeSet(snap *collector.Snapshot) map[string]bool {
	out := make(map[string]bool)
	for _, n := range snap.Nodes {
		phase := n.GetMetadata()["bootstrap_phase"]
		switch phase {
		case "", "workload_ready", "storage_joining":
			// terminal or legacy — not bootstrapping
		default:
			out[n.GetNodeId()] = true
		}
	}
	return out
}

func verifierFindingToDoctorFinding(v verifier.Verdict, f verifier.Finding, bootstrapNodes map[string]bool) Finding {
	tgt := v.Target
	entity := tgt.NodeID + "/" + tgt.Service
	summary := fmt.Sprintf("[%s] %s/%s: %s",
		shortNodeID(tgt.NodeID), tgt.Service, tgt.Service, f.ID)

	sev := severityFromVerifier(f.Severity)
	// Info-severity findings are informational markers (e.g.
	// service.bootstrap_ordering_skew on first install — process started
	// before ApplyTime by design because install.sh fires services
	// before the controller records the apply). They MUST NOT surface
	// as failing invariants — otherwise the workflow incident scanner
	// opens an OPEN incident per (service, node) for what is just a
	// "this is normal" diagnostic note. Mark them as PASS so the
	// incident scanner's "skip PASS" filter drops them.
	//
	// runtime_identity_unproven on a bootstrapping node is equally benign:
	// the service hasn't been installed via the pipeline yet so there is no
	// entrypoint_checksum to compare against. Downgrade to INFO+PASS until
	// the node reaches workload_ready.
	status := cluster_doctorpb.InvariantStatus_INVARIANT_FAIL
	if sev == cluster_doctorpb.Severity_SEVERITY_INFO {
		status = cluster_doctorpb.InvariantStatus_INVARIANT_PASS
	} else if f.ID == verifier.FindingRuntimeIdentityUnproven && bootstrapNodes[tgt.NodeID] {
		sev = cluster_doctorpb.Severity_SEVERITY_INFO
		status = cluster_doctorpb.InvariantStatus_INVARIANT_PASS
	}

	return Finding{
		FindingID:       FindingID(f.ID, entity, v.Reason),
		InvariantID:     f.ID,
		Severity:        sev,
		Category:        "diagnostic.runtime",
		EntityRef:       entity,
		Summary:         summary,
		Evidence:        []*cluster_doctorpb.Evidence{kvEvidence("verifier", "VerifyTarget", verifierEvidence(v, f))},
		InvariantStatus: status,
	}
}

func verifierCrossFindingToDoctorFinding(f verifier.Finding) Finding {
	entity := f.NodeID
	if entity == "" {
		entity = f.Service
	}
	summary := fmt.Sprintf("[%s] %s: %s",
		shortNodeID(f.NodeID), f.Service, f.ID)

	return Finding{
		FindingID:       FindingID(f.ID, entity, summary),
		InvariantID:     f.ID,
		Severity:        severityFromVerifier(f.Severity),
		Category:        "diagnostic.runtime",
		EntityRef:       entity,
		Summary:         summary,
		Evidence:        []*cluster_doctorpb.Evidence{kvEvidence("verifier", "AggregateResult", f.Evidence)},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}
}

// severityFromVerifier maps verifier severity strings to the proto enum.
// The verifier uses lower-case strings (critical/high/degraded/info) per
// failure_modes.yaml conventions; existing rules use the cluster_doctorpb
// enum.
//
// Note: "degraded" stays at WARN — it signals partial proof / unknown
// runtime state, which is actionable. "info" maps to its own INFO level
// so caller code (verifierFindingToDoctorFinding) can decide not to
// raise an INVARIANT_FAIL for an informational marker.
func severityFromVerifier(s string) cluster_doctorpb.Severity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical", "error":
		return cluster_doctorpb.Severity_SEVERITY_ERROR
	case "high", "warn", "warning":
		return cluster_doctorpb.Severity_SEVERITY_WARN
	case "degraded":
		return cluster_doctorpb.Severity_SEVERITY_WARN
	case "info":
		return cluster_doctorpb.Severity_SEVERITY_INFO
	default:
		return cluster_doctorpb.Severity_SEVERITY_WARN
	}
}

// verifierEvidence builds the KV map for Evidence. Includes the verifier's
// structured evidence verbatim plus the per-target ProofStatus + Reason so
// operators see the verdict shape without having to cross-reference.
func verifierEvidence(v verifier.Verdict, f verifier.Finding) map[string]string {
	kv := make(map[string]string, len(f.Evidence)+4)
	for k, val := range f.Evidence {
		kv[k] = val
	}
	kv["proof_status"] = v.ProofStatus
	if v.Reason != "" {
		kv["verdict_reason"] = v.Reason
	}
	if v.Target.NodeID != "" {
		kv["node_id"] = v.Target.NodeID
	}
	if v.Target.Service != "" {
		kv["service"] = v.Target.Service
	}
	// Stable key order isn't enforced by Evidence but tests prefer it.
	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(kv))
	for _, k := range keys {
		out[k] = kv[k]
	}
	return out
}
