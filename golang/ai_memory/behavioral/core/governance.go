package core

// governance.go implements the PR-3 governance half of the ladder:
// ProposePrinciple → PromotePrinciple (through the promotion gate) →
// RevokePrinciple, plus ExplainPrinciple.
//
// The promotion gate is the heart of the contract: a candidate becomes a
// promoted behavioral principle ONLY when evidence, provenance, authority,
// conditions, a performed contradiction check, no open contradiction, a
// revocation rule, a promotion reason, and a classified risk level are all
// present — and high/irreversible risk additionally requires explicit human
// approval. Every attempt (allowed, blocked, review-required) is recorded as a
// promotion_decisions row: blocked promotion is also memory.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
)

var validRiskLevels = map[string]bool{"info": true, "low": true, "high": true, "irreversible": true}

func isHighRisk(level string) bool { return level == "high" || level == "irreversible" }

// ProposePrinciple creates a candidate principle at PROPOSED_PRINCIPLE. It must
// NOT accept a promoted/terminal status as direct input.
func (s *Service) ProposePrinciple(ctx context.Context, req *api.ProposePrincipleRequest) (*api.ProposePrincipleResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	p := req.Principle
	if err := requireScope(p.Project, p.Domain); err != nil {
		return nil, err
	}
	// Only an unspecified or already-proposed status may be supplied; promotion
	// and revocation statuses are produced ONLY by Promote/Revoke.
	if p.Status != api.StatusUnspecified && p.Status != api.StatusProposedPrinciple {
		return nil, fmt.Errorf("ProposePrinciple may only create PROPOSED_PRINCIPLE, got %q", p.Status)
	}
	if p.ID == "" {
		p.ID = newID()
	}
	p.Status = api.StatusProposedPrinciple
	if p.Version == 0 {
		p.Version = 1
	}
	if p.ProposedBy == "" {
		p.ProposedBy = p.Provenance.AgentID
	}
	now := time.Now().Unix()
	if p.Provenance.CreatedAt == 0 {
		p.Provenance.CreatedAt = now
	}
	p.Provenance.UpdatedAt = now
	if err := s.store.CreatePrinciple(ctx, &p); err != nil {
		return nil, fmt.Errorf("propose principle: %w", err)
	}
	return &api.ProposePrincipleResponse{PrincipleID: p.ID, Status: p.Status}, nil
}

// gateResult is the outcome of evaluating the promotion gate.
type gateResult struct {
	decision               api.PromotionDecision
	reasons                []string
	missingEvidence        []string
	unresolvedAuthority    []string
	unresolvedConditions   []string
	blockingContradictions []string
	reviewRequired         bool
}

