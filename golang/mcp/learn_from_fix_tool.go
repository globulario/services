package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/awareness/learning"
)

// learnFromFixProposal is one entry in the proposals array returned by the tool.
type learnFromFixProposal struct {
	ProposalID             string `json:"proposal_id"`
	TargetFile             string `json:"target_file"`
	Operation              string `json:"operation"`
	EntryID                string `json:"entry_id"`
	Reason                 string `json:"reason"`
	YAMLPatch              string `json:"yaml_patch"`
	Confidence             string `json:"confidence"`
	RequiresHumanApproval  bool   `json:"requires_human_approval"`
}

func registerLearnFromFixTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.learn_from_fix",
		Description: "Synthesize a draft awareness proposal from a verified fix. Proposes failure modes, forbidden fixes, scan rules, invariants, and metric thresholds as appropriate. Always requires human approval — never directly edits knowledge YAML files.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"incident_id":           {Type: "string", Description: "Optional incident ID to link this fix to."},
				"snapshot_id":           {Type: "string", Description: "Optional snapshot ID captured before/after the fix."},
				"symptom_text":          {Type: "string", Description: "Raw error text or log line that triggered the investigation."},
				"root_cause":            {Type: "string", Description: "What was actually wrong."},
				"fix_summary":           {Type: "string", Description: "What changed to fix it."},
				"verification":          {Type: "string", Description: "How the fix was proven (test run, manual check, metric change)."},
				"changed_files":         {Type: "array", Description: "Source files changed as part of the fix.", Items: &propSchema{Type: "string"}},
				"tests_added":           {Type: "array", Description: "Test function names added to cover this fix.", Items: &propSchema{Type: "string"}},
				"known_bad_fix":         {Type: "string", Description: "Optional: a tempting wrong fix that must be avoided."},
				"related_failure_mode":  {Type: "string", Description: "Optional: existing failure mode ID this fix relates to."},
				"related_invariant":     {Type: "string", Description: "Optional: existing invariant ID this fix relates to."},
			},
			Required: []string{"symptom_text", "root_cause", "fix_summary"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		_ = ctx

		symptomText := strArg(args, "symptom_text")
		rootCause := strArg(args, "root_cause")
		fixSummary := strArg(args, "fix_summary")

		if symptomText == "" || rootCause == "" || fixSummary == "" {
			return nil, fmt.Errorf("symptom_text, root_cause, and fix_summary are required")
		}

		docsDir := st.docsDir
		if docsDir == "" {
			return nil, fmt.Errorf("docs dir not configured")
		}

		incidentID := strArg(args, "incident_id")
		verification := strArg(args, "verification")
		knownBadFix := strArg(args, "known_bad_fix")
		relatedFM := strArg(args, "related_failure_mode")
		relatedInv := strArg(args, "related_invariant")
		changedFiles := strSliceArg(args, "changed_files")
		testsAdded := strSliceArg(args, "tests_added")

		timestamp := time.Now().UTC().Format("20060102T150405Z")
		proposalID := "learned-fix-" + timestamp
		if incidentID != "" {
			proposalID = "learned-fix-" + sanitiseID(incidentID) + "-" + timestamp
		}

		// --- Build blind spots ---
		var blindSpots []string
		if verification == "" {
			blindSpots = append(blindSpots, "no verification provided — cannot confirm fix effectiveness")
		}
		if len(testsAdded) == 0 {
			blindSpots = append(blindSpots, "no tests_added — regression coverage is unconfirmed")
		}
		if len(changedFiles) == 0 {
			blindSpots = append(blindSpots, "no changed_files provided — scan rule proposal may be imprecise")
		}

		// --- Synthesise proposals ---
		var proposals []learnFromFixProposal
		var spec learning.ProposalSpec

		spec.LearnSource = "learn_from_fix"
		spec.Proposal = learning.ProposalHeader{
			ID:             proposalID,
			SourceIncident: incidentID,
			Status:         learning.StatusDraft,
			CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		}
		spec.Evidence = learning.ProposalEvidence{
			SourceIncident: incidentID,
			Symptoms:       []string{symptomText},
			ManualRepairs:  []string{fixSummary},
		}
		if verification != "" {
			spec.Evidence.StateDeltas = []string{"verification: " + verification}
		}

		// 1. Failure mode proposal
		fmID := "failure_mode." + sanitiseID(symptomText[:minInt(40, len(symptomText))])
		if relatedFM != "" {
			fmID = relatedFM + ".updated"
		}
		fmSeverity := deriveSeverity(symptomText, rootCause)
		var fmRelatedInvariants []string
		if relatedInv != "" {
			fmRelatedInvariants = []string{relatedInv}
		}
		fm := learning.ProposedFailureMode{
			ID:                fmID,
			Title:             truncate(symptomText, 80),
			Severity:          fmSeverity,
			Symptoms:          []string{symptomText},
			RootCause:         rootCause,
			ArchitectureFix:   fixSummary,
			RelatedInvariants: fmRelatedInvariants,
		}
		if len(testsAdded) > 0 {
			fm.RequiredTests = testsAdded
		}
		spec.FailureModes = []learning.ProposedFailureMode{fm}

		fmYAML := renderFailureModeYAML(fm)
		proposals = append(proposals, learnFromFixProposal{
			ProposalID:            proposalID + "/failure_mode",
			TargetFile:            "failure_modes.yaml",
			Operation:             "add_or_update",
			EntryID:               fmID,
			Reason:                "New failure mode derived from verified fix: " + truncate(symptomText, 60),
			YAMLPatch:             fmYAML,
			Confidence:            "medium",
			RequiresHumanApproval: true,
		})

		// 2. Forbidden fix proposal (when knownBadFix is provided)
		if knownBadFix != "" {
			ffID := "forbidden_fix." + sanitiseID(knownBadFix[:minInt(40, len(knownBadFix))])
			ff := learning.ProposedForbiddenFix{
				ID:      ffID,
				Title:   truncate(knownBadFix, 80),
				Summary: "Tempting but incorrect fix: " + knownBadFix + ". Root cause was: " + rootCause,
			}
			if relatedInv != "" {
				ff.RelatedInvariants = []string{relatedInv}
			}
			spec.ForbiddenFixes = []learning.ProposedForbiddenFix{ff}

			ffYAML := renderForbiddenFixYAML(ff)
			proposals = append(proposals, learnFromFixProposal{
				ProposalID:            proposalID + "/forbidden_fix",
				TargetFile:            "forbidden_fixes.yaml",
				Operation:             "add_or_update",
				EntryID:               ffID,
				Reason:                "Known-bad fix identified during incident resolution",
				YAMLPatch:             ffYAML,
				Confidence:            "high",
				RequiresHumanApproval: true,
			})
			// Also add reference in the failure mode
			fm.ForbiddenFixes = append(fm.ForbiddenFixes, ffID)
		}

		// 3. Scan rule proposal — when changed Go files contain relevant loopback/exec/env patterns in symptom text
		if scanRuleNeeded(symptomText, changedFiles) {
			srID := "scan_rule." + sanitiseID(symptomText[:minInt(40, len(symptomText))])
			sr := learning.ProposedScanRule{
				ID:          srID,
				Description: "Detected dangerous pattern from fix: " + truncate(symptomText, 60),
				Language:    "go",
				Severity:    "high",
				KnowledgeID: fmID,
				SafeAlternative: fixSummary,
			}
			// Heuristically choose patterns from symptom text
			sr.Patterns = deriveScanPatterns(symptomText)
			if len(sr.Patterns) > 0 {
				spec.ScanRules = []learning.ProposedScanRule{sr}
				srYAML := renderScanRuleYAML(sr)
				proposals = append(proposals, learnFromFixProposal{
					ProposalID:            proposalID + "/scan_rule",
					TargetFile:            "knowledge/scan_rules.yaml",
					Operation:             "add",
					EntryID:               srID,
					Reason:                "Go files changed; dangerous pattern identified in symptom",
					YAMLPatch:             srYAML,
					Confidence:            "low",
					RequiresHumanApproval: true,
				})
			}
		}

		// 4. Invariant proposal — if rootCause contains strong architectural language
		if invariantNeeded(rootCause, symptomText) {
			invID := "invariant." + sanitiseID(rootCause[:minInt(40, len(rootCause))])
			if relatedInv != "" {
				invID = relatedInv + ".updated"
			}
			inv := learning.ProposedInvariant{
				ID:       invID,
				Title:    "Invariant from fix: " + truncate(rootCause, 60),
				Severity: fmSeverity,
				Summary:  rootCause + " — fix: " + fixSummary,
			}
			if len(testsAdded) > 0 {
				inv.RequiredTests = testsAdded
			}
			spec.Invariants = []learning.ProposedInvariant{inv}
			invYAML := renderInvariantYAML(inv)
			proposals = append(proposals, learnFromFixProposal{
				ProposalID:            proposalID + "/invariant",
				TargetFile:            "invariants.yaml",
				Operation:             "add_or_update",
				EntryID:               invID,
				Reason:                "Root cause implies an architectural invariant violation",
				YAMLPatch:             invYAML,
				Confidence:            "low",
				RequiresHumanApproval: true,
			})
		}

		// --- Persist proposal to proposals dir ---
		proposalsDir := filepath.Join(docsDir, "proposals")
		if err := os.MkdirAll(proposalsDir, 0o755); err != nil {
			return nil, fmt.Errorf("create proposals dir: %w", err)
		}
		proposalPath := filepath.Join(proposalsDir, proposalID+".yaml")

		if err := learning.SaveProposal(proposalPath, &spec); err != nil {
			return nil, fmt.Errorf("save proposal: %w", err)
		}

		summary := fmt.Sprintf("Generated %d proposal(s) from fix for: %s", len(proposals), truncate(symptomText, 60))
		confidence := "medium"
		if len(blindSpots) >= 2 {
			confidence = "low"
		}

		return map[string]interface{}{
			"proposals":                proposals,
			"proposal_path":            proposalPath,
			"summary":                  summary,
			"confidence":               confidence,
			"blind_spots":              blindSpots,
			"recommended_next_action":  "review_knowledge_proposal",
		}, nil
	})
}

