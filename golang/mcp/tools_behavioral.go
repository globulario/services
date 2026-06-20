package main

// tools_behavioral.go exposes the behavioral-memory runtime (the
// BehavioralMemoryService RPCs) to AI agents as MCP tools, so agents can run the
// governed operator loop:
//
//	record_signal → resolve_context → check_action → (act outside) → record_outcome
//	            → optionally propose_principle
//
// These tools ADVISE, GATE, RECORD, and LEARN. They never execute cluster
// operations, never run probes, and never bypass the promotion gate — promotion
// and revocation flow through the gate-enforcing RPCs (which additionally carry
// ai.behavioral.promote authz). The tools are additive; the existing ai-memory
// MCP tools are untouched.
//
// For cluster operation, examples use project=globular-services and
// domain=cluster_operator, but both are tool inputs — no cluster_operator package
// is imported here (the generic boundary stays clean).

import (
	"context"
	"fmt"
	"strings"
	"time"

	behavioralpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
	"github.com/globulario/services/golang/config"
)

// behavioralEndpoint resolves the address of behavioral_memory.BehavioralMemoryService.
// That service is co-hosted in the ai-memory binary (same port), so it is reached
// at the ai-memory service address via the normal discovery path. It is a var so
// tests can point it at an in-process server.
var behavioralEndpoint = func() string {
	return config.ResolveServiceAddr("ai_memory.AiMemoryService", gatewayEndpoint())
}

// csvArg splits a comma-separated string argument into trimmed, non-empty parts.
func csvArg(args map[string]interface{}, key string) []string {
	raw := strArg(args, key)
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func boolArg(args map[string]interface{}, key string) bool {
	if b, ok := args[key].(bool); ok {
		return b
	}
	return false
}

func f32Arg(args map[string]interface{}, key string) float32 {
	if f, ok := args[key].(float64); ok {
		return float32(f)
	}
	return 0
}

func parseSignalKind(s string) behavioralpb.SignalKind {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "OBSERVED_RUNTIME_FACT", "OBSERVED", "RUNTIME_FACT":
		return behavioralpb.SignalKind_SIGNAL_OBSERVED_RUNTIME_FACT
	case "AGENT_INTERPRETATION", "INTERPRETATION":
		return behavioralpb.SignalKind_SIGNAL_AGENT_INTERPRETATION
	case "HUMAN_CORRECTION", "CORRECTION":
		return behavioralpb.SignalKind_SIGNAL_HUMAN_CORRECTION
	case "AUTOMATED_HEALTH", "HEALTH":
		return behavioralpb.SignalKind_SIGNAL_AUTOMATED_HEALTH
	case "HISTORICAL_MEMORY", "MEMORY":
		return behavioralpb.SignalKind_SIGNAL_HISTORICAL_MEMORY
	default:
		return behavioralpb.SignalKind_SIGNAL_KIND_UNSPECIFIED
	}
}

func behavioralClient(ctx context.Context, s *server) (behavioralpb.BehavioralMemoryServiceClient, error) {
	conn, err := s.clients.get(ctx, behavioralEndpoint())
	if err != nil {
		return nil, err
	}
	return behavioralpb.NewBehavioralMemoryServiceClient(conn), nil
}

const signalKindEnum = "OBSERVED_RUNTIME_FACT | AGENT_INTERPRETATION | HUMAN_CORRECTION | AUTOMATED_HEALTH | HISTORICAL_MEMORY"

func registerBehavioralTools(s *server) {
	registerResolveContextTool(s)
	registerCheckActionTool(s)
	registerRecordSignalTool(s)
	registerRecordOutcomeTool(s)
	registerExplainPrincipleTool(s)
	registerProposePrincipleTool(s)
	registerPromotePrincipleTool(s)
	registerRevokePrincipleTool(s)
}

// ── behavioral_resolve_context ────────────────────────────────────────────────

