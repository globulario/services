package store

import (
	"context"
	"sort"
	"sync"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
)

// MemoryStore is an in-memory Store adapter. It backs unit tests (so the
// ingestion logic is exercised end-to-end without a Scylla container) and is a
// legitimate local-first dev backend. It is generic — no database driver, no
// cluster imports — so it lives in the kernel without violating hygiene.
//
// It is safe for concurrent use.
type MemoryStore struct {
	mu                    sync.RWMutex
	signals               map[string]*api.Signal
	claims                map[string]*api.Claim
	evidence              map[string]*api.Evidence
	evidenceByTgt         map[string][]string // (project|domain|targetID) -> evidence ids
	authorities           map[string]*api.Authority
	conditions            map[string]*api.Condition
	contradictions        map[string]*api.Contradiction
	contradictionByTgt    map[string][]string // (project|domain|targetID) -> contradiction ids
	principles            map[string]*api.Principle
	promotionDecisions    map[string]*api.PromotionDecisionRecord
	revocationRules       map[string]*api.RevocationRule
	princByCondition      map[string][]string // (project|domain|conditionID) -> promoted principle ids
	actionChecks          map[string]*api.ActionCheck
	outcomes              map[string]*api.Outcome
	outcomesByTheme       map[string][]string // (project|domain|theme) -> outcome ids
	promotionCandidates   map[string]*api.PromotionCandidate
	reconciliationReports map[string]*api.ReconciliationReport
	coverage              map[string][2]int64 // (project|domain) -> [governed, ungoverned]
}

var _ Store = (*MemoryStore)(nil)

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		signals:               map[string]*api.Signal{},
		claims:                map[string]*api.Claim{},
		evidence:              map[string]*api.Evidence{},
		evidenceByTgt:         map[string][]string{},
		authorities:           map[string]*api.Authority{},
		conditions:            map[string]*api.Condition{},
		contradictions:        map[string]*api.Contradiction{},
		contradictionByTgt:    map[string][]string{},
		principles:            map[string]*api.Principle{},
		promotionDecisions:    map[string]*api.PromotionDecisionRecord{},
		revocationRules:       map[string]*api.RevocationRule{},
		princByCondition:      map[string][]string{},
		actionChecks:          map[string]*api.ActionCheck{},
		outcomes:              map[string]*api.Outcome{},
		outcomesByTheme:       map[string][]string{},
		promotionCandidates:   map[string]*api.PromotionCandidate{},
		reconciliationReports: map[string]*api.ReconciliationReport{},
		coverage:              map[string][2]int64{},
	}
}

func (*MemoryStore) Backend() string { return "memory" }

func key(project, domain, id string) string { return project + "|" + domain + "|" + id }

func (m *MemoryStore) PutSignal(_ context.Context, s *api.Signal) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *s
	m.signals[key(s.Project, string(s.Domain), s.ID)] = &cp
	return nil
}

func (m *MemoryStore) GetSignal(_ context.Context, project, domain, id string) (*api.Signal, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.signals[key(project, domain, id)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (m *MemoryStore) PutClaim(_ context.Context, c *api.Claim) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *c
	m.claims[key(c.Project, string(c.Domain), c.ID)] = &cp
	return nil
}

func (m *MemoryStore) GetClaim(_ context.Context, project, domain, id string) (*api.Claim, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.claims[key(project, domain, id)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *c
	return &cp, nil
}

func (m *MemoryStore) UpdateClaimStatus(_ context.Context, project, domain, id string, status api.GovernanceStatus, updatedAt int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.claims[key(project, domain, id)]
	if !ok {
		return ErrNotFound
	}
	c.Status = status
	c.Provenance.UpdatedAt = updatedAt
	return nil
}

func (m *MemoryStore) PutEvidence(_ context.Context, e *api.Evidence) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *e
	m.evidence[key(e.Project, string(e.Domain), e.ID)] = &cp
	if e.TargetID != "" {
		tk := key(e.Project, string(e.Domain), e.TargetID)
		m.evidenceByTgt[tk] = append(m.evidenceByTgt[tk], e.ID)
	}
	return nil
}

func (m *MemoryStore) ListEvidenceForTarget(_ context.Context, project, domain, targetID string) ([]api.Evidence, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := m.evidenceByTgt[key(project, domain, targetID)]
	out := make([]api.Evidence, 0, len(ids))
	for _, id := range ids {
		if e, ok := m.evidence[key(project, domain, id)]; ok {
			out = append(out, *e)
		}
	}
	return out, nil
}

func (m *MemoryStore) PutAuthority(_ context.Context, a *api.Authority) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *a
	m.authorities[key(a.Project, string(a.Domain), a.ID)] = &cp
	return nil
}

