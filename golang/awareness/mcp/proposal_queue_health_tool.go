package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SLA defaults.
const (
	defaultDraftSLAHours       = 24.0
	defaultValidatedSLAHours   = 24.0
	defaultApprovedSLAHours    = 24.0
)

// ---------------------------------------------------------------------------
// Output types
// ---------------------------------------------------------------------------

type proposalCounts struct {
	Draft            int `json:"draft"`
	Validated        int `json:"validated"`
	NeedsReview      int `json:"needs_review"`
	Approved         int `json:"approved"`
	Promoted         int `json:"promoted"`
	Stale            int `json:"stale"`
}

type staleProposalEntry struct {
	ProposalID string `json:"proposal_id"`
	AgeHours   int    `json:"age_hours"`
	TargetFile string `json:"target_file,omitempty"`
	EntryID    string `json:"entry_id,omitempty"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
}

type proposalQueueResult struct {
	QueueStatus        string               `json:"queue_status"` // healthy | needs_review | stale | blocked
	Counts             proposalCounts       `json:"counts"`
	StaleProposals     []staleProposalEntry `json:"stale_proposals"`
	RecommendedActions []string             `json:"recommended_actions"`
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func registerProposalQueueHealthTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.proposal_queue_health",
		Description: "Report the health of the awareness proposal review queue. Identifies stale DRAFT/VALIDATED/APPROVED proposals that are waiting for human review or promotion. Prevents proposals from accumulating silently without action. Includes recommended actions for each stale proposal.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"draft_sla_hours": {
					Type:        "number",
					Description: "Hours after which a DRAFT proposal is considered stale. Default: 24.",
				},
				"validated_sla_hours": {
					Type:        "number",
					Description: "Hours after which a VALIDATED proposal is considered needs_review. Default: 24.",
				},
				"approved_sla_hours": {
					Type:        "number",
					Description: "Hours after which an APPROVED proposal is considered promotion_pending. Default: 24.",
				},
				"include_promoted": {
					Type:        "boolean",
					Description: "If true, include PROMOTED proposals in counts. Default: false.",
					Default:     false,
				},
			},
			Required: []string{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		_ = ctx

		docsDir := s.resolvedDocsDir()
		if docsDir == "" {
			return nil, fmt.Errorf("docs dir not configured")
		}

		draftSLA := defaultDraftSLAHours
		validatedSLA := defaultValidatedSLAHours
		approvedSLA := defaultApprovedSLAHours
		if v, ok := args["draft_sla_hours"].(float64); ok && v > 0 {
			draftSLA = v
		}
		if v, ok := args["validated_sla_hours"].(float64); ok && v > 0 {
			validatedSLA = v
		}
		if v, ok := args["approved_sla_hours"].(float64); ok && v > 0 {
			approvedSLA = v
		}
		includePromoted := false
		if v, ok := args["include_promoted"].(bool); ok {
			includePromoted = v
		}

		proposalsDir := filepath.Join(docsDir, "proposals")
		entries, err := os.ReadDir(proposalsDir)
		if err != nil {
			// No proposals directory → healthy (empty queue).
			return &proposalQueueResult{
				QueueStatus:        "healthy",
				RecommendedActions: []string{"No proposals directory found — queue is empty."},
			}, nil
		}

		now := time.Now()
		counts := proposalCounts{}
		var staleEntries []staleProposalEntry
		var actions []string
		seenIDs := make(map[string]bool)
		var duplicates []string

		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			path := filepath.Join(proposalsDir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			var raw struct {
				Proposal struct {
					ID        string `yaml:"id"`
					Status    string `yaml:"status"`
					CreatedAt string `yaml:"created_at"`
				} `yaml:"proposal"`
				FailureModes []struct {
					ID string `yaml:"id"`
				} `yaml:"failure_modes"`
			}
			if err := yaml.Unmarshal(data, &raw); err != nil {
				continue
			}

			propID := raw.Proposal.ID
			status := strings.ToUpper(raw.Proposal.Status)

			// Duplicate detection.
			if seenIDs[propID] {
				duplicates = append(duplicates, propID)
			}
			seenIDs[propID] = true

			age := proposalAge(raw.Proposal.CreatedAt, path, now)
			ageHours := int(age.Hours())

			// Extract first failure mode ID as entry_id hint.
			entryID := ""
			if len(raw.FailureModes) > 0 {
				entryID = raw.FailureModes[0].ID
			}

			switch status {
			case "DRAFT":
				counts.Draft++
				if age.Hours() > draftSLA {
					counts.Stale++
					staleEntries = append(staleEntries, staleProposalEntry{
						ProposalID: propID,
						AgeHours:   ageHours,
						EntryID:    entryID,
						Status:     "DRAFT",
						Reason:     fmt.Sprintf("DRAFT older than %.0fh SLA — needs validation", draftSLA),
					})
				}
			case "VALIDATED":
				counts.Validated++
				if age.Hours() > validatedSLA {
					counts.Stale++
					staleEntries = append(staleEntries, staleProposalEntry{
						ProposalID: propID,
						AgeHours:   ageHours,
						EntryID:    entryID,
						Status:     "VALIDATED",
						Reason:     fmt.Sprintf("VALIDATED older than %.0fh SLA — needs human review and approval", validatedSLA),
					})
				}
			case "NEEDS_REVIEW":
				counts.NeedsReview++
				counts.Stale++
				staleEntries = append(staleEntries, staleProposalEntry{
					ProposalID: propID,
					AgeHours:   ageHours,
					EntryID:    entryID,
					Status:     "NEEDS_REVIEW",
					Reason:     "Proposal is in NEEDS_REVIEW state — operator action required",
				})
			case "APPROVED":
				counts.Approved++
				if age.Hours() > approvedSLA {
					counts.Stale++
					staleEntries = append(staleEntries, staleProposalEntry{
						ProposalID: propID,
						AgeHours:   ageHours,
						EntryID:    entryID,
						Status:     "APPROVED",
						Reason:     fmt.Sprintf("APPROVED older than %.0fh SLA — not yet promoted to knowledge YAML", approvedSLA),
					})
				}
			case "PROMOTED":
				if includePromoted {
					counts.Promoted++
				}
			}
		}

		// Build recommended actions.
		for _, s := range staleEntries {
			switch s.Status {
			case "DRAFT":
				actions = append(actions, fmt.Sprintf("Validate proposal %s (age %dh) — run awareness.validate_proposal or review YAML manually.", s.ProposalID, s.AgeHours))
			case "VALIDATED":
				actions = append(actions, fmt.Sprintf("Approve or reject proposal %s (age %dh) — this requires operator review.", s.ProposalID, s.AgeHours))
			case "APPROVED":
				actions = append(actions, fmt.Sprintf("Promote approved proposal %s (age %dh) — run awareness.approve_proposal with promote=true.", s.ProposalID, s.AgeHours))
			case "NEEDS_REVIEW":
				actions = append(actions, fmt.Sprintf("Review proposal %s immediately — it is flagged NEEDS_REVIEW.", s.ProposalID))
			}
		}
		for _, dup := range duplicates {
			actions = append(actions, fmt.Sprintf("Duplicate proposal ID detected: %q — merge or delete one copy.", dup))
		}
		if len(actions) == 0 {
			actions = append(actions, "Proposal queue is healthy — no stale or blocked proposals.")
		}

		// Overall queue status.
		queueStatus := computeQueueStatus(counts, len(staleEntries), len(duplicates))

		return &proposalQueueResult{
			QueueStatus:        queueStatus,
			Counts:             counts,
			StaleProposals:     staleEntries,
			RecommendedActions: actions,
		}, nil
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func computeQueueStatus(counts proposalCounts, staleCount, dupCount int) string {
	if dupCount > 0 {
		return "blocked"
	}
	if counts.NeedsReview > 0 {
		return "needs_review"
	}
	if staleCount > 0 {
		return "stale"
	}
	return "healthy"
}