func registerResolveContextTool(s *server) {
	s.register(toolDef{
		Name: "behavioral_resolve_context",
		Description: "Pre-action briefing: given a goal and current runtime conditions, return the " +
			"governed behavioral memory that applies BEFORE acting — applicable promoted principles, " +
			"recommended behavior, required evidence, governing authorities, forbidden moves, open " +
			"contradictions, prior outcomes, and a confidence class. It does NOT run probes and does " +
			"NOT decide allowed/blocked (use behavioral_check_action for that). Cluster ops: " +
			"project=globular-services, domain=cluster_operator.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"project":    {Type: "string", Description: "Project, e.g. 'globular-services'"},
				"domain":     {Type: "string", Description: "Domain, e.g. 'cluster_operator'"},
				"goal":       {Type: "string", Description: "What the agent is trying to do"},
				"conditions": {Type: "string", Description: "Comma-separated current condition refs, e.g. 'condition.cluster.etcd.nospace_alarm'"},
				"entity_ref": {Type: "string", Description: "Optional: the entity in scope (node/service id)"},
				"cluster_id": {Type: "string", Description: "Optional: cluster scope id"},
				"agent_id":   {Type: "string", Description: "Optional: calling agent id"},
			},
			Required: []string{"project", "domain"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		client, err := behavioralClient(ctx, s)
		if err != nil {
			return nil, err
		}
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()
		rsp, err := client.ResolveGovernedContext(callCtx, &behavioralpb.ResolveGovernedContextRequest{
			Project: strArg(args, "project"), Domain: strArg(args, "domain"), Goal: strArg(args, "goal"),
			Conditions: csvArg(args, "conditions"), EntityRef: strArg(args, "entity_ref"), Scope: strArg(args, "cluster_id"),
		})
		if err != nil {
			return nil, fmt.Errorf("behavioral_resolve_context: %w", err)
		}
		c := rsp.GetContext()
		principles := make([]map[string]interface{}, 0, len(c.GetApplicablePrinciples()))
		for _, p := range c.GetApplicablePrinciples() {
			principles = append(principles, map[string]interface{}{
				"id": p.GetId(), "title": p.GetTitle(), "risk_level": p.GetRiskLevel(),
				"recommended_action": p.GetRecommendedAction(),
				"forbidden_moves":    p.GetForbiddenMoves(), "required_evidence": p.GetRequiredEvidence(),
				"authorities": p.GetAuthorities(),
			})
		}
		return map[string]interface{}{
			"applicable_principles": principles,
			"recommended_behavior":  c.GetRecommendedBehavior(),
			"required_evidence":     entryIDs(c.GetRequiredEvidence()),
			"forbidden_moves":       forbiddenIDs(c.GetForbiddenMoves()),
			"unresolved_authority":  authorityIDs(c.GetUnresolvedAuthority()),
			"open_contradictions":   len(c.GetKnownContradictions()),
			"prior_outcomes":        len(c.GetPriorOutcomes()),
			"confidence":            c.GetConfidence(),
		}, nil
	})
}

// ── behavioral_check_action ───────────────────────────────────────────────────

func registerCheckActionTool(s *server) {
	s.register(toolDef{
		Name: "behavioral_check_action",
		Description: "The safety gate: ask whether a proposed action is allowed under promoted " +
			"behavioral principles. Returns allowed | blocked | needs_evidence | needs_authority | " +
			"needs_human_approval, with the violated principles, missing evidence, matched forbidden " +
			"moves, recommended next steps, and an action_check_id (every call is audited). It does " +
			"NOT run probes — it evaluates already-recorded evidence and the declared " +
			"provided_evidence_refs. It does NOT execute the action.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"project":                {Type: "string", Description: "Project, e.g. 'globular-services'"},
				"domain":                 {Type: "string", Description: "Domain, e.g. 'cluster_operator'"},
				"action_type":            {Type: "string", Description: "The action the agent proposes (matched against forbidden moves)"},
				"target_ref":             {Type: "string", Description: "The target of the action (node/service/resource ref)"},
				"conditions":             {Type: "string", Description: "Comma-separated current condition refs"},
				"provided_evidence_refs": {Type: "string", Description: "Comma-separated required-evidence refs the agent already holds"},
				"human_approval":         {Type: "string", Description: "Optional approver id; satisfies needs_human_approval"},
				"agent_id":               {Type: "string", Description: "Optional: calling agent id"},
				"cluster_id":             {Type: "string", Description: "Optional: cluster scope id"},
			},
			Required: []string{"project", "domain", "action_type"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		client, err := behavioralClient(ctx, s)
		if err != nil {
			return nil, err
		}
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()
		rsp, err := client.CheckAction(callCtx, &behavioralpb.CheckActionRequest{
			Project: strArg(args, "project"), Domain: strArg(args, "domain"),
			ActionType: strArg(args, "action_type"), Target: strArg(args, "target_ref"),
			CurrentConditions: csvArg(args, "conditions"), ProvidedEvidenceRefs: csvArg(args, "provided_evidence_refs"),
			HumanApproval: strArg(args, "human_approval"), AgentId: strArg(args, "agent_id"), Scope: strArg(args, "cluster_id"),
		})
		if err != nil {
			return nil, fmt.Errorf("behavioral_check_action: %w", err)
		}
		r := rsp.GetResult()
		return map[string]interface{}{
			"allowed":              r.GetAllowed(),
			"status":               r.GetStatus(),
			"violated_principles":  r.GetViolatedPrinciples(),
			"missing_evidence":     r.GetMissingEvidence(),
			"unresolved_authority": r.GetUnresolvedAuthority(),
			"forbidden_matched":    r.GetForbiddenMatched(),
			"recommended_steps":    r.GetRecommendedSteps(),
			"explanation":          r.GetExplanation(),
			"action_check_id":      r.GetId(),
		}, nil
	})
}

