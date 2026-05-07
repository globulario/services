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
// Output types
// ---------------------------------------------------------------------------

type candidateCausalRule struct {
	ProposalID           string   `json:"proposal_id"`
	RootSignal           string   `json:"root_signal"`
	Sequence             []string `json:"sequence"`
	Confidence           string   `json:"confidence"` // low | medium (never high without human confirmation)
	EvidenceCount        int      `json:"evidence_count"`
	RequiresHumanApproval bool    `json:"requires_human_approval"`
	YAMLPatch            string   `json:"yaml_patch,omitempty"`
}

type suggestCausalRuleResult struct {
	ExistingRuleMatch bool                  `json:"existing_rule_match"`
	ExistingRuleID    string                `json:"existing_rule_id,omitempty"`
	CandidateRules    []candidateCausalRule `json:"candidate_rules"`
	Warnings          []string              `json:"warnings"`
	Confidence        string                `json:"confidence"`
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func registerSuggestCausalRuleTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.suggest_causal_rule",
		Description: "Given an ordered sequence of events or symptoms (from offline_diagnose, a timeline, or repeated incidents), check whether an existing causal rule already matches. If not, and if the evidence meets the minimum repetition threshold, propose a DRAFT causal rule for human review. Never auto-applies rules. Correlation is not causation — all proposals require operator approval.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"events": {
					Type:        "array",
					Description: "Ordered list of event strings or symptom descriptions forming the suspected causal sequence.",
					Items:       &propSchema{Type: "string"},
				},
				"offline_diagnosis_id": {
					Type:        "string",
					Description: "Optional ID of a prior offline_diagnose call whose timeline provides the evidence.",
				},
				"incident_ids": {
					Type:        "array",
					Description: "Optional list of incident IDs where this symptom sequence was observed.",
					Items:       &propSchema{Type: "string"},
				},
				"min_repetitions": {
					Type:        "number",
					Description: "Minimum number of times the sequence must have been observed before proposing a rule. Default: 2. Set to 1 to propose from a single observation.",
					Default:     2,
				},
				"require_same_order": {
					Type:        "boolean",
					Description: "If true, events must appear in the given order for a rule to be proposed. Default: true.",
					Default:     true,
				},
				"save_proposal": {
					Type:        "boolean",
					Description: "If true, save the candidate rule as a DRAFT proposal YAML. Default: false.",
					Default:     false,
				},
			},
			Required: []string{"events"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		_ = ctx

		events := strSliceArg(args, "events")
		if len(events) == 0 {
			return nil, fmt.Errorf("events is required and must be non-empty")
		}

		incidentIDs := strSliceArg(args, "incident_ids")
		offlineDiagID := strArg(args, "offline_diagnosis_id")

		minRep := 2
		if v, ok := args["min_repetitions"].(float64); ok && v >= 1 {
			minRep = int(v)
		}
		saveProposal := false
		if v, ok := args["save_proposal"].(bool); ok {
			saveProposal = v
		}

		docsDir := s.resolvedDocsDir()

		// Count evidence: each incident_id counts as one repetition.
		// The events themselves count as 1 if no incident_ids provided.
		evidenceCount := len(incidentIDs)
		if evidenceCount == 0 {
			evidenceCount = 1 // the current call is one observation
			if offlineDiagID != "" {
				evidenceCount = 1
			}
		}

		warnings := []string{
			"Correlation is not causation. Human review required before applying this rule.",
			"This proposal is a DRAFT. It must be validated and approved before being added to causal_rules.yaml.",
		}

		// Check existing rules for a match.
		existingRuleID := matchExistingCausalRule(docsDir, events)
		if existingRuleID != "" {
			return &suggestCausalRuleResult{
				ExistingRuleMatch: true,
				ExistingRuleID:    existingRuleID,
				CandidateRules:    nil,
				Warnings:          []string{"An existing causal rule already covers this symptom sequence — no new rule needed."},
				Confidence:        "high",
			}, nil
		}

		// Check minimum repetitions.
		if evidenceCount < minRep {
			return &suggestCausalRuleResult{
				ExistingRuleMatch: false,
				CandidateRules:    nil,
				Warnings: append(warnings,
					fmt.Sprintf("Only %d observation(s) provided; min_repetitions=%d required before proposing a rule. Observe this sequence in more incidents first.", evidenceCount, minRep)),
				Confidence: "low",
			}, nil
		}

		if len(events) < 2 {
			return &suggestCausalRuleResult{
				ExistingRuleMatch: false,
				CandidateRules:    nil,
				Warnings: append(warnings, "A causal rule requires at least 2 events to define a sequence."),
				Confidence: "low",
			}, nil
		}

		// Build candidate rule.
		candidate := buildCandidateCausalRule(events, evidenceCount, incidentIDs)

		// Save proposal if requested.
		if saveProposal && docsDir != "" {
			if err := saveCausalRuleProposal(docsDir, candidate); err != nil {
				warnings = append(warnings, fmt.Sprintf("Failed to save proposal: %v", err))
			} else {
				warnings = append(warnings, fmt.Sprintf("Proposal saved as DRAFT: %s — review and validate before applying.", candidate.ProposalID))
			}
		}

		confidence := "low"
		if evidenceCount >= 3 {
			confidence = "medium"
		}

		return &suggestCausalRuleResult{
			ExistingRuleMatch: false,
			CandidateRules:    []candidateCausalRule{candidate},
			Warnings:          warnings,
			Confidence:        confidence,
		}, nil
	})
}

