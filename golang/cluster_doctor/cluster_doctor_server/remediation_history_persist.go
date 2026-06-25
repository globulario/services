// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.remediation_history_persist
// @awareness file_role=ai_memory_persistence_and_warm_load_for_remediation_audit_ring
// @awareness implements=globular.platform:intent.autonomy.remediation_is_bounded_and_escalates
// @awareness risk=high
//
// EX-3b: rate-limit memory continuity (the matching bolt to EX-3's safety-refusal
// persistence — same architectural family, lower severity).
//
// The failure-rate gate (countRecentFailedActionAttempts) reads recent remediation
// audits from an in-process ring (remediation_history.go). On a doctor restart or
// leader failover that ring is empty, so a still-flapping action's recent failures
// are forgotten and the breaker briefly resets — a few extra auto-retries until it
// re-accumulates and re-trips. This restores that history.
//
// CONTRACT:
//
//	The doctor's audit ring is operational memory, not cluster state. It may be
//	restored from ai-memory to preserve remediation rate-limit history across
//	restart/failover, but malformed or unavailable memory must never block
//	observation or remediation.
//
// So audits persist to ai-memory (the doctor's typed history store) — NEVER etcd
// (invariant:cluster_doctor.observer_only_never_writes_etcd). Records are stored
// already-redacted (auditRemediation calls Redacted() before this) and carry a
// RemediationAuditRetention TTL so the append-only store stays bounded. A TTL is
// appropriate here because an audit is aging history, not a safety-refusal flag —
// unlike the escalation gate, forbidden_fix:auto_clear_escalation_without_operator
// _approval does not apply.
//
// Authority contour: persist is leader-only (auditRemediation runs only inside
// leader-gated ExecuteRemediation); warm-load runs on becoming leader and is a
// read, not a remediation. Both degrade to no-op when ai-memory is unavailable.
package main

import (
	"context"
	"encoding/json"
	"time"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"google.golang.org/grpc"
)

const remediationAuditTagBase = "remediation_audit"

// remediationAuditMemory is the narrow ai-memory surface the audit ring needs:
// Store to persist a record, Query to warm-load recent records. The full
// ai_memorypb.AiMemoryServiceClient satisfies it; tests inject a minimal fake.
type remediationAuditMemory interface {
	Store(ctx context.Context, in *ai_memorypb.StoreRqst, opts ...grpc.CallOption) (*ai_memorypb.StoreRsp, error)
	Query(ctx context.Context, in *ai_memorypb.QueryRqst, opts ...grpc.CallOption) (*ai_memorypb.QueryRsp, error)
}

// remediationAuditMem is the ai-memory persistence surface for the audit ring,
// wired at server startup. nil until wired (and whenever ai-memory is unreachable)
// — every path below is nil-safe and degrades to in-memory-only.
var remediationAuditMem remediationAuditMemory

// setRemediationAuditAiMemoryClient wires the audit ring's persistence to
// ai-memory. Called once at server startup; safe to pass nil (degrades).
func setRemediationAuditAiMemoryClient(c remediationAuditMemory) {
	remediationAuditMem = c
}

// persistRemediationAudit stores a (already-redacted) remediation audit to
// ai-memory so the failure-rate gate's recent-attempt history survives
// restart/failover. Observer memory — NEVER etcd. Leader-only by construction.
// TTL = RemediationAuditRetention. Degrades to no-op when ai-memory is
// unwired/unreachable; never returns an error (audit persistence must not fail a
// remediation).
func persistRemediationAudit(ctx context.Context, audit RemediationAudit) {
	if remediationAuditMem == nil {
		return
	}
	body := audit.JSON()
	if body == "" {
		return
	}
	_, _ = remediationAuditMem.Store(ctx, &ai_memorypb.StoreRqst{Memory: &ai_memorypb.Memory{
		Project: remediationGateMemoryProject,
		Type:    ai_memorypb.MemoryType_DEBUG,
		Tags:    []string{remediationAuditTagBase},
		Title:   "remediation audit: " + audit.ActionType + " (" + audit.FindingID + ")",
		Content: body,
		Metadata: map[string]string{
			"audit_id":     audit.AuditID,
			"invariant_id": audit.InvariantID,
			"action_type":  audit.ActionType,
		},
		TtlSeconds: int32(RemediationAuditRetention / time.Second),
	}})
}

// warmLoadRemediationAudits repopulates the in-process audit ring from ai-memory,
// called on becoming leader (and at startup), so a fresh leader after
// restart/failover recovers the failure-rate gate's recent history instead of
// counting zero. It dedups by AuditID against whatever the ring already holds,
// parses each record defensively (a malformed entry is skipped, never poisoning
// the ring), and is a graceful no-op when ai-memory is unwired/unreachable. It is
// a read — never a remediation.
func warmLoadRemediationAudits(ctx context.Context) {
	if remediationAuditMem == nil {
		return
	}
	resp, err := remediationAuditMem.Query(ctx, &ai_memorypb.QueryRqst{
		Project: remediationGateMemoryProject,
		Tags:    []string{remediationAuditTagBase},
		Limit:   int32(remediationAudits.maxSize),
	})
	if err != nil || resp == nil {
		return
	}
	seen := map[string]bool{}
	for _, a := range remediationAudits.list(0) {
		seen[a.AuditID] = true
	}
	for _, m := range resp.GetMemories() {
		var audit RemediationAudit
		if err := json.Unmarshal([]byte(m.GetContent()), &audit); err != nil {
			continue // malformed — skip, never poison the ring
		}
		if audit.AuditID == "" || seen[audit.AuditID] {
			continue
		}
		seen[audit.AuditID] = true
		remediationAudits.push(audit)
	}
}
