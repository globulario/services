package behavioral_backfill

import (
	"context"
	"errors"
	"strings"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

// memoryBackfill carries per-memory mapping state.
type memoryBackfill struct {
	opts Options
	st   store.Store
	rep  *Report
}

// ── deterministic ids + provenance ────────────────────────────────────────────

func signalID(m *ai_memorypb.Memory) string    { return "signal.ai_memory." + m.GetId() }
func claimID(m *ai_memorypb.Memory, k string) string { return "claim.ai_memory." + m.GetId() + "." + k }
func outcomeID(m *ai_memorypb.Memory) string   { return "outcome.ai_memory." + m.GetId() }
func principleID(m *ai_memorypb.Memory) string { return "principle.ai_memory." + m.GetId() }

func sourceRefs(m *ai_memorypb.Memory) []string {
	refs := []string{SourcePrefix + m.GetId()}
	if c := m.GetConversationId(); c != "" {
		refs = append(refs, SourcePrefix+"conversation:"+c)
	}
	return refs
}
func generatedFrom(m *ai_memorypb.Memory) []string { return []string{SourcePrefix + m.GetId()} }

func provenance(m *ai_memorypb.Memory) api.Provenance {
	return api.Provenance{AgentID: m.GetAgentId(), MemoryID: m.GetId(), CreatedAt: m.GetCreatedAt(), UpdatedAt: m.GetUpdatedAt()}
}

// provMeta carries provenance breadcrumbs for rows whose proto has no first-class
// source_refs field (e.g. Signal/Claim/Outcome). It NEVER carries governance
// relations — those are first-class fields only.
func provMeta(m *ai_memorypb.Memory) map[string]string {
	meta := map[string]string{
		"source":         "ai_memory_backfill",
		"memory_id":      m.GetId(),
		"source_refs":    strings.Join(sourceRefs(m), ","),
		"generated_from": strings.Join(generatedFrom(m), ","),
	}
	if c := m.GetConversationId(); c != "" {
		meta["conversation_id"] = c
	}
	return meta
}

func entityRef(m *ai_memorypb.Memory) string {
	if r := m.GetMetadata()["related_to"]; r != "" {
		return r
	}
	if len(m.GetTags()) > 0 {
		return m.GetTags()[0]
	}
	return ""
}

func payload(m *ai_memorypb.Memory) string {
	c := m.GetContent()
	if len(c) > 1024 {
		c = c[:1024]
	}
	if t := m.GetTitle(); t != "" {
		return t + "\n" + c
	}
	return c
}

// process maps one memory into behavioral candidate rows.
func (bf *memoryBackfill) process(ctx context.Context, m *ai_memorypb.Memory) error {
	if !convertibleTypes[m.GetType()] {
		bf.rep.skip("non-operational memory type")
		return nil
	}

	// 1. Always: a historical-memory signal.
	if err := bf.writeSignal(ctx, &api.Signal{
		ID: signalID(m), Project: bf.opts.Project, Domain: api.DomainRef(bf.opts.Domain),
		Kind: api.SignalHistoricalMemory, SourceKind: "ai_memory", SourceRef: m.GetId(),
		EntityRef: entityRef(m), Payload: payload(m), Confidence: defaultConfidence,
		Status: api.StatusRawSignal, Provenance: provenance(m), Metadata: provMeta(m),
	}); err != nil {
		return err
	}

	// 2. Metadata → claim candidates (only the known, unambiguous keys).
	for _, k := range claimKeys {
		v := strings.TrimSpace(m.GetMetadata()[k])
		if v == "" {
			continue
		}
		if err := bf.writeClaim(ctx, &api.Claim{
			ID: claimID(m, k), Project: bf.opts.Project, Domain: api.DomainRef(bf.opts.Domain),
			SignalID: signalID(m), Statement: v, Predicate: k, SubjectEntity: entityRef(m),
			Status: api.StatusCandidateFact, Confidence: defaultConfidence, SourceID: m.GetId(),
			Provenance: provenance(m), Metadata: provMeta(m),
		}); err != nil {
			return err
		}
	}

	// 3. Feedback → outcome, ONLY when an explicit outcome status exists.
	if st := explicitOutcomeStatus(m); st != "" {
		if err := bf.writeOutcome(ctx, &api.Outcome{
			ID: outcomeID(m), Project: bf.opts.Project, Domain: api.DomainRef(bf.opts.Domain),
			Status: st, Theme: outcomeTheme(m), Note: m.GetTitle(), AgentID: m.GetAgentId(),
			IncidentID: m.GetMetadata()["incident_id"], CreatedAt: m.GetCreatedAt(), Metadata: provMeta(m),
		}); err != nil {
			return err
		}
	} else if m.GetType() == ai_memorypb.MemoryType_FEEDBACK {
		bf.rep.skip("feedback without explicit outcome status")
	}

	// 4. Decision/architecture → PROPOSED principle, ONLY when every governance
	//    field is present; otherwise report the gap (a signal/claim already exists).
	bf.maybePrinciple(ctx, m)
	return nil
}

// explicitOutcomeStatus returns a trusted outcome status from explicit metadata
// or a clear tag, or "" — it never infers success/failure from prose.
func explicitOutcomeStatus(m *ai_memorypb.Memory) string {
	for _, k := range []string{"outcome", "result", "status"} {
		if v := strings.ToLower(strings.TrimSpace(m.GetMetadata()[k])); outcomeStatuses[v] {
			return v
		}
	}
	for _, t := range m.GetTags() {
		if outcomeStatuses[strings.ToLower(t)] {
			return strings.ToLower(t)
		}
	}
	return ""
}

func outcomeTheme(m *ai_memorypb.Memory) string {
	if t := m.GetMetadata()["theme"]; t != "" {
		return t
	}
	if len(m.GetTags()) > 0 {
		return m.GetTags()[0]
	}
	return ""
}

// requiredPrincipleFields are the governance fields a memory must explicitly carry
// to become a PROPOSED principle. Anything missing → no principle.
var requiredPrincipleFields = []string{"applies_when", "authority", "required_evidence", "forbidden_or_safe", "promotion_reason", "revocation_rule", "risk_level"}

func (bf *memoryBackfill) maybePrinciple(ctx context.Context, m *ai_memorypb.Memory) {
	if m.GetType() != ai_memorypb.MemoryType_DECISION && m.GetType() != ai_memorypb.MemoryType_ARCHITECTURE {
		return
	}
	md := m.GetMetadata()
	// Only evaluate memories that look like a principle candidate (carry at least
	// one governance signal) — avoids reporting every note as a missing principle.
	if md["condition"] == "" && md["applies_when"] == "" && md["risk_level"] == "" && md["promotion_reason"] == "" && md["authority"] == "" {
		return
	}

	get := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(md[k]); v != "" {
				return v
			}
		}
		return ""
	}
	appliesWhen := get("applies_when", "condition")
	authority := get("authority", "authorities")
	requiredEv := get("required_evidence", "proof")
	forbiddenOrSafe := get("forbidden_move", "forbidden_moves", "safe_behavior", "recommended_behavior")
	promotionReason := get("promotion_reason")
	revocationRule := get("revocation_rule")
	riskLevel := strings.ToLower(get("risk_level"))

	vals := map[string]string{
		"applies_when": appliesWhen, "authority": authority, "required_evidence": requiredEv,
		"forbidden_or_safe": forbiddenOrSafe, "promotion_reason": promotionReason,
		"revocation_rule": revocationRule, "risk_level": riskLevel,
	}
	var missing []string
	for _, f := range requiredPrincipleFields {
		if vals[f] == "" {
			missing = append(missing, f)
		}
	}
	if m.GetTitle() == "" {
		missing = append(missing, "title")
	}
	if len(missing) > 0 {
		bf.rep.MissingFields = append(bf.rep.MissingFields, PrincipleGap{MemoryID: m.GetId(), Missing: missing})
		return
	}

	p := &api.Principle{
		ID: principleID(m), Project: bf.opts.Project, Domain: api.DomainRef(bf.opts.Domain), Title: m.GetTitle(),
		AppliesWhen:      toCondRefs(csv(appliesWhen)),
		Authorities:      toAuthRefs(csv(authority)),
		RequiredEvidence: toReqRefs(csv(requiredEv)),
		ForbiddenMoves:   toFmRefs(csv(get("forbidden_move", "forbidden_moves"))),
		RecommendedAction: get("recommended_behavior", "safe_behavior"),
		RiskLevel:         riskLevel, PromotionReason: promotionReason, RevocationRule: revocationRule,
		Status: api.StatusProposedPrinciple, Version: 1, ProposedBy: "backfill:ai_memory",
		SourceRefs: sourceRefs(m), GeneratedFrom: generatedFrom(m),
		Provenance: provenance(m), Metadata: provMeta(m),
	}
	bf.writePrinciple(ctx, p)
}

