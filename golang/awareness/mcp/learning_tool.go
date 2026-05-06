package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/awareness/learning"
)

func registerLearningTools(s *Server) {
	registerProposeFromIncidentTool(s)
	registerValidateProposalTool(s)
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
			"proposal_id":   p.Proposal.ID,
			"proposal_path": outputPath,
			"status":        p.Proposal.Status,
			"note":          "Proposal is DRAFT. Use CLI 'globular awareness validate-proposal' then 'approve-proposal' before promotion.",
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
			"approved":     true,
			"proposal_id":  p.Proposal.ID,
			"status":       p.Proposal.Status,
			"note":         "Proposal is now APPROVED. Use CLI 'globular awareness promote-proposal' to promote to docs/awareness.",
			"promoted":     false,
		}, nil
	})
}