// evaluateGate runs all promotion-gate checks against a principle.
func (s *Service) evaluateGate(ctx context.Context, p *api.Principle, approvedBy string) (*gateResult, error) {
	g := &gateResult{}
	proj, dom := p.Project, string(p.Domain)

	// 1. evidence exists (evidence rows targeting the principle).
	ev, err := s.store.ListEvidenceForTarget(ctx, proj, dom, p.ID)
	if err != nil {
		return nil, err
	}
	if len(ev) == 0 {
		g.reasons = append(g.reasons, "no evidence")
		g.missingEvidence = refStrings(p.RequiredEvidence)
		if len(g.missingEvidence) == 0 {
			g.missingEvidence = []string{"<any evidence>"}
		}
	}

	// 2. provenance exists.
	if p.ProposedBy == "" && p.Provenance.AgentID == "" {
		g.reasons = append(g.reasons, "no provenance")
	}

	// 3. authority mapped and resolvable.
	if len(p.Authorities) == 0 {
		g.reasons = append(g.reasons, "no authority mapped")
	} else {
		unresolved, err := s.store.ResolveAuthorityRefs(ctx, proj, dom, refStrings(p.Authorities))
		if err != nil {
			return nil, err
		}
		if len(unresolved) > 0 {
			g.reasons = append(g.reasons, "unresolved authority")
			g.unresolvedAuthority = unresolved
		}
	}

	// 4. conditions explicit and resolvable.
	if len(p.AppliesWhen) == 0 {
		g.reasons = append(g.reasons, "no conditions")
	} else {
		unresolved, err := s.store.ResolveConditionRefs(ctx, proj, dom, refStrings(p.AppliesWhen))
		if err != nil {
			return nil, err
		}
		if len(unresolved) > 0 {
			g.reasons = append(g.reasons, "unresolved conditions")
			g.unresolvedConditions = unresolved
		}
	}

	// 5. contradiction check performed.
	if !p.ContradictionChecked {
		g.reasons = append(g.reasons, "contradiction check not performed")
	}

	// 6. no open contradiction blocks the principle.
	contras, err := s.store.ListContradictionsForTarget(ctx, proj, dom, p.ID)
	if err != nil {
		return nil, err
	}
	for _, c := range contras {
		if c.Resolution == "" || c.Resolution == "open" {
			g.blockingContradictions = append(g.blockingContradictions, c.ID)
		}
	}
	if len(g.blockingContradictions) > 0 {
		g.reasons = append(g.reasons, "open contradiction blocks principle")
	}

	// 7. revocation rule exists.
	if strings.TrimSpace(p.RevocationRule) == "" {
		g.reasons = append(g.reasons, "no revocation rule")
	}

	// 8. promotion reason exists.
	if strings.TrimSpace(p.PromotionReason) == "" {
		g.reasons = append(g.reasons, "no promotion reason")
	}

	// 9. risk level classified.
	if !validRiskLevels[p.RiskLevel] {
		g.reasons = append(g.reasons, "risk level not classified")
	}

	// Decide.
	switch {
	case len(g.reasons) > 0:
		g.decision = api.PromotionBlocked
	case isHighRisk(p.RiskLevel) && approvedBy == "":
		g.decision = api.PromotionReviewRequired
		g.reviewRequired = true
		g.reasons = append(g.reasons, "high-risk principle requires explicit human approval")
	default:
		g.decision = api.PromotionAllowed
	}
	return g, nil
}

// PromotePrinciple runs the promotion gate, records the decision (always), and
// transitions the principle to PROMOTED_PRINCIPLE only when the gate allows.
func (s *Service) PromotePrinciple(ctx context.Context, req *api.PromotePrincipleRequest) (*api.PromotePrincipleResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	if req.PrincipleID == "" {
		return nil, fmt.Errorf("principle_id is required")
	}
	p, err := s.store.GetPrinciple(ctx, req.Project, string(req.Domain), req.PrincipleID)
	if err != nil {
		return nil, fmt.Errorf("promote principle: %w", err)
	}

	approvedBy := req.ApprovedBy
	if approvedBy == "" {
		approvedBy = req.Approver // deprecated alias
	}
	actor := req.Actor
	if actor == "" {
		actor = approvedBy
	}
	if actor == "" {
		actor = "system"
	}

	g, err := s.evaluateGate(ctx, p, approvedBy)
	if err != nil {
		return nil, fmt.Errorf("promote principle: gate: %w", err)
	}

	now := time.Now().Unix()
	rec := api.PromotionDecisionRecord{
		ID:                     newID(),
		Project:                req.Project,
		Domain:                 req.Domain,
		PrincipleID:            p.ID,
		Decision:               g.decision,
		Verdict:                strings.Join(g.reasons, "; "),
		Reason:                 strings.Join(g.reasons, "; "),
		MissingEvidence:        g.missingEvidence,
		UnresolvedAuthority:    g.unresolvedAuthority,
		UnresolvedConditions:   g.unresolvedConditions,
		BlockingContradictions: g.blockingContradictions,
		RiskLevel:              p.RiskLevel,
		ReviewRequired:         g.reviewRequired,
		ApprovedBy:             approvedBy,
		PromotionReason:        p.PromotionReason,
		Actor:                  actor,
		CreatedAt:              now,
	}
	if g.decision == api.PromotionAllowed && len(g.reasons) == 0 {
		rec.Verdict = "all gate checks passed"
	}
	// Every attempt is memory — record before mutating the principle.
	if err := s.store.RecordPromotionDecision(ctx, &rec); err != nil {
		return nil, fmt.Errorf("promote principle: record decision: %w", err)
	}

	if g.decision == api.PromotionAllowed {
		p.Status = api.StatusPromotedPrinciple
		p.PromotionDecisionID = rec.ID
		p.PromotedBy = actor
		if approvedBy != "" {
			p.ApprovedBy = approvedBy
			p.ApprovalReason = req.ApprovalReason
			p.ApprovedAt = now
		}
		p.Provenance.UpdatedAt = now
		if err := s.store.CreatePrinciple(ctx, p); err != nil {
			return nil, fmt.Errorf("promote principle: persist promotion: %w", err)
		}
		// Bridge to runtime behavior: index this promoted principle by its
		// conditions. This is the ONLY path that writes active promoted mappings.
		if err := s.store.IndexPromotedPrinciple(ctx, p); err != nil {
			return nil, fmt.Errorf("promote principle: index by condition: %w", err)
		}
	}

	return &api.PromotePrincipleResponse{Decision: g.decision, Status: p.Status, Record: rec}, nil
}

