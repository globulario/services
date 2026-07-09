package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

// discovery.go — governance-legibility operations (P4 discovery + P6 amend).
// Discovery lets an agent find resolvable refs through the API instead of
// grepping seed files; AmendProposal edits a PROPOSED principle in place so a
// missing authority/condition/ref does not force a full re-propose + supersede.

// ListAuthorities returns the authority catalog for a domain.
func (s *Service) ListAuthorities(ctx context.Context, req *api.ListAuthoritiesRequest) (*api.ListAuthoritiesResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	auths, err := s.store.ListAuthorities(ctx, req.Project, string(req.Domain), req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list authorities: %w", err)
	}
	return &api.ListAuthoritiesResponse{Authorities: auths}, nil
}

// ListConditions returns the condition catalog for a domain.
func (s *Service) ListConditions(ctx context.Context, req *api.ListConditionsRequest) (*api.ListConditionsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	conds, err := s.store.ListConditions(ctx, req.Project, string(req.Domain), req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list conditions: %w", err)
	}
	return &api.ListConditionsResponse{Conditions: conds}, nil
}

// ResolveRef reports whether a single canonical ref resolves in a domain and to
// what kind. A genuine store failure is surfaced, never absorbed as "not found".
func (s *Service) ResolveRef(ctx context.Context, req *api.ResolveRefRequest) (*api.ResolveRefResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	if req.Ref == "" {
		return nil, fmt.Errorf("ref is required")
	}
	dom := string(req.Domain)

	if a, err := s.store.GetAuthority(ctx, req.Project, dom, req.Ref); err == nil {
		return &api.ResolveRefResponse{Resolved: true, Kind: "authority", Authority: a}, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("resolve ref: %w", err)
	}

	if c, err := s.store.GetCondition(ctx, req.Project, dom, req.Ref); err == nil {
		return &api.ResolveRefResponse{Resolved: true, Kind: "condition", Condition: c}, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("resolve ref: %w", err)
	}

	return &api.ResolveRefResponse{Resolved: false}, nil
}

// AmendProposal edits a PROPOSED principle in place: set-merge ref lists and set
// scalar gate inputs. A promoted/terminal principle is NEVER amended (a changed
// contract must be a new proposal). Because the content changes, any prior
// contradiction check is invalidated, and the amended refs are re-validated with
// the same syntactic rules as ProposePrinciple (P5).
func (s *Service) AmendProposal(ctx context.Context, req *api.AmendProposalRequest) (*api.AmendProposalResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, fmt.Errorf("id is required")
	}
	p, err := s.store.GetPrinciple(ctx, req.Project, string(req.Domain), req.ID)
	if err != nil {
		return nil, fmt.Errorf("amend proposal: %w", err)
	}
	if p.Status != api.StatusProposedPrinciple {
		return nil, &api.GovernanceError{
			Code:    api.CodeUnsafeOperationRefused,
			Message: fmt.Sprintf("only a PROPOSED principle can be amended; %s is %q — re-propose a new principle for a changed contract", req.ID, p.Status),
		}
	}

	changed := false
	if len(req.AddAuthorityRefs) > 0 || len(req.RemoveAuthorityRefs) > 0 {
		p.Authorities = amendRefs(p.Authorities, req.AddAuthorityRefs, req.RemoveAuthorityRefs)
		changed = true
	}
	if len(req.AddConditionRefs) > 0 || len(req.RemoveConditionRefs) > 0 {
		p.AppliesWhen = amendRefs(p.AppliesWhen, req.AddConditionRefs, req.RemoveConditionRefs)
		changed = true
	}
	if len(req.AddEvidenceRefs) > 0 || len(req.RemoveEvidenceRefs) > 0 {
		p.RequiredEvidence = amendRefs(p.RequiredEvidence, req.AddEvidenceRefs, req.RemoveEvidenceRefs)
		changed = true
	}
	if req.RiskLevel != "" {
		p.RiskLevel = req.RiskLevel
		changed = true
	}
	if req.RevocationRule != "" {
		p.RevocationRule = req.RevocationRule
		changed = true
	}
	if req.PromotionReason != "" {
		p.PromotionReason = req.PromotionReason
		changed = true
	}
	if !changed {
		return nil, fmt.Errorf("amend proposal: no changes specified")
	}

	// Re-validate refs with the same syntactic rules as ProposePrinciple (P5).
	if err := validateProposalRefs(p); err != nil {
		return nil, err
	}

	// A content change invalidates any prior contradiction check — the gate must
	// require a fresh check before this can promote.
	contradictionReset := p.ContradictionChecked
	p.ContradictionChecked = false
	p.Version++
	p.Provenance.UpdatedAt = time.Now().Unix()

	if err := s.store.CreatePrinciple(ctx, p); err != nil {
		return nil, fmt.Errorf("amend proposal: persist: %w", err)
	}
	return &api.AmendProposalResponse{
		PrincipleID:        p.ID,
		Status:             p.Status,
		Version:            p.Version,
		ContradictionReset: contradictionReset,
	}, nil
}

// amendRefs set-merges a typed ref slice: removes any ref in remove, appends any
// ref in add that is not already present, and de-duplicates — order-stable.
func amendRefs[T ~string](cur []T, add, remove []string) []T {
	rm := make(map[string]bool, len(remove))
	for _, r := range remove {
		rm[r] = true
	}
	seen := make(map[string]bool, len(cur)+len(add))
	out := make([]T, 0, len(cur)+len(add))
	for _, c := range cur {
		s := string(c)
		if rm[s] || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, c)
	}
	for _, a := range add {
		if rm[a] || seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, T(a))
	}
	return out
}
