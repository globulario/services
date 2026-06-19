// @awareness namespace=globular.platform
// @awareness component=platform_ai_executor.dedup
// @awareness file_role=incident_fingerprint_ledger_skips_repeat_diagnosis
// @awareness implements=globular.platform:intent.ai.memory_queried_before_claude_to_boost_confidence
// @awareness implements=globular.platform:intent.ai.supplementary_not_required
// @awareness risk=high
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"google.golang.org/grpc"
)

// incidentLedger persists one durable record per distinct incident *signature*
// (rule + trigger event + affected service) and counts how often that signature
// recurs. It is the dedup gate that makes the diagnoser honour the intent
// ai.memory_queried_before_claude_to_boost_confidence: a signature that has
// already been diagnosed is reused — the expensive LLM call is skipped and the
// occurrence counter is bumped instead.
//
// Without this, a failing workflow that emits the same incident every ~90s
// triggers a full AI diagnosis on every repeat — re-doing identical work and
// burning tokens for no new information.
type incidentLedger struct {
	memoryAddr string // optional override; falls back to service discovery
}

// dedupTagPrefix namespaces the fingerprint tag so the read query and the write
// store agree on the same key. The historical mismatch — reads tagged by ruleID,
// writes tagged by root_cause — is exactly why dedup never fired before.
const dedupTagPrefix = "fp:"

// ledgerEntry is the decoded prior diagnosis for a recurring signature.
type ledgerEntry struct {
	memoryID       string
	fingerprint    string
	rootCause      string
	proposedAction string
	actionReason   string
	confidence     float32
	occurrences    int32
}

// incidentFingerprint derives a stable signature from the parts of an incident
// that identify "the same problem recurring": the rule, the trigger event, and
// the affected service/unit. The incident_id is deliberately excluded — it is
// unique per occurrence, so including it would make every repeat look new.
func incidentFingerprint(req *ai_executorpb.ProcessIncidentRequest) string {
	var payload map[string]interface{}
	if len(req.GetTriggerEventData()) > 0 {
		_ = json.Unmarshal(req.GetTriggerEventData(), &payload)
	}
	svc, _ := payload["service"].(string)
	if svc == "" {
		svc, _ = payload["unit"].(string)
	}
	key := strings.Join([]string{req.GetRuleId(), req.GetTriggerEventName(), svc}, "|")
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:8])
}

// dial opens a short-lived mTLS connection to ai_memory, or returns nil if the
// service can't be resolved. ai_memory is non-critical: when it's unreachable
// the ledger degrades to "no dedup" rather than blocking diagnosis (the cluster
// must converge without AI services — intent.ai.supplementary_not_required).
func (l *incidentLedger) dial() (*grpc.ClientConn, error) {
	addr := l.memoryAddr
	if addr == "" {
		addr = config.ResolveServiceAddr("ai_memory.AiMemoryService", "")
	}
	if addr == "" {
		return nil, fmt.Errorf("ai_memory not resolvable")
	}
	baseOpts, err := globular.InternalDialOptions()
	if err != nil {
		return nil, err
	}
	opts := append(baseOpts, grpc.WithTimeout(2*time.Second))
	return grpc.Dial(addr, opts...)
}

// lookup returns the prior diagnosis for this signature, or nil if it has never
// been seen (or ai_memory is unavailable). A nil result is "unknown — treat as
// new and re-diagnose", never "known-good": absence scope is explicit, so a
// missing record never silences a real incident.
func (l *incidentLedger) lookup(ctx context.Context, fingerprint string) *ledgerEntry {
	cc, err := l.dial()
	if err != nil {
		return nil
	}
	defer cc.Close()

	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := ai_memorypb.NewAiMemoryServiceClient(cc).Query(callCtx, &ai_memorypb.QueryRqst{
		Project: "globular-services",
		Type:    ai_memorypb.MemoryType_DEBUG,
		Tags:    []string{"incident", dedupTagPrefix + fingerprint},
		Limit:   1,
	})
	if err != nil || resp == nil || len(resp.Memories) == 0 {
		return nil
	}

	mem := resp.Memories[0]
	occ, _ := strconv.Atoi(mem.Metadata["occurrences"])
	conf, _ := strconv.ParseFloat(mem.Metadata["confidence"], 32)
	return &ledgerEntry{
		memoryID:       mem.Id,
		fingerprint:    fingerprint,
		rootCause:      mem.Metadata["root_cause"],
		proposedAction: mem.Metadata["proposed_action"],
		actionReason:   mem.Metadata["action_reason"],
		confidence:     float32(conf),
		occurrences:    int32(occ),
	}
}

// recordNew persists the first diagnosis for a signature so the next occurrence
// can be deduped. Best-effort and fire-and-forget — a failed write only means
// the next repeat re-diagnoses, never a stall.
func (l *incidentLedger) recordNew(ctx context.Context, fingerprint string, d *ai_executorpb.Diagnosis) {
	cc, err := l.dial()
	if err != nil {
		return
	}
	defer cc.Close()

	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, _ = ai_memorypb.NewAiMemoryServiceClient(cc).Store(callCtx, &ai_memorypb.StoreRqst{
		Memory: &ai_memorypb.Memory{
			Project: "globular-services",
			Type:    ai_memorypb.MemoryType_DEBUG,
			Title:   fmt.Sprintf("incident-signature: %s", d.GetRootCause()),
			Content: d.GetDetail(),
			Tags:    []string{"incident", "dedup", dedupTagPrefix + fingerprint},
			Metadata: map[string]string{
				"fingerprint":     fingerprint,
				"root_cause":      d.GetRootCause(),
				"proposed_action": d.GetProposedAction(),
				"action_reason":   d.GetActionReason(),
				"confidence":      fmt.Sprintf("%.2f", d.GetConfidence()),
				"occurrences":     "1",
			},
		},
	})
}

// recordRepeat bumps the occurrence counter for a known signature. The full
// metadata is rewritten (not just the counter) because Update replaces the
// metadata map wholesale — sending only "occurrences" would drop root_cause and
// the rest, breaking the next lookup.
func (l *incidentLedger) recordRepeat(ctx context.Context, e *ledgerEntry) {
	cc, err := l.dial()
	if err != nil {
		return
	}
	defer cc.Close()

	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, _ = ai_memorypb.NewAiMemoryServiceClient(cc).Update(callCtx, &ai_memorypb.UpdateRqst{
		Memory: &ai_memorypb.Memory{
			Id:      e.memoryID,
			Project: "globular-services",
			Metadata: map[string]string{
				"fingerprint":     e.fingerprint,
				"root_cause":      e.rootCause,
				"proposed_action": e.proposedAction,
				"action_reason":   e.actionReason,
				"confidence":      fmt.Sprintf("%.2f", e.confidence),
				"occurrences":     strconv.Itoa(int(e.occurrences + 1)),
			},
		},
	})
}
