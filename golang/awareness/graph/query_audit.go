package graph

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// PreflightAuditRecord is a durable record of one preflight run.
type PreflightAuditRecord struct {
	ID             string   `json:"id"`
	Task           string   `json:"task,omitempty"`
	Timestamp      int64    `json:"timestamp,omitempty"`
	GitSHA         string   `json:"git_sha,omitempty"`
	Files          []string `json:"files,omitempty"`
	ForbiddenFixes []string `json:"forbidden_fixes,omitempty"`
	Invariants     []string `json:"invariants,omitempty"`
	CodeSmells     []string `json:"code_smells,omitempty"`
	CreatedAt      int64    `json:"created_at,omitempty"`
}

// InsertPreflightAudit inserts a durable preflight audit record.
func (g *Graph) InsertPreflightAudit(ctx context.Context, r PreflightAuditRecord) error {
	if g.readOnly {
		return fmt.Errorf("InsertPreflightAudit: graph is read-only")
	}
	if r.ID == "" {
		r.ID = fmt.Sprintf("preflight-audit-%d", time.Now().UnixNano())
	}
	now := time.Now().Unix()
	if r.Timestamp == 0 {
		r.Timestamp = now
	}
	if r.CreatedAt == 0 {
		r.CreatedAt = now
	}

	g.preflightMu.Lock()
	replaced := false
	for i, existing := range g.preflights {
		if existing.ID == r.ID {
			cp := r
			g.preflights[i] = &cp
			replaced = true
			break
		}
	}
	if !replaced {
		cp := r
		g.preflights = append(g.preflights, &cp)
	}
	g.preflightMu.Unlock()

	return g.writeJSON("preflight_audits", r.ID, &r)
}

// QueryPreflightAudits returns audit records filtered by since (unix timestamp,
// 0 = no bound) and gitSHA (empty = no filter), ordered by timestamp descending.
func (g *Graph) QueryPreflightAudits(ctx context.Context, since int64, gitSHA string) ([]*PreflightAuditRecord, error) {
	g.preflightMu.RLock()
	var out []*PreflightAuditRecord
	for _, r := range g.preflights {
		if r.Timestamp < since {
			continue
		}
		if gitSHA != "" && r.GitSHA != gitSHA {
			continue
		}
		cp := *r
		out = append(out, &cp)
	}
	g.preflightMu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp > out[j].Timestamp })
	return out, nil
}

