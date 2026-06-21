// Package store is the persistence port for the behavioral-memory kernel.
//
// The port (this file) is generic: it speaks only api types and stdlib errors,
// never a database driver. Concrete adapters implement it:
//   - scylla_store.go  — the production ScyllaDB adapter (the ONLY file in this
//     package that imports gocql / speaks CQL).
//   - memory_store.go  — an in-memory adapter for tests and local-first dev.
//
// Dependency direction (enforced by the kernel-hygiene test):
//
//	behavioral/core  →  store.Store (interface)
//	store/scylla_store.go  →  ScyllaDB
//	ai_memory_server →  behavioral/core + the chosen adapter (composition root)
//
// PR-2 surface: persistence for the ingestion half of the ladder (signals,
// claims, evidence, authorities, conditions, contradictions). Promotion/runtime
// tables and their methods arrive in later PRs.
package store

import (
	"context"
	"errors"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
)

// ErrNotFound is returned by Get* methods when no row matches.
var ErrNotFound = errors.New("behavioral-memory store: not found")

// ErrUnconfigured is returned by the Unconfigured store for any data method.
var ErrUnconfigured = errors.New("behavioral-memory store: no persistence backend configured")

// Store is the persistence port. All methods are scoped by (project, domain) and
// address entities by their stable canonical id — the same id used for RDF
// projection later. No method uses ALLOW FILTERING.
type Store interface {
	// Backend names the persistence implementation (e.g. "scylla", "memory",
	// "unconfigured").
	Backend() string

	// Signals.
	PutSignal(ctx context.Context, s *api.Signal) error
	GetSignal(ctx context.Context, project, domain, id string) (*api.Signal, error)

	// Claims.
	PutClaim(ctx context.Context, c *api.Claim) error
	GetClaim(ctx context.Context, project, domain, id string) (*api.Claim, error)
	UpdateClaimStatus(ctx context.Context, project, domain, id string, status api.GovernanceStatus, updatedAt int64) error

	// Evidence. PutEvidence also maintains the evidence_by_target lookup.
	PutEvidence(ctx context.Context, e *api.Evidence) error
	ListEvidenceForTarget(ctx context.Context, project, domain, targetID string) ([]api.Evidence, error)

	// Authorities. AddAuthorityGoverns records a target governed by an authority,
	// creating the authority row if it does not yet exist (set-add semantics).
	PutAuthority(ctx context.Context, a *api.Authority) error
	GetAuthority(ctx context.Context, project, domain, id string) (*api.Authority, error)
	AddAuthorityGoverns(ctx context.Context, project, domain, authorityID, targetRef string, updatedAt int64) error
	// ResolveAuthorityRefs returns the subset of refs that do NOT resolve to an
	// authority row (used by the promotion gate).
	ResolveAuthorityRefs(ctx context.Context, project, domain string, refs []string) (unresolved []string, err error)

	// Conditions (catalog table; written by the domain pack in a later PR — the
	// store methods exist now so the schema is exercised).
	PutCondition(ctx context.Context, c *api.Condition) error
	GetCondition(ctx context.Context, project, domain, id string) (*api.Condition, error)
	// ResolveConditionRefs returns the subset of refs that do NOT resolve to a
	// condition row (used by the promotion gate).
	ResolveConditionRefs(ctx context.Context, project, domain string, refs []string) (unresolved []string, err error)

	// Contradictions. PutContradiction also maintains contradictions_by_target.
	PutContradiction(ctx context.Context, c *api.Contradiction) error
	GetContradiction(ctx context.Context, project, domain, id string) (*api.Contradiction, error)
	ListContradictionsForTarget(ctx context.Context, project, domain, targetID string) ([]api.Contradiction, error)

	// ── PR-3 governance ──────────────────────────────────────────────────────

	// Principles.
	CreatePrinciple(ctx context.Context, p *api.Principle) error
	GetPrinciple(ctx context.Context, project, domain, id string) (*api.Principle, error)
	UpdatePrincipleStatus(ctx context.Context, project, domain, id string, status api.GovernanceStatus, updatedAt int64) error

	// Promotion decisions (every attempt, including blocked/review-required).
	RecordPromotionDecision(ctx context.Context, d *api.PromotionDecisionRecord) error
	GetPromotionDecision(ctx context.Context, project, domain, id string) (*api.PromotionDecisionRecord, error)

	// Revocation rules (revoke/supersede/narrow — never delete the principle).
	RecordRevocationRule(ctx context.Context, r *api.RevocationRule) error
	GetRevocationRule(ctx context.Context, project, domain, id string) (*api.RevocationRule, error)

	// ── PR-4 runtime ───────────────────────────────────────────────────────────

	// principles_by_condition index — written ONLY by promotion, removed by
	// revocation, so it holds only active promoted mappings.
	IndexPromotedPrinciple(ctx context.Context, p *api.Principle) error
	DeindexPromotedPrinciple(ctx context.Context, p *api.Principle) error
	ListPrincipleIDsByCondition(ctx context.Context, project, domain, conditionID string) ([]string, error)

	// Action-check audit trail.
	RecordActionCheck(ctx context.Context, a *api.ActionCheck) error
	GetActionCheck(ctx context.Context, project, domain, id string) (*api.ActionCheck, error)

	// Outcomes. RecordOutcome also maintains outcomes_by_theme.
	RecordOutcome(ctx context.Context, o *api.Outcome) error
	GetOutcome(ctx context.Context, project, domain, id string) (*api.Outcome, error)
	ListOutcomesByTheme(ctx context.Context, project, domain, theme string) ([]api.Outcome, error)

	// Promotion-candidate review queue (PR-10). Queue entries are not principles
	// and never imply auto-promotion.
	UpsertPromotionCandidate(ctx context.Context, c *api.PromotionCandidate) error
	GetPromotionCandidate(ctx context.Context, project, domain, id string) (*api.PromotionCandidate, error)
	ListPromotionCandidates(ctx context.Context, project, domain, theme string, status api.PromotionCandidateStatus, limit int32) ([]api.PromotionCandidate, error)

	// Reconciliation reports (PR-11). Advisory bridge artifacts only.
	PutReconciliationReport(ctx context.Context, r *api.ReconciliationReport) error
	GetReconciliationReport(ctx context.Context, project, domain, id string) (*api.ReconciliationReport, error)
	ListReconciliationReports(ctx context.Context, project, domain, theme, promotionCandidateID string, limit int32) ([]api.ReconciliationReport, error)
}

