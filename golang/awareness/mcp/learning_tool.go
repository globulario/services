package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/awareness/learning"
)

func registerLearningTools(s *Server) {
	registerProposeFromIncidentTool(s)
	registerValidateProposalTool(s)
	registerPendingProposalsTool(s)
	registerApproveProposalTool(s)
	// promote-proposal is intentionally NOT registered over MCP.
	// Promotion must remain a CLI-only operation.
}

func registerProposeFromIncidentTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.propose_from_incident",
		Description: "Generate a DRAFT awareness proposal from an incident bundle. The proposal requires CLI approval and promotion — this tool only generates it. Path traversal outside docs/awareness/proposals is rejected.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"incident_id": {
					Type:        "string",
					Description: "Incident ID to load from docs/awareness/incidents/<id>.yaml",
				},
				"output_name": {
					Type:        "string",
					Description: "Output filename (without path or extension). Defaults to incident_id.",
				},
			},
			Required: []string{"incident_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		incidentID := strArg(args, "incident_id")
		if incidentID == "" {
			return nil, fmt.Errorf("incident_id is required")
		}

		docsDir := s.resolvedDocsDir()
		if docsDir == "" {
			return nil, fmt.Errorf("docs dir not configured")
		}

		// Validate output_name first — before any filesystem access.
		outputName := strArg(args, "output_name")
		if outputName == "" {
			outputName = incidentID
		}
		// Hard reject: output_name must be a plain filename with no path components.
		if filepath.IsAbs(outputName) {
			return nil, fmt.Errorf("output_name must be a plain filename, not an absolute path: %q", outputName)
		}
		if strings.ContainsAny(outputName, "/\\") {
			return nil, fmt.Errorf("output_name must be a plain filename with no directory separators: %q", outputName)
		}
		outputName = strings.TrimSuffix(outputName, ".yaml")
		outputName = strings.TrimSpace(outputName)
		if outputName == "" || outputName == "." || outputName == ".." {
			return nil, fmt.Errorf("output_name is invalid: %q", strArg(args, "output_name"))
		}

		// Load incident bundle.
		bundlePath := filepath.Join(docsDir, "incidents", incidentID+".yaml")
		b, err := learning.LoadIncidentBundle(bundlePath)
		if err != nil {
			return nil, fmt.Errorf("load incident bundle: %w", err)
		}

		// Generate proposal.
		p := learning.GenerateProposalFromBundle(b)

		proposalsDir := filepath.Join(docsDir, "proposals")
		outputPath := filepath.Join(proposalsDir, outputName+".yaml")

		// Path safety: resolve both and confirm output is inside proposals dir.
		absOut, err := filepath.Abs(outputPath)
		if err != nil {
			return nil, fmt.Errorf("resolve output path: %w", err)
		}
		absProposals, err := filepath.Abs(proposalsDir)
		if err != nil {
			return nil, fmt.Errorf("resolve proposals dir: %w", err)
		}
		if !strings.HasPrefix(absOut, absProposals+string(filepath.Separator)) {
			return nil, fmt.Errorf("path traversal rejected: output %q is outside proposals dir", outputPath)
		}

		if err := learning.SaveProposal(outputPath, p); err != nil {
			return nil, fmt.Errorf("save proposal: %w", err)
		}

		return map[string]interface{}{
			"proposal_id":        p.Proposal.ID,
			"proposal_path":      outputPath,
			"status":             p.Proposal.Status,
			"approval_sla_hours": 24,
			"review_deadline":    time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
			"escalation":         "If this proposal is still DRAFT/VALIDATED/NEEDS_REVIEW after the deadline, surface it with awareness.pending_proposals and treat the incident as still open.",
			"note":               "Proposal is DRAFT. Use CLI 'globular awareness validate-proposal' then 'approve-proposal' before promotion.",
		}, nil
	})
}

func registerValidateProposalTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.validate_proposal",
		Description: "Validate a proposal YAML against the 12 awareness admission rules. Returns PASS/FAIL/WARN with per-rule findings. Use strict=true to make missing graph a hard failure.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"file": {
					Type:        "string",
					Description: "Path to the proposal YAML file",
				},
				"strict": {
					Type:        "boolean",
					Description: "If true, missing awareness graph causes FAIL instead of WARN; reference checks (rules 4-9) become required.",
				},
			},
			Required: []string{"file"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		file := strArg(args, "file")
		if file == "" {
			return nil, fmt.Errorf("file is required")
		}
		strict := boolArg(args, "strict")

		p, err := learning.LoadProposalFromFile(file)
		if err != nil {
			return nil, fmt.Errorf("load proposal: %w", err)
		}

		// Guard against nil graph — graph-dependent rules (4-9) cannot run without it.
		if s.g == nil {
			if strict {
				return map[string]interface{}{
					"status":      string(learning.ValidationFail),
					"proposal_id": p.Proposal.ID,
					"findings": []map[string]interface{}{{
						"rule":    0,
						"status":  string(learning.ValidationFail),
						"message": "awareness graph is unavailable; reference checks (rules 4-9) cannot run (strict=true → FAIL)",
					}},
				}, nil
			}
			return map[string]interface{}{
				"status":      "WARN",
				"proposal_id": p.Proposal.ID,
				"findings": []map[string]interface{}{{
					"rule":    0,
					"status":  "WARN",
					"message": "awareness graph is unavailable; reference checks (rules 4-9) skipped",
				}},
			}, nil
		}

		vr, err := learning.ValidateProposal(ctx, p, s.g)
		if err != nil {
			return nil, fmt.Errorf("validate: %w", err)
		}

		findings := make([]map[string]interface{}, 0, len(vr.Findings))
		for _, f := range vr.Findings {
			findings = append(findings, map[string]interface{}{
				"rule":    f.Rule,
				"status":  string(f.Status),
				"message": f.Message,
			})
		}

		return map[string]interface{}{
			"status":      string(vr.Status),
			"proposal_id": p.Proposal.ID,
			"findings":    findings,
		}, nil
	})
}

func registerPendingProposalsTool(s *Server) {
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
		docsDir := s.resolvedDocsDir()
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

func registerApproveProposalTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.approve_proposal",
		Description: "Validate a proposal and mark it APPROVED. Does NOT promote — promotion remains CLI-only. Returns the updated proposal status.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"file": {
					Type:        "string",
					Description: "Path to the proposal YAML file",
				},
			},
			Required: []string{"file"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		file := strArg(args, "file")
		if file == "" {
			return nil, fmt.Errorf("file is required")
		}

		p, err := learning.LoadProposalFromFile(file)
		if err != nil {
			return nil, fmt.Errorf("load proposal: %w", err)
		}

		// Validate first.
		vr, err := learning.ValidateProposal(ctx, p, s.g)
		if err != nil {
			return nil, fmt.Errorf("validate: %w", err)
		}
		if vr.Status == learning.ValidationFail {
			return map[string]interface{}{
				"approved": false,
				"reason":   "validation failed — fix findings before approving",
				"status":   string(vr.Status),
			}, nil
		}

		// Mark APPROVED.
		learning.ApproveProposal(p)
		if err := learning.SaveProposal(file, p); err != nil {
			return nil, fmt.Errorf("save: %w", err)
		}

		return map[string]interface{}{
			"approved":    true,
			"proposal_id": p.Proposal.ID,
			"status":      p.Proposal.Status,
			"note":        "Proposal is now APPROVED. Use CLI 'globular awareness promote-proposal' to promote to docs/awareness.",
			"promoted":    false,
		}, nil
	})
}
