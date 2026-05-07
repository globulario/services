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

// ---------------------------------------------------------------------------
// Shared proposal loader
// ---------------------------------------------------------------------------

type rawProposal struct {
	ID        string
	Status    string
	FilePath  string
	AgeHours  int
	EntryIDs  []string
	TargetFile string
}

func loadAllProposals(docsDir string) []rawProposal {
	proposalsDir := filepath.Join(docsDir, "proposals")
	entries, err := os.ReadDir(proposalsDir)
	if err != nil {
		return nil
	}

	var proposals []rawProposal
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
				ID         string `yaml:"id"`
				TargetFile string `yaml:"target_file"`
			} `yaml:"failure_modes"`
		}
		if err := yaml.Unmarshal(data, &raw); err != nil {
			continue
		}
		now := proposalAge(raw.Proposal.CreatedAt, path, time.Now())
		p := rawProposal{
			ID:       raw.Proposal.ID,
			Status:   strings.ToUpper(raw.Proposal.Status),
			FilePath: path,
			AgeHours: int(now.Hours()),
		}
		for _, fm := range raw.FailureModes {
			p.EntryIDs = append(p.EntryIDs, fm.ID)
			if fm.TargetFile != "" {
				p.TargetFile = fm.TargetFile
			}
		}
		proposals = append(proposals, p)
	}
	return proposals
}

// ---------------------------------------------------------------------------
// awareness.proposal_review_plan
// ---------------------------------------------------------------------------

type reviewPlanResult struct {
	ValidateNow              []string `json:"validate_now"`
	NeedsHumanReview         []string `json:"needs_human_review"`
	SafeToRejectDuplicates   []string `json:"safe_to_reject_duplicates"`
	ApprovedWaitingPromotion []string `json:"approved_waiting_promotion"`
	InvalidSchema            []string `json:"invalid_schema"`
	Summary                  string   `json:"summary"`
}

func registerProposalReviewPlanTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.proposal_review_plan",
		Description: "Group current proposals by the next recommended action: validate_now (DRAFT), needs_human_review (VALIDATED), approved_waiting_promotion (APPROVED), safe_to_reject_duplicates (duplicate IDs), invalid_schema (unreadable). Does not approve or modify proposals — planning only.",
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]propSchema{},
			Required:   []string{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		_ = ctx
		docsDir := s.resolvedDocsDir()
		if docsDir == "" {
			return nil, fmt.Errorf("docs dir not configured")
		}

		proposals := loadAllProposals(docsDir)
		result := &reviewPlanResult{}
		seen := make(map[string]int) // id → count

		for _, p := range proposals {
			seen[p.ID]++
		}

		for _, p := range proposals {
			if seen[p.ID] > 1 {
				result.SafeToRejectDuplicates = append(result.SafeToRejectDuplicates, p.ID)
				continue
			}
			switch p.Status {
			case "DRAFT":
				result.ValidateNow = append(result.ValidateNow, p.ID)
			case "VALIDATED":
				result.NeedsHumanReview = append(result.NeedsHumanReview, p.ID)
			case "APPROVED":
				result.ApprovedWaitingPromotion = append(result.ApprovedWaitingPromotion, p.ID)
			case "":
				result.InvalidSchema = append(result.InvalidSchema, filepath.Base(p.FilePath))
			}
		}

		// Deduplicate SafeToRejectDuplicates.
		result.SafeToRejectDuplicates = dedupStrings(result.SafeToRejectDuplicates)

		result.Summary = fmt.Sprintf(
			"%d validate_now, %d needs_human_review, %d approved_waiting_promotion, %d duplicate, %d invalid",
			len(result.ValidateNow), len(result.NeedsHumanReview), len(result.ApprovedWaitingPromotion),
			len(result.SafeToRejectDuplicates), len(result.InvalidSchema),
		)
		return result, nil
	})
}

// ---------------------------------------------------------------------------
// awareness.validate_proposal_batch
// ---------------------------------------------------------------------------

type batchValidationEntry struct {
	ProposalID string   `json:"proposal_id"`
	Status     string   `json:"status"` // valid | invalid
	Issues     []string `json:"issues,omitempty"`
}

