package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// agent_playbooks.yaml types
// ---------------------------------------------------------------------------

// agentPlaybooks is the top-level structure of agent_playbooks.yaml.
type agentPlaybooks struct {
	Playbooks            []playbook           `yaml:"playbooks"`
	CapabilityGapPatterns []capabilityGapPattern `yaml:"capability_gap_patterns"`
}

type playbook struct {
	ID              string   `yaml:"id"`
	Title           string   `yaml:"title"`
	Trigger         []string `yaml:"trigger"`
	RequiredOutput  []string `yaml:"required_output"`
	ForbiddenBehavior []string `yaml:"forbidden_behavior"`
}

type knowledgeUpdate struct {
	TargetFile string `yaml:"target_file"`
	Entry      string `yaml:"entry"`
	Operation  string `yaml:"operation"`
}

// capabilityGapPattern is a known awareness capability gap with full requirement structure.
type capabilityGapPattern struct {
	ID                    string            `yaml:"id"`
	Priority              string            `yaml:"priority"`
	Keywords              []string          `yaml:"keywords"`
	Title                 string            `yaml:"title"`
	Criticism             string            `yaml:"criticism"`
	WhyItMatters          string            `yaml:"why_it_matters"`
	Requirement           string            `yaml:"requirement"`
	ImplementationPlan    []string          `yaml:"implementation_plan"`
	TestsRequired         []string          `yaml:"tests_required"`
	ClosureCondition      string            `yaml:"closure_condition"`
	KnowledgeUpdates      []knowledgeUpdate `yaml:"knowledge_updates"`
	PreventsRepeatCriticism string          `yaml:"prevents_repeat_criticism"`
	Status                string            `yaml:"status"`
}

// ---------------------------------------------------------------------------
// Output types
// ---------------------------------------------------------------------------

type capabilityGapResult struct {
	GapID                   string            `json:"gap_id"`
	Priority                string            `json:"priority"`
	Status                  string            `json:"status"`
	Criticism               string            `json:"criticism"`
	WhyItMatters            string            `json:"why_it_matters"`
	Requirement             string            `json:"requirement"`
	ImplementationPlan      []string          `json:"implementation_plan"`
	TestsRequired           []string          `json:"tests_required"`
	ClosureCondition        string            `json:"closure_condition"`
	KnowledgeUpdates        []knowledgeUpdate `json:"knowledge_updates"`
	PreventsRepeatCriticism string            `json:"prevents_repeat_criticism"`
	AlreadyProposed         bool              `json:"already_proposed"`
	DuplicateOf             string            `json:"duplicate_of,omitempty"`
}

type closedGapResult struct {
	GapID                   string `json:"gap_id"`
	Status                  string `json:"status"`
	ClosureCondition        string `json:"closure_condition"`
	PreventsRepeatCriticism string `json:"prevents_repeat_criticism"`
	// VerificationStatus reports whether required tests were found in the codebase.
	// Values: "tests_found" | "tests_partial" | "tests_not_found" | "no_tests_required" | "unverified"
	VerificationStatus string `json:"verification_status,omitempty"`
	VerificationNote   string `json:"verification_note,omitempty"`
}

type incompleteCriticism struct {
	Text            string `json:"text"`
	Status          string `json:"status"`
	MissingEvidence string `json:"missing_evidence"`
}

type selfReviewResult struct {
	Summary                string                `json:"summary"`
	CapabilityGaps         []capabilityGapResult `json:"capability_gaps"`
	ClosedGaps             []closedGapResult     `json:"closed_gaps"`
	IncompleteCriticisms   []incompleteCriticism `json:"incomplete_criticisms"`
	GlobalClosureCondition string                `json:"global_closure_condition"`
	Confidence             string                `json:"confidence"`
	ConfidenceReason       string                `json:"confidence_reason"`
	BlindSpots             []string              `json:"blind_spots"`
	RecommendedNextAction  string                `json:"recommended_next_action"`
}

type requirementFromCritiqueResult struct {
	GapID                   string            `json:"gap_id"`
	Priority                string            `json:"priority"`
	Criticism               string            `json:"criticism"`
	WhyItMatters            string            `json:"why_it_matters"`
	Requirement             string            `json:"requirement"`
	ImplementationPlan      []string          `json:"implementation_plan"`
	TestsRequired           []string          `json:"tests_required"`
	ClosureCondition        string            `json:"closure_condition"`
	KnowledgeUpdates        []knowledgeUpdate `json:"knowledge_updates"`
	PreventsRepeatCriticism string            `json:"prevents_repeat_criticism"`
	Confidence              string            `json:"confidence"`
	BlindSpots              []string          `json:"blind_spots"`
}

