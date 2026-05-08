package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// causalRulesFile is the path within the knowledge dir.
const causalRulesFile = "knowledge/causal_rules.yaml"

// causalRule defines a sequence-based failure chain rule.
type causalRule struct {
	ID                  string           `yaml:"id"`
	RootSignal          string           `yaml:"root_signal"`
	TriggerKeywords     []string         `yaml:"trigger_keywords"`
	Sequence            []causalStep     `yaml:"sequence"`
	Confidence          string           `yaml:"confidence"`
	ExplanationTemplate string           `yaml:"explanation_template"`
	RecommendedFixOrder []string         `yaml:"recommended_fix_order"`
}

type causalStep struct {
	Event     string   `yaml:"event"`
	Component string   `yaml:"component"`
	Keywords  []string `yaml:"keywords"`
}

type causalRulesDoc struct {
	Rules []causalRule `yaml:"rules"`
}

// causalChainResult is one matched causal chain.
type causalChainResult struct {
	ChainID             string            `json:"chain_id"`
	Confidence          string            `json:"confidence"`
	RootSignal          string            `json:"root_signal"`
	MatchedSteps        int               `json:"matched_steps"`
	TotalSteps          int               `json:"total_steps"`
	Events              []chainEventMatch `json:"events"`
	Explanation         string            `json:"explanation"`
	RecommendedFixOrder []string          `json:"recommended_fix_order"`
	BlindSpots          []string          `json:"blind_spots,omitempty"`
}

type chainEventMatch struct {
	Order     int    `json:"order"`
	Event     string `json:"event"`
	Component string `json:"component"`
	Evidence  string `json:"evidence"`
	Matched   bool   `json:"matched"`
}

func registerCausalChainTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.causal_chain",
		Description: "Identify root causal chains from log events or offline evidence. Matches multi-step failure sequences against known causal rules. Confidence is heuristic — always check blind_spots. Use alongside awareness.offline_diagnose and awareness.explain_symptom.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"snapshot_id":      {Type: "string", Description: "Optional snapshot ID from awareness.runtime_snapshot."},
				"live":             {Type: "boolean", Description: "If true, collect live evidence from runtime snapshot (requires cluster access)."},
				"events":           {Type: "array", Description: "Explicit text event strings to match against.", Items: &propSchema{Type: "string"}},
				"offline_evidence": {Type: "string", Description: "Combined log/journalctl text to extract events from."},
				"time_window":      {Type: "string", Description: "Optional time window, e.g. '1h'."},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		_ = ctx

		explicitEvents := strSliceArg(args, "events")
		offlineEvidence := strArg(args, "offline_evidence")

		// --- Load causal rules ---
		docsDir := st.docsDir
		rules, err := loadCausalRules(docsDir)
		if err != nil || len(rules) == 0 {
			return map[string]interface{}{
				"chains":      []causalChainResult{},
				"confidence":  "unknown",
				"blind_spots": []string{"causal_rules.yaml not found or empty — no rules loaded"},
			}, nil
		}

		// --- Collect all evidence text ---
		var allText []string
		allText = append(allText, explicitEvents...)
		if offlineEvidence != "" {
			allText = append(allText, extractTextLines(offlineEvidence)...)
		}

		combined := strings.Join(allText, " \n ")

		// --- Match rules ---
		var chains []causalChainResult
		for _, rule := range rules {
			chain := matchCausalRule(rule, combined, allText)
			if chain != nil {
				chains = append(chains, *chain)
			}
		}

		// Sort chains: high confidence first, then by step coverage.
		sort.Slice(chains, func(i, j int) bool {
			ci := confidenceOrder(chains[i].Confidence)
			cj := confidenceOrder(chains[j].Confidence)
			if ci != cj {
				return ci > cj
			}
			coverageI := float64(chains[i].MatchedSteps) / float64(chains[i].TotalSteps)
			coverageJ := float64(chains[j].MatchedSteps) / float64(chains[j].TotalSteps)
			return coverageI > coverageJ
		})

		overallConfidence := "unknown"
		var blindSpots []string
		if len(allText) == 0 {
			blindSpots = append(blindSpots, "no events or offline_evidence provided")
		}
		if len(chains) > 0 {
			overallConfidence = chains[0].Confidence
		} else if len(allText) > 0 {
			blindSpots = append(blindSpots, "no causal chains matched — symptoms may be outside known rule set")
		}
		if docsDir == "" {
			blindSpots = append(blindSpots, "docs dir not configured — rules loaded from embedded defaults only")
		}

		return map[string]interface{}{
			"chains":      chains,
			"confidence":  overallConfidence,
			"blind_spots": blindSpots,
		}, nil
	})
}

// matchCausalRule scores a rule against the combined evidence text.
// Returns nil if fewer than 50% of steps match.
func matchCausalRule(rule causalRule, combined string, allLines []string) *causalChainResult {
	totalSteps := len(rule.Sequence)
	if totalSteps == 0 {
		return nil
	}

	var matchedEvents []chainEventMatch
	matchedCount := 0

	for i, step := range rule.Sequence {
		matched, evidence := stepMatchesEvidence(step, combined, allLines)
		matchedEvents = append(matchedEvents, chainEventMatch{
			Order:     i + 1,
			Event:     step.Event,
			Component: step.Component,
			Evidence:  evidence,
			Matched:   matched,
		})
		if matched {
			matchedCount++
		}
	}

	// Below 50% threshold → not returned.
	if float64(matchedCount)/float64(totalSteps) < 0.5 {
		return nil
	}

	// Build blind spots for unmatched steps.
	var blindSpots []string
	for _, ev := range matchedEvents {
		if !ev.Matched {
			blindSpots = append(blindSpots, fmt.Sprintf("step %q not matched in evidence", ev.Event))
		}
	}

	// Generate chain ID from rule ID + a short hash of the evidence.
	hash := fmt.Sprintf("%x", md5.Sum([]byte(combined)))[:8]
	chainID := fmt.Sprintf("causal-%s-%s", rule.ID, hash)

	return &causalChainResult{
		ChainID:             chainID,
		Confidence:          rule.Confidence,
		RootSignal:          rule.RootSignal,
		MatchedSteps:        matchedCount,
		TotalSteps:          totalSteps,
		Events:              matchedEvents,
		Explanation:         rule.ExplanationTemplate,
		RecommendedFixOrder: rule.RecommendedFixOrder,
		BlindSpots:          blindSpots,
	}
}

// stepMatchesEvidence returns true if any of the step's keywords appear in the combined text.
func stepMatchesEvidence(step causalStep, combined string, allLines []string) (bool, string) {
	lower := strings.ToLower(combined)
	for _, kw := range step.Keywords {
		kwLower := strings.ToLower(kw)
		if strings.Contains(lower, kwLower) {
			// Find the matching line for the evidence snippet.
			for _, line := range allLines {
				if strings.Contains(strings.ToLower(line), kwLower) {
					return true, truncate(strings.TrimSpace(line), 120)
				}
			}
			return true, kw + " detected in evidence"
		}
	}
	return false, ""
}

// extractTextLines splits a multiline text blob into individual lines.
func extractTextLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// loadCausalRules loads the causal rules YAML from the docs dir.
func loadCausalRules(docsDir string) ([]causalRule, error) {
	if docsDir == "" {
		return nil, fmt.Errorf("docs dir not configured")
	}
	path := filepath.Join(docsDir, causalRulesFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read causal_rules.yaml: %w", err)
	}
	var doc causalRulesDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse causal_rules.yaml: %w", err)
	}
	return doc.Rules, nil
}

func confidenceOrder(c string) int {
	switch c {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}
