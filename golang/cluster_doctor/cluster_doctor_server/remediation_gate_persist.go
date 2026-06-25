// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.remediation_gate_persist
// @awareness file_role=ai_memory_persistence_for_remediation_escalation_gate
// @awareness implements=globular.platform:intent.autonomy.remediation_is_bounded_and_escalates
// @awareness risk=critical
//
// EX-3: persistence of a SAFETY REFUSAL, not bookkeeping.
//
// When the auto-remediation gate escalates (autoRemediationEscalationThreshold
// cooldown rejections → "this keeps failing, a human must decide"), that decision
// must survive a doctor restart or leader failover. Otherwise the in-memory
// counter resets to zero, the gate reads "not escalated", and a previously
// escalated unsafe remediation silently becomes auto-executable again
// (failure_mode:doctor.escalation_state_lost_on_restart).
//
// CONTRACT (the architectural hinge):
//
//	A doctor escalation is NOT desired state and must not be persisted through
//	cluster storage. It is observer memory, scoped to the doctor/remediation key,
//	recoverable across restart, and safely ignorable when ai-memory is
//	unavailable.
//
// So this persists to ai-memory (the doctor's typed history store) — NEVER etcd.
// It is the sanctioned replacement for the etcd persistence removed in v1.2.166
// (invariant:cluster_doctor.observer_only_never_writes_etcd), not a rollback of it.
//
// NO TTL: the escalation memory has no expiry. Auto-expiring it on a timer would
// clear a safety refusal without operator intervention —
// forbidden_fix:auto_clear_escalation_without_operator_approval. The record is
// removed only by remediationGateClear (successful authoritative remediation) or
// an operator, never by a clock.
//
// Authority contour:
//   - persist / delete : leader-only (callers are downstream of isAuthoritative)
//   - load             : any instance (read-only; warms the in-memory cache)
//   - ai-memory down   : graceful no-op / not-found → in-memory behavior only
//   - etcd             : never
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"google.golang.org/grpc"
)

const (
	remediationGateMemoryProject = "globular-services"
	remediationGateTagBase       = "remediation_gate"
)

// remediationGateMemory is the narrow slice of the ai-memory client this gate
// needs. The full ai_memorypb.AiMemoryServiceClient satisfies it structurally;
// tests inject a minimal fake.
type remediationGateMemory interface {
	Store(ctx context.Context, in *ai_memorypb.StoreRqst, opts ...grpc.CallOption) (*ai_memorypb.StoreRsp, error)
	Query(ctx context.Context, in *ai_memorypb.QueryRqst, opts ...grpc.CallOption) (*ai_memorypb.QueryRsp, error)
	Update(ctx context.Context, in *ai_memorypb.UpdateRqst, opts ...grpc.CallOption) (*ai_memorypb.UpdateRsp, error)
	Delete(ctx context.Context, in *ai_memorypb.DeleteRqst, opts ...grpc.CallOption) (*ai_memorypb.DeleteRsp, error)
}

// remediationGateMem is the ai-memory persistence surface, wired at server
// startup. nil until wired (and whenever ai-memory is unreachable) — every path
// below is nil-safe and degrades to in-memory-only.
var remediationGateMem remediationGateMemory

// setRemediationGateAiMemoryClient wires the escalation gate's persistence to
// ai-memory. Called once at server startup; safe to pass nil (degrades).
func setRemediationGateAiMemoryClient(c remediationGateMemory) {
	remediationGateMem = c
}

// remediationGateTag derives a deterministic, collision-resistant ai-memory tag
// from a gate key (which contains '|' separators unsuitable as a raw tag).
func remediationGateTag(key string) string {
	sum := sha256.Sum256([]byte(key))
	return remediationGateTagBase + ":" + hex.EncodeToString(sum[:8])
}

func remediationGateMetadata(key string, state remediationGateState) map[string]string {
	return map[string]string{
		"gate_key":               key,
		"cooldown_rejections":    strconv.Itoa(state.CooldownRejections),
		"last_rejection_at_unix": strconv.FormatInt(state.LastRejectionAt, 10),
		"escalated":              strconv.FormatBool(state.Escalated),
	}
}

