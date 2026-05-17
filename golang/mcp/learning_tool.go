package main

// learning_tool.go — pending_proposals MCP tool.
//
// The broader learning/failurelearning API (LoadProposalFromFile, StatusApproved, etc.)
// was removed from the standalone awareness module during Phase 3. Proposal-file loading
// is inlined here using a minimal struct; the status constants are local copies.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ── Inline proposal types (formerly in awareness/learning/failurelearning) ───

const (
	learningStatusDraft       = "draft"
	learningStatusValidated   = "validated"
	learningStatusNeedsReview = "needs_review"
	learningStatusApproved    = "approved"
	learningStatusPromoted    = "promoted"
	learningStatusRejected    = "rejected"
	learningStatusSuperseded  = "superseded"
)

// minimalProposalFile is just enough to read proposal status, identity, and counts.
type minimalProposalFile struct {
	LearnSource string `yaml:"learn_source"`
	Proposal    struct {
		ID             string `yaml:"id"`
		SourceIncident string `yaml:"source_incident"`
		Status         string `yaml:"status"`
		CreatedAt      string `yaml:"created_at"`
	} `yaml:"proposal"`
	FailureModes   []map[string]interface{} `yaml:"failure_modes"`
	Invariants     []map[string]interface{} `yaml:"invariants"`
	ForbiddenFixes []map[string]interface{} `yaml:"forbidden_fixes"`
}

func loadMinimalProposalFromFile(path string) (*minimalProposalFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read proposal: %w", err)
	}
	var p minimalProposalFile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse proposal: %w", err)
	}
	return &p, nil
}

// ── Tool registration ─────────────────────────────────────────────────────────

func registerPendingProposalsTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.pending_proposals",
		Description: "List awareness proposals still waiting for approval or promotion, with SLA age and escalation status.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"sla_hours": {
					Type:        "number",
					Description: "Age in hours before a pending proposal is considered overdue. Defaults to 24.",
					Default:     24,
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		_ = ctx
		docsDir := st.docsDir
		if docsDir == "" {
			return nil, fmt.Errorf("docs dir not configured")
		}
		slaHours := 24.0
		if v, ok := args["sla_hours"].(float64); ok && v > 0 {
			slaHours = v
		}
		proposalsDir := filepath.Join(docsDir, "proposals")
		entries, err := os.ReadDir(proposalsDir)
		if err != nil {
			if os.IsNotExist(err) {
				return map[string]interface{}{"pending": []map[string]interface{}{}, "overdue_count": 0}, nil
			}
			return nil, fmt.Errorf("read proposals dir: %w", err)
		}

		now := time.Now().UTC()
		pending := make([]map[string]interface{}, 0)
		overdueCount := 0
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			path := filepath.Join(proposalsDir, e.Name())
			p, err := loadMinimalProposalFromFile(path)
			if err != nil {
				continue
			}
			status := p.Proposal.Status
			if status == learningStatusApproved || status == learningStatusPromoted ||
				status == learningStatusRejected || status == learningStatusSuperseded {
				continue
			}
			createdAt := now
			if p.Proposal.CreatedAt != "" {
				if parsed, err := time.Parse(time.RFC3339, p.Proposal.CreatedAt); err == nil {
					createdAt = parsed.UTC()
				}
			} else if info, err := e.Info(); err == nil {
				createdAt = info.ModTime().UTC()
			}
			ageHours := now.Sub(createdAt).Hours()
			overdue := ageHours >= slaHours
			if overdue {
				overdueCount++
			}
			pending = append(pending, map[string]interface{}{
				"proposal_id":     p.Proposal.ID,
				"source_incident": p.Proposal.SourceIncident,
				"file":            path,
				"status":          status,
				"created_at":      createdAt.Format(time.RFC3339),
				"age_hours":       ageHours,
				"overdue":         overdue,
				"next_action":     pendingProposalNextAction(status, overdue),
			})
		}
		return map[string]interface{}{
			"pending":       pending,
			"overdue_count": overdueCount,
			"sla_hours":     slaHours,
		}, nil
	})
}

func pendingProposalNextAction(status string, overdue bool) string {
	if overdue {
		return "ESCALATE_REVIEW: incident remains open until proposal is approved/promoted or rejected"
	}
	switch status {
	case learningStatusDraft:
		return "validate proposal"
	case learningStatusValidated, learningStatusNeedsReview:
		return "review and approve or reject"
	default:
		return "inspect proposal status"
	}
}