// ---------------------------------------------------------------------------
// Tool registration
// ---------------------------------------------------------------------------

func registerSelfReviewTools(s *Server) {
	registerSelfReviewTool(s)
	registerRequirementFromCritiqueTool(s)
}

func registerSelfReviewTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.self_review",
		Description: "Convert awareness criticism/feedback into structured capability gap requirements. Each criticism becomes a testable requirement with closure condition, implementation plan, tests, and prevents_repeat_criticism. Already-implemented gaps are listed separately. Vague feedback is marked incomplete — never invented. Pure keyword matching — no LLM, no external calls.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"goal":     {Type: "string", Description: "The improvement goal for this review."},
				"feedback": {Type: "string", Description: "Criticism or review feedback text to convert into requirements."},
				"context": {
					Type:        "object",
					Description: "Optional context flags controlling what data is included in the review.",
				},
				"scope": {
					Type:        "array",
					Description: "Areas to scope the review: awareness, runtime, preflight, scan_violations, knowledge_base, proposal_workflow.",
					Items:       &propSchema{Type: "string"},
				},
				"strict": {
					Type:        "boolean",
					Description: "If true, vague feedback without any keyword match causes a hard error instead of incomplete_criticisms.",
					Default:     false,
				},
			},
			Required: []string{"feedback"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		_ = ctx

		feedback := strArg(args, "feedback")
		if feedback == "" {
			return nil, fmt.Errorf("feedback is required")
		}

		strict := boolArg(args, "strict")

		docsDir := s.resolvedDocsDir()
		repoRoot := s.resolvedRepoRoot()
		if docsDir == "" {
			return &selfReviewResult{
				Summary:                "docs dir not configured — cannot load agent_playbooks.yaml",
				GlobalClosureCondition: "Every criticism in the feedback is either converted into a testable requirement or marked as incomplete with missing evidence.",
				Confidence:             "low",
				ConfidenceReason:       "docs dir not configured",
				BlindSpots:             []string{"docs dir not configured — agent_playbooks.yaml not available"},
				RecommendedNextAction:  "configure_docs_dir",
			}, nil
		}

		// Load agent_playbooks.yaml.
		playbooks, err := loadAgentPlaybooks(docsDir)
		if err != nil {
			return &selfReviewResult{
				Summary:                fmt.Sprintf("could not load agent_playbooks.yaml: %v", err),
				GlobalClosureCondition: "Every criticism in the feedback is either converted into a testable requirement or marked as incomplete with missing evidence.",
				Confidence:             "low",
				ConfidenceReason:       "agent_playbooks.yaml not available",
				BlindSpots:             []string{"agent_playbooks.yaml missing or invalid: " + err.Error()},
				RecommendedNextAction:  "fix_playbooks_yaml",
			}, nil
		}

		// Load pending proposals for duplicate detection.
		pendingProposalIDs := loadPendingProposalIDs(docsDir)

		// Parse feedback into segments.
		segments := parseFeedbackSegments(feedback)

		// Score each gap pattern against all segments.
		type scoredGap struct {
			pattern capabilityGapPattern
			score   int
			matched []string // which segments matched
		}

		gapScores := make(map[string]*scoredGap)
		for i := range playbooks.CapabilityGapPatterns {
			pat := playbooks.CapabilityGapPatterns[i]
			sg := &scoredGap{pattern: pat}
			for _, seg := range segments {
				segLower := strings.ToLower(seg)
				for _, kw := range pat.Keywords {
					if strings.Contains(segLower, strings.ToLower(kw)) {
						sg.score++
						sg.matched = append(sg.matched, seg)
						break // count segment once per gap
					}
				}
			}
			// Also score the full feedback as one block.
			fullLower := strings.ToLower(feedback)
			for _, kw := range pat.Keywords {
				if strings.Contains(fullLower, strings.ToLower(kw)) {
					if sg.score == 0 {
						sg.score++
					}
					break
				}
			}
			if sg.score > 0 {
				gapScores[pat.ID] = sg
			}
		}

		// Track which segments matched any gap (for incomplete detection).
		segmentMatched := make(map[string]bool)
		for _, sg := range gapScores {
			for _, m := range sg.matched {
				segmentMatched[m] = true
			}
		}

		// Build output.
		var capGaps []capabilityGapResult
		var closedGaps []closedGapResult

		for _, sg := range gapScores {
			pat := sg.pattern
			if pat.Status == "implemented" {
				verStatus, verNote := verifyGapTests(repoRoot, pat.TestsRequired)
				closedGaps = append(closedGaps, closedGapResult{
					GapID:                   pat.ID,
					Status:                  "implemented",
					ClosureCondition:        pat.ClosureCondition,
					PreventsRepeatCriticism: pat.PreventsRepeatCriticism,
					VerificationStatus:      verStatus,
					VerificationNote:        verNote,
				})
				// Mark those segments as handled.
				for _, m := range sg.matched {
					segmentMatched[m] = true
				}
				continue
			}

			// Check if a pending proposal already covers this gap.
			dupOf := findDuplicateProposal(pat.ID, pendingProposalIDs)
			alreadyProposed := dupOf != ""

			kg := pat.KnowledgeUpdates
			if kg == nil {
				kg = []knowledgeUpdate{}
			}
			ip := pat.ImplementationPlan
			if ip == nil {
				ip = []string{}
			}
			tr := pat.TestsRequired
			if tr == nil {
				tr = []string{}
			}

			capGaps = append(capGaps, capabilityGapResult{
				GapID:                   pat.ID,
				Priority:                pat.Priority,
				Status:                  "open",
				Criticism:               pat.Criticism,
				WhyItMatters:            pat.WhyItMatters,
				Requirement:             pat.Requirement,
				ImplementationPlan:      ip,
				TestsRequired:           tr,
				ClosureCondition:        pat.ClosureCondition,
				KnowledgeUpdates:        kg,
				PreventsRepeatCriticism: pat.PreventsRepeatCriticism,
				AlreadyProposed:         alreadyProposed,
				DuplicateOf:             dupOf,
			})
		}

		// Incomplete criticisms: segments that matched no gap.
		var incompletes []incompleteCriticism
		for _, seg := range segments {
			if segmentMatched[seg] {
				continue
			}
			words := strings.Fields(seg)
			if len(words) < 5 {
				// Too short / vague.
				if strict {
					return nil, fmt.Errorf("strict mode: vague feedback segment has no keyword match: %q", seg)
				}
				incompletes = append(incompletes, incompleteCriticism{
					Text:            seg,
					Status:          "incomplete",
					MissingEvidence: "Criticism is too vague to map to a specific capability gap. Provide more detail about what behavior is wrong and what evidence exists.",
				})
			}
			// Longer unmatched segments are also incomplete but with a different message.
			if len(words) >= 5 {
				incompletes = append(incompletes, incompleteCriticism{
					Text:            seg,
					Status:          "incomplete",
					MissingEvidence: "Criticism does not match any known capability gap pattern. Add specific keywords or add a new capability_gap_pattern entry to agent_playbooks.yaml.",
				})
			}
		}

		// Compute confidence.
		totalMatched := len(capGaps) + len(closedGaps)
		confidence := "low"
		confidenceReason := "no keyword matches found"
		if totalMatched >= 3 {
			confidence = "high"
			confidenceReason = fmt.Sprintf("%d gaps identified with keyword matching", totalMatched)
		} else if totalMatched >= 1 {
			confidence = "medium"
			confidenceReason = fmt.Sprintf("%d gap(s) identified with keyword matching", totalMatched)
		}
		if len(incompletes) > 0 && totalMatched == 0 {
			confidence = "low"
			confidenceReason = "feedback is too vague — no keywords matched known gap patterns"
		}

		// Blind spots.
		var blindSpots []string
		if len(incompletes) > 0 {
			blindSpots = append(blindSpots, fmt.Sprintf("%d feedback segment(s) could not be mapped to a known gap pattern", len(incompletes)))
		}
		if docsDir == "" {
			blindSpots = append(blindSpots, "docs dir not configured — agent_playbooks.yaml may be incomplete")
		}

		// Recommended next action.
		nextAction := "no_action_required"
		hasP0 := false
		for _, g := range capGaps {
			if g.Priority == "P0" {
				hasP0 = true
				break
			}
		}
		if hasP0 {
			nextAction = "implement_p0_gaps"
		} else if len(capGaps) > 0 {
			nextAction = "implement_open_gaps"
		} else if len(incompletes) > 0 {
			nextAction = "refine_feedback_or_add_gap_patterns"
		}

		summary := buildSelfReviewSummary(capGaps, closedGaps, incompletes)

		return &selfReviewResult{
			Summary:                summary,
			CapabilityGaps:         capGaps,
			ClosedGaps:             closedGaps,
			IncompleteCriticisms:   incompletes,
			GlobalClosureCondition: "Every criticism in the feedback is either converted into a testable requirement or marked as incomplete with missing evidence.",
			Confidence:             confidence,
			ConfidenceReason:       confidenceReason,
			BlindSpots:             blindSpots,
			RecommendedNextAction:  nextAction,
		}, nil
	})
}

func registerRequirementFromCritiqueTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.requirement_from_critique",
		Description: "Convert a single criticism string into a structured capability gap requirement. Uses the same keyword-matching playbook as awareness.self_review but operates on one criticism at a time. Returns one gap with requirement, tests, closure condition, and prevents_repeat_criticism.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"criticism": {Type: "string", Description: "A single criticism or gap observation to convert into a requirement."},
				"goal":      {Type: "string", Description: "Optional: the improvement goal this criticism relates to."},
				"scope":     {Type: "string", Description: "Optional: the area being criticized (e.g. scan_violations, preflight, runtime)."},
			},
			Required: []string{"criticism"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		_ = ctx

		criticism := strArg(args, "criticism")
		if criticism == "" {
			return nil, fmt.Errorf("criticism is required")
		}

		docsDir := s.resolvedDocsDir()

		var patterns []capabilityGapPattern
		if docsDir != "" {
			if pb, err := loadAgentPlaybooks(docsDir); err == nil {
				patterns = pb.CapabilityGapPatterns
			}
		}

		// Score each pattern.
		type scoredPattern struct {
			pattern capabilityGapPattern
			score   int
		}
		var best *scoredPattern

		criticismLower := strings.ToLower(criticism)
		for i := range patterns {
			pat := patterns[i]
			score := 0
			for _, kw := range pat.Keywords {
				if strings.Contains(criticismLower, strings.ToLower(kw)) {
					score++
				}
			}
			if score > 0 && (best == nil || score > best.score) {
				best = &scoredPattern{pattern: pat, score: score}
			}
		}

		var blindSpots []string

		if best == nil {
			// No match — return a skeleton with low confidence.
			blindSpots = append(blindSpots, "No keyword match found in agent_playbooks.yaml. Add a new capability_gap_pattern entry or refine the criticism with more specific terms.")
			if docsDir == "" {
				blindSpots = append(blindSpots, "docs dir not configured — agent_playbooks.yaml not available")
			}
			return &requirementFromCritiqueResult{
				GapID:                   "awareness.unknown_gap",
				Priority:                "P1",
				Criticism:               criticism,
				WhyItMatters:            "Impact not determined — no matching pattern found.",
				Requirement:             "Requirement not determined — add a capability_gap_pattern entry to agent_playbooks.yaml that matches this criticism.",
				ImplementationPlan:      []string{},
				TestsRequired:           []string{},
				ClosureCondition:        "",
				KnowledgeUpdates:        []knowledgeUpdate{},
				PreventsRepeatCriticism: "",
				Confidence:              "low",
				BlindSpots:              blindSpots,
			}, nil
		}

		pat := best.pattern
		confidence := "medium"
		if best.score >= 3 {
			confidence = "high"
		}

		kg := pat.KnowledgeUpdates
		if kg == nil {
			kg = []knowledgeUpdate{}
		}
		ip := pat.ImplementationPlan
		if ip == nil {
			ip = []string{}
		}
		tr := pat.TestsRequired
		if tr == nil {
			tr = []string{}
		}

		return &requirementFromCritiqueResult{
			GapID:                   pat.ID,
			Priority:                pat.Priority,
			Criticism:               pat.Criticism,
			WhyItMatters:            pat.WhyItMatters,
			Requirement:             pat.Requirement,
			ImplementationPlan:      ip,
			TestsRequired:           tr,
			ClosureCondition:        pat.ClosureCondition,
			KnowledgeUpdates:        kg,
			PreventsRepeatCriticism: pat.PreventsRepeatCriticism,
			Confidence:              confidence,
			BlindSpots:              blindSpots,
		}, nil
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// loadAgentPlaybooks loads and parses agent_playbooks.yaml from docsDir/knowledge/.
func loadAgentPlaybooks(docsDir string) (*agentPlaybooks, error) {
	path := filepath.Join(docsDir, "knowledge", "agent_playbooks.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read agent_playbooks.yaml: %w", err)
	}
	var pb agentPlaybooks
	if err := yaml.Unmarshal(data, &pb); err != nil {
		return nil, fmt.Errorf("parse agent_playbooks.yaml: %w", err)
	}
	return &pb, nil
}

