package graph

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Invariant is the specialized invariant record stored in the invariants table.
type Invariant struct {
	ID       string
	Title    string
	Summary  string
	Severity string
	Status   string
}

// FailureMode is the specialized failure mode record stored in the failure_modes table.
type FailureMode struct {
	ID              string
	Title           string
	Summary         string
	Symptoms        []string
	RootCause       string
	ArchitectureFix string
}

// BuildStats holds graph build statistics.
type BuildStats struct {
	Nodes        int `json:"nodes"`
	Edges        int `json:"edges"`
	Invariants   int `json:"invariants"`
	FailureModes int `json:"failure_modes"`
}

// FindNode returns a node by ID, or (nil, nil) if not found.
func (g *Graph) FindNode(ctx context.Context, id string) (*Node, error) {
	row := g.db.QueryRowContext(ctx, `
		SELECT id, type, name, path, summary, metadata_json, created_at, updated_at
		FROM nodes WHERE id = ?
	`, id)
	return scanNode(row)
}

// FindNodesByType returns all nodes of the given type ordered by name.
func (g *Graph) FindNodesByType(ctx context.Context, nodeType string) ([]*Node, error) {
	rows, err := g.db.QueryContext(ctx, `
		SELECT id, type, name, path, summary, metadata_json, created_at, updated_at
		FROM nodes WHERE type = ? ORDER BY name
	`, nodeType)
	if err != nil {
		return nil, fmt.Errorf("FindNodesByType %s: %w", nodeType, err)
	}
	defer rows.Close()
	return scanNodes(rows)
}

// FindNodesByPath returns nodes whose path exactly matches the given value.
func (g *Graph) FindNodesByPath(ctx context.Context, path string) ([]*Node, error) {
	rows, err := g.db.QueryContext(ctx, `
		SELECT id, type, name, path, summary, metadata_json, created_at, updated_at
		FROM nodes WHERE path = ? ORDER BY name
	`, path)
	if err != nil {
		return nil, fmt.Errorf("FindNodesByPath %q: %w", path, err)
	}
	defer rows.Close()
	return scanNodes(rows)
}

// FindNodeByTypeAndName returns the first node matching type + exact name.
func (g *Graph) FindNodeByTypeAndName(ctx context.Context, nodeType, name string) (*Node, error) {
	row := g.db.QueryRowContext(ctx, `
		SELECT id, type, name, path, summary, metadata_json, created_at, updated_at
		FROM nodes WHERE type = ? AND name = ? LIMIT 1
	`, nodeType, name)
	return scanNode(row)
}

// FindNodesByNameLike returns nodes whose name contains the query string.
func (g *Graph) FindNodesByNameLike(ctx context.Context, query string) ([]*Node, error) {
	rows, err := g.db.QueryContext(ctx, `
		SELECT id, type, name, path, summary, metadata_json, created_at, updated_at
		FROM nodes WHERE name LIKE ? ORDER BY name
	`, "%"+query+"%")
	if err != nil {
		return nil, fmt.Errorf("FindNodesByNameLike %q: %w", query, err)
	}
	defer rows.Close()
	return scanNodes(rows)
}

// Neighbors returns edges connected to id.
// direction is "out" (outgoing), "in" (incoming), or "both".
func (g *Graph) Neighbors(ctx context.Context, id, direction string) ([]Edge, error) {
	var (
		rows *sql.Rows
		err  error
	)
	switch direction {
	case "in":
		rows, err = g.db.QueryContext(ctx, `
			SELECT src, kind, dst, phase, required, confidence, metadata_json
			FROM edges WHERE dst = ?
		`, id)
	case "out":
		rows, err = g.db.QueryContext(ctx, `
			SELECT src, kind, dst, phase, required, confidence, metadata_json
			FROM edges WHERE src = ?
		`, id)
	default:
		rows, err = g.db.QueryContext(ctx, `
			SELECT src, kind, dst, phase, required, confidence, metadata_json
			FROM edges WHERE src = ? OR dst = ?
		`, id, id)
	}
	if err != nil {
		return nil, fmt.Errorf("Neighbors %s: %w", id, err)
	}
	defer rows.Close()
	return scanEdges(rows)
}