// Unconfigured is the no-persistence Store. It is the fallback when no backend is
// wired (e.g. the Scylla session is unavailable); every data method returns
// ErrUnconfigured rather than panicking.
type Unconfigured struct{}

var _ Store = Unconfigured{}

func (Unconfigured) Backend() string { return "unconfigured" }

func (Unconfigured) PutSignal(context.Context, *api.Signal) error { return ErrUnconfigured }
func (Unconfigured) GetSignal(context.Context, string, string, string) (*api.Signal, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) PutClaim(context.Context, *api.Claim) error { return ErrUnconfigured }
func (Unconfigured) GetClaim(context.Context, string, string, string) (*api.Claim, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) UpdateClaimStatus(context.Context, string, string, string, api.GovernanceStatus, int64) error {
	return ErrUnconfigured
}
func (Unconfigured) PutEvidence(context.Context, *api.Evidence) error { return ErrUnconfigured }
func (Unconfigured) ListEvidenceForTarget(context.Context, string, string, string) ([]api.Evidence, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) PutAuthority(context.Context, *api.Authority) error { return ErrUnconfigured }
func (Unconfigured) GetAuthority(context.Context, string, string, string) (*api.Authority, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) AddAuthorityGoverns(context.Context, string, string, string, string, int64) error {
	return ErrUnconfigured
}
func (Unconfigured) ResolveAuthorityRefs(context.Context, string, string, []string) ([]string, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) PutCondition(context.Context, *api.Condition) error { return ErrUnconfigured }
func (Unconfigured) GetCondition(context.Context, string, string, string) (*api.Condition, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) ResolveConditionRefs(context.Context, string, string, []string) ([]string, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) PutContradiction(context.Context, *api.Contradiction) error {
	return ErrUnconfigured
}
func (Unconfigured) GetContradiction(context.Context, string, string, string) (*api.Contradiction, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) ListContradictionsForTarget(context.Context, string, string, string) ([]api.Contradiction, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) CreatePrinciple(context.Context, *api.Principle) error { return ErrUnconfigured }
func (Unconfigured) GetPrinciple(context.Context, string, string, string) (*api.Principle, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) UpdatePrincipleStatus(context.Context, string, string, string, api.GovernanceStatus, int64) error {
	return ErrUnconfigured
}
func (Unconfigured) RecordPromotionDecision(context.Context, *api.PromotionDecisionRecord) error {
	return ErrUnconfigured
}
func (Unconfigured) GetPromotionDecision(context.Context, string, string, string) (*api.PromotionDecisionRecord, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) RecordRevocationRule(context.Context, *api.RevocationRule) error {
	return ErrUnconfigured
}
func (Unconfigured) GetRevocationRule(context.Context, string, string, string) (*api.RevocationRule, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) IndexPromotedPrinciple(context.Context, *api.Principle) error {
	return ErrUnconfigured
}
func (Unconfigured) DeindexPromotedPrinciple(context.Context, *api.Principle) error {
	return ErrUnconfigured
}
func (Unconfigured) ListPrincipleIDsByCondition(context.Context, string, string, string) ([]string, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) RecordActionCheck(context.Context, *api.ActionCheck) error {
	return ErrUnconfigured
}
func (Unconfigured) GetActionCheck(context.Context, string, string, string) (*api.ActionCheck, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) RecordOutcome(context.Context, *api.Outcome) error { return ErrUnconfigured }
func (Unconfigured) GetOutcome(context.Context, string, string, string) (*api.Outcome, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) ListOutcomesByTheme(context.Context, string, string, string) ([]api.Outcome, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) UpsertPromotionCandidate(context.Context, *api.PromotionCandidate) error {
	return ErrUnconfigured
}
func (Unconfigured) GetPromotionCandidate(context.Context, string, string, string) (*api.PromotionCandidate, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) ListPromotionCandidates(context.Context, string, string, string, api.PromotionCandidateStatus, int32) ([]api.PromotionCandidate, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) PutReconciliationReport(context.Context, *api.ReconciliationReport) error {
	return ErrUnconfigured
}
func (Unconfigured) GetReconciliationReport(context.Context, string, string, string) (*api.ReconciliationReport, error) {
	return nil, ErrUnconfigured
}
func (Unconfigured) ListReconciliationReports(context.Context, string, string, string, string, int32) ([]api.ReconciliationReport, error) {
	return nil, ErrUnconfigured
}
