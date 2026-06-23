package core

// promotion_completion.go closes two authority-surface gaps in the promotion
// gate. The gate (governance.go) requires a principle's applies_when conditions
// to resolve to catalog entries and a contradiction check to have completed —
// but before this file those two requirements were only satisfiable in tests
// (store.PutCondition had no production caller; Principle.ContradictionChecked
// was a test-fixture field). A governed external behavior authority must be
// promotable through its OWN public surface, never via direct store writes or
// test-only magic. These two RPCs are that surface.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
)

// RegisterCondition adds a runtime condition to the domain catalog through a
// governed public path, so the promotion gate's ResolveConditionRefs
// (store.GetCondition) finds it. Replaces the test-only store.PutCondition seam.
func (s *Service) RegisterCondition(ctx context.Context, req *api.RegisterConditionRequest) (*api.RegisterConditionResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	c := req.Condition
	if err := requireScope(c.Project, c.Domain); err != nil {
		return nil, err
	}
	if strings.TrimSpace(c.ID) == "" {
		return nil, fmt.Errorf("condition id is required")
	}
	if c.Status == "" {
		c.Status = api.StatusConditionScoped
	}
	if err := s.store.PutCondition(ctx, &c); err != nil {
		return nil, fmt.Errorf("register condition %q: %w", c.ID, err)
	}
	return &api.RegisterConditionResponse{ConditionID: c.ID, Status: c.Status}, nil
}

// RunContradictionCheck performs the contradiction check for a principle and
// records the result — the governed completion path for the gate's
// contradiction-check requirement.
//
// The check is real, not a flag flip:
//  1. internal consistency — the recommended action must not endorse a move the
//     principle itself forbids;
//  2. it surfaces any contradiction already recorded against the principle.
//
// Conflicts found are recorded and returned as open contradictions; the gate
// blocks on those separately (governance.go step 6). ContradictionChecked is set
// true ONLY after the check completes — never speculatively.
func (s *Service) RunContradictionCheck(ctx context.Context, req *api.RunContradictionCheckRequest) (*api.RunContradictionCheckResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.PrincipleID) == "" {
		return nil, fmt.Errorf("principle_id is required")
	}
	proj, dom := req.Project, string(req.Domain)
	p, err := s.store.GetPrinciple(ctx, proj, dom, req.PrincipleID)
	if err != nil {
		return nil, fmt.Errorf("run contradiction check: principle %q: %w", req.PrincipleID, err)
	}
	now := time.Now().Unix()
	principleRef := api.CanonicalURI(api.KindPrinciple, p.ID)
	var open []string

	// 1. Internal-consistency check (a real result, never a no-op): the
	// recommended action must not endorse a move the principle itself forbids.
	for _, fm := range p.ForbiddenMoves {
		move := strings.TrimSpace(string(fm))
		if move == "" || !strings.Contains(p.RecommendedAction, move) {
			continue
		}
		c := api.Contradiction{
			ID:         newID(),
			Project:    proj,
			Domain:     req.Domain,
			Kind:       "rule_conflict",
			LeftRef:    principleRef,
			RightRef:   principleRef,
			Resolution: "open",
			Note:       fmt.Sprintf("self-contradiction: recommended_action endorses forbidden move %q", move),
			CreatedAt:  now,
		}
		if err := s.store.PutContradiction(ctx, &c); err != nil {
			return nil, fmt.Errorf("run contradiction check: record self-contradiction: %w", err)
		}
		open = append(open, c.ID)
	}

	// 2. Surface contradictions already recorded against this principle.
	existing, err := s.store.ListContradictionsForTarget(ctx, proj, dom, p.ID)
	if err != nil {
		return nil, fmt.Errorf("run contradiction check: list contradictions: %w", err)
	}
	for _, c := range existing {
		if c.Resolution == "" || c.Resolution == "open" {
			open = append(open, c.ID)
		}
	}

	// 3. The check completed — record it through the governed store path.
	if err := s.store.SetPrincipleContradictionChecked(ctx, proj, dom, p.ID, true, now); err != nil {
		return nil, fmt.Errorf("run contradiction check: mark checked: %w", err)
	}
	return &api.RunContradictionCheckResponse{ContradictionChecked: true, OpenContradictionIDs: open}, nil
}
