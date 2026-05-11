package semanticdiff

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/globulario/services/golang/awareness/graph"
	"github.com/google/uuid"
)

// Store persists and retrieves semantic diff reports.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store backed by the awareness graph.
func NewStore(g *graph.Graph) *Store {
	return &Store{db: g.DB()}
}

// StoreReport persists a full semantic diff report.
func (s *Store) StoreReport(ctx context.Context, r *SemanticDiffReport) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO semantic_diff_reports
		  (id,session_id,diff_source,git_base,git_head,task,verdict,severity,summary,diff_fingerprint,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.SessionID, r.DiffSource, r.GitBase, r.GitHead, r.Task,
		r.Verdict, r.Severity, r.Summary, r.Fingerprint, r.CreatedAt)
	if err != nil {
		return fmt.Errorf("semanticdiff: store report: %w", err)
	}
	for _, f := range r.Findings {
		if err := s.storeFinding(ctx, r.ID, f); err != nil {
			return err
		}
	}
	for _, a := range r.Atoms {
		if err := s.storeAtom(ctx, r.ID, a); err != nil {
			return err
		}
	}
	for _, t := range r.Transitions {
		if err := s.storeTransition(ctx, r.ID, t); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) storeFinding(ctx context.Context, reportID string, f SemanticDiffFinding) error {
	if f.ID == "" {
		f.ID = "FND-" + uuid.New().String()[:8]
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO semantic_diff_findings
		  (id,report_id,kind,severity,file_path,symbol,layer_from,layer_to,authority_from,authority_to,
		   invariant_id,message,evidence,recommendation,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		f.ID, reportID, f.Kind, f.Severity, f.FilePath, f.Symbol,
		f.LayerFrom, f.LayerTo, f.AuthorityFrom, f.AuthorityTo,
		f.InvariantID, f.Message, f.Evidence, f.Recommendation, time.Now().Unix())
	return err
}

func (s *Store) storeAtom(ctx context.Context, reportID string, a SemanticDiffAtom) error {
	if a.ID == "" {
		a.ID = "AT-" + uuid.New().String()[:8]
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO semantic_diff_atoms
		  (id,report_id,file_path,symbol,atom_kind,before_summary,after_summary,confidence,evidence)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		a.ID, reportID, a.FilePath, a.Symbol, a.AtomKind,
		a.BeforeSummary, a.AfterSummary, a.Confidence, a.Evidence)
	return err
}

func (s *Store) storeTransition(ctx context.Context, reportID string, t LayerTransition) error {
	allowed := 0
	if t.Allowed {
		allowed = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO semantic_layer_transitions
		  (id,report_id,file_path,symbol,layer_from,layer_to,transition_kind,allowed,reason)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		"LT-"+uuid.New().String()[:8], reportID, t.FilePath, t.Symbol,
		t.LayerFrom, t.LayerTo, t.TransitionKind, allowed, t.Reason)
	return err
}

// GetReport loads a report and all its findings/atoms by ID.
func (s *Store) GetReport(ctx context.Context, reportID string) (*SemanticDiffReport, error) {
	var r SemanticDiffReport
	err := s.db.QueryRowContext(ctx, `
		SELECT id,session_id,diff_source,git_base,git_head,task,verdict,severity,summary,diff_fingerprint,created_at
		FROM semantic_diff_reports WHERE id=?`, reportID).Scan(
		&r.ID, &r.SessionID, &r.DiffSource, &r.GitBase, &r.GitHead, &r.Task,
		&r.Verdict, &r.Severity, &r.Summary, &r.Fingerprint, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("semantic diff report %s not found", reportID)
	}
	if err != nil {
		return nil, err
	}
	r.Findings, _ = s.loadFindings(ctx, reportID)
	r.Atoms, _ = s.loadAtoms(ctx, reportID)
	r.Transitions, _ = s.loadTransitions(ctx, reportID)
	r.AuthorityChange, r.AuthorityBudget = computeAuthorityBudget(r.Transitions, r.Findings)
	return &r, nil
}

func (s *Store) loadFindings(ctx context.Context, reportID string) ([]SemanticDiffFinding, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,kind,severity,file_path,symbol,layer_from,layer_to,authority_from,authority_to,
		       invariant_id,message,evidence,recommendation
		FROM semantic_diff_findings WHERE report_id=?`, reportID)
	if err != nil {
		return nil, err
	}
	var findings []SemanticDiffFinding
	for rows.Next() {
		var f SemanticDiffFinding
		if err := rows.Scan(&f.ID, &f.Kind, &f.Severity, &f.FilePath, &f.Symbol,
			&f.LayerFrom, &f.LayerTo, &f.AuthorityFrom, &f.AuthorityTo,
			&f.InvariantID, &f.Message, &f.Evidence, &f.Recommendation); err != nil {
			rows.Close()
			return nil, err
		}
		findings = append(findings, f)
	}
	rows.Close()
	return findings, rows.Err()
}

func (s *Store) loadAtoms(ctx context.Context, reportID string) ([]SemanticDiffAtom, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,file_path,symbol,atom_kind,before_summary,after_summary,confidence,evidence
		FROM semantic_diff_atoms WHERE report_id=?`, reportID)
	if err != nil {
		return nil, err
	}
	var atoms []SemanticDiffAtom
	for rows.Next() {
		var a SemanticDiffAtom
		if err := rows.Scan(&a.ID, &a.FilePath, &a.Symbol, &a.AtomKind,
			&a.BeforeSummary, &a.AfterSummary, &a.Confidence, &a.Evidence); err != nil {
			rows.Close()
			return nil, err
		}
		atoms = append(atoms, a)
	}
	rows.Close()
	return atoms, rows.Err()
}

func (s *Store) loadTransitions(ctx context.Context, reportID string) ([]LayerTransition, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT file_path,symbol,layer_from,layer_to,transition_kind,allowed,reason
		FROM semantic_layer_transitions WHERE report_id=?`, reportID)
	if err != nil {
		return nil, err
	}
	var out []LayerTransition
	for rows.Next() {
		var t LayerTransition
		var allowed int
		if err := rows.Scan(&t.FilePath, &t.Symbol, &t.LayerFrom, &t.LayerTo, &t.TransitionKind, &allowed, &t.Reason); err != nil {
			rows.Close()
			return nil, err
		}
		t.Allowed = allowed != 0
		out = append(out, t)
	}
	rows.Close()
	return out, rows.Err()
}

// IsReportStale returns true if the current diff fingerprint doesn't match the report's.
func IsReportStale(report *SemanticDiffReport, currentDiff string) bool {
	if report.Fingerprint == "" || currentDiff == "" {
		return false
	}
	return report.Fingerprint != DiffFingerprint(currentDiff)
}
