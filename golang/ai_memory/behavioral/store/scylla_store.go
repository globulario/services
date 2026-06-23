package store

// scylla_store.go is the ScyllaDB adapter — the ONLY file in the behavioral
// kernel permitted to import gocql / speak CQL (the kernel-hygiene test enforces
// this). It implements the Store port against the behavioral_memory keyspace.
//
// Tables are FULLY QUALIFIED (behavioral_memory.<table>) because the shared
// ai-memory gocql session is opened with keyspace=ai_memory. Every read/write
// addresses a single entity by ((project, domain, id)) — no ALLOW FILTERING.

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/gocql/gocql"
)

// ScyllaStore is the production persistence adapter.
type ScyllaStore struct {
	session *gocql.Session
}

var _ Store = (*ScyllaStore)(nil)

// NewScyllaStore builds the adapter over an existing gocql session (shared with
// the ai-memory service).
func NewScyllaStore(session *gocql.Session) *ScyllaStore {
	return &ScyllaStore{session: session}
}

func (*ScyllaStore) Backend() string { return "scylla" }

func mapNotFound(err error) error {
	if errors.Is(err, gocql.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

func refsToStrings[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}

// ── Signals ───────────────────────────────────────────────────────────────────

func (s *ScyllaStore) PutSignal(ctx context.Context, sig *api.Signal) error {
	const q = `INSERT INTO behavioral_memory.signals
(project, domain, id, kind, source_kind, source_ref, entity_ref, scope, observed_at, payload, confidence, status, agent_id, memory_id, created_at, updated_at, metadata, cluster_id, condition_ref, severity, authority_level)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if err := s.session.Query(q,
		sig.Project, string(sig.Domain), sig.ID, string(sig.Kind), sig.SourceKind, sig.SourceRef,
		sig.EntityRef, sig.Scope, sig.ObservedAt, sig.Payload, sig.Confidence, string(sig.Status),
		sig.Provenance.AgentID, sig.Provenance.MemoryID, sig.Provenance.CreatedAt, sig.Provenance.UpdatedAt, sig.Metadata,
		sig.ClusterID, sig.ConditionRef, sig.Severity, string(sig.AuthorityLevel),
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("put signal: %w", err)
	}
	return nil
}

func (s *ScyllaStore) GetSignal(ctx context.Context, project, domain, id string) (*api.Signal, error) {
	const q = `SELECT kind, source_kind, source_ref, entity_ref, scope, observed_at, payload, confidence, status, agent_id, memory_id, created_at, updated_at, metadata, cluster_id, condition_ref, severity, authority_level
FROM behavioral_memory.signals WHERE project = ? AND domain = ? AND id = ?`
	sig := &api.Signal{ID: id, Project: project, Domain: api.DomainRef(domain)}
	var kind, status, authorityLevel string
	if err := s.session.Query(q, project, domain, id).WithContext(ctx).Scan(
		&kind, &sig.SourceKind, &sig.SourceRef, &sig.EntityRef, &sig.Scope, &sig.ObservedAt, &sig.Payload,
		&sig.Confidence, &status, &sig.Provenance.AgentID, &sig.Provenance.MemoryID,
		&sig.Provenance.CreatedAt, &sig.Provenance.UpdatedAt, &sig.Metadata,
		&sig.ClusterID, &sig.ConditionRef, &sig.Severity, &authorityLevel,
	); err != nil {
		return nil, mapNotFound(err)
	}
	sig.Kind = api.SignalKind(kind)
	sig.Status = api.GovernanceStatus(status)
	sig.AuthorityLevel = api.ObservationAuthorityLevel(authorityLevel)
	return sig, nil
}

// ── Claims ──────────────────────────────────────────────────────────────────

func (s *ScyllaStore) PutClaim(ctx context.Context, c *api.Claim) error {
	const q = `INSERT INTO behavioral_memory.claims
(project, domain, id, signal_id, statement, subject_entity, predicate, object_value, time_ref, status, confidence, source_id, agent_id, memory_id, created_at, updated_at, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if err := s.session.Query(q,
		c.Project, string(c.Domain), c.ID, c.SignalID, c.Statement, c.SubjectEntity, c.Predicate, c.ObjectValue,
		c.TimeRef, string(c.Status), c.Confidence, c.SourceID, c.Provenance.AgentID, c.Provenance.MemoryID,
		c.Provenance.CreatedAt, c.Provenance.UpdatedAt, c.Metadata,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("put claim: %w", err)
	}
	return nil
}

func (s *ScyllaStore) GetClaim(ctx context.Context, project, domain, id string) (*api.Claim, error) {
	const q = `SELECT signal_id, statement, subject_entity, predicate, object_value, time_ref, status, confidence, source_id, agent_id, memory_id, created_at, updated_at, metadata
FROM behavioral_memory.claims WHERE project = ? AND domain = ? AND id = ?`
	c := &api.Claim{ID: id, Project: project, Domain: api.DomainRef(domain)}
	var status string
	if err := s.session.Query(q, project, domain, id).WithContext(ctx).Scan(
		&c.SignalID, &c.Statement, &c.SubjectEntity, &c.Predicate, &c.ObjectValue, &c.TimeRef, &status,
		&c.Confidence, &c.SourceID, &c.Provenance.AgentID, &c.Provenance.MemoryID,
		&c.Provenance.CreatedAt, &c.Provenance.UpdatedAt, &c.Metadata,
	); err != nil {
		return nil, mapNotFound(err)
	}
	c.Status = api.GovernanceStatus(status)
	return c, nil
}

func (s *ScyllaStore) UpdateClaimStatus(ctx context.Context, project, domain, id string, status api.GovernanceStatus, updatedAt int64) error {
	const q = `UPDATE behavioral_memory.claims SET status = ?, updated_at = ? WHERE project = ? AND domain = ? AND id = ?`
	if err := s.session.Query(q, string(status), updatedAt, project, domain, id).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("update claim status: %w", err)
	}
	return nil
}

// ── Evidence ──────────────────────────────────────────────────────────────────

func (s *ScyllaStore) PutEvidence(ctx context.Context, e *api.Evidence) error {
	// A logged batch keeps evidence and its evidence_by_target index consistent.
	batch := s.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.Query(`INSERT INTO behavioral_memory.evidence
(project, domain, id, target_kind, target_id, evidence_kind, lane, result, probe_ref, observed_at, payload, provenance, observed_from, satisfies, created_at, updated_at, metadata, source_kind, source_ref, entity_ref, cluster_id, condition_ref, severity, authority_level)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Project, string(e.Domain), e.ID, e.TargetKind, e.TargetID, e.Kind, string(e.Lane), e.Result, e.ProbeRef,
		e.ObservedAt, e.Payload, e.Provenance.SourceRef, e.ObservedFrom, refsToStrings(e.Satisfies),
		e.Provenance.CreatedAt, e.Provenance.UpdatedAt, e.Metadata,
		e.SourceKind, e.SourceRef, e.EntityRef, e.ClusterID, e.ConditionRef, e.Severity, string(e.AuthorityLevel),
	)
	if e.TargetID != "" {
		batch.Query(`INSERT INTO behavioral_memory.evidence_by_target
(project, domain, target_id, id, target_kind, evidence_kind, lane, result, observed_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.Project, string(e.Domain), e.TargetID, e.ID, e.TargetKind, e.Kind, string(e.Lane), e.Result,
			e.ObservedAt, e.Provenance.CreatedAt,
		)
	}
	if err := s.session.ExecuteBatch(batch); err != nil {
		return fmt.Errorf("put evidence: %w", err)
	}
	return nil
}

func (s *ScyllaStore) ListEvidenceForTarget(ctx context.Context, project, domain, targetID string) ([]api.Evidence, error) {
	const q = `SELECT id, target_kind, evidence_kind, lane, result, observed_at, created_at
FROM behavioral_memory.evidence_by_target WHERE project = ? AND domain = ? AND target_id = ?`
	iter := s.session.Query(q, project, domain, targetID).WithContext(ctx).Iter()
	var out []api.Evidence
	var id, targetKind, kind, lane, result string
	var observedAt, createdAt int64
	for iter.Scan(&id, &targetKind, &kind, &lane, &result, &observedAt, &createdAt) {
		out = append(out, api.Evidence{
			ID: id, Project: project, Domain: api.DomainRef(domain), TargetKind: targetKind, TargetID: targetID,
			Kind: kind, Lane: api.EvidenceLane(lane), Result: result, ObservedAt: observedAt,
			Provenance: api.Provenance{CreatedAt: createdAt},
		})
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("list evidence by target: %w", err)
	}
	return out, nil
}

// ── Authorities ───────────────────────────────────────────────────────────────

func (s *ScyllaStore) PutAuthority(ctx context.Context, a *api.Authority) error {
	const q = `INSERT INTO behavioral_memory.authorities
(project, domain, id, title, governs, owner_kind, read_path, write_path, identity_source, governs_refs, status, created_at, updated_at, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if err := s.session.Query(q,
		a.Project, string(a.Domain), a.ID, a.Title, a.Governs, a.OwnerKind, a.ReadPath, a.WritePath,
		a.IdentitySource, a.GovernsRefs, string(a.Status), int64(0), int64(0), a.Metadata,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("put authority: %w", err)
	}
	return nil
}

func (s *ScyllaStore) GetAuthority(ctx context.Context, project, domain, id string) (*api.Authority, error) {
	const q = `SELECT title, governs, owner_kind, read_path, write_path, identity_source, governs_refs, status, metadata
FROM behavioral_memory.authorities WHERE project = ? AND domain = ? AND id = ?`
	a := &api.Authority{ID: id, Project: project, Domain: api.DomainRef(domain)}
	var status string
	if err := s.session.Query(q, project, domain, id).WithContext(ctx).Scan(
		&a.Title, &a.Governs, &a.OwnerKind, &a.ReadPath, &a.WritePath, &a.IdentitySource,
		&a.GovernsRefs, &status, &a.Metadata,
	); err != nil {
		return nil, mapNotFound(err)
	}
	a.Status = api.GovernanceStatus(status)
	return a, nil
}

func (s *ScyllaStore) AddAuthorityGoverns(ctx context.Context, project, domain, authorityID, targetRef string, updatedAt int64) error {
	// UPDATE upserts: creates the authority row if absent, set-adds the target.
	const q = `UPDATE behavioral_memory.authorities SET governs_refs = governs_refs + ?, status = ?, updated_at = ?
WHERE project = ? AND domain = ? AND id = ?`
	if err := s.session.Query(q,
		[]string{targetRef}, string(api.StatusAuthorityMapped), updatedAt, project, domain, authorityID,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("add authority governs: %w", err)
	}
	return nil
}

// ── Conditions ────────────────────────────────────────────────────────────────

func (s *ScyllaStore) PutCondition(ctx context.Context, c *api.Condition) error {
	const q = `INSERT INTO behavioral_memory.conditions
(project, domain, id, title, detect_spec, severity, status, created_at, updated_at, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if err := s.session.Query(q,
		c.Project, string(c.Domain), c.ID, c.Title, c.DetectSpec, c.Severity, string(c.Status), int64(0), int64(0), c.Metadata,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("put condition: %w", err)
	}
	return nil
}

func (s *ScyllaStore) GetCondition(ctx context.Context, project, domain, id string) (*api.Condition, error) {
	const q = `SELECT title, detect_spec, severity, status, metadata
FROM behavioral_memory.conditions WHERE project = ? AND domain = ? AND id = ?`
	c := &api.Condition{ID: id, Project: project, Domain: api.DomainRef(domain)}
	var status string
	if err := s.session.Query(q, project, domain, id).WithContext(ctx).Scan(
		&c.Title, &c.DetectSpec, &c.Severity, &status, &c.Metadata,
	); err != nil {
		return nil, mapNotFound(err)
	}
	c.Status = api.GovernanceStatus(status)
	return c, nil
}

// ── Contradictions ────────────────────────────────────────────────────────────

func (s *ScyllaStore) PutContradiction(ctx context.Context, c *api.Contradiction) error {
	// Batch the contradiction with its target-index rows so a contradiction is
	// always discoverable by either referenced entity (no ALLOW FILTERING).
	batch := s.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.Query(`INSERT INTO behavioral_memory.contradictions
(project, domain, id, kind, left_ref, right_ref, resolution, note, created_at, updated_at, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Project, string(c.Domain), c.ID, c.Kind, c.LeftRef, c.RightRef, c.Resolution, c.Note, c.CreatedAt, c.CreatedAt, c.Metadata,
	)
	for _, ref := range []string{c.LeftRef, c.RightRef} {
		if ref == "" {
			continue
		}
		batch.Query(`INSERT INTO behavioral_memory.contradictions_by_target
(project, domain, target_id, id, kind, resolution, left_ref, right_ref, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			c.Project, string(c.Domain), ref, c.ID, c.Kind, c.Resolution, c.LeftRef, c.RightRef, c.CreatedAt,
		)
	}
	if err := s.session.ExecuteBatch(batch); err != nil {
		return fmt.Errorf("put contradiction: %w", err)
	}
	return nil
}

func (s *ScyllaStore) GetContradiction(ctx context.Context, project, domain, id string) (*api.Contradiction, error) {
	const q = `SELECT kind, left_ref, right_ref, resolution, note, created_at, metadata
FROM behavioral_memory.contradictions WHERE project = ? AND domain = ? AND id = ?`
	c := &api.Contradiction{ID: id, Project: project, Domain: api.DomainRef(domain)}
	if err := s.session.Query(q, project, domain, id).WithContext(ctx).Scan(
		&c.Kind, &c.LeftRef, &c.RightRef, &c.Resolution, &c.Note, &c.CreatedAt, &c.Metadata,
	); err != nil {
		return nil, mapNotFound(err)
	}
	return c, nil
}

func (s *ScyllaStore) ListContradictionsForTarget(ctx context.Context, project, domain, targetID string) ([]api.Contradiction, error) {
	const q = `SELECT id, kind, resolution, left_ref, right_ref, created_at
FROM behavioral_memory.contradictions_by_target WHERE project = ? AND domain = ? AND target_id = ?`
	iter := s.session.Query(q, project, domain, targetID).WithContext(ctx).Iter()
	var out []api.Contradiction
	var id, kind, resolution, left, right string
	var createdAt int64
	for iter.Scan(&id, &kind, &resolution, &left, &right, &createdAt) {
		out = append(out, api.Contradiction{
			ID: id, Project: project, Domain: api.DomainRef(domain), Kind: kind,
			Resolution: resolution, LeftRef: left, RightRef: right, CreatedAt: createdAt,
		})
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("list contradictions for target: %w", err)
	}
	return out, nil
}

// ── ref resolution (promotion gate) ───────────────────────────────────────────

func (s *ScyllaStore) ResolveAuthorityRefs(ctx context.Context, project, domain string, refs []string) ([]string, error) {
	var unresolved []string
	for _, r := range refs {
		if _, err := s.GetAuthority(ctx, project, domain, r); err != nil {
			if errors.Is(err, ErrNotFound) {
				unresolved = append(unresolved, r)
				continue
			}
			return nil, err
		}
	}
	return unresolved, nil
}

func (s *ScyllaStore) ResolveConditionRefs(ctx context.Context, project, domain string, refs []string) ([]string, error) {
	var unresolved []string
	for _, r := range refs {
		if _, err := s.GetCondition(ctx, project, domain, r); err != nil {
			if errors.Is(err, ErrNotFound) {
				unresolved = append(unresolved, r)
				continue
			}
			return nil, err
		}
	}
	return unresolved, nil
}

// ── PR-3 governance ───────────────────────────────────────────────────────────

func (s *ScyllaStore) CreatePrinciple(ctx context.Context, p *api.Principle) error {
	const q = `INSERT INTO behavioral_memory.principles
(project, domain, id, title, applies_when, authorities, required_evidence, forbidden_moves, recommended_action,
 risk_level, revocation_rule, promotion_reason, status, superseded_by, narrowed_by, version, proposed_by, promoted_by,
 promotion_decision_id, revocation_rule_id, contradiction_checked, approved_by, approval_reason, approved_at,
 source_refs, generated_from, agent_id, memory_id, created_at, updated_at, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if err := s.session.Query(q,
		p.Project, string(p.Domain), p.ID, p.Title,
		refsToStrings(p.AppliesWhen), refsToStrings(p.Authorities), refsToStrings(p.RequiredEvidence), refsToStrings(p.ForbiddenMoves),
		p.RecommendedAction, p.RiskLevel, p.RevocationRule, p.PromotionReason, string(p.Status), p.SupersededBy, p.NarrowedBy,
		p.Version, p.ProposedBy, p.PromotedBy, p.PromotionDecisionID, p.RevocationRuleID, p.ContradictionChecked,
		p.ApprovedBy, p.ApprovalReason, p.ApprovedAt, p.SourceRefs, p.GeneratedFrom, p.Provenance.AgentID, p.Provenance.MemoryID,
		p.Provenance.CreatedAt, p.Provenance.UpdatedAt, p.Metadata,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("create principle: %w", err)
	}
	return nil
}

func (s *ScyllaStore) GetPrinciple(ctx context.Context, project, domain, id string) (*api.Principle, error) {
	const q = `SELECT title, applies_when, authorities, required_evidence, forbidden_moves, recommended_action,
 risk_level, revocation_rule, promotion_reason, status, superseded_by, narrowed_by, version, proposed_by, promoted_by,
 promotion_decision_id, revocation_rule_id, contradiction_checked, approved_by, approval_reason, approved_at,
 source_refs, generated_from, agent_id, memory_id, created_at, updated_at, metadata
FROM behavioral_memory.principles WHERE project = ? AND domain = ? AND id = ?`
	p := &api.Principle{ID: id, Project: project, Domain: api.DomainRef(domain)}
	var status string
	var appliesWhen, authorities, requiredEvidence, forbiddenMoves []string
	if err := s.session.Query(q, project, domain, id).WithContext(ctx).Scan(
		&p.Title, &appliesWhen, &authorities, &requiredEvidence, &forbiddenMoves, &p.RecommendedAction,
		&p.RiskLevel, &p.RevocationRule, &p.PromotionReason, &status, &p.SupersededBy, &p.NarrowedBy,
		&p.Version, &p.ProposedBy, &p.PromotedBy, &p.PromotionDecisionID, &p.RevocationRuleID, &p.ContradictionChecked,
		&p.ApprovedBy, &p.ApprovalReason, &p.ApprovedAt, &p.SourceRefs, &p.GeneratedFrom, &p.Provenance.AgentID, &p.Provenance.MemoryID,
		&p.Provenance.CreatedAt, &p.Provenance.UpdatedAt, &p.Metadata,
	); err != nil {
		return nil, mapNotFound(err)
	}
	p.Status = api.GovernanceStatus(status)
	p.AppliesWhen = toConditionRefs(appliesWhen)
	p.Authorities = toAuthorityRefsStore(authorities)
	p.RequiredEvidence = toRequiredEvidenceRefsStore(requiredEvidence)
	p.ForbiddenMoves = toForbiddenMoveRefs(forbiddenMoves)
	return p, nil
}

func (s *ScyllaStore) UpdatePrincipleStatus(ctx context.Context, project, domain, id string, status api.GovernanceStatus, updatedAt int64) error {
	const q = `UPDATE behavioral_memory.principles SET status = ?, updated_at = ? WHERE project = ? AND domain = ? AND id = ?`
	if err := s.session.Query(q, string(status), updatedAt, project, domain, id).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("update principle status: %w", err)
	}
	return nil
}

func (s *ScyllaStore) SetPrincipleContradictionChecked(ctx context.Context, project, domain, id string, checked bool, updatedAt int64) error {
	const q = `UPDATE behavioral_memory.principles SET contradiction_checked = ?, updated_at = ? WHERE project = ? AND domain = ? AND id = ?`
	if err := s.session.Query(q, checked, updatedAt, project, domain, id).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("set principle contradiction_checked: %w", err)
	}
	return nil
}

func (s *ScyllaStore) RecordPromotionDecision(ctx context.Context, d *api.PromotionDecisionRecord) error {
	const q = `INSERT INTO behavioral_memory.promotion_decisions
(project, domain, id, principle_id, decision, verdict, missing_evidence, unresolved_authority, unresolved_conditions,
 blocking_contradictions, blocked_by_forbidden, risk_level, review_required, approved_by, reviewer, promotion_reason,
 reason, actor, created_at, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if err := s.session.Query(q,
		d.Project, string(d.Domain), d.ID, d.PrincipleID, string(d.Decision), d.Verdict,
		d.MissingEvidence, d.UnresolvedAuthority, d.UnresolvedConditions, d.BlockingContradictions, d.BlockedByForbidden,
		d.RiskLevel, d.ReviewRequired, d.ApprovedBy, d.Reviewer, d.PromotionReason, d.Reason, d.Actor, d.CreatedAt, d.Metadata,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("record promotion decision: %w", err)
	}
	return nil
}

func (s *ScyllaStore) GetPromotionDecision(ctx context.Context, project, domain, id string) (*api.PromotionDecisionRecord, error) {
	const q = `SELECT principle_id, decision, verdict, missing_evidence, unresolved_authority, unresolved_conditions,
 blocking_contradictions, blocked_by_forbidden, risk_level, review_required, approved_by, reviewer, promotion_reason,
 reason, actor, created_at, metadata
FROM behavioral_memory.promotion_decisions WHERE project = ? AND domain = ? AND id = ?`
	d := &api.PromotionDecisionRecord{ID: id, Project: project, Domain: api.DomainRef(domain)}
	var decision string
	if err := s.session.Query(q, project, domain, id).WithContext(ctx).Scan(
		&d.PrincipleID, &decision, &d.Verdict, &d.MissingEvidence, &d.UnresolvedAuthority, &d.UnresolvedConditions,
		&d.BlockingContradictions, &d.BlockedByForbidden, &d.RiskLevel, &d.ReviewRequired, &d.ApprovedBy, &d.Reviewer,
		&d.PromotionReason, &d.Reason, &d.Actor, &d.CreatedAt, &d.Metadata,
	); err != nil {
		return nil, mapNotFound(err)
	}
	d.Decision = api.PromotionDecision(decision)
	return d, nil
}

func (s *ScyllaStore) RecordRevocationRule(ctx context.Context, r *api.RevocationRule) error {
	const q = `INSERT INTO behavioral_memory.revocation_rules
(project, domain, id, principle_id, action, revocation_reason, condition, note, actor, superseded_by, narrowed_scope, created_at, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if err := s.session.Query(q,
		r.Project, string(r.Domain), r.ID, r.PrincipleID, r.Action, r.RevocationReason, r.Condition, r.Note,
		r.Actor, r.SupersededBy, r.NarrowedScope, r.CreatedAt, r.Metadata,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("record revocation rule: %w", err)
	}
	return nil
}

func (s *ScyllaStore) GetRevocationRule(ctx context.Context, project, domain, id string) (*api.RevocationRule, error) {
	const q = `SELECT principle_id, action, revocation_reason, condition, note, actor, superseded_by, narrowed_scope, created_at, metadata
FROM behavioral_memory.revocation_rules WHERE project = ? AND domain = ? AND id = ?`
	r := &api.RevocationRule{ID: id, Project: project, Domain: api.DomainRef(domain)}
	if err := s.session.Query(q, project, domain, id).WithContext(ctx).Scan(
		&r.PrincipleID, &r.Action, &r.RevocationReason, &r.Condition, &r.Note, &r.Actor, &r.SupersededBy, &r.NarrowedScope, &r.CreatedAt, &r.Metadata,
	); err != nil {
		return nil, mapNotFound(err)
	}
	return r, nil
}

// ── PR-4 runtime ──────────────────────────────────────────────────────────────

func (s *ScyllaStore) IndexPromotedPrinciple(ctx context.Context, p *api.Principle) error {
	const q = `INSERT INTO behavioral_memory.principles_by_condition
(project, domain, condition_id, principle_id, risk_level, promoted_at) VALUES (?, ?, ?, ?, ?, ?)`
	for _, c := range p.AppliesWhen {
		if err := s.session.Query(q, p.Project, string(p.Domain), string(c), p.ID, p.RiskLevel, p.Provenance.UpdatedAt).WithContext(ctx).Exec(); err != nil {
			return fmt.Errorf("index promoted principle: %w", err)
		}
	}
	return nil
}

func (s *ScyllaStore) DeindexPromotedPrinciple(ctx context.Context, p *api.Principle) error {
	const q = `DELETE FROM behavioral_memory.principles_by_condition
WHERE project = ? AND domain = ? AND condition_id = ? AND principle_id = ?`
	for _, c := range p.AppliesWhen {
		if err := s.session.Query(q, p.Project, string(p.Domain), string(c), p.ID).WithContext(ctx).Exec(); err != nil {
			return fmt.Errorf("deindex promoted principle: %w", err)
		}
	}
	return nil
}

func (s *ScyllaStore) ListPrincipleIDsByCondition(ctx context.Context, project, domain, conditionID string) ([]string, error) {
	const q = `SELECT principle_id FROM behavioral_memory.principles_by_condition
WHERE project = ? AND domain = ? AND condition_id = ?`
	iter := s.session.Query(q, project, domain, conditionID).WithContext(ctx).Iter()
	var out []string
	var id string
	for iter.Scan(&id) {
		out = append(out, id)
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("list principles by condition: %w", err)
	}
	return out, nil
}

func (s *ScyllaStore) RecordActionCheck(ctx context.Context, a *api.ActionCheck) error {
	const q = `INSERT INTO behavioral_memory.action_checks
(project, domain, id, action_type, target, conditions, allowed, status, violated_principles, checked_against_principles,
 missing_evidence, unresolved_authority, forbidden_matched, recommended_steps, explanation, agent_id, created_at, metadata, governed)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if err := s.session.Query(q,
		a.Project, string(a.Domain), a.ID, a.ActionType, a.Target, refsToStrings(a.Conditions), a.Allowed, a.Status,
		a.ViolatedPrinciples, a.CheckedAgainstPrinciples, refsToStrings(a.MissingEvidence), refsToStrings(a.UnresolvedAuthority),
		refsToStrings(a.ForbiddenMatched), a.RecommendedSteps, a.Explanation, a.AgentID, a.CreatedAt, a.Metadata, a.Governed,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("record action check: %w", err)
	}
	return nil
}

func (s *ScyllaStore) GetActionCheck(ctx context.Context, project, domain, id string) (*api.ActionCheck, error) {
	const q = `SELECT action_type, target, conditions, allowed, status, violated_principles, checked_against_principles,
 missing_evidence, unresolved_authority, forbidden_matched, recommended_steps, explanation, agent_id, created_at, metadata, governed
FROM behavioral_memory.action_checks WHERE project = ? AND domain = ? AND id = ?`
	a := &api.ActionCheck{ID: id, Project: project, Domain: api.DomainRef(domain)}
	var conditions, missing, unresolved, forbidden []string
	if err := s.session.Query(q, project, domain, id).WithContext(ctx).Scan(
		&a.ActionType, &a.Target, &conditions, &a.Allowed, &a.Status, &a.ViolatedPrinciples, &a.CheckedAgainstPrinciples,
		&missing, &unresolved, &forbidden, &a.RecommendedSteps, &a.Explanation, &a.AgentID, &a.CreatedAt, &a.Metadata, &a.Governed,
	); err != nil {
		return nil, mapNotFound(err)
	}
	a.Conditions = toConditionRefs(conditions)
	a.MissingEvidence = toRequiredEvidenceRefsStore(missing)
	a.UnresolvedAuthority = toAuthorityRefsStore(unresolved)
	a.ForbiddenMatched = toForbiddenMoveRefs(forbidden)
	return a, nil
}

func (s *ScyllaStore) IncrementCoverage(ctx context.Context, project, domain string, governed bool) error {
	// Two fixed const queries (never interpolate the column) so the counter update
	// stays injection-free and the no-ALLOW-FILTERING scanners see literal CQL.
	q := `UPDATE behavioral_memory.governance_coverage SET ungoverned_count = ungoverned_count + 1 WHERE project = ? AND domain = ?`
	if governed {
		q = `UPDATE behavioral_memory.governance_coverage SET governed_count = governed_count + 1 WHERE project = ? AND domain = ?`
	}
	if err := s.session.Query(q, project, domain).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("increment coverage: %w", err)
	}
	return nil
}

func (s *ScyllaStore) GetCoverage(ctx context.Context, project, domain string) (int64, int64, error) {
	const q = `SELECT governed_count, ungoverned_count FROM behavioral_memory.governance_coverage WHERE project = ? AND domain = ?`
	var g, u int64
	if err := s.session.Query(q, project, domain).WithContext(ctx).Scan(&g, &u); err != nil {
		if err == gocql.ErrNotFound {
			return 0, 0, nil // no checks recorded yet → zero coverage, not an error
		}
		return 0, 0, fmt.Errorf("get coverage: %w", err)
	}
	return g, u, nil
}

func (s *ScyllaStore) RecordOutcome(ctx context.Context, o *api.Outcome) error {
	batch := s.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.Query(`INSERT INTO behavioral_memory.outcomes
(project, domain, id, action_check_id, principle_ids, evidence_ids, supports_principles, weakens_principles,
 status, severe, human_marked, incident_id, theme, note, agent_id, created_at, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.Project, string(o.Domain), o.ID, o.ActionCheckID, o.PrincipleIDs, o.EvidenceIDs, o.SupportsPrinciples, o.WeakensPrinciples,
		o.Status, o.Severe, o.HumanMarked, o.IncidentID, o.Theme, o.Note, o.AgentID, o.CreatedAt, o.Metadata,
	)
	if o.Theme != "" {
		batch.Query(`INSERT INTO behavioral_memory.outcomes_by_theme
(project, domain, theme, created_at, id, status, severe, human_marked, incident_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			o.Project, string(o.Domain), o.Theme, o.CreatedAt, o.ID, o.Status, o.Severe, o.HumanMarked, o.IncidentID,
		)
	}
	if err := s.session.ExecuteBatch(batch); err != nil {
		return fmt.Errorf("record outcome: %w", err)
	}
	return nil
}

func (s *ScyllaStore) GetOutcome(ctx context.Context, project, domain, id string) (*api.Outcome, error) {
	const q = `SELECT action_check_id, principle_ids, evidence_ids, supports_principles, weakens_principles,
 status, severe, human_marked, incident_id, theme, note, agent_id, created_at, metadata
FROM behavioral_memory.outcomes WHERE project = ? AND domain = ? AND id = ?`
	o := &api.Outcome{ID: id, Project: project, Domain: api.DomainRef(domain)}
	if err := s.session.Query(q, project, domain, id).WithContext(ctx).Scan(
		&o.ActionCheckID, &o.PrincipleIDs, &o.EvidenceIDs, &o.SupportsPrinciples, &o.WeakensPrinciples,
		&o.Status, &o.Severe, &o.HumanMarked, &o.IncidentID, &o.Theme, &o.Note, &o.AgentID, &o.CreatedAt, &o.Metadata,
	); err != nil {
		return nil, mapNotFound(err)
	}
	return o, nil
}

func (s *ScyllaStore) ListOutcomesByTheme(ctx context.Context, project, domain, theme string) ([]api.Outcome, error) {
	const q = `SELECT id, status, severe, human_marked, incident_id, created_at
FROM behavioral_memory.outcomes_by_theme WHERE project = ? AND domain = ? AND theme = ?`
	iter := s.session.Query(q, project, domain, theme).WithContext(ctx).Iter()
	var out []api.Outcome
	var id, status, incident string
	var severe, human bool
	var createdAt int64
	for iter.Scan(&id, &status, &severe, &human, &incident, &createdAt) {
		out = append(out, api.Outcome{
			ID: id, Project: project, Domain: api.DomainRef(domain), Status: status,
			Severe: severe, HumanMarked: human, IncidentID: incident, Theme: theme, CreatedAt: createdAt,
		})
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("list outcomes by theme: %w", err)
	}
	return out, nil
}

func (s *ScyllaStore) UpsertPromotionCandidate(ctx context.Context, c *api.PromotionCandidate) error {
	const q = `INSERT INTO behavioral_memory.promotion_candidates
(project, domain, id, theme, status, title, summary, rationale, supporting_outcome_ids, supporting_evidence_ids,
 repeat_count, draft_principle_id, draft_title, draft_applies_when, draft_authorities, draft_required_evidence,
 draft_forbidden_moves, draft_recommended_action, draft_risk_level, draft_revocation_rule, draft_promotion_reason,
 draft_status, draft_version, draft_proposed_by, draft_source_refs, draft_generated_from, generated_by, created_at,
 updated_at, materialized_principle_id, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if err := s.session.Query(q,
		c.Project, string(c.Domain), c.ID, c.Theme, string(c.Status), c.Title, c.Summary, c.Rationale,
		c.SupportingOutcomeIDs, c.SupportingEvidenceIDs, c.RepeatCount, c.DraftPrinciple.ID, c.DraftPrinciple.Title,
		refsToStrings(c.DraftPrinciple.AppliesWhen), refsToStrings(c.DraftPrinciple.Authorities), refsToStrings(c.DraftPrinciple.RequiredEvidence),
		refsToStrings(c.DraftPrinciple.ForbiddenMoves), c.DraftPrinciple.RecommendedAction, c.DraftPrinciple.RiskLevel,
		c.DraftPrinciple.RevocationRule, c.DraftPrinciple.PromotionReason, string(c.DraftPrinciple.Status),
		c.DraftPrinciple.Version, c.DraftPrinciple.ProposedBy, c.DraftPrinciple.SourceRefs, c.DraftPrinciple.GeneratedFrom,
		c.GeneratedBy, c.CreatedAt, c.UpdatedAt, c.MaterializedPrincipleID, c.Metadata,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("upsert promotion candidate: %w", err)
	}
	// Maintain the by-scope list index (single-partition enumeration; see
	// promotion_candidates_by_scope). Upsert is idempotent on (project,domain,id).
	const idx = `INSERT INTO behavioral_memory.promotion_candidates_by_scope (project, domain, id) VALUES (?, ?, ?)`
	if err := s.session.Query(idx, c.Project, string(c.Domain), c.ID).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("index promotion candidate by scope: %w", err)
	}
	return nil
}

func (s *ScyllaStore) GetPromotionCandidate(ctx context.Context, project, domain, id string) (*api.PromotionCandidate, error) {
	const q = `SELECT theme, status, title, summary, rationale, supporting_outcome_ids, supporting_evidence_ids,
 repeat_count, draft_principle_id, draft_title, draft_applies_when, draft_authorities, draft_required_evidence,
 draft_forbidden_moves, draft_recommended_action, draft_risk_level, draft_revocation_rule, draft_promotion_reason,
 draft_status, draft_version, draft_proposed_by, draft_source_refs, draft_generated_from, generated_by, created_at,
 updated_at, materialized_principle_id, metadata
FROM behavioral_memory.promotion_candidates WHERE project = ? AND domain = ? AND id = ?`
	var status string
	var appliesWhen, authorities, requiredEvidence, forbiddenMoves []string
	c := &api.PromotionCandidate{ID: id, Project: project, Domain: api.DomainRef(domain)}
	if err := s.session.Query(q, project, domain, id).WithContext(ctx).Scan(
		&c.Theme, &status, &c.Title, &c.Summary, &c.Rationale, &c.SupportingOutcomeIDs, &c.SupportingEvidenceIDs,
		&c.RepeatCount, &c.DraftPrinciple.ID, &c.DraftPrinciple.Title, &appliesWhen, &authorities,
		&requiredEvidence, &forbiddenMoves, &c.DraftPrinciple.RecommendedAction,
		&c.DraftPrinciple.RiskLevel, &c.DraftPrinciple.RevocationRule, &c.DraftPrinciple.PromotionReason,
		&c.DraftPrinciple.Status, &c.DraftPrinciple.Version, &c.DraftPrinciple.ProposedBy, &c.DraftPrinciple.SourceRefs,
		&c.DraftPrinciple.GeneratedFrom, &c.GeneratedBy, &c.CreatedAt, &c.UpdatedAt, &c.MaterializedPrincipleID, &c.Metadata,
	); err != nil {
		return nil, mapNotFound(err)
	}
	c.Status = api.PromotionCandidateStatus(status)
	c.DraftPrinciple.Project = project
	c.DraftPrinciple.Domain = api.DomainRef(domain)
	c.DraftPrinciple.AppliesWhen = toConditionRefs(appliesWhen)
	c.DraftPrinciple.Authorities = toAuthorityRefsStore(authorities)
	c.DraftPrinciple.RequiredEvidence = toRequiredEvidenceRefsStore(requiredEvidence)
	c.DraftPrinciple.ForbiddenMoves = toForbiddenMoveRefs(forbiddenMoves)
	return c, nil
}

func (s *ScyllaStore) ListPromotionCandidates(ctx context.Context, project, domain, theme string, status api.PromotionCandidateStatus, limit int32) ([]api.PromotionCandidate, error) {
	// Enumerate ids from the single-partition by-scope index. Listing directly
	// from promotion_candidates would be a (project,domain) prefix query against a
	// composite ((project,domain,id)) partition key — rejected without ALLOW
	// FILTERING. theme/status are filtered in memory below against the entity row.
	const q = `SELECT id FROM behavioral_memory.promotion_candidates_by_scope WHERE project = ? AND domain = ?`
	iter := s.session.Query(q, project, domain).WithContext(ctx).Iter()
	var id string
	var out []api.PromotionCandidate
	for iter.Scan(&id) {
		c, err := s.GetPromotionCandidate(ctx, project, domain, id)
		if err != nil {
			return nil, err
		}
		if theme != "" && c.Theme != theme {
			continue
		}
		if status != api.PromotionCandidateStatusUnspecified && c.Status != status {
			continue
		}
		out = append(out, *c)
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("list promotion candidates: %w", err)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt == out[j].CreatedAt {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt > out[j].CreatedAt
	})
	if limit > 0 && int(limit) < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (s *ScyllaStore) PutReconciliationReport(ctx context.Context, r *api.ReconciliationReport) error {
	const q = `INSERT INTO behavioral_memory.reconciliation_reports
(project, domain, id, promotion_candidate_id, theme, awg_invariant_ids, awg_failure_mode_ids, awg_test_ids,
 findings, summary, outcome_count, failure_count, success_count, severe_count, proposed_awg_invariant_ids,
 proposed_awg_failure_mode_ids, proposed_awg_test_ids, proposed_behavioral_theme, actor, created_at, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if err := s.session.Query(q,
		r.Project, string(r.Domain), r.ID, r.PromotionCandidateID, r.Theme, r.AWGInvariantIDs, r.AWGFailureModeIDs, r.AWGTestIDs,
		r.Findings, r.Summary, r.OutcomeCount, r.FailureCount, r.SuccessCount, r.SevereCount, r.ProposedAWGInvariantIDs,
		r.ProposedAWGFailureModeIDs, r.ProposedAWGTestIDs, r.ProposedBehavioralTheme, r.Actor, r.CreatedAt, r.Metadata,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("put reconciliation report: %w", err)
	}
	// Maintain the by-scope list index (single-partition enumeration; see
	// reconciliation_reports_by_scope). Idempotent on (project,domain,id).
	const idx = `INSERT INTO behavioral_memory.reconciliation_reports_by_scope (project, domain, id) VALUES (?, ?, ?)`
	if err := s.session.Query(idx, r.Project, string(r.Domain), r.ID).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("index reconciliation report by scope: %w", err)
	}
	return nil
}

func (s *ScyllaStore) GetReconciliationReport(ctx context.Context, project, domain, id string) (*api.ReconciliationReport, error) {
	const q = `SELECT promotion_candidate_id, theme, awg_invariant_ids, awg_failure_mode_ids, awg_test_ids,
 findings, summary, outcome_count, failure_count, success_count, severe_count, proposed_awg_invariant_ids,
 proposed_awg_failure_mode_ids, proposed_awg_test_ids, proposed_behavioral_theme, actor, created_at, metadata
FROM behavioral_memory.reconciliation_reports WHERE project = ? AND domain = ? AND id = ?`
	r := &api.ReconciliationReport{ID: id, Project: project, Domain: api.DomainRef(domain)}
	if err := s.session.Query(q, project, domain, id).WithContext(ctx).Scan(
		&r.PromotionCandidateID, &r.Theme, &r.AWGInvariantIDs, &r.AWGFailureModeIDs, &r.AWGTestIDs,
		&r.Findings, &r.Summary, &r.OutcomeCount, &r.FailureCount, &r.SuccessCount, &r.SevereCount, &r.ProposedAWGInvariantIDs,
		&r.ProposedAWGFailureModeIDs, &r.ProposedAWGTestIDs, &r.ProposedBehavioralTheme, &r.Actor, &r.CreatedAt, &r.Metadata,
	); err != nil {
		return nil, mapNotFound(err)
	}
	return r, nil
}

func (s *ScyllaStore) ListReconciliationReports(ctx context.Context, project, domain, theme, promotionCandidateID string, limit int32) ([]api.ReconciliationReport, error) {
	// Enumerate ids from the single-partition by-scope index (see
	// ListPromotionCandidates for why the entity table cannot be listed directly).
	const q = `SELECT id FROM behavioral_memory.reconciliation_reports_by_scope WHERE project = ? AND domain = ?`
	iter := s.session.Query(q, project, domain).WithContext(ctx).Iter()
	var id string
	var out []api.ReconciliationReport
	for iter.Scan(&id) {
		r, err := s.GetReconciliationReport(ctx, project, domain, id)
		if err != nil {
			return nil, err
		}
		if theme != "" && r.Theme != theme {
			continue
		}
		if promotionCandidateID != "" && r.PromotionCandidateID != promotionCandidateID {
			continue
		}
		out = append(out, *r)
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("list reconciliation reports: %w", err)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt == out[j].CreatedAt {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt > out[j].CreatedAt
	})
	if limit > 0 && int(limit) < len(out) {
		out = out[:limit]
	}
	return out, nil
}

// ── ref slice conversions (store-local) ───────────────────────────────────────

func toConditionRefs(in []string) []api.ConditionRef {
	out := make([]api.ConditionRef, len(in))
	for i, v := range in {
		out[i] = api.ConditionRef(v)
	}
	return out
}
func toAuthorityRefsStore(in []string) []api.AuthorityRef {
	out := make([]api.AuthorityRef, len(in))
	for i, v := range in {
		out[i] = api.AuthorityRef(v)
	}
	return out
}
func toRequiredEvidenceRefsStore(in []string) []api.RequiredEvidenceRef {
	out := make([]api.RequiredEvidenceRef, len(in))
	for i, v := range in {
		out[i] = api.RequiredEvidenceRef(v)
	}
	return out
}
func toForbiddenMoveRefs(in []string) []api.ForbiddenMoveRef {
	out := make([]api.ForbiddenMoveRef, len(in))
	for i, v := range in {
		out[i] = api.ForbiddenMoveRef(v)
	}
	return out
}