func actionToStatus(action string) (api.GovernanceStatus, error) {
	switch strings.ToUpper(strings.TrimSpace(action)) {
	case "REVOKED", "REVOKE":
		return api.StatusRevoked, nil
	case "SUPERSEDED", "SUPERSEDE":
		return api.StatusSuperseded, nil
	case "NARROWED", "NARROW":
		return api.StatusNarrowed, nil
	default:
		return api.StatusUnspecified, fmt.Errorf("unknown revocation action %q (want REVOKED|SUPERSEDED|NARROWED)", action)
	}
}

// RevokePrinciple records a revocation rule and updates the principle status —
// it never deletes the principle.
func (s *Service) RevokePrinciple(ctx context.Context, req *api.RevokePrincipleRequest) (*api.RevokePrincipleResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	if req.PrincipleID == "" {
		return nil, fmt.Errorf("principle_id is required")
	}
	newStatus, err := actionToStatus(req.Action)
	if err != nil {
		return nil, err
	}
	if newStatus == api.StatusSuperseded && strings.TrimSpace(req.SupersededBy) == "" {
		return nil, fmt.Errorf("SUPERSEDED requires superseded_by")
	}
	if newStatus == api.StatusNarrowed && strings.TrimSpace(req.NarrowedScope) == "" {
		return nil, fmt.Errorf("NARROWED requires narrowed_scope")
	}
	p, err := s.store.GetPrinciple(ctx, req.Project, string(req.Domain), req.PrincipleID)
	if err != nil {
		return nil, fmt.Errorf("revoke principle: %w", err)
	}

	actor := req.Actor
	if actor == "" {
		actor = "system"
	}
	now := time.Now().Unix()
	rule := api.RevocationRule{
		ID:               newID(),
		Project:          req.Project,
		Domain:           req.Domain,
		PrincipleID:      p.ID,
		Action:           string(newStatus),
		RevocationReason: req.Reason,
		Note:             req.Reason,
		Actor:            actor,
		SupersededBy:     req.SupersededBy,
		NarrowedScope:    req.NarrowedScope,
		Condition:        req.NarrowedScope,
		CreatedAt:        now,
	}
	if err := s.store.RecordRevocationRule(ctx, &rule); err != nil {
		return nil, fmt.Errorf("revoke principle: record rule: %w", err)
	}

	// Update the principle in place (never delete) with first-class links.
	p.Status = newStatus
	p.RevocationRuleID = rule.ID
	switch newStatus {
	case api.StatusSuperseded:
		p.SupersededBy = req.SupersededBy
	case api.StatusNarrowed:
		p.NarrowedBy = rule.ID
	}
	p.Provenance.UpdatedAt = now
	if err := s.store.CreatePrinciple(ctx, p); err != nil {
		return nil, fmt.Errorf("revoke principle: persist status: %w", err)
	}
	// Remove the now-inactive principle from the runtime condition index so
	// CheckAction / ResolveGovernedContext stop returning it. Only promoted
	// principles remain in the active lookup.
	if err := s.store.DeindexPromotedPrinciple(ctx, p); err != nil {
		return nil, fmt.Errorf("revoke principle: deindex by condition: %w", err)
	}
	return &api.RevokePrincipleResponse{Status: newStatus}, nil
}

