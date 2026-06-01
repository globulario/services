// @awareness namespace=globular.platform
// @awareness component=platform_controller.join_lifecycle
// @awareness file_role=node_admission_status_tracking
// @awareness implements=globular.platform:intent.cluster.membership.earned_trust
// @awareness risk=high
package main

import (
	"fmt"
	"time"
)

// ── Finding IDs ───────────────────────────────────────────────────────────────
// These constants are the machine-readable identifiers for doctor findings
// related to admission stalls. Keep in sync with docs/awareness/failure_modes.yaml.
const (
	FindingAdmissionMissingJoinPlan             = "node.admission_pending_missing_join_plan"
	FindingAdmissionIdentityMismatch            = "node.admission_pending_identity_mismatch"
	FindingAdmissionJoinIDMismatch              = "node.admission_pending_join_id_mismatch"
	FindingAdmissionProfilesMismatch            = "node.admission_pending_profiles_mismatch"
	FindingAdmissionEtcdUnverified              = "node.admission_pending_etcd_unverified"
	FindingAdmissionScyllaUnverified            = "node.admission_pending_scylla_unverified"
	FindingAdmissionObjectstoreGenerationMismatch = "node.admission_pending_objectstore_generation_mismatch"
	FindingAdmissionAgentUnreachable            = "node.admission_pending_agent_unreachable"
	FindingAdmissionBuildMissing                = "node.admission_pending_build_missing"
	FindingActiveBootstrapNotReady              = "node.active_blocked_workload_ready_missing"
)

// ── AdmissionProofStatus ──────────────────────────────────────────────────────

