package core

// ingestion.go implements the PR-2 "ingestion half" of the governance ladder:
// RecordSignal → ExtractClaim → RecordEvidence → MapAuthority →
// RecordContradiction. Each advances a claim along the allowed PR-2 rungs:
//
//	RAW_SIGNAL → EXTRACTED_CLAIM → EVIDENCE_LINKED → AUTHORITY_MAPPED → CONTRADICTION_TESTED
//
// The kernel sets ids, timestamps, and ladder statuses; persistence is delegated
// to the store port. No cluster-specific logic and no driver imports live here.

import (
	"context"
	"fmt"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
)

func requireScope(project string, domain api.DomainRef) error {
	if project == "" {
		return fmt.Errorf("project is required")
	}
	if domain == "" {
		return fmt.Errorf("domain is required")
	}
	return nil
}

func entityKindFromString(targetKind string) api.EntityKind {
	switch targetKind {
	case "claim":
		return api.KindClaim
	case "principle":
		return api.KindPrinciple
	default:
		return api.EntityKind(targetKind)
	}
}

// RecordSignal persists a typed raw operational signal at status RAW_SIGNAL.
func (s *Service) RecordSignal(ctx context.Context, req *api.RecordSignalRequest) (*api.RecordSignalResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	sig := req.Signal
	if err := requireScope(sig.Project, sig.Domain); err != nil {
		return nil, err
	}
	if !statusAllowedInPR2(sig.Status) {
		return nil, fmt.Errorf("status %q is not permitted in the ingestion half (PR-2)", sig.Status)
	}
	if sig.ID == "" {
		sig.ID = newID()
	}
	if sig.Status == api.StatusUnspecified {
		sig.Status = api.StatusRawSignal
	}
	now := time.Now().Unix()
	if sig.Provenance.CreatedAt == 0 {
		sig.Provenance.CreatedAt = now
	}
	sig.Provenance.UpdatedAt = now
	if err := s.store.PutSignal(ctx, &sig); err != nil {
		return nil, fmt.Errorf("record signal: %w", err)
	}
	return &api.RecordSignalResponse{SignalID: sig.ID, Status: sig.Status}, nil
}

// ExtractClaim creates one or more claims linked to an existing signal, each at
// status EXTRACTED_CLAIM.
func (s *Service) ExtractClaim(ctx context.Context, req *api.ExtractClaimRequest) (*api.ExtractClaimResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	if req.SignalID == "" {
		return nil, fmt.Errorf("signal_id is required")
	}
	if len(req.Claims) == 0 {
		return nil, fmt.Errorf("at least one claim is required")
	}
	// The claim must link to a real signal — fail loud if the signal is absent.
	if _, err := s.store.GetSignal(ctx, req.Project, string(req.Domain), req.SignalID); err != nil {
		return nil, fmt.Errorf("extract claim: signal %q: %w", req.SignalID, err)
	}
	now := time.Now().Unix()
	ids := make([]string, 0, len(req.Claims))
	for i := range req.Claims {
		c := req.Claims[i]
		c.Project = req.Project
		c.Domain = req.Domain
		c.SignalID = req.SignalID
		if !statusAllowedInPR2(c.Status) {
			return nil, fmt.Errorf("status %q is not permitted in the ingestion half (PR-2)", c.Status)
		}
		if c.ID == "" {
			c.ID = newID()
		}
		if c.Status == api.StatusUnspecified {
			c.Status = api.StatusExtractedClaim
		}
		if c.Provenance.CreatedAt == 0 {
			c.Provenance.CreatedAt = now
		}
		c.Provenance.UpdatedAt = now
		if err := s.store.PutClaim(ctx, &c); err != nil {
			return nil, fmt.Errorf("extract claim: persist: %w", err)
		}
		ids = append(ids, c.ID)
	}
	return &api.ExtractClaimResponse{ClaimIDs: ids}, nil
}