// parseFeedbackSegments splits feedback text into meaningful segments for scoring.
// Splits on: double newlines, bullet markers, sentence endings followed by newline.
// Also scores the full text as one implicit segment.
func parseFeedbackSegments(feedback string) []string {
	// Normalize CRLF.
	feedback = strings.ReplaceAll(feedback, "\r\n", "\n")

	// Split on double newlines (paragraph boundaries).
	paragraphs := strings.Split(feedback, "\n\n")

	var segments []string
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		// Within a paragraph, split on bullet markers and sentence endings.
		lines := strings.Split(para, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Remove leading bullet markers.
			for _, marker := range []string{"- ", "* ", "• "} {
				if strings.HasPrefix(line, marker) {
					line = strings.TrimPrefix(line, marker)
					break
				}
			}
			// Remove leading numbered list markers (e.g. "1. ", "2. ").
			if len(line) > 2 && unicode.IsDigit(rune(line[0])) && line[1] == '.' {
				line = strings.TrimSpace(line[2:])
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Split on ". " sentence boundaries within a line.
			parts := splitSentences(line)
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					segments = append(segments, p)
				}
			}
		}
	}

	// Also add full feedback as one segment for cross-sentence patterns.
	segments = append(segments, strings.TrimSpace(feedback))
	return dedupeStrings(segments)
}