// ── behavioral_record_signal ──────────────────────────────────────────────────

func registerRecordSignalTool(s *server) {
	s.register(toolDef{
		Name: "behavioral_record_signal",
		Description: "Record a raw operational signal — an observed runtime fact, an agent " +
			"interpretation, a human correction, an automated health fact, or a historical-memory " +
			"signal. Signals enter at RAW_SIGNAL; this tool CANNOT create promoted principles. " +
			"Returns the signal id, status, and canonical_uri.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"project":     {Type: "string", Description: "Project, e.g. 'globular-services'"},
				"domain":      {Type: "string", Description: "Domain, e.g. 'cluster_operator'"},
				"signal_kind": {Type: "string", Description: "Signal kind: " + signalKindEnum, Enum: []string{"OBSERVED_RUNTIME_FACT", "AGENT_INTERPRETATION", "HUMAN_CORRECTION", "AUTOMATED_HEALTH", "HISTORICAL_MEMORY"}},
				"source_kind": {Type: "string", Description: "Origin kind: log|metric|probe|agent|human|test|status"},
				"source_ref":  {Type: "string", Description: "Pointer to the origin"},
				"entity_ref":  {Type: "string", Description: "The entity the signal concerns"},
				"payload":     {Type: "string", Description: "The signal content"},
				"confidence":  {Type: "number", Description: "0..1 confidence"},
				"agent_id":    {Type: "string", Description: "Optional: calling agent id"},
				"cluster_id":  {Type: "string", Description: "Optional: cluster scope id"},
			},
			Required: []string{"project", "domain", "signal_kind", "payload"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		client, err := behavioralClient(ctx, s)
		if err != nil {
			return nil, err
		}
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()
		rsp, err := client.RecordSignal(callCtx, &behavioralpb.RecordSignalRequest{Signal: &behavioralpb.Signal{
			Project: strArg(args, "project"), Domain: strArg(args, "domain"), Kind: parseSignalKind(strArg(args, "signal_kind")),
			SourceKind: strArg(args, "source_kind"), SourceRef: strArg(args, "source_ref"), EntityRef: strArg(args, "entity_ref"),
			Scope: strArg(args, "cluster_id"), Payload: strArg(args, "payload"), Confidence: f32Arg(args, "confidence"),
			AgentId: strArg(args, "agent_id"),
		}})
		if err != nil {
			return nil, fmt.Errorf("behavioral_record_signal: %w", err)
		}
		return map[string]interface{}{
			"signal_id":     rsp.GetSignalId(),
			"status":        rsp.GetStatus().String(),
			"canonical_uri": "behavioral:signal/" + rsp.GetSignalId(),
		}, nil
	})
}

// ── behavioral_record_outcome ─────────────────────────────────────────────────

