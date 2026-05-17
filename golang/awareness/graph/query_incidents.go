package graph

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// IncidentRecord maps to the incidents store.
type IncidentRecord struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Severity     string `json:"severity,omitempty"`
	Status       string `json:"status,omitempty"`
	StartedAt    int64  `json:"started_at,omitempty"`
	EndedAt      int64  `json:"ended_at,omitempty"`
	Summary      string `json:"summary,omitempty"`
	EvidenceJSON string `json:"evidence_json,omitempty"`
	CreatedAt    int64  `json:"created_at,omitempty"`
	UpdatedAt    int64  `json:"updated_at,omitempty"`
}

// ProposalStatus values for awareness_proposals.
const (
	ProposalStatusDraft       = "DRAFT"
	ProposalStatusValidated   = "VALIDATED"
	ProposalStatusNeedsReview = "NEEDS_REVIEW"
	ProposalStatusApproved    = "APPROVED"
	ProposalStatusRejected    = "REJECTED"
	ProposalStatusPromoted    = "PROMOTED"
	ProposalStatusSuperseded  = "SUPERSEDED"
)

// ProposalRecord maps to the awareness_proposals store.
type ProposalRecord struct {
	ID             string `json:"id"`
	IncidentID     string `json:"incident_id,omitempty"`
	Status         string `json:"status"`
	ProposalYAML   string `json:"proposal_yaml"`
	ValidationJSON string `json:"validation_json,omitempty"`
	CreatedBy      string `json:"created_by,omitempty"`
	CreatedAt      int64  `json:"created_at,omitempty"`
	PromotedAt     int64  `json:"promoted_at,omitempty"`
}

// UpsertIncident inserts or updates an incident record.
func (g *Graph) UpsertIncident(ctx context.Context, inc IncidentRecord) error {
	if g.readOnly {
		return fmt.Errorf("UpsertIncident %s: graph is read-only", inc.ID)
	}
	now := time.Now().Unix()
	if inc.CreatedAt == 0 {
		inc.CreatedAt = now
	}
	inc.UpdatedAt = now

	g.incidentMu.Lock()
	cp := inc
	g.incidents[inc.ID] = &cp
	g.incidentMu.Unlock()

	return g.writeJSON("incidents", inc.ID, &inc)
}

// FindIncident returns an incident by ID, or (nil, nil) if not found.
func (g *Graph) FindIncident(ctx context.Context, id string) (*IncidentRecord, error) {
	g.incidentMu.RLock()
	rec := g.incidents[id]
	g.incidentMu.RUnlock()
	if rec == nil {
		return nil, nil
	}
	cp := *rec
	return &cp, nil
}

// AllProposals returns all proposal records ordered by created_at descending.
func (g *Graph) AllProposals(ctx context.Context) ([]*ProposalRecord, error) {
	g.proposalMu.RLock()
	out := make([]*ProposalRecord, 0, len(g.proposals))
	for _, p := range g.proposals {
		cp := *p
		out = append(out, &cp)
	}
	g.proposalMu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

// UpdateProposalStatus sets the status (and promoted_at if PROMOTED) of a proposal.
func (g *Graph) UpdateProposalStatus(ctx context.Context, id, status string) error {
	if g.readOnly {
		return fmt.Errorf("UpdateProposalStatus %s: graph is read-only", id)
	}
	g.proposalMu.Lock()
	p := g.proposals[id]
	if p != nil {
		p.Status = status
		if status == ProposalStatusPromoted {
			p.PromotedAt = time.Now().Unix()
		}
	}
	g.proposalMu.Unlock()

	if p != nil {
		return g.writeJSON("proposals", id, p)
	}
	return nil
}