// RecordEvidence stores evidence for a target (claim/principle), maintains the
// evidence_by_target lookup, and advances a claim target to EVIDENCE_LINKED.
func (s *Service) RecordEvidence(ctx context.Context, req *api.RecordEvidenceRequest) (*api.RecordEvidenceResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	e := req.Evidence
	if err := requireScope(e.Project, e.Domain); err != nil {
		return nil, err
	}
	if e.TargetKind == "" || e.TargetID == "" {
		return nil, fmt.Errorf("evidence target_kind and target_id are required")
	}
	// Claim targets must exist (so we never index evidence against a phantom row
	// or silently upsert a partial claim via the status update).
	if e.TargetKind == "claim" {
		if _, err := s.store.GetClaim(ctx, e.Project, string(e.Domain), e.TargetID); err != nil {
			return nil, fmt.Errorf("record evidence: target claim %q: %w", e.TargetID, err)
		}
	}
	if e.ID == "" {
		e.ID = newID()
	}
	now := time.Now().Unix()
	if e.Provenance.CreatedAt == 0 {
		e.Provenance.CreatedAt = now
	}
	e.Provenance.UpdatedAt = now
	if err := s.store.PutEvidence(ctx, &e); err != nil {
		return nil, fmt.Errorf("record evidence: %w", err)
	}
	if e.TargetKind == "claim" {
		if err := s.store.UpdateClaimStatus(ctx, e.Project, string(e.Domain), e.TargetID, api.StatusEvidenceLinked, now); err != nil {
			return nil, fmt.Errorf("record evidence: advance claim status: %w", err)
		}
	}
	return &api.RecordEvidenceResponse{EvidenceID: e.ID}, nil
}

// MapAuthority records the governing authorities for a target and advances a
// claim target to AUTHORITY_MAPPED.
func (s *Service) MapAuthority(ctx context.Context, req *api.MapAuthorityRequest) (*api.MapAuthorityResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := requireScope(req.Project, req.Domain); err != nil {
		return nil, err
	}
	if req.TargetKind == "" || req.TargetID == "" {
		return nil, fmt.Errorf("target_kind and target_id are required")
	}
	if len(req.Authorities) == 0 {
		return nil, fmt.Errorf("at least one authority is required")
	}
	if req.TargetKind == "claim" {
		if _, err := s.store.GetClaim(ctx, req.Project, string(req.Domain), req.TargetID); err != nil {
			return nil, fmt.Errorf("map authority: target claim %q: %w", req.TargetID, err)
		}
	}
	now := time.Now().Unix()
	targetRef := api.CanonicalURI(entityKindFromString(req.TargetKind), req.TargetID)
	for _, a := range req.Authorities {
		if a == "" {
			continue
		}
		if err := s.store.AddAuthorityGoverns(ctx, req.Project, string(req.Domain), string(a), targetRef, now); err != nil {
			return nil, fmt.Errorf("map authority %q: %w", a, err)
		}
	}
	if req.TargetKind == "claim" {
		if err := s.store.UpdateClaimStatus(ctx, req.Project, string(req.Domain), req.TargetID, api.StatusAuthorityMapped, now); err != nil {
			return nil, fmt.Errorf("map authority: advance claim status: %w", err)
		}
	}
	return &api.MapAuthorityResponse{Status: api.StatusAuthorityMapped}, nil
}

// RecordContradiction persists a contradiction and, for claim-vs-claim conflicts,
// advances the referenced claims to CONTRADICTION_TESTED.
func (s *Service) RecordContradiction(ctx context.Context, req *api.RecordContradictionRequest) (*api.RecordContradictionResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	c := req.Contradiction
	if err := requireScope(c.Project, c.Domain); err != nil {
		return nil, err
	}
	if c.LeftRef == "" || c.RightRef == "" {
		return nil, fmt.Errorf("contradiction left_ref and right_ref are required")
	}
	if c.ID == "" {
		c.ID = newID()
	}
	if c.Resolution == "" {
		c.Resolution = "open"
	}
	if c.CreatedAt == 0 {
		c.CreatedAt = time.Now().Unix()
	}
	if err := s.store.PutContradiction(ctx, &c); err != nil {
		return nil, fmt.Errorf("record contradiction: %w", err)
	}
	if c.Kind == "claim_vs_claim" {
		for _, ref := range []string{c.LeftRef, c.RightRef} {
			if _, err := s.store.GetClaim(ctx, c.Project, string(c.Domain), ref); err != nil {
				return nil, fmt.Errorf("record contradiction: claim %q: %w", ref, err)
			}
			if err := s.store.UpdateClaimStatus(ctx, c.Project, string(c.Domain), ref, api.StatusContradictionTested, c.CreatedAt); err != nil {
				return nil, fmt.Errorf("record contradiction: advance claim %q status: %w", ref, err)
			}
		}
	}
	return &api.RecordContradictionResponse{ContradictionID: c.ID}, nil
}
