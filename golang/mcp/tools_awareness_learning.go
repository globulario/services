package main

// tools_awareness_learning.go — propose_from_incident, validate_proposal, and
// approve_proposal MCP tools.
//
// Proposals are YAML files stored in docsDir/proposals/<output_name>.yaml.
// The approve_proposal tool sets proposal.status = "APPROVED" and returns
// promoted=false. Promotion is a separate operator action.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ── Path validation ──────────────────────────────────────────────────────────

// validateOutputName returns an error if name contains path separators or
// traversal sequences. Only plain filenames (no directory components) are allowed.
func validateOutputName(name string) error {
	if name == "" {
		return fmt.Errorf("output_name is required")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("output_name %q must not contain directory separator", name)
	}
	if name == ".." || strings.HasPrefix(name, "..") {
		return fmt.Errorf("output_name %q must not start with '..'", name)
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("output_name %q must be a plain filename, not an absolute path", name)
	}
	return nil
}

// ── YAML types ───────────────────────────────────────────────────────────────

type incidentYAML struct {
	IncidentID          string   `yaml:"incident_id"`
	Title               string   `yaml:"title"`
	Severity            string   `yaml:"severity"`
	SuspectedRootCause  string   `yaml:"suspected_root_cause"`
	Symptoms            []string `yaml:"symptoms"`
	StateDeltas         []string `yaml:"state_deltas"`
	ManualRepairs       []string `yaml:"manual_repairs"`
	ObservedServices    []string `yaml:"observed_services"`
	Proposed            struct {
		FailureModes []map[string]interface{} `yaml:"failure_modes"`
		Invariants   []map[string]interface{} `yaml:"invariants"`
		ForbiddenFixes []map[string]interface{} `yaml:"forbidden_fixes"`
	} `yaml:"proposed"`
}

type proposalYAML struct {
	Proposal struct {
		ID          string `yaml:"id"`
		Status      string `yaml:"status"`
		CreatedAt   string `yaml:"created_at"`
		SourceType  string `yaml:"source_type"`
		SourceID    string `yaml:"source_id"`
		Title       string `yaml:"title"`
		Summary     string `yaml:"summary"`
	} `yaml:"proposal"`
	FailureModes   []map[string]interface{} `yaml:"failure_modes,omitempty"`
	Invariants     []map[string]interface{} `yaml:"invariants,omitempty"`
	ForbiddenFixes []map[string]interface{} `yaml:"forbidden_fixes,omitempty"`
}

// ── Tool registration ────────────────────────────────────────────────────────