// AllEdges returns every edge in the graph (used by cycle detection).
func (g *Graph) AllEdges(ctx context.Context) ([]Edge, error) {
	rows, err := g.db.QueryContext(ctx, `
		SELECT src, kind, dst, phase, required, confidence, metadata_json FROM edges
	`)
	if err != nil {
		return nil, fmt.Errorf("AllEdges: %w", err)
	}
	defer rows.Close()
	return scanEdges(rows)
}

// OutgoingEdges returns all edges where src == nodeID.
func (g *Graph) OutgoingEdges(ctx context.Context, nodeID string) ([]Edge, error) {
	rows, err := g.db.QueryContext(ctx, `
		SELECT src, kind, dst, phase, required, confidence, metadata_json
		FROM edges WHERE src = ?
	`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("OutgoingEdges %s: %w", nodeID, err)
	}
	defer rows.Close()
	return scanEdges(rows)
}

// EdgesByKind returns all edges of the given kind.
func (g *Graph) EdgesByKind(ctx context.Context, kind string) ([]Edge, error) {
	rows, err := g.db.QueryContext(ctx, `
		SELECT src, kind, dst, phase, required, confidence, metadata_json
		FROM edges WHERE kind = ?
	`, kind)
	if err != nil {
		return nil, fmt.Errorf("EdgesByKind %s: %w", kind, err)
	}
	defer rows.Close()
	return scanEdges(rows)
}

// AllInvariants returns all invariant records ordered by ID.
func (g *Graph) AllInvariants(ctx context.Context) ([]*Invariant, error) {
	rows, err := g.db.QueryContext(ctx, `
		SELECT id, title, summary, severity, status, metadata_json
		FROM invariants ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("AllInvariants: %w", err)
	}
	defer rows.Close()
	var out []*Invariant
	for rows.Next() {
		var inv Invariant
		var meta string
		if err := rows.Scan(&inv.ID, &inv.Title, &inv.Summary, &inv.Severity, &inv.Status, &meta); err != nil {
			return nil, err
		}
		out = append(out, &inv)
	}
	return out, rows.Err()
}

// AllFailureModes returns all failure mode records ordered by ID.
func (g *Graph) AllFailureModes(ctx context.Context) ([]*FailureMode, error) {
	rows, err := g.db.QueryContext(ctx, `
		SELECT id, title, summary, symptoms_json, root_cause, architecture_fix, metadata_json
		FROM failure_modes ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("AllFailureModes: %w", err)
	}
	defer rows.Close()
	var out []*FailureMode
	for rows.Next() {
		var fm FailureMode
		var sympJSON, metaJSON string
		if err := rows.Scan(&fm.ID, &fm.Title, &fm.Summary, &sympJSON, &fm.RootCause, &fm.ArchitectureFix, &metaJSON); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(sympJSON), &fm.Symptoms)
		out = append(out, &fm)
	}
	return out, rows.Err()
}

// UpsertInvariant upserts an invariant record.
func (g *Graph) UpsertInvariant(ctx context.Context, inv Invariant) error {
	_, err := g.db.ExecContext(ctx, `
		INSERT INTO invariants (id, title, summary, severity, status, metadata_json)
		VALUES (?, ?, ?, ?, ?, '{}')
		ON CONFLICT(id) DO UPDATE SET
			title    = excluded.title,
			summary  = excluded.summary,
			severity = excluded.severity,
			status   = excluded.status
	`, inv.ID, inv.Title, inv.Summary, inv.Severity, inv.Status)
	if err != nil {
		return fmt.Errorf("UpsertInvariant %s: %w", inv.ID, err)
	}
	return nil
}