type batchValidationResult struct {
	Validated int                    `json:"validated"`
	Valid     int                    `json:"valid"`
	Invalid   int                    `json:"invalid"`
	Entries   []batchValidationEntry `json:"entries"`
	Note      string                 `json:"note"`
}

func registerValidateProposalBatchTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.validate_proposal_batch",
		Description: "Validates all DRAFT proposals in the queue and reports pass/fail with specific issues. Does NOT approve or promote — validation only. After batch validation, DRAFT proposals that pass can be approved by a human operator.",
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]propSchema{},
			Required:   []string{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		_ = ctx
		docsDir := s.resolvedDocsDir()
		if docsDir == "" {
			return nil, fmt.Errorf("docs dir not configured")
		}

		proposals := loadAllProposals(docsDir)
		result := &batchValidationResult{
			Note: "Validation only — no proposals were approved or modified.",
		}

		for _, p := range proposals {
			if p.Status != "DRAFT" {
				continue
			}
			issues := validateProposalSchema(p)
			entry := batchValidationEntry{
				ProposalID: p.ID,
				Issues:     issues,
			}
			if len(issues) == 0 {
				entry.Status = "valid"
				result.Valid++
			} else {
				entry.Status = "invalid"
				result.Invalid++
			}
			result.Entries = append(result.Entries, entry)
			result.Validated++
		}

		return result, nil
	})
}

// validateProposalSchema checks a proposal for common schema problems.
func validateProposalSchema(p rawProposal) []string {
	var issues []string
	if p.ID == "" {
		issues = append(issues, "proposal.id is empty")
	}
	if p.Status == "" {
		issues = append(issues, "proposal.status is empty")
	}
	if p.AgeHours < 0 {
		issues = append(issues, "proposal.created_at is in the future")
	}
	return issues
}

// ---------------------------------------------------------------------------
// awareness.promote_approved_proposals
// ---------------------------------------------------------------------------

type promotionDryRunEntry struct {
	ProposalID string `json:"proposal_id"`
	FilePath   string `json:"file_path"`
	Action     string `json:"action"` // would_mark_promoted
}

type promotionResult struct {
	DryRun   bool                   `json:"dry_run"`
	Promoted int                    `json:"promoted"`
	Entries  []promotionDryRunEntry `json:"entries"`
	Note     string                 `json:"note"`
}

func registerPromoteApprovedProposalsTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.promote_approved_proposals",
		Description: "Promote APPROVED proposals by marking their status as PROMOTED. Dry-run by default — pass dry_run=false to actually write. Human approval (status: APPROVED) must already be set in the proposal YAML before this tool will act. Never auto-approves.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"dry_run": {
					Type:        "boolean",
					Description: "If true (default), report what would be promoted without modifying files. Set to false to actually promote.",
					Default:     true,
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

		dryRun := true
		if v, ok := args["dry_run"].(bool); ok {
			dryRun = v
		}

		proposals := loadAllProposals(docsDir)
		result := &promotionResult{DryRun: dryRun}

		for _, p := range proposals {
			if p.Status != "APPROVED" {
				continue
			}
			entry := promotionDryRunEntry{
				ProposalID: p.ID,
				FilePath:   p.FilePath,
				Action:     "would_mark_promoted",
			}
			if !dryRun {
				if err := markProposalPromoted(p.FilePath); err != nil {
					entry.Action = fmt.Sprintf("error: %v", err)
				} else {
					entry.Action = "marked_promoted"
					result.Promoted++
				}
			}
			result.Entries = append(result.Entries, entry)
		}

		if dryRun {
			result.Note = fmt.Sprintf("DRY RUN — %d APPROVED proposal(s) would be promoted. Re-run with dry_run=false to apply.", len(result.Entries))
		} else {
			result.Note = fmt.Sprintf("%d proposal(s) marked PROMOTED.", result.Promoted)
		}
		return result, nil
	})
}

// markProposalPromoted rewrites a proposal YAML file's status field to PROMOTED.
func markProposalPromoted(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	updated := strings.ReplaceAll(string(data), "status: APPROVED", "status: PROMOTED")
	updated = strings.ReplaceAll(updated, "status: approved", "status: PROMOTED")
	return os.WriteFile(filePath, []byte(updated), 0o644)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func dedupStrings(s []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