// ---------------------------------------------------------------------------
// Matching existing causal rules
// ---------------------------------------------------------------------------

// matchExistingCausalRule checks whether any existing rule in causal_rules.yaml
// matches the provided event sequence. Returns the matching rule ID or "".
func matchExistingCausalRule(docsDir string, events []string) string {
	if docsDir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(docsDir, "knowledge", "causal_rules.yaml"))
	if err != nil {
		return ""
	}

	var root struct {
		Rules []struct {
			ID              string   `yaml:"id"`
			TriggerKeywords []string `yaml:"trigger_keywords"`
			Sequence        []struct {
				Event string `yaml:"event"`
				Keywords []string `yaml:"keywords"`
			} `yaml:"sequence"`
		} `yaml:"rules"`
	}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return ""
	}

	eventsLower := make([]string, len(events))
	for i, e := range events {
		eventsLower[i] = strings.ToLower(e)
	}

	for _, rule := range root.Rules {
		// Check how many trigger keywords appear in the events.
		triggerMatched := 0
		for _, kw := range rule.TriggerKeywords {
			kwLower := strings.ToLower(kw)
			for _, e := range eventsLower {
				if strings.Contains(e, kwLower) {
					triggerMatched++
					break
				}
			}
		}

		// Check how many sequence steps are covered by events.
		if len(rule.Sequence) == 0 {
			continue
		}
		seqMatched := 0
		for _, step := range rule.Sequence {
			stepEvent := strings.ToLower(step.Event)
			for _, e := range eventsLower {
				if strings.Contains(e, stepEvent) {
					seqMatched++
					break
				}
				for _, kw := range step.Keywords {
					if strings.Contains(e, strings.ToLower(kw)) {
						seqMatched++
						break
					}
				}
			}
		}

		// Rule matches if ≥50% of trigger keywords OR ≥50% of sequence steps match.
		triggerRatio := float64(triggerMatched) / float64(max(1, len(rule.TriggerKeywords)))
		seqRatio := float64(seqMatched) / float64(len(rule.Sequence))
		if triggerRatio >= 0.5 || seqRatio >= 0.5 {
			return rule.ID
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Building candidate rules
// ---------------------------------------------------------------------------

func buildCandidateCausalRule(events []string, evidenceCount int, incidentIDs []string) candidateCausalRule {
	proposalID := fmt.Sprintf("causal-rule-proposal-%s", time.Now().UTC().Format("20060102T150405Z"))

	// Root signal: first event keyword (lowercase, underscored).
	rootSignal := eventToSignal(events[0])

	// Sequence: one entry per event.
	sequence := make([]string, len(events))
	for i, e := range events {
		sequence[i] = e
	}

	// Generate YAML patch.
	yamlPatch := buildCausalRuleYAML(proposalID, rootSignal, events, incidentIDs)

	confidence := "low"
	if evidenceCount >= 3 {
		confidence = "medium"
	}

	return candidateCausalRule{
		ProposalID:            proposalID,
		RootSignal:            rootSignal,
		Sequence:              sequence,
		Confidence:            confidence,
		EvidenceCount:         evidenceCount,
		RequiresHumanApproval: true,
		YAMLPatch:             yamlPatch,
	}
}

// eventToSignal converts an event string to a snake_case signal name.
func eventToSignal(event string) string {
	lower := strings.ToLower(event)
	// Take first few meaningful words.
	words := strings.FieldsFunc(lower, func(r rune) bool {
		return r == ' ' || r == ':' || r == '-' || r == '/' || r == '.' || r == ','
	})
	var parts []string
	stopWords := map[string]bool{"the": true, "a": true, "an": true, "and": true, "or": true, "is": true, "in": true, "to": true, "of": true, "at": true, "by": true}
	for _, w := range words {
		if !stopWords[w] && len(w) >= 2 {
			parts = append(parts, w)
			if len(parts) >= 3 {
				break
			}
		}
	}
	if len(parts) == 0 {
		return "unknown_signal"
	}
	return strings.Join(parts, "_")
}

func buildCausalRuleYAML(proposalID, rootSignal string, events, incidentIDs []string) string {
	var sb strings.Builder
	sb.WriteString("# Proposed causal rule (DRAFT — requires human review and approval)\n")
	sb.WriteString("# Generated by awareness.suggest_causal_rule\n")
	sb.WriteString(fmt.Sprintf("# Proposal ID: %s\n", proposalID))
	if len(incidentIDs) > 0 {
		sb.WriteString(fmt.Sprintf("# Evidence from incidents: %s\n", strings.Join(incidentIDs, ", ")))
	}
	sb.WriteString("#\n")
	sb.WriteString("rules:\n")
	sb.WriteString(fmt.Sprintf("  - id: %s_cascade\n", rootSignal))
	sb.WriteString(fmt.Sprintf("    root_signal: %s\n", rootSignal))
	sb.WriteString("    trigger_keywords:\n")
	// Extract keywords from first event.
	for _, w := range strings.Fields(strings.ToLower(events[0])) {
		if len(w) >= 4 {
			sb.WriteString(fmt.Sprintf("      - %s\n", w))
			break
		}
	}
	sb.WriteString("    sequence:\n")
	for i, e := range events {
		sb.WriteString(fmt.Sprintf("      - event: step_%d\n", i+1))
		sb.WriteString(fmt.Sprintf("        description: %q\n", e))
		sb.WriteString("        keywords: []\n")
	}
	sb.WriteString("    recommended_fix_order:\n")
	sb.WriteString(fmt.Sprintf("      - \"Investigate and resolve root cause: %s\"\n", events[0]))
	for _, e := range events[1:] {
		sb.WriteString(fmt.Sprintf("      - \"Verify downstream symptom resolved: %s\"\n", e))
	}
	return sb.String()
}

func saveCausalRuleProposal(docsDir string, candidate candidateCausalRule) error {
	proposalsDir := filepath.Join(docsDir, "proposals")
	if err := os.MkdirAll(proposalsDir, 0o755); err != nil {
		return err
	}
	filename := fmt.Sprintf("%s.yaml", candidate.ProposalID)
	content := fmt.Sprintf(`proposal:
  id: %s
  status: DRAFT
  created_at: %q
causal_rule_candidates:
%s`, candidate.ProposalID, time.Now().UTC().Format(time.RFC3339), indent(candidate.YAMLPatch, "  "))
	return os.WriteFile(filepath.Join(proposalsDir, filename), []byte(content), 0o644)
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = prefix + l
		}
	}
	return strings.Join(lines, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