// splitSentences splits a line on sentence-ending punctuation.
func splitSentences(line string) []string {
	var parts []string
	var cur strings.Builder
	for i := 0; i < len(line); i++ {
		ch := rune(line[i])
		cur.WriteRune(ch)
		if (ch == '.' || ch == '!' || ch == '?') && i+1 < len(line) && line[i+1] == ' ' {
			s := strings.TrimSpace(cur.String())
			if s != "" {
				parts = append(parts, s)
			}
			cur.Reset()
			i++ // skip the space
		}
	}
	if rem := strings.TrimSpace(cur.String()); rem != "" {
		parts = append(parts, rem)
	}
	return parts
}

// dedupeStrings removes duplicate strings preserving order.
func dedupeStrings(ss []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// loadPendingProposalIDs scans docs/awareness/proposals/*.yaml and returns
// a set of proposal IDs and failure mode IDs (for duplicate detection).
func loadPendingProposalIDs(docsDir string) map[string]string {
	result := make(map[string]string) // id → filename
	proposalsDir := filepath.Join(docsDir, "proposals")
	entries, err := os.ReadDir(proposalsDir)
	if err != nil {
		return result
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(proposalsDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		// Lightweight parse — just extract proposal.id and failure_modes[*].id.
		var raw struct {
			Proposal struct {
				ID     string `yaml:"id"`
				Status string `yaml:"status"`
			} `yaml:"proposal"`
			FailureModes []struct {
				ID string `yaml:"id"`
			} `yaml:"failure_modes"`
		}
		if err := yaml.Unmarshal(data, &raw); err != nil {
			continue
		}
		if raw.Proposal.ID != "" {
			result[raw.Proposal.ID] = e.Name()
		}
		for _, fm := range raw.FailureModes {
			if fm.ID != "" {
				result[fm.ID] = e.Name()
			}
		}
	}
	return result
}

// findDuplicateProposal checks if a gap_id pattern is already covered by a pending proposal.
// It looks for overlap by checking if any proposal ID contains the gap pattern's last segment,
// or if any failure mode ID contains a substring of the gap ID.
func findDuplicateProposal(gapID string, pendingIDs map[string]string) string {
	gapLower := strings.ToLower(gapID)
	// Extract the leaf portion (after last dot).
	parts := strings.Split(gapID, ".")
	leaf := strings.ToLower(parts[len(parts)-1])

	for proposalID, filename := range pendingIDs {
		proposalLower := strings.ToLower(proposalID)
		if strings.Contains(proposalLower, leaf) || strings.Contains(proposalLower, gapLower) {
			return filename
		}
	}
	return ""
}

// buildSelfReviewSummary generates a human-readable summary of the review.
func buildSelfReviewSummary(gaps []capabilityGapResult, closed []closedGapResult, incomplete []incompleteCriticism) string {
	openCount := 0
	for _, g := range gaps {
		if !g.AlreadyProposed {
			openCount++
		}
	}
	var parts []string
	if openCount > 0 {
		parts = append(parts, fmt.Sprintf("%d open gap(s) identified", openCount))
	}
	if len(closed) > 0 {
		parts = append(parts, fmt.Sprintf("%d already-implemented gap(s)", len(closed)))
	}
	if len(incomplete) > 0 {
		parts = append(parts, fmt.Sprintf("%d vague criticism(s) marked incomplete", len(incomplete)))
	}
	proposed := 0
	for _, g := range gaps {
		if g.AlreadyProposed {
			proposed++
		}
	}
	if proposed > 0 {
		parts = append(parts, fmt.Sprintf("%d gap(s) already have pending proposals", proposed))
	}
	if len(parts) == 0 {
		return "No gaps identified — feedback matched no known capability gap patterns."
	}
	return strings.Join(parts, "; ") + "."
}

// ---------------------------------------------------------------------------
// Test verification for closed gaps
// ---------------------------------------------------------------------------

// isValidTestFuncName returns true if s is a valid Go test function name:
// starts with "Test" followed by an uppercase letter (e.g. TestFoo_Bar).
func isValidTestFuncName(s string) bool {
	return len(s) >= 5 && strings.HasPrefix(s, "Test") && s[4] >= 'A' && s[4] <= 'Z'
}

// verifyGapTests checks whether the test function names listed in testsRequired
// can be found in any *_test.go file under golang/awareness/ in the repo.
// This prevents self_review from reporting false confidence when a gap is marked
// "implemented" in agent_playbooks.yaml but its required tests no longer exist.
//
// Returns a verification status string and a human-readable note.
// Status values: "tests_found" | "tests_partial" | "tests_not_found" |
//                "no_tests_required" | "unverified"
func verifyGapTests(repoRoot string, testsRequired []string) (status string, note string) {
	if len(testsRequired) == 0 {
		return "no_tests_required", ""
	}
	if repoRoot == "" {
		return "unverified", "repo root unavailable — cannot scan test files"
	}

	testDir := filepath.Join(repoRoot, "golang", "awareness")
	// Normalize: strip trailing annotation notes like "(not objectstore)" from
	// entries such as "TestFoo (note)" so only the Go function name is matched.
	normalized := make([]string, len(testsRequired))
	for i, entry := range testsRequired {
		if idx := strings.IndexByte(entry, ' '); idx >= 0 {
			normalized[i] = strings.TrimSpace(entry[:idx])
		} else {
			normalized[i] = strings.TrimSpace(entry)
		}
	}

	// Reject description-style entries before scanning. An entry like
	// "etcd NOSPACE in journalctl text → etcd failure mode matched" normalizes
	// to "etcd" — scanning for func etcd( always returns zero matches and
	// silently produces a misleading tests_not_found result.
	// Valid entries must start with "Test" followed by an uppercase letter.
	var invalidEntries []string
	for i, norm := range normalized {
		if !isValidTestFuncName(norm) {
			invalidEntries = append(invalidEntries, fmt.Sprintf("%q (from %q)", norm, testsRequired[i]))
		}
	}
	if len(invalidEntries) > 0 {
		return "invalid_metadata", fmt.Sprintf(
			"tests_required contains %d non-function-name entry(ies) — use exact Go func names starting with TestXxx: %s",
			len(invalidEntries), strings.Join(invalidEntries, "; "))
	}

	found := make(map[string]bool)

	_ = filepath.Walk(testDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		content := string(data)
		for _, testName := range normalized {
			if strings.Contains(content, "func "+testName+"(") {
				found[testName] = true
			}
		}
		return nil
	})

	foundCount := len(found)
	total := len(normalized)

	switch {
	case foundCount == total:
		return "tests_found", fmt.Sprintf("%d/%d required tests found", foundCount, total)
	case foundCount > 0:
		var missing []string
		for _, name := range normalized {
			if !found[name] {
				missing = append(missing, name)
			}
		}
		return "tests_partial", fmt.Sprintf("%d/%d found; missing: %s", foundCount, total, strings.Join(missing, ", "))
	default:
		return "tests_not_found", fmt.Sprintf("0/%d required tests found in golang/awareness/", total)
	}
}
