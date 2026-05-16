package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/awareness/learning"
)

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
			p, err := learning.LoadProposalFromFile(path)
			if err != nil {
				continue
			}
			status := p.Proposal.Status
			if status == learning.StatusApproved || status == learning.StatusPromoted || status == learning.StatusRejected || status == learning.StatusSuperseded {
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
	case learning.StatusDraft:
		return "validate proposal"
	case learning.StatusValidated, learning.StatusNeedsReview:
		return "review and approve or reject"
	default:
		return "inspect proposal status"
	}
}