// remediationGateFindMemoryID returns the ai-memory id of the persisted state for
// a gate key, or "" if none / on any error.
func remediationGateFindMemoryID(ctx context.Context, key string) string {
	if remediationGateMem == nil {
		return ""
	}
	resp, err := remediationGateMem.Query(ctx, &ai_memorypb.QueryRqst{
		Project: remediationGateMemoryProject,
		Tags:    []string{remediationGateTag(key)},
		Limit:   1,
	})
	if err != nil || resp == nil || len(resp.GetMemories()) == 0 {
		return ""
	}
	return resp.GetMemories()[0].GetId()
}

// remediationGatePersist writes the escalation state to ai-memory (upsert by tag)
// so it survives restart/failover. Leader-only by construction (callers are
// downstream of isAuthoritative). No TTL — see file header. Degrades to no-op when
// ai-memory is unwired/unreachable; never returns an error (persistence must not
// fail a remediation).
func remediationGatePersist(ctx context.Context, key string, state remediationGateState) {
	if remediationGateMem == nil {
		return
	}
	meta := remediationGateMetadata(key, state)
	if id := remediationGateFindMemoryID(ctx, key); id != "" {
		_, _ = remediationGateMem.Update(ctx, &ai_memorypb.UpdateRqst{Memory: &ai_memorypb.Memory{
			Id:       id,
			Project:  remediationGateMemoryProject,
			Metadata: meta,
		}})
		return
	}
	_, _ = remediationGateMem.Store(ctx, &ai_memorypb.StoreRqst{Memory: &ai_memorypb.Memory{
		Project:  remediationGateMemoryProject,
		Type:     ai_memorypb.MemoryType_DEBUG,
		Tags:     []string{remediationGateTagBase, remediationGateTag(key)},
		Title:    "remediation escalation gate: " + key,
		Content:  "Auto-remediation escalation state for doctor gate key " + key + ". Observer memory — a safety refusal, not cluster desired/runtime state.",
		Metadata: meta,
		// TtlSeconds: 0 — no expiry; a safety refusal must not auto-clear on a
		// timer (forbidden_fix:auto_clear_escalation_without_operator_approval).
	}})
}

// remediationGateLoad reads persisted escalation state for a gate key from
// ai-memory. Returns (zero, false) when unwired, unreachable, absent, OR malformed
// — a corrupt record must never make the doctor behave unpredictably; the caller
// falls back to in-memory/default. (cautious owl, not haunted filing cabinet)
func remediationGateLoad(ctx context.Context, key string) (remediationGateState, bool) {
	if remediationGateMem == nil {
		return remediationGateState{}, false
	}
	resp, err := remediationGateMem.Query(ctx, &ai_memorypb.QueryRqst{
		Project: remediationGateMemoryProject,
		Tags:    []string{remediationGateTag(key)},
		Limit:   1,
	})
	if err != nil || resp == nil || len(resp.GetMemories()) == 0 {
		return remediationGateState{}, false
	}
	return parseRemediationGateState(resp.GetMemories()[0].GetMetadata())
}

// parseRemediationGateState defensively parses persisted metadata. If ANY field is
// missing or malformed it returns (zero, false): the doctor ignores the record and
// continues with in-memory/default behavior rather than trusting a corrupt
// escalation memory.
func parseRemediationGateState(meta map[string]string) (remediationGateState, bool) {
	if meta == nil {
		return remediationGateState{}, false
	}
	cr, err1 := strconv.Atoi(meta["cooldown_rejections"])
	lr, err2 := strconv.ParseInt(meta["last_rejection_at_unix"], 10, 64)
	esc, err3 := strconv.ParseBool(meta["escalated"])
	if err1 != nil || err2 != nil || err3 != nil || cr < 0 || lr < 0 {
		return remediationGateState{}, false
	}
	return remediationGateState{
		CooldownRejections: cr,
		LastRejectionAt:    lr,
		Escalated:          esc,
	}, true
}

// remediationGateDelete removes the persisted escalation memory for a gate key.
// Leader-only by construction. Called by remediationGateClear on successful
// authoritative remediation — the legitimate, success/operator-driven clear path
// (never a timer). Degrades to no-op when unwired/unreachable.
func remediationGateDelete(ctx context.Context, key string) {
	if remediationGateMem == nil {
		return
	}
	id := remediationGateFindMemoryID(ctx, key)
	if id == "" {
		return
	}
	_, _ = remediationGateMem.Delete(ctx, &ai_memorypb.DeleteRqst{
		Id:      id,
		Project: remediationGateMemoryProject,
	})
}
