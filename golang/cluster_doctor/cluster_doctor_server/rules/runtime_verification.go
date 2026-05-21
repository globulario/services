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

	var out []Finding

	// Per-target verdicts → one doctor Finding per verifier.Finding.
	for _, v := range r.Verdicts {
		for _, f := range v.Findings {
			out = append(out, verifierFindingToDoctorFinding(v, f))
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

func verifierFindingToDoctorFinding(v verifier.Verdict, f verifier.Finding) Finding {
	tgt := v.Target
	entity := tgt.NodeID + "/" + tgt.Service
	summary := fmt.Sprintf("[%s] %s/%s: %s",
		shortNodeID(tgt.NodeID), tgt.Service, tgt.Service, f.ID)

	return Finding{
		FindingID:       FindingID(f.ID, entity, v.Reason),
		InvariantID:     f.ID,
		Severity:        severityFromVerifier(f.Severity),
		Category:        "diagnostic.runtime",
		EntityRef:       entity,
		Summary:         summary,
		Evidence:        []*cluster_doctorpb.Evidence{kvEvidence("verifier", "VerifyTarget", verifierEvidence(v, f))},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
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
func severityFromVerifier(s string) cluster_doctorpb.Severity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical", "error":
		return cluster_doctorpb.Severity_SEVERITY_ERROR
	case "high", "warn", "warning":
		return cluster_doctorpb.Severity_SEVERITY_WARN
	case "degraded", "info":
		return cluster_doctorpb.Severity_SEVERITY_WARN
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