// --- helpers ---

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func deriveSeverity(symptom, rootCause string) string {
	combined := strings.ToLower(symptom + " " + rootCause)
	for _, kw := range []string{"critical", "data loss", "cluster unreachable", "quorum", "split-brain", "split brain", "corruption"} {
		if strings.Contains(combined, kw) {
			return "critical"
		}
	}
	for _, kw := range []string{"blocked", "timeout", "failed", "crash", "panic", "restart loop", "unavailable"} {
		if strings.Contains(combined, kw) {
			return "high"
		}
	}
	return "medium"
}

func scanRuleNeeded(symptomText string, changedFiles []string) bool {
	// Need Go files changed.
	hasGoFile := false
	for _, f := range changedFiles {
		if strings.HasSuffix(f, ".go") {
			hasGoFile = true
			break
		}
	}
	if !hasGoFile && len(changedFiles) > 0 {
		return false
	}
	// Check if symptom implies a detectable code pattern.
	lower := strings.ToLower(symptomText)
	for _, kw := range []string{"127.0.0.1", "localhost", "os.getenv", "os/exec", "hardcoded", "grpc.dial", "insecure"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func deriveScanPatterns(symptomText string) []string {
	lower := strings.ToLower(symptomText)
	var patterns []string
	if strings.Contains(lower, "127.0.0.1") {
		patterns = append(patterns, `"127\.0\.0\.1`)
	}
	if strings.Contains(lower, "localhost") {
		patterns = append(patterns, `"localhost`)
	}
	if strings.Contains(lower, "os.getenv") || strings.Contains(lower, "getenv") {
		patterns = append(patterns, `os\.Getenv\(`)
	}
	if strings.Contains(lower, "os/exec") || strings.Contains(lower, "exec.command") {
		patterns = append(patterns, `"os/exec"`)
	}
	if strings.Contains(lower, "grpc.dial") || strings.Contains(lower, "grpc.newclient") {
		patterns = append(patterns, `grpc\.Dial\(`)
	}
	if strings.Contains(lower, "insecure") {
		patterns = append(patterns, `insecure\.NewCredentials\(\)`)
	}
	return patterns
}

func invariantNeeded(rootCause, symptomText string) bool {
	combined := strings.ToLower(rootCause + " " + symptomText)
	for _, kw := range []string{"must not", "must always", "never", "always must", "invariant", "forbidden", "authority", "source of truth"} {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

func renderFailureModeYAML(fm learning.ProposedFailureMode) string {
	var b strings.Builder
	b.WriteString("- id: " + fm.ID + "\n")
	b.WriteString("  title: " + fm.Title + "\n")
	b.WriteString("  severity: " + fm.Severity + "\n")
	b.WriteString("  symptoms:\n")
	for _, s := range fm.Symptoms {
		b.WriteString("    - " + s + "\n")
	}
	b.WriteString("  root_cause: |\n    " + strings.ReplaceAll(fm.RootCause, "\n", "\n    ") + "\n")
	b.WriteString("  architecture_fix: |\n    " + strings.ReplaceAll(fm.ArchitectureFix, "\n", "\n    ") + "\n")
	if len(fm.RequiredTests) > 0 {
		b.WriteString("  required_tests:\n")
		for _, t := range fm.RequiredTests {
			b.WriteString("    - " + t + "\n")
		}
	}
	return b.String()
}

func renderForbiddenFixYAML(ff learning.ProposedForbiddenFix) string {
	var b strings.Builder
	b.WriteString("- id: " + ff.ID + "\n")
	b.WriteString("  title: " + ff.Title + "\n")
	b.WriteString("  summary: " + ff.Summary + "\n")
	if len(ff.RelatedInvariants) > 0 {
		b.WriteString("  related_invariants:\n")
		for _, inv := range ff.RelatedInvariants {
			b.WriteString("    - " + inv + "\n")
		}
	}
	return b.String()
}

func renderInvariantYAML(inv learning.ProposedInvariant) string {
	var b strings.Builder
	b.WriteString("- id: " + inv.ID + "\n")
	b.WriteString("  title: " + inv.Title + "\n")
	b.WriteString("  severity: " + inv.Severity + "\n")
	b.WriteString("  summary: |\n    " + strings.ReplaceAll(inv.Summary, "\n", "\n    ") + "\n")
	if len(inv.RequiredTests) > 0 {
		b.WriteString("  required_tests:\n")
		for _, t := range inv.RequiredTests {
			b.WriteString("    - " + t + "\n")
		}
	}
	return b.String()
}

func renderScanRuleYAML(sr learning.ProposedScanRule) string {
	var b strings.Builder
	b.WriteString("- id: " + sr.ID + "\n")
	b.WriteString("  description: " + sr.Description + "\n")
	b.WriteString("  language: " + sr.Language + "\n")
	b.WriteString("  severity: " + sr.Severity + "\n")
	if sr.KnowledgeID != "" {
		b.WriteString("  knowledge_id: " + sr.KnowledgeID + "\n")
	}
	if sr.SafeAlternative != "" {
		b.WriteString("  safe_alternative: |\n    " + strings.ReplaceAll(sr.SafeAlternative, "\n", "\n    ") + "\n")
	}
	if len(sr.Patterns) > 0 {
		b.WriteString("  patterns:\n")
		for _, p := range sr.Patterns {
			b.WriteString("    - " + p + "\n")
		}
	}
	return b.String()
}

// sanitiseID is re-exported from the learning package scope; replicated here
// so the mcp package can use it without importing an unexported symbol.
// (The learning package already has the same function but unexported.)
func sanitiseID(s string) string {
	s = strings.ToLower(s)
	var buf strings.Builder
	lastWasUnderscore := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' {
			buf.WriteRune(r)
			lastWasUnderscore = false
		} else {
			if !lastWasUnderscore {
				buf.WriteRune('_')
				lastWasUnderscore = true
			}
		}
	}
	return strings.Trim(buf.String(), "_")
}