func (m *MemoryStore) GetAuthority(_ context.Context, project, domain, id string) (*api.Authority, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.authorities[key(project, domain, id)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *a
	return &cp, nil
}

func (m *MemoryStore) ListAuthorities(_ context.Context, project, domain string, limit int32) ([]api.Authority, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []api.Authority
	for _, a := range m.authorities {
		if a.Project != project || string(a.Domain) != domain {
			continue
		}
		out = append(out, *a)
		if limit > 0 && int32(len(out)) >= limit {
			break
		}
	}
	return out, nil
}

func (m *MemoryStore) AddAuthorityGoverns(_ context.Context, project, domain, authorityID, targetRef string, updatedAt int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key(project, domain, authorityID)
	a, ok := m.authorities[k]
	if !ok {
		a = &api.Authority{ID: authorityID, Project: project, Domain: api.DomainRef(domain), Status: api.StatusAuthorityMapped}
		m.authorities[k] = a
	}
	for _, g := range a.GovernsRefs {
		if g == targetRef {
			return nil // already present (set semantics)
		}
	}
	a.GovernsRefs = append(a.GovernsRefs, targetRef)
	return nil
}

func (m *MemoryStore) PutCondition(_ context.Context, c *api.Condition) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *c
	m.conditions[key(c.Project, string(c.Domain), c.ID)] = &cp
	return nil
}