// AdmissionProofStatus is the persisted snapshot of the last
// EvaluateNodeAdmissionProof result for a node. Stored on nodeState as
// LastAdmissionProof so operators can query it without re-running the
// evaluator. JSON-serializable and included in ListNodes metadata.
type AdmissionProofStatus struct {
	// Overall verdict
	OK       bool   `json:"ok"`
	ActiveOK bool   `json:"active_ok"`
	Reason   string `json:"reason,omitempty"`
	// FindingID is the machine-readable doctor finding that best describes the
	// primary failure cause. Empty when OK=true.
	FindingID string            `json:"finding_id,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
	CheckedAt time.Time         `json:"checked_at"`

	// Per-check flags — expose all proof dimensions for operator queries.
	// Naming mirrors NodeAdmissionProof fields.
	IdentityPresent         bool `json:"identity_present"`
	AgentReachable          bool `json:"agent_reachable"`
	BuildPresent            bool `json:"build_present"`
	HasJoinPlan             bool `json:"has_join_plan"`
	IdentityMatchesPlan     bool `json:"identity_matches_plan"`
	JoinIDMatch             bool `json:"join_id_match"`
	ProfilesConsistent      bool `json:"profiles_consistent"`
	EtcdRequired            bool `json:"etcd_required"`
	EtcdProofOK             bool `json:"etcd_proof_ok"`
	EtcdFullyVerified       bool `json:"etcd_fully_verified"`
	ScyllaRequired          bool `json:"scylla_required"`
	ScyllaProofOK           bool `json:"scylla_proof_ok"`
	ScyllaFullyVerified     bool `json:"scylla_fully_verified"`
	ObjectstoreRequired     bool `json:"objectstore_required"`
	ObjectstoreGenerationOK bool `json:"objectstore_generation_ok"`
}

// buildNodeAdmissionStatus converts a NodeAdmissionProofResult to the
// persisted AdmissionProofStatus snapshot. Called after every evaluation.
func buildNodeAdmissionStatus(result NodeAdmissionProofResult) *AdmissionProofStatus {
	p := result.Proof
	return &AdmissionProofStatus{
		OK:        result.OK,
		ActiveOK:  result.ActiveOK,
		Reason:    result.Reason,
		FindingID: admissionFindingID(result),
		Details:   result.Details,
		CheckedAt: result.CheckedAt,

		IdentityPresent:         p.IdentityPresent,
		AgentReachable:          p.AgentReachable,
		BuildPresent:            p.BuildPresent,
		HasJoinPlan:             p.HasJoinPlan,
		IdentityMatchesPlan:     p.IdentityMatchesPlan,
		JoinIDMatch:             p.JoinIDMatch,
		ProfilesConsistent:      p.ProfilesConsistent,
		EtcdRequired:            p.EtcdRequired,
		EtcdProofOK:             p.EtcdProofOK,
		EtcdFullyVerified:       p.EtcdFullyVerified,
		ScyllaRequired:          p.ScyllaRequired,
		ScyllaProofOK:           p.ScyllaProofOK,
		ScyllaFullyVerified:     p.ScyllaFullyVerified,
		ObjectstoreRequired:     p.ObjectstoreRequired,
		ObjectstoreGenerationOK: p.ObjectstoreGenerationOK,
	}
}

// admissionFindingID returns the highest-priority finding ID for a failed
// admission proof. Returns "" when the proof passed (OK=true).
//
// Priority order matches what an operator should fix first:
//  1. Agent unreachable (nothing else can succeed without connectivity)
//  2. Missing JoinPlan (v2 node with no stored plan)
//  3. Identity/join_id mismatch (wrong node, possible replay)
//  4. Profiles inconsistent (authorization drift)
//  5. Infra membership failures (etcd → scylla → objectstore)
//  6. Build missing (informational only; never alone blocks admission)
func admissionFindingID(result NodeAdmissionProofResult) string {
	if result.OK {
		return ""
	}
	p := result.Proof
	switch {
	case !p.AgentReachable:
		return FindingAdmissionAgentUnreachable
	case !p.HasJoinPlan && p.IdentityPresent && p.AgentReachable:
		// Node is reachable but has no JoinPlan: v2 node missing authorization.
		return FindingAdmissionMissingJoinPlan
	case p.HasJoinPlan && !p.IdentityMatchesPlan:
		return FindingAdmissionIdentityMismatch
	case p.HasJoinPlan && !p.JoinIDMatch:
		return FindingAdmissionJoinIDMismatch
	case p.HasJoinPlan && !p.ProfilesConsistent:
		return FindingAdmissionProfilesMismatch
	case p.EtcdRequired && !p.EtcdProofOK:
		return FindingAdmissionEtcdUnverified
	case p.ScyllaRequired && !p.ScyllaProofOK:
		return FindingAdmissionScyllaUnverified
	}
	return ""
}

// admissionFindingIDForActive returns the finding ID for a node that is
// admitted but cannot reach active status.
func admissionFindingIDForActive(result NodeAdmissionProofResult, node *nodeState) string {
	if result.ActiveOK {
		return ""
	}
	p := result.Proof
	if !bootstrapPhaseReady(node.BootstrapPhase) {
		return FindingActiveBootstrapNotReady
	}
	if p.EtcdRequired && !p.EtcdFullyVerified {
		return FindingAdmissionEtcdUnverified
	}
	if p.ScyllaRequired && !p.ScyllaFullyVerified {
		return FindingAdmissionScyllaUnverified
	}
	if p.ObjectstoreRequired && !p.ObjectstoreGenerationOK {
		return FindingAdmissionObjectstoreGenerationMismatch
	}
	return ""
}

// admissionOperatorMessage returns the human-readable operator message for a
// given finding ID. Used in logs, status output, and doctor alerts.
func admissionOperatorMessage(findingID string) string {
	switch findingID {
	case FindingAdmissionMissingJoinPlan:
		return "node admission pending: JoinPlan missing"
	case FindingAdmissionIdentityMismatch:
		return "node admission pending: reported identity does not match JoinPlan"
	case FindingAdmissionJoinIDMismatch:
		return "node admission pending: join_id does not match stored authorization"
	case FindingAdmissionProfilesMismatch:
		return "node admission pending: assigned profiles inconsistent with JoinPlan"
	case FindingAdmissionEtcdUnverified:
		return "node admission pending: etcd membership not verified"
	case FindingAdmissionScyllaUnverified:
		return "node admission pending: Scylla membership not verified"
	case FindingAdmissionObjectstoreGenerationMismatch:
		return "node active blocked: objectstore topology generation mismatch"
	case FindingAdmissionAgentUnreachable:
		return "node admission pending: node-agent not reachable (empty endpoint)"
	case FindingAdmissionBuildMissing:
		return "node admission pending: node-agent build_id/version not yet reported"
	case FindingActiveBootstrapNotReady:
		return "node active blocked: workload_ready bootstrap phase not yet reached"
	}
	return ""
}

// admissionPathLabel returns the operator-facing admission path label for a
// node. Distinguishes v2 (JoinPlan), v1 (RequestJoin, no plan), and legacy
// (empty lifecycle phase) flows.
func admissionPathLabel(node *nodeState) string {
	if node == nil {
		return "unknown"
	}
	// Legacy nodes have no lifecycle phase — they bypass the v2 proof gate.
	if node.JoinLifecyclePhase == "" {
		return "legacy_compat"
	}
	switch node.JoinLifecyclePhase {
	case JoinPhaseBootstrapping, JoinPhaseAuthorized:
		return "bootstrapping"
	case JoinPhaseNodeAgentRegistered:
		return "registered"
	case JoinPhaseAdmissionPending:
		return "admission_pending"
	case JoinPhaseAdmitted, JoinPhaseConverging:
		return "admitted"
	case JoinPhaseActive:
		return "active"
	case JoinPhaseBlocked, JoinPhaseQuarantined:
		return "blocked"
	case JoinPhaseRejected:
		return "rejected"
	case JoinPhaseRemoved, JoinPhaseRemoving:
		return "removed"
	}
	return string(node.JoinLifecyclePhase)
}

// admissionStatusMetadata builds the metadata key/value pairs to include in
// ListNodes output for a node's admission status. Returns nil for legacy nodes.
func admissionStatusMetadata(node *nodeState) map[string]string {
	if node == nil {
		return nil
	}
	m := make(map[string]string, 8)
	m["admission_path"] = admissionPathLabel(node)

	// Legacy nodes have no proof status — document that clearly.
	if node.JoinLifecyclePhase == "" {
		m["admission_path"] = "legacy_compat"
		m["admission_note"] = "node using legacy admission compatibility path"
		return m
	}

	status := node.LastAdmissionProof
	if status == nil {
		// Phase not yet evaluated on this heartbeat cycle.
		m["admission_ok"] = "unknown"
		return m
	}

	if status.OK {
		m["admission_ok"] = "true"
	} else {
		m["admission_ok"] = "false"
		m["admission_reason"] = status.Reason
	}
	if status.ActiveOK {
		m["admission_active_ok"] = "true"
	} else {
		m["admission_active_ok"] = "false"
	}
	if status.FindingID != "" {
		m["admission_finding"] = status.FindingID
		msg := admissionOperatorMessage(status.FindingID)
		if msg != "" {
			m["admission_message"] = msg
		}
	}
	if status.HasJoinPlan {
		m["admission_plan"] = "v2"
	} else {
		m["admission_plan"] = "v1"
	}
	if !status.CheckedAt.IsZero() {
		m["admission_checked_at"] = status.CheckedAt.UTC().Format(time.RFC3339)
	}
	// Per-check summary: include only failing checks to keep the output concise.
	fails := admissionFailingSummary(status)
	if fails != "" {
		m["admission_failing_checks"] = fails
	}
	return m
}

// admissionFailingSummary returns a comma-separated list of the check names
// that are required but currently failing. Empty when all required checks pass.
func admissionFailingSummary(s *AdmissionProofStatus) string {
	if s == nil || s.OK {
		return ""
	}
	var parts []string
	add := func(cond bool, name string) {
		if !cond {
			parts = append(parts, name)
		}
	}
	add(s.IdentityPresent, "identity_present")
	add(s.AgentReachable, "agent_reachable")
	if s.HasJoinPlan {
		add(s.IdentityMatchesPlan, "identity_matches_plan")
		add(s.JoinIDMatch, "join_id_match")
		add(s.ProfilesConsistent, "profiles_consistent")
	}
	if s.EtcdRequired {
		add(s.EtcdProofOK, "etcd_proof")
	}
	if s.ScyllaRequired {
		add(s.ScyllaProofOK, "scylla_proof")
	}
	if len(parts) == 0 {
		return ""
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}

// admissionShouldLog returns true when the admission proof reason has changed
// since the last logged evaluation (simple change-detection rate limiter).
// prev is the previously stored reason; empty means "not yet logged".
func admissionShouldLog(prev, next string) bool {
	return prev != next
}

// admissionProofSummaryLine builds a single log line summarising a proof
// result for use in ReportNodeStatus. Kept short to avoid log spam.
func admissionProofSummaryLine(nodeID string, result NodeAdmissionProofResult) string {
	if result.OK {
		if result.ActiveOK {
			return fmt.Sprintf("admission proof: node %s OK+active", nodeID)
		}
		return fmt.Sprintf("admission proof: node %s OK (not yet active: %s)",
			nodeID, admissionFindingIDForActive(result, nil))
	}
	msg := admissionOperatorMessage(admissionFindingID(result))
	if msg == "" {
		msg = result.Reason
	}
	return fmt.Sprintf("admission proof: node %s BLOCKED — %s", nodeID, msg)
}