func registerRecordOutcomeTool(s *server) {
	s.register(toolDef{
		Name: "behavioral_record_outcome",
		Description: "Record what happened after an action/check (success|failure|blocked|reverted), " +
			"optionally severe / human_marked, linked to an action_check_id, principles, evidence, an " +
			"incident, and a theme (for later pattern detection). This is NOT a promotion tool — it " +
			"records outcome facts only.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"project":         {Type: "string", Description: "Project, e.g. 'globular-services'"},
				"domain":          {Type: "string", Description: "Domain, e.g. 'cluster_operator'"},
				"action_check_id": {Type: "string", Description: "The action_check_id this outcome followed"},
				"principle_ids":   {Type: "string", Description: "Comma-separated principle ids referenced"},
				"evidence_ids":    {Type: "string", Description: "Comma-separated evidence ids"},
				"status":          {Type: "string", Description: "success | failure | blocked | reverted", Enum: []string{"success", "failure", "blocked", "reverted"}},
				"severe":          {Type: "boolean", Description: "Whether this was a severe outcome"},
				"human_marked":    {Type: "boolean", Description: "Whether a human flagged this outcome"},
				"incident_id":     {Type: "string", Description: "Optional linked incident id"},
				"theme":           {Type: "string", Description: "Grouping key for repeated patterns"},
				"note":            {Type: "string", Description: "Free-form note"},
				"agent_id":        {Type: "string", Description: "Optional: calling agent id"},
			},
			Required: []string{"project", "domain", "status"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		client, err := behavioralClient(ctx, s)
		if err != nil {
			return nil, err
		}
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()
		rsp, err := client.RecordOutcome(callCtx, &behavioralpb.RecordOutcomeRequest{Outcome: &behavioralpb.Outcome{
			Project: strArg(args, "project"), Domain: strArg(args, "domain"), ActionCheckId: strArg(args, "action_check_id"),
			PrincipleIds: csvArg(args, "principle_ids"), EvidenceIds: csvArg(args, "evidence_ids"),
			Status: strArg(args, "status"), Severe: boolArg(args, "severe"), HumanMarked: boolArg(args, "human_marked"),
			IncidentId: strArg(args, "incident_id"), Theme: strArg(args, "theme"), Note: strArg(args, "note"), AgentId: strArg(args, "agent_id"),
		}})
		if err != nil {
			return nil, fmt.Errorf("behavioral_record_outcome: %w", err)
		}
		return map[string]interface{}{
			"outcome_id": rsp.GetOutcomeId(),
			"theme":      strArg(args, "theme"),
			"status":     strArg(args, "status"),
		}, nil
	})
}

// ── behavioral_explain_principle ──────────────────────────────────────────────

func registerExplainPrincipleTool(s *server) {
	s.register(toolDef{
		Name: "behavioral_explain_principle",
		Description: "Explain why a behavioral principle exists and what it requires: status, applies-when " +
			"conditions, governing authorities, required evidence, forbidden moves, recommended behavior, " +
			"the latest promotion decision, revocation status, and source lineage (source_refs / " +
			"generated_from). Read-only; no probes.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"project":      {Type: "string", Description: "Project, e.g. 'globular-services'"},
				"domain":       {Type: "string", Description: "Domain, e.g. 'cluster_operator'"},
				"principle_id": {Type: "string", Description: "The principle id to explain"},
			},
			Required: []string{"project", "domain", "principle_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		client, err := behavioralClient(ctx, s)
		if err != nil {
			return nil, err
		}
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()
		rsp, err := client.ExplainPrinciple(callCtx, &behavioralpb.ExplainPrincipleRequest{
			PrincipleId: strArg(args, "principle_id"), Project: strArg(args, "project"), Domain: strArg(args, "domain"),
		})
		if err != nil {
			return nil, fmt.Errorf("behavioral_explain_principle: %w", err)
		}
		p := rsp.GetPrinciple()
		out := map[string]interface{}{
			"id": p.GetId(), "title": p.GetTitle(), "status": p.GetStatus().String(), "risk_level": p.GetRiskLevel(),
			"conditions": p.GetAppliesWhen(), "authorities": p.GetAuthorities(),
			"required_evidence": p.GetRequiredEvidence(), "forbidden_moves": p.GetForbiddenMoves(),
			"recommended_behavior": p.GetRecommendedAction(),
			"source_refs":          p.GetSourceRefs(), "generated_from": p.GetGeneratedFrom(),
			"explanation": rsp.GetExplanation(),
		}
		if hist := rsp.GetPromotionHistory(); len(hist) > 0 {
			last := hist[len(hist)-1]
			out["promotion_decision"] = map[string]interface{}{"decision": last.GetDecision().String(), "verdict": last.GetVerdict()}
		}
		if rules := rsp.GetRevocationRules(); len(rules) > 0 {
			out["revocation"] = map[string]interface{}{"action": rules[len(rules)-1].GetAction(), "reason": rules[len(rules)-1].GetRevocationReason()}
		}
		return out, nil
	})
}

// ── behavioral_propose_principle ──────────────────────────────────────────────

