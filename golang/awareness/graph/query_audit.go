package graph

import (
	"context"
	"errors"
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

// AgentUsageEvent is a single recorded preflight/agent-context call.
type AgentUsageEvent struct {
	ID                string `json:"id"`
	EventTime         int64  `json:"event_time,omitempty"`
	Agent             string `json:"agent,omitempty"`
	SessionIDHash     string `json:"session_id_hash,omitempty"`
	Repo              string `json:"repo,omitempty"`
	Tool              string `json:"tool,omitempty"`
	Operation         string `json:"operation,omitempty"`
	ResultStatus      string `json:"result_status,omitempty"`
	Confidence        string `json:"confidence,omitempty"`
	TaskType          string `json:"task_type,omitempty"`
	ChangedFilesCount int    `json:"changed_files_count,omitempty"`
}

// RecordAgentUsage inserts a usage event. Raw prompts are never stored.
func (g *Graph) RecordAgentUsage(ctx context.Context, e AgentUsageEvent) error {
	if g.readOnly {
		return fmt.Errorf("RecordAgentUsage: graph is read-only")
	}
	if e.ID == "" {
		return errors.New("RecordAgentUsage: id required")
	}
	if e.EventTime == 0 {
		e.EventTime = time.Now().Unix()
	}

	g.usageMu.Lock()
	if _, exists := g.usageEvents[e.ID]; !exists {
		cp := e
		g.usageEvents[e.ID] = &cp
	}
	g.usageMu.Unlock()
	return nil
}

// AgentUsageSummary holds aggregate usage stats over a time window.
type AgentUsageSummary struct {
	WindowDays                   int     `json:"window_days"`
	SessionsTotal                int     `json:"sessions_total"`
	PreflightCalls               int     `json:"preflight_calls"`
	AgentContextCalls            int     `json:"agent_context_calls"`
	ScanViolationsCalls          int     `json:"scan_violations_calls"`
	PreEditContextCalls          int     `json:"pre_edit_context_calls"`
	CommitsWithoutIntegrityCheck int     `json:"commits_without_integrity_check"`
	PreflightSkipRatePct         float64 `json:"preflight_skip_rate_pct"`
	Status                       string  `json:"status"`
	RecommendedAction            string  `json:"recommended_action,omitempty"`
}

// QueryAgentUsageSummary returns aggregate usage stats for a rolling window.
func (g *Graph) QueryAgentUsageSummary(ctx context.Context, windowDays int) (*AgentUsageSummary, error) {
	since := time.Now().AddDate(0, 0, -windowDays).Unix()

	g.usageMu.RLock()
	s := &AgentUsageSummary{WindowDays: windowDays}
	sessions := map[string]bool{}
	for _, e := range g.usageEvents {
		if e.EventTime < since {
			continue
		}
		if e.SessionIDHash != "" {
			sessions[e.SessionIDHash] = true
		}
		if e.Operation == "called" {
			switch e.Tool {
			case "awareness.preflight":
				s.PreflightCalls++
			case "awareness.agent_context":
				s.AgentContextCalls++
			case "awareness.scan_violations":
				s.ScanViolationsCalls++
			case "awareness.pre_edit_context":
				s.PreEditContextCalls++
			}
		}
		if e.Tool == "commit.graph_integrity" && e.Operation == "skipped" {
			s.CommitsWithoutIntegrityCheck++
		}
	}
	g.usageMu.RUnlock()

	s.SessionsTotal = len(sessions)
	if s.SessionsTotal > 0 {
		s.PreflightSkipRatePct = (1 - float64(s.PreflightCalls)/float64(s.SessionsTotal)) * 100
		if s.PreflightSkipRatePct < 0 {
			s.PreflightSkipRatePct = 0
		}
	}

	switch {
	case s.SessionsTotal == 0:
		s.Status = "no_data"
		s.RecommendedAction = "Configure session-start hook to call awareness.agent_context"
	case s.CommitsWithoutIntegrityCheck > 0:
		s.Status = "warning"
		s.RecommendedAction = fmt.Sprintf("%d commits bypassed graph integrity check — run awareness.graph_integrity_check before committing", s.CommitsWithoutIntegrityCheck)
	case s.PreflightSkipRatePct > 50:
		s.Status = "warning"
		s.RecommendedAction = "Configure session-start hook to call awareness.agent_context — skip rate is high"
	default:
		s.Status = "ok"
	}

	return s, nil
}