func (m *MemoryStore) GetCondition(_ context.Context, project, domain, id string) (*api.Condition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.conditions[key(project, domain, id)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *c
	return &cp, nil
}

func (m *MemoryStore) ListConditions(_ context.Context, project, domain string, limit int32) ([]api.Condition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []api.Condition
	for _, c := range m.conditions {
		if c.Project != project || string(c.Domain) != domain {
			continue
		}
		out = append(out, *c)
		if limit > 0 && int32(len(out)) >= limit {
			break
		}
	}
	return out, nil
}

func (m *MemoryStore) PutContradiction(_ context.Context, c *api.Contradiction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *c
	m.contradictions[key(c.Project, string(c.Domain), c.ID)] = &cp
	for _, ref := range []string{c.LeftRef, c.RightRef} {
		if ref == "" {
			continue
		}
		tk := key(c.Project, string(c.Domain), ref)
		m.contradictionByTgt[tk] = append(m.contradictionByTgt[tk], c.ID)
	}
	return nil
}

func (m *MemoryStore) GetContradiction(_ context.Context, project, domain, id string) (*api.Contradiction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.contradictions[key(project, domain, id)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *c
	return &cp, nil
}

func (m *MemoryStore) ListContradictionsForTarget(_ context.Context, project, domain, targetID string) ([]api.Contradiction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := m.contradictionByTgt[key(project, domain, targetID)]
	out := make([]api.Contradiction, 0, len(ids))
	for _, id := range ids {
		if c, ok := m.contradictions[key(project, domain, id)]; ok {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (m *MemoryStore) ResolveAuthorityRefs(_ context.Context, project, domain string, refs []string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var unresolved []string
	for _, r := range refs {
		if _, ok := m.authorities[key(project, domain, r)]; !ok {
			unresolved = append(unresolved, r)
		}
	}
	return unresolved, nil
}

func (m *MemoryStore) ResolveConditionRefs(_ context.Context, project, domain string, refs []string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var unresolved []string
	for _, r := range refs {
		if _, ok := m.conditions[key(project, domain, r)]; !ok {
			unresolved = append(unresolved, r)
		}
	}
	return unresolved, nil
}

// ── PR-3 governance ────────────────────────────────────────────────────────

func (m *MemoryStore) CreatePrinciple(_ context.Context, p *api.Principle) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *p
	m.principles[key(p.Project, string(p.Domain), p.ID)] = &cp
	return nil
}

func (m *MemoryStore) GetPrinciple(_ context.Context, project, domain, id string) (*api.Principle, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.principles[key(project, domain, id)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (m *MemoryStore) UpdatePrincipleStatus(_ context.Context, project, domain, id string, status api.GovernanceStatus, updatedAt int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.principles[key(project, domain, id)]
	if !ok {
		return ErrNotFound
	}
	p.Status = status
	p.Provenance.UpdatedAt = updatedAt
	return nil
}

func (m *MemoryStore) SetPrincipleContradictionChecked(_ context.Context, project, domain, id string, checked bool, updatedAt int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.principles[key(project, domain, id)]
	if !ok {
		return ErrNotFound
	}
	p.ContradictionChecked = checked
	p.Provenance.UpdatedAt = updatedAt
	return nil
}

func (m *MemoryStore) RecordPromotionDecision(_ context.Context, d *api.PromotionDecisionRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *d
	m.promotionDecisions[key(d.Project, string(d.Domain), d.ID)] = &cp
	return nil
}

func (m *MemoryStore) GetPromotionDecision(_ context.Context, project, domain, id string) (*api.PromotionDecisionRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.promotionDecisions[key(project, domain, id)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *d
	return &cp, nil
}

func (m *MemoryStore) RecordRevocationRule(_ context.Context, r *api.RevocationRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.revocationRules[key(r.Project, string(r.Domain), r.ID)] = &cp
	return nil
}

func (m *MemoryStore) GetRevocationRule(_ context.Context, project, domain, id string) (*api.RevocationRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.revocationRules[key(project, domain, id)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *r
	return &cp, nil
}

// ── PR-4 runtime ───────────────────────────────────────────────────────────

func (m *MemoryStore) IndexPromotedPrinciple(_ context.Context, p *api.Principle) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range p.AppliesWhen {
		k := key(p.Project, string(p.Domain), string(c))
		found := false
		for _, id := range m.princByCondition[k] {
			if id == p.ID {
				found = true
				break
			}
		}
		if !found {
			m.princByCondition[k] = append(m.princByCondition[k], p.ID)
		}
	}
	return nil
}

func (m *MemoryStore) DeindexPromotedPrinciple(_ context.Context, p *api.Principle) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range p.AppliesWhen {
		k := key(p.Project, string(p.Domain), string(c))
		out := m.princByCondition[k][:0:0]
		for _, id := range m.princByCondition[k] {
			if id != p.ID {
				out = append(out, id)
			}
		}
		m.princByCondition[k] = out
	}
	return nil
}

func (m *MemoryStore) ListPrincipleIDsByCondition(_ context.Context, project, domain, conditionID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := m.princByCondition[key(project, domain, conditionID)]
	out := make([]string, len(ids))
	copy(out, ids)
	return out, nil
}

func (m *MemoryStore) RecordActionCheck(_ context.Context, a *api.ActionCheck) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *a
	m.actionChecks[key(a.Project, string(a.Domain), a.ID)] = &cp
	return nil
}

func (m *MemoryStore) GetActionCheck(_ context.Context, project, domain, id string) (*api.ActionCheck, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.actionChecks[key(project, domain, id)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *a
	return &cp, nil
}

func (m *MemoryStore) IncrementCoverage(_ context.Context, project, domain string, governed bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key(project, domain, "")
	c := m.coverage[k]
	if governed {
		c[0]++
	} else {
		c[1]++
	}
	m.coverage[k] = c
	return nil
}

func (m *MemoryStore) GetCoverage(_ context.Context, project, domain string) (int64, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c := m.coverage[key(project, domain, "")]
	return c[0], c[1], nil
}

func (m *MemoryStore) RecordOutcome(_ context.Context, o *api.Outcome) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *o
	m.outcomes[key(o.Project, string(o.Domain), o.ID)] = &cp
	if o.Theme != "" {
		tk := key(o.Project, string(o.Domain), o.Theme)
		m.outcomesByTheme[tk] = append(m.outcomesByTheme[tk], o.ID)
	}
	return nil
}

func (m *MemoryStore) GetOutcome(_ context.Context, project, domain, id string) (*api.Outcome, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	o, ok := m.outcomes[key(project, domain, id)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *o
	return &cp, nil
}

func (m *MemoryStore) ListOutcomesByTheme(_ context.Context, project, domain, theme string) ([]api.Outcome, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := m.outcomesByTheme[key(project, domain, theme)]
	out := make([]api.Outcome, 0, len(ids))
	for _, id := range ids {
		if o, ok := m.outcomes[key(project, domain, id)]; ok {
			out = append(out, *o)
		}
	}
	return out, nil
}

func (m *MemoryStore) UpsertPromotionCandidate(_ context.Context, c *api.PromotionCandidate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *c
	m.promotionCandidates[key(c.Project, string(c.Domain), c.ID)] = &cp
	return nil
}

func (m *MemoryStore) GetPromotionCandidate(_ context.Context, project, domain, id string) (*api.PromotionCandidate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.promotionCandidates[key(project, domain, id)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *c
	return &cp, nil
}

func (m *MemoryStore) ListPromotionCandidates(_ context.Context, project, domain, theme string, status api.PromotionCandidateStatus, limit int32) ([]api.PromotionCandidate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]api.PromotionCandidate, 0, len(m.promotionCandidates))
	for _, c := range m.promotionCandidates {
		if c.Project != project || string(c.Domain) != domain {
			continue
		}
		if theme != "" && c.Theme != theme {
			continue
		}
		if status != api.PromotionCandidateStatusUnspecified && c.Status != status {
			continue
		}
		out = append(out, *c)
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

func (m *MemoryStore) PutReconciliationReport(_ context.Context, r *api.ReconciliationReport) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.reconciliationReports[key(r.Project, string(r.Domain), r.ID)] = &cp
	return nil
}

func (m *MemoryStore) GetReconciliationReport(_ context.Context, project, domain, id string) (*api.ReconciliationReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.reconciliationReports[key(project, domain, id)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *r
	return &cp, nil
}

func (m *MemoryStore) ListReconciliationReports(_ context.Context, project, domain, theme, promotionCandidateID string, limit int32) ([]api.ReconciliationReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]api.ReconciliationReport, 0, len(m.reconciliationReports))
	for _, r := range m.reconciliationReports {
		if r.Project != project || string(r.Domain) != domain {
			continue
		}
		if theme != "" && r.Theme != theme {
			continue
		}
		if promotionCandidateID != "" && r.PromotionCandidateID != promotionCandidateID {
			continue
		}
		out = append(out, *r)
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