func registerProposePrincipleTool(s *server) {
	s.register(toolDef{
		Name: "behavioral_propose_principle",
		Description: "Propose a NEW behavioral principle candidate from repeated outcomes or a human " +
			"correction. The principle is created at PROPOSED_PRINCIPLE — this tool NEVER promotes. " +
			"Promotion happens later through behavioral_promote_principle and the gate. Governance " +
			"relations (applies_when / authorities / required_evidence / forbidden_moves) are " +
			"first-class inputs, not metadata.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"project":              {Type: "string", Description: "Project, e.g. 'globular-services'"},
				"domain":               {Type: "string", Description: "Domain, e.g. 'cluster_operator'"},
				"title":                {Type: "string", Description: "Short principle title"},
				"description":          {Type: "string", Description: "Narrative description of the principle"},
				"applies_when":         {Type: "string", Description: "Comma-separated condition refs the principle applies under"},
				"authorities":          {Type: "string", Description: "Comma-separated governing authority refs"},
				"required_evidence":    {Type: "string", Description: "Comma-separated required-evidence refs"},
				"forbidden_moves":      {Type: "string", Description: "Comma-separated forbidden-move refs"},
				"recommended_behavior": {Type: "string", Description: "The generative safe behavior to prefer"},
				"risk_level":           {Type: "string", Description: "info | low | high | irreversible", Enum: []string{"info", "low", "high", "irreversible"}},
				"promotion_reason":     {Type: "string", Description: "Why this should eventually be promoted"},
				"revocation_rule":      {Type: "string", Description: "When this principle should be revoked/narrowed"},
				"source_refs":          {Type: "string", Description: "Comma-separated provenance source refs"},
				"generated_from":       {Type: "string", Description: "Comma-separated lineage refs"},
				"actor":                {Type: "string", Description: "Who is proposing this principle"},
			},
			Required: []string{"project", "domain", "title", "recommended_behavior", "risk_level", "promotion_reason", "revocation_rule", "actor"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		client, err := behavioralClient(ctx, s)
		if err != nil {
			return nil, err
		}
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()
		principle := &behavioralpb.Principle{
			Project: strArg(args, "project"), Domain: strArg(args, "domain"), Title: strArg(args, "title"),
			AppliesWhen: csvArg(args, "applies_when"), Authorities: csvArg(args, "authorities"),
			RequiredEvidence: csvArg(args, "required_evidence"), ForbiddenMoves: csvArg(args, "forbidden_moves"),
			RecommendedAction: strArg(args, "recommended_behavior"), RiskLevel: strArg(args, "risk_level"),
			PromotionReason: strArg(args, "promotion_reason"), RevocationRule: strArg(args, "revocation_rule"),
			SourceRefs: csvArg(args, "source_refs"), GeneratedFrom: csvArg(args, "generated_from"),
			ProposedBy: strArg(args, "actor"),
		}
		if d := strArg(args, "description"); d != "" {
			principle.Metadata = map[string]string{"description": d}
		}
		rsp, err := client.ProposePrinciple(callCtx, &behavioralpb.ProposePrincipleRequest{Principle: principle})
		if err != nil {
			return nil, fmt.Errorf("behavioral_propose_principle: %w", err)
		}
		return map[string]interface{}{
			"principle_id": rsp.GetPrincipleId(),
			"status":       rsp.GetStatus().String(),
		}, nil
	})
}

// ── behavioral_promote_principle (gated) ──────────────────────────────────────