// ExplainPrinciple returns a readable governance explanation. It never performs
// runtime action checking and never calls cluster probes.
func (s *Service) ExplainPrinciple(ctx context.Context, req *api.ExplainPrincipleRequest) (*api.ExplainPrincipleResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	if req.PrincipleID == "" {
		return nil, fmt.Errorf("principle_id is required")
	}
	proj, dom := req.Project, string(req.Domain)
	p, err := s.store.GetPrinciple(ctx, proj, dom, req.PrincipleID)
	if err != nil {
		return nil, fmt.Errorf("explain principle: %w", err)
	}
	resp := &api.ExplainPrincipleResponse{Principle: *p}

	if ev, err := s.store.ListEvidenceForTarget(ctx, proj, dom, p.ID); err == nil {
		resp.Evidence = ev
	}
	for _, a := range p.Authorities {
		if auth, err := s.store.GetAuthority(ctx, proj, dom, string(a)); err == nil {
			resp.Authorities = append(resp.Authorities, *auth)
		}
	}
	for _, c := range p.AppliesWhen {
		if cond, err := s.store.GetCondition(ctx, proj, dom, string(c)); err == nil {
			resp.Conditions = append(resp.Conditions, *cond)
		}
	}
	if contras, err := s.store.ListContradictionsForTarget(ctx, proj, dom, p.ID); err == nil {
		resp.Contradictions = contras
	}
	if p.PromotionDecisionID != "" {
		if d, err := s.store.GetPromotionDecision(ctx, proj, dom, p.PromotionDecisionID); err == nil {
			resp.PromotionHistory = append(resp.PromotionHistory, *d)
		}
	}
	if p.RevocationRuleID != "" {
		if r, err := s.store.GetRevocationRule(ctx, proj, dom, p.RevocationRuleID); err == nil {
			resp.RevocationRules = append(resp.RevocationRules, *r)
		}
	}
	resp.Explanation = explainText(p, resp)
	return resp, nil
}

func explainText(p *api.Principle, r *api.ExplainPrincipleResponse) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Principle %s (%q) is %s, risk=%s.\n", p.ID, p.Title, p.Status, p.RiskLevel)
	fmt.Fprintf(&b, "Applies when: %s\n", strings.Join(refStrings(p.AppliesWhen), ", "))
	fmt.Fprintf(&b, "Requires evidence: %s\n", strings.Join(refStrings(p.RequiredEvidence), ", "))
	fmt.Fprintf(&b, "Governed by authorities: %s\n", strings.Join(refStrings(p.Authorities), ", "))
	fmt.Fprintf(&b, "Forbids moves: %s\n", strings.Join(refStrings(p.ForbiddenMoves), ", "))
	fmt.Fprintf(&b, "Promotion reason: %s\n", p.PromotionReason)
	if len(r.PromotionHistory) > 0 {
		last := r.PromotionHistory[len(r.PromotionHistory)-1]
		fmt.Fprintf(&b, "Latest promotion decision: %s (%s)\n", last.Decision, last.Verdict)
	}
	if p.RevocationRule != "" {
		fmt.Fprintf(&b, "Revocation rule (when): %s\n", p.RevocationRule)
	}
	if len(r.RevocationRules) > 0 {
		fmt.Fprintf(&b, "Revocation applied: %s\n", r.RevocationRules[len(r.RevocationRules)-1].Action)
	}
	var open int
	for _, c := range r.Contradictions {
		if c.Resolution == "" || c.Resolution == "open" {
			open++
		}
	}
	fmt.Fprintf(&b, "Open contradictions: %d\n", open)
	if p.ApprovedBy != "" {
		fmt.Fprintf(&b, "Human approval: %s (%s)\n", p.ApprovedBy, p.ApprovalReason)
	} else {
		b.WriteString("Human approval: none\n")
	}
	return b.String()
}

// refStrings converts any ~string ref slice to []string.
func refStrings[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}