// ── idempotent writes (get-before-put; dry-run counts only) ───────────────────

func (bf *memoryBackfill) writeSignal(ctx context.Context, s *api.Signal) error {
	if bf.opts.DryRun {
		bf.rep.record("signal", true)
		return nil
	}
	if _, err := bf.st.GetSignal(ctx, s.Project, string(s.Domain), s.ID); err == nil {
		bf.rep.skip("signal exists (idempotent)")
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if err := bf.st.PutSignal(ctx, s); err != nil {
		return err
	}
	bf.rep.record("signal", false)
	return nil
}

func (bf *memoryBackfill) writeClaim(ctx context.Context, c *api.Claim) error {
	if bf.opts.DryRun {
		bf.rep.record("claim", true)
		return nil
	}
	if _, err := bf.st.GetClaim(ctx, c.Project, string(c.Domain), c.ID); err == nil {
		bf.rep.skip("claim exists (idempotent)")
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if err := bf.st.PutClaim(ctx, c); err != nil {
		return err
	}
	bf.rep.record("claim", false)
	return nil
}

func (bf *memoryBackfill) writeOutcome(ctx context.Context, o *api.Outcome) error {
	if bf.opts.DryRun {
		bf.rep.record("outcome", true)
		return nil
	}
	if _, err := bf.st.GetOutcome(ctx, o.Project, string(o.Domain), o.ID); err == nil {
		bf.rep.skip("outcome exists (idempotent)")
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if err := bf.st.RecordOutcome(ctx, o); err != nil {
		return err
	}
	bf.rep.record("outcome", false)
	return nil
}

func (bf *memoryBackfill) writePrinciple(ctx context.Context, p *api.Principle) error {
	if bf.opts.DryRun {
		bf.rep.record("principle", true)
		return nil
	}
	existing, err := bf.st.GetPrinciple(ctx, p.Project, string(p.Domain), p.ID)
	if err == nil {
		// NEVER overwrite a governed principle; never demote.
		if existing.Status != api.StatusProposedPrinciple {
			bf.rep.skip("principle already governed — not overwritten")
			return nil
		}
		if !bf.opts.Overwrite {
			bf.rep.skip("principle exists (idempotent)")
			return nil
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if err := bf.st.CreatePrinciple(ctx, p); err != nil {
		return err
	}
	bf.rep.record("principle", false)
	return nil
}

// ── small helpers ─────────────────────────────────────────────────────────────

func csv(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func toCondRefs(in []string) []api.ConditionRef {
	out := make([]api.ConditionRef, len(in))
	for i, v := range in {
		out[i] = api.ConditionRef(v)
	}
	return out
}
func toAuthRefs(in []string) []api.AuthorityRef {
	out := make([]api.AuthorityRef, len(in))
	for i, v := range in {
		out[i] = api.AuthorityRef(v)
	}
	return out
}
func toReqRefs(in []string) []api.RequiredEvidenceRef {
	out := make([]api.RequiredEvidenceRef, len(in))
	for i, v := range in {
		out[i] = api.RequiredEvidenceRef(v)
	}
	return out
}
func toFmRefs(in []string) []api.ForbiddenMoveRef {
	out := make([]api.ForbiddenMoveRef, len(in))
	for i, v := range in {
		out[i] = api.ForbiddenMoveRef(v)
	}
	return out
}