func registerPromotePrincipleTool(s *server) {
	s.register(toolDef{
		Name: "behavioral_promote_principle",
		Description: "Run the promotion GATE for a candidate principle. Requires actor and reason; " +
			"high/irreversible-risk principles additionally require approved_by. Returns the promotion " +
			"decision (ALLOWED | BLOCKED | REVIEW_REQUIRED) with the full verdict — BLOCKED decisions " +
			"are returned, never hidden, and the gate is never bypassed. The underlying RPC enforces " +
			"the ai.behavioral.promote (admin) permission, stricter than read/check tools.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"project":         {Type: "string", Description: "Project, e.g. 'globular-services'"},
				"domain":          {Type: "string", Description: "Domain, e.g. 'cluster_operator'"},
				"principle_id":    {Type: "string", Description: "Candidate principle id to promote"},
				"actor":           {Type: "string", Description: "Who is attempting the promotion (required, audited)"},
				"reason":          {Type: "string", Description: "Why promote now (required)"},
				"approved_by":     {Type: "string", Description: "Human approver — required to promote high/irreversible risk"},
				"approval_reason": {Type: "string", Description: "Approval rationale"},
			},
			Required: []string{"project", "domain", "principle_id", "actor", "reason"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if strArg(args, "actor") == "" || strArg(args, "reason") == "" {
			return nil, fmt.Errorf("behavioral_promote_principle: actor and reason are required")
		}
		client, err := behavioralClient(ctx, s)
		if err != nil {
			return nil, err
		}
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()
		rsp, err := client.PromotePrinciple(callCtx, &behavioralpb.PromotePrincipleRequest{
			PrincipleId: strArg(args, "principle_id"), Project: strArg(args, "project"), Domain: strArg(args, "domain"),
			Actor: strArg(args, "actor"), ApprovedBy: strArg(args, "approved_by"), ApprovalReason: strArg(args, "approval_reason"),
		})
		if err != nil {
			return nil, fmt.Errorf("behavioral_promote_principle: %w", err)
		}
		rec := rsp.GetRecord()
		return map[string]interface{}{
			"decision":                rsp.GetDecision().String(),
			"status":                  rsp.GetStatus().String(),
			"verdict":                 rec.GetVerdict(),
			"missing_evidence":        rec.GetMissingEvidence(),
			"unresolved_authority":    rec.GetUnresolvedAuthority(),
			"blocking_contradictions": rec.GetBlockingContradictions(),
			"review_required":         rec.GetReviewRequired(),
			"decision_id":             rec.GetId(),
		}, nil
	})
}

// ── behavioral_revoke_principle (gated) ───────────────────────────────────────

func registerRevokePrincipleTool(s *server) {
	s.register(toolDef{
		Name: "behavioral_revoke_principle",
		Description: "Revoke, supersede, or narrow a promoted principle. Requires actor and reason; " +
			"SUPERSEDED requires superseded_by, NARROWED requires narrowed_scope. The principle is " +
			"never deleted — a revocation record is written. The underlying RPC enforces the " +
			"ai.behavioral.promote (admin) permission.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"project":        {Type: "string", Description: "Project, e.g. 'globular-services'"},
				"domain":         {Type: "string", Description: "Domain, e.g. 'cluster_operator'"},
				"principle_id":   {Type: "string", Description: "Promoted principle id"},
				"action":         {Type: "string", Description: "REVOKED | SUPERSEDED | NARROWED", Enum: []string{"REVOKED", "SUPERSEDED", "NARROWED"}},
				"reason":         {Type: "string", Description: "Revocation reason (required)"},
				"actor":          {Type: "string", Description: "Who is revoking (required, audited)"},
				"superseded_by":  {Type: "string", Description: "Required when action=SUPERSEDED: the replacing principle id"},
				"narrowed_scope": {Type: "string", Description: "Required when action=NARROWED: the narrowed scope/condition"},
			},
			Required: []string{"project", "domain", "principle_id", "action", "reason", "actor"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if strArg(args, "actor") == "" || strArg(args, "reason") == "" {
			return nil, fmt.Errorf("behavioral_revoke_principle: actor and reason are required")
		}
		client, err := behavioralClient(ctx, s)
		if err != nil {
			return nil, err
		}
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()
		rsp, err := client.RevokePrinciple(callCtx, &behavioralpb.RevokePrincipleRequest{
			PrincipleId: strArg(args, "principle_id"), Project: strArg(args, "project"), Domain: strArg(args, "domain"),
			Action: strArg(args, "action"), Reason: strArg(args, "reason"), Actor: strArg(args, "actor"),
			SupersededBy: strArg(args, "superseded_by"), NarrowedScope: strArg(args, "narrowed_scope"),
		})
		if err != nil {
			return nil, fmt.Errorf("behavioral_revoke_principle: %w", err)
		}
		return map[string]interface{}{
			"principle_id": strArg(args, "principle_id"),
			"status":       rsp.GetStatus().String(),
		}, nil
	})
}

// ── small response helpers ────────────────────────────────────────────────────

func entryIDs(es []*behavioralpb.RequiredEvidence) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.GetId()
	}
	return out
}

func forbiddenIDs(es []*behavioralpb.ForbiddenMove) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.GetId()
	}
	return out
}

func authorityIDs(es []*behavioralpb.Authority) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.GetId()
	}
	return out
}