// UpsertFailureMode upserts a failure mode record.
func (g *Graph) UpsertFailureMode(ctx context.Context, fm FailureMode) error {
	sympJSON, err := json.Marshal(fm.Symptoms)
	if err != nil {
		return err
	}
	_, err = g.db.ExecContext(ctx, `
		INSERT INTO failure_modes (id, title, summary, symptoms_json, root_cause, architecture_fix, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, '{}')
		ON CONFLICT(id) DO UPDATE SET
			title            = excluded.title,
			summary          = excluded.summary,
			symptoms_json    = excluded.symptoms_json,
			root_cause       = excluded.root_cause,
			architecture_fix = excluded.architecture_fix
	`, fm.ID, fm.Title, fm.Summary, string(sympJSON), fm.RootCause, fm.ArchitectureFix)
	if err != nil {
		return fmt.Errorf("UpsertFailureMode %s: %w", fm.ID, err)
	}
	return nil
}

// Stats returns current node/edge/invariant/failure-mode counts.
func (g *Graph) Stats(ctx context.Context) (BuildStats, error) {
	var s BuildStats
	for _, row := range []struct {
		dest *int
		q    string
	}{
		{&s.Nodes, `SELECT COUNT(*) FROM nodes`},
		{&s.Edges, `SELECT COUNT(*) FROM edges`},
		{&s.Invariants, `SELECT COUNT(*) FROM invariants`},
		{&s.FailureModes, `SELECT COUNT(*) FROM failure_modes`},
	} {
		if err := g.db.QueryRowContext(ctx, row.q).Scan(row.dest); err != nil {
			return s, err
		}
	}
	return s, nil
}

// UpsertBuildRecord records a completed graph build with its stats.
func (g *Graph) UpsertBuildRecord(ctx context.Context, id, repoRoot, gitCommit, releaseID string, stats BuildStats) error {
	statsJSON, _ := json.Marshal(stats)
	now := time.Now().Unix()
	_, err := g.db.ExecContext(ctx, `
		INSERT INTO graph_builds (id, repo_root, git_commit, release_id, created_at, stats_json)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET stats_json = excluded.stats_json, created_at = excluded.created_at
	`, id, repoRoot, gitCommit, releaseID, now, string(statsJSON))
	if err != nil {
		return fmt.Errorf("UpsertBuildRecord: %w", err)
	}
	return nil
}

// ---- incident records ----

// IncidentRecord maps to the incidents table.
type IncidentRecord struct {
	ID           string
	Title        string
	Severity     string
	Status       string
	StartedAt    int64
	EndedAt      int64
	Summary      string
	EvidenceJSON string
	CreatedAt    int64
	UpdatedAt    int64
}

// ProposalStatus values for awareness_proposals.
const (
	ProposalStatusDraft      = "DRAFT"
	ProposalStatusValidated  = "VALIDATED"
	ProposalStatusNeedsReview = "NEEDS_REVIEW"
	ProposalStatusApproved   = "APPROVED"
	ProposalStatusRejected   = "REJECTED"
	ProposalStatusPromoted   = "PROMOTED"
	ProposalStatusSuperseded = "SUPERSEDED"
)

// ProposalRecord maps to the awareness_proposals table.
type ProposalRecord struct {
	ID             string
	IncidentID     string
	Status         string
	ProposalYAML   string
	ValidationJSON string
	CreatedBy      string
	CreatedAt      int64
	PromotedAt     int64
}

// ContextAliasRecord maps to the context_aliases table.
type ContextAliasRecord struct {
	ID         string
	TargetID   string
	Alias      string
	Confidence float64
	Source     string
	CreatedAt  int64
}

// UpsertIncident inserts or updates an incident record.
func (g *Graph) UpsertIncident(ctx context.Context, inc IncidentRecord) error {
	now := time.Now().Unix()
	if inc.CreatedAt == 0 {
		inc.CreatedAt = now
	}
	_, err := g.db.ExecContext(ctx, `
		INSERT INTO incidents (id, title, severity, status, started_at, ended_at, summary, evidence_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title         = excluded.title,
			severity      = excluded.severity,
			status        = excluded.status,
			started_at    = excluded.started_at,
			ended_at      = excluded.ended_at,
			summary       = excluded.summary,
			evidence_json = excluded.evidence_json,
			updated_at    = excluded.updated_at
	`, inc.ID, inc.Title, inc.Severity, inc.Status, inc.StartedAt, inc.EndedAt,
		inc.Summary, inc.EvidenceJSON, inc.CreatedAt, now)
	if err != nil {
		return fmt.Errorf("UpsertIncident %s: %w", inc.ID, err)
	}
	return nil
}