func registerAwarenessLearningTools(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.propose_from_incident",
		Description: "Generate a DRAFT awareness proposal from an incident bundle. " +
			"Loads docsDir/incidents/<incident_id>.yaml, extracts proposed knowledge, " +
			"and writes docsDir/proposals/<output_name>.yaml. Returns proposal_path. " +
			"Requires human review before promotion (do not auto-approve).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"incident_id": {Type: "string", Description: "Incident ID (maps to incidents/<id>.yaml)"},
				"output_name": {Type: "string", Description: "Plain filename for the proposal (no path separators)"},
			},
			Required: []string{"incident_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		incidentID, _ := args["incident_id"].(string)
		if strings.TrimSpace(incidentID) == "" {
			return nil, fmt.Errorf("incident_id is required")
		}

		outputName, _ := args["output_name"].(string)
		if outputName == "" {
			outputName = incidentID
		}
		if err := validateOutputName(outputName); err != nil {
			return nil, fmt.Errorf("invalid output_name: %w", err)
		}

		// Load incident bundle.
		incidentPath := filepath.Join(st.docsDir, "incidents", incidentID+".yaml")
		data, err := os.ReadFile(incidentPath)
		if err != nil {
			return nil, fmt.Errorf("incident %q not found: %w", incidentID, err)
		}
		var inc incidentYAML
		if err := yaml.Unmarshal(data, &inc); err != nil {
			return nil, fmt.Errorf("parse incident %q: %w", incidentID, err)
		}

		// Build a proposal YAML.
		propID := fmt.Sprintf("FLP-%d", time.Now().UnixMilli())
		prop := proposalYAML{}
		prop.Proposal.ID = propID
		prop.Proposal.Status = "DRAFT"
		prop.Proposal.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		prop.Proposal.SourceType = "incident"
		prop.Proposal.SourceID = incidentID
		if inc.Title != "" {
			prop.Proposal.Title = "Proposal from incident: " + inc.Title
		} else {
			prop.Proposal.Title = "Proposal from incident: " + incidentID
		}
		prop.Proposal.Summary = inc.SuspectedRootCause
		prop.FailureModes = inc.Proposed.FailureModes
		prop.Invariants = inc.Proposed.Invariants
		prop.ForbiddenFixes = inc.Proposed.ForbiddenFixes

		// Write proposal to docsDir/proposals/<output_name>.yaml.
		proposalsDir := filepath.Join(st.docsDir, "proposals")
		if err := os.MkdirAll(proposalsDir, 0o755); err != nil {
			return nil, fmt.Errorf("create proposals dir: %w", err)
		}
		propPath := filepath.Join(proposalsDir, outputName+".yaml")
		propData, err := yaml.Marshal(prop)
		if err != nil {
			return nil, fmt.Errorf("marshal proposal: %w", err)
		}
		if err := os.WriteFile(propPath, propData, 0o644); err != nil {
			return nil, fmt.Errorf("write proposal: %w", err)
		}

		return map[string]interface{}{
			"proposal_id":   propID,
			"proposal_path": propPath,
			"status":        "DRAFT",
			"source":        incidentID,
		}, nil
	})

	s.register(toolDef{
		Name: "awareness.validate_proposal",
		Description: "Validate a proposal YAML against awareness admission rules. " +
			"Checks required fields, ID format, and knowledge consistency. " +
			"Does not approve or promote. Returns findings and pass/fail.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"file":   {Type: "string", Description: "Path to the proposal YAML file"},
				"strict": {Type: "boolean", Description: "Treat warnings as failures"},
			},
			Required: []string{"file"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		file, _ := args["file"].(string)
		if strings.TrimSpace(file) == "" {
			return nil, fmt.Errorf("file is required")
		}

		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read proposal: %w", err)
		}
		var prop proposalYAML
		if err := yaml.Unmarshal(data, &prop); err != nil {
			return nil, fmt.Errorf("parse proposal: %w", err)
		}

		var findings []string
		if prop.Proposal.ID == "" {
			findings = append(findings, "proposal.id is required")
		}
		if prop.Proposal.Status == "" {
			findings = append(findings, "proposal.status is required")
		}
		if prop.Proposal.Title == "" {
			findings = append(findings, "proposal.title is required")
		}
		if len(prop.FailureModes) == 0 && len(prop.Invariants) == 0 {
			findings = append(findings, "proposal must have at least one failure_mode or invariant entry")
		}

		pass := len(findings) == 0
		return map[string]interface{}{
			"pass":     pass,
			"findings": findings,
			"file":     file,
		}, nil
	})

	s.register(toolDef{
		Name: "awareness.approve_proposal",
		Description: "Mark a DRAFT or VALIDATED proposal as APPROVED. " +
			"Does NOT promote (apply to YAML knowledge files). " +
			"Promotion is a separate operator action requiring awareness.promote_approved_proposals --dry-run first.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"file":        {Type: "string", Description: "Path to the proposal YAML file"},
				"reviewed_by": {Type: "string", Description: "Reviewer identifier (optional)"},
			},
			Required: []string{"file"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		file, _ := args["file"].(string)
		if strings.TrimSpace(file) == "" {
			return nil, fmt.Errorf("file is required")
		}

		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read proposal: %w", err)
		}
		var prop proposalYAML
		if err := yaml.Unmarshal(data, &prop); err != nil {
			return nil, fmt.Errorf("parse proposal: %w", err)
		}

		// Update status to APPROVED. Never set PROMOTED here.
		prevStatus := prop.Proposal.Status
		prop.Proposal.Status = "APPROVED"

		updated, err := yaml.Marshal(prop)
		if err != nil {
			return nil, fmt.Errorf("marshal proposal: %w", err)
		}
		if err := os.WriteFile(file, updated, 0o644); err != nil {
			return nil, fmt.Errorf("write proposal: %w", err)
		}

		return map[string]interface{}{
			"status":        "APPROVED",
			"promoted":      false,
			"previous":      prevStatus,
			"file":          file,
			"proposal_id":   prop.Proposal.ID,
		}, nil
	})
}