// FindIncident returns an incident by ID, or (nil, nil) if not found.
func (g *Graph) FindIncident(ctx context.Context, id string) (*IncidentRecord, error) {
	var inc IncidentRecord
	err := g.db.QueryRowContext(ctx, `
		SELECT id, title, severity, status, started_at, ended_at, summary, evidence_json, created_at, updated_at
		FROM incidents WHERE id = ?
	`, id).Scan(&inc.ID, &inc.Title, &inc.Severity, &inc.Status, &inc.StartedAt,
		&inc.EndedAt, &inc.Summary, &inc.EvidenceJSON, &inc.CreatedAt, &inc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("FindIncident %s: %w", id, err)
	}
	return &inc, nil
}

// AllIncidents returns all incident records ordered by created_at descending.
func (g *Graph) AllIncidents(ctx context.Context) ([]*IncidentRecord, error) {
	rows, err := g.db.QueryContext(ctx, `
		SELECT id, title, severity, status, started_at, ended_at, summary, evidence_json, created_at, updated_at
		FROM incidents ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("AllIncidents: %w", err)
	}
	defer rows.Close()
	var out []*IncidentRecord
	for rows.Next() {
		var inc IncidentRecord
		if err := rows.Scan(&inc.ID, &inc.Title, &inc.Severity, &inc.Status, &inc.StartedAt,
			&inc.EndedAt, &inc.Summary, &inc.EvidenceJSON, &inc.CreatedAt, &inc.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &inc)
	}
	return out, rows.Err()
}

// UpsertProposal inserts or updates a proposal record.
func (g *Graph) UpsertProposal(ctx context.Context, p ProposalRecord) error {
	now := time.Now().Unix()
	if p.CreatedAt == 0 {
		p.CreatedAt = now
	}
	_, err := g.db.ExecContext(ctx, `
		INSERT INTO awareness_proposals (id, incident_id, status, proposal_yaml, validation_json, created_by, created_at, promoted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			incident_id     = excluded.incident_id,
			status          = excluded.status,
			proposal_yaml   = excluded.proposal_yaml,
			validation_json = excluded.validation_json,
			created_by      = excluded.created_by,
			promoted_at     = excluded.promoted_at
	`, p.ID, p.IncidentID, p.Status, p.ProposalYAML, p.ValidationJSON, p.CreatedBy, p.CreatedAt, p.PromotedAt)
	if err != nil {
		return fmt.Errorf("UpsertProposal %s: %w", p.ID, err)
	}
	return nil
}

// FindProposal returns a proposal by ID, or (nil, nil) if not found.
func (g *Graph) FindProposal(ctx context.Context, id string) (*ProposalRecord, error) {
	var p ProposalRecord
	err := g.db.QueryRowContext(ctx, `
		SELECT id, incident_id, status, proposal_yaml, validation_json, created_by, created_at, promoted_at
		FROM awareness_proposals WHERE id = ?
	`, id).Scan(&p.ID, &p.IncidentID, &p.Status, &p.ProposalYAML, &p.ValidationJSON,
		&p.CreatedBy, &p.CreatedAt, &p.PromotedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("FindProposal %s: %w", id, err)
	}
	return &p, nil
}

// AllProposals returns all proposal records ordered by created_at descending.
func (g *Graph) AllProposals(ctx context.Context) ([]*ProposalRecord, error) {
	rows, err := g.db.QueryContext(ctx, `
		SELECT id, incident_id, status, proposal_yaml, validation_json, created_by, created_at, promoted_at
		FROM awareness_proposals ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("AllProposals: %w", err)
	}
	defer rows.Close()
	var out []*ProposalRecord
	for rows.Next() {
		var p ProposalRecord
		if err := rows.Scan(&p.ID, &p.IncidentID, &p.Status, &p.ProposalYAML,
			&p.ValidationJSON, &p.CreatedBy, &p.CreatedAt, &p.PromotedAt); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// UpdateProposalStatus sets the status (and promoted_at if PROMOTED) of a proposal.
func (g *Graph) UpdateProposalStatus(ctx context.Context, id, status string) error {
	promotedAt := int64(0)
	if status == ProposalStatusPromoted {
		promotedAt = time.Now().Unix()
	}
	_, err := g.db.ExecContext(ctx, `
		UPDATE awareness_proposals SET status = ?, promoted_at = ? WHERE id = ?
	`, status, promotedAt, id)
	if err != nil {
		return fmt.Errorf("UpdateProposalStatus %s: %w", id, err)
	}
	return nil
}

// UpsertContextAlias inserts or replaces a context alias entry.
func (g *Graph) UpsertContextAlias(ctx context.Context, a ContextAliasRecord) error {
	now := time.Now().Unix()
	if a.CreatedAt == 0 {
		a.CreatedAt = now
	}
	conf := a.Confidence
	if conf == 0 {
		conf = 1.0
	}
	_, err := g.db.ExecContext(ctx, `
		INSERT INTO context_aliases (id, target_id, alias, confidence, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			target_id  = excluded.target_id,
			alias      = excluded.alias,
			confidence = excluded.confidence,
			source     = excluded.source
	`, a.ID, a.TargetID, a.Alias, conf, a.Source, a.CreatedAt)
	if err != nil {
		return fmt.Errorf("UpsertContextAlias %s: %w", a.ID, err)
	}
	return nil
}

// AllContextAliases returns all context alias records.
func (g *Graph) AllContextAliases(ctx context.Context) ([]*ContextAliasRecord, error) {
	rows, err := g.db.QueryContext(ctx, `
		SELECT id, target_id, alias, confidence, source, created_at
		FROM context_aliases ORDER BY target_id, alias
	`)
	if err != nil {
		return nil, fmt.Errorf("AllContextAliases: %w", err)
	}
	defer rows.Close()
	var out []*ContextAliasRecord
	for rows.Next() {
		var a ContextAliasRecord
		if err := rows.Scan(&a.ID, &a.TargetID, &a.Alias, &a.Confidence, &a.Source, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}

// FindInvariant returns an invariant record by ID, or (nil, nil) if not found.
func (g *Graph) FindInvariant(ctx context.Context, id string) (*Invariant, error) {
	var inv Invariant
	var meta string
	err := g.db.QueryRowContext(ctx, `
		SELECT id, title, summary, severity, status, metadata_json
		FROM invariants WHERE id = ?
	`, id).Scan(&inv.ID, &inv.Title, &inv.Summary, &inv.Severity, &inv.Status, &meta)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("FindInvariant %s: %w", id, err)
	}
	return &inv, nil
}

// ---- internal scan helpers ----

func scanNode(row *sql.Row) (*Node, error) {
	var n Node
	var meta string
	err := row.Scan(&n.ID, &n.Type, &n.Name, &n.Path, &n.Summary, &meta, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	n.Metadata = unmarshalMeta(meta)
	return &n, nil
}

func scanNodes(rows *sql.Rows) ([]*Node, error) {
	var out []*Node
	for rows.Next() {
		var n Node
		var meta string
		if err := rows.Scan(&n.ID, &n.Type, &n.Name, &n.Path, &n.Summary, &meta, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		n.Metadata = unmarshalMeta(meta)
		out = append(out, &n)
	}
	return out, rows.Err()
}

func scanEdges(rows *sql.Rows) ([]Edge, error) {
	var out []Edge
	for rows.Next() {
		var e Edge
		var req int
		var meta string
		if err := rows.Scan(&e.Src, &e.Kind, &e.Dst, &e.Phase, &req, &e.Confidence, &meta); err != nil {
			return nil, err
		}
		e.Required = req == 1
		e.Metadata = unmarshalMeta(meta)
		out = append(out, e)
	}
	return out, rows.Err()
}
