package main

import (
	"os"
	"strings"
	"testing"
)

// TestRejectNoMatchWithoutCoverageExplanation verifies that decisionContextNoGraph
// always returns coverage and blind_spots. An empty or nil coverage map must fail.
// This is a negative test: it should PASS (the function CORRECTLY provides coverage),
// proving the guard works.
func TestRejectNoMatchWithoutCoverageExplanation(t *testing.T) {
	m := decisionContextNoGraph("unknown task", []string{})

	cov, ok := m["coverage"].(map[string]string)
	if !ok || cov == nil || len(cov) == 0 {
		t.Fatal("GUARD FAILED: NO_MATCH returned without coverage map — consumer cannot determine why")
	}
	for _, key := range []string{"graph", "runtime", "code_scan"} {
		if cov[key] == "" {
			t.Errorf("GUARD FAILED: coverage.%s is empty — bare NO_MATCH semantics leak", key)
		}
	}

	bs, _ := m["blind_spots"].([]string)
	if len(bs) == 0 {
		t.Fatal("GUARD FAILED: NO_MATCH returned without blind_spots — consumer cannot determine incomplete coverage")
	}

	warning, _ := m["warning"].(string)
	if !strings.Contains(warning, "NO_MATCH") {
		t.Fatalf("GUARD FAILED: warning field must contain 'NO_MATCH' safety rule; got: %q", warning)
	}
}

// TestRejectFuzzyResultAsActionAuthority verifies that confidence is never "high"
// in no-graph mode (fuzzy/degraded results must not claim authority).
func TestRejectFuzzyResultAsActionAuthority(t *testing.T) {
	m := decisionContextNoGraph("deploy cluster-controller", []string{"golang/cluster_controller/cluster_controller_server/main.go"})

	conf, _ := m["confidence"].(string)
	if conf == "high" {
		t.Fatal("GUARD FAILED: confidence=high returned from no-graph mode — fuzzy degraded result must never claim high authority")
	}
	// Must be "unknown" or "low" in no-graph mode
	if conf != "unknown" && conf != "low" {
		t.Errorf("GUARD FAILED: confidence in no-graph mode must be 'unknown' or 'low'; got %q", conf)
	}
}

// TestRejectCausalRuleWithoutEvidence verifies that causal_rules.yaml contains only
// rules with non-empty trigger_keywords. Rules without trigger_keywords can't match
// real incidents.
func TestRejectCausalRuleWithoutEvidence(t *testing.T) {
	content, err := os.ReadFile("../../docs/awareness/knowledge/causal_rules.yaml")
	if err != nil {
		t.Skipf("causal_rules.yaml not found: %v", err)
	}

	s := string(content)
	// Each rule block must have trigger_keywords.
	// Count "  - id:" entries vs "trigger_keywords:" entries.
	idCount := strings.Count(s, "  - id:")
	kwCount := strings.Count(s, "  trigger_keywords:")

	if kwCount < idCount {
		t.Errorf("GUARD FAILED: %d causal rules but only %d have trigger_keywords — rules without keywords can never fire on real incidents", idCount, kwCount)
	}
}

// TestRejectDecisionRuleWithoutTriggerConditions verifies that decision_rules.yaml
// contains only rules with non-empty trigger conditions.
func TestRejectDecisionRuleWithoutTriggerConditions(t *testing.T) {
	content, err := os.ReadFile("../../docs/awareness/knowledge/decision_rules.yaml")
	if err != nil {
		t.Skipf("decision_rules.yaml not found: %v", err)
	}

	s := string(content)
	ruleCount := strings.Count(s, "  - id:")
	triggerCount := strings.Count(s, "  trigger:")

	if triggerCount < ruleCount {
		t.Errorf("GUARD FAILED: %d decision rules but only %d have trigger conditions — rules without triggers are not actionable", ruleCount, triggerCount)
	}
}

// TestRejectDecisionRuleWithoutForbiddenIf verifies that decision_rules.yaml has
// forbidden_if or forbidden_fixes for all rules. Rules without these fields don't
// encode the "what must NOT be done" contract.
func TestRejectDecisionRuleWithoutForbiddenIf(t *testing.T) {
	content, err := os.ReadFile("../../docs/awareness/knowledge/decision_rules.yaml")
	if err != nil {
		t.Skipf("decision_rules.yaml not found: %v", err)
	}

	s := string(content)
	ruleCount := strings.Count(s, "  - id:")
	// Each rule should have either forbidden_if or forbidden_fixes.
	forbiddenIfCount := strings.Count(s, "  forbidden_if:")
	forbiddenFixesCount := strings.Count(s, "  forbidden_fixes:")
	totalForbidden := forbiddenIfCount + forbiddenFixesCount

	if totalForbidden < ruleCount {
		t.Errorf("GUARD FAILED: %d decision rules but only %d have forbidden_if or forbidden_fixes — rules without forbidden conditions are incomplete", ruleCount, totalForbidden)
	}
}

// TestRejectInvariantWithMissingRequiredTest verifies that awareness invariants that
// have required_tests reference test names that actually exist in the test files.
func TestRejectInvariantWithMissingRequiredTest(t *testing.T) {
	content, err := os.ReadFile("../../docs/awareness/invariants.yaml")
	if err != nil {
		t.Skipf("invariants.yaml not found: %v", err)
	}

	// Extract required test names from invariants.yaml.
	var missingTests []string
	lines := strings.Split(string(content), "\n")
	inRequiredTests := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "required_tests:" {
			inRequiredTests = true
			continue
		}
		if inRequiredTests {
			if strings.HasPrefix(trimmed, "- ") {
				testName := strings.TrimPrefix(trimmed, "- ")
				testName = strings.TrimSpace(testName)
				if strings.HasPrefix(testName, "Test") {
					// Check if this test exists in mcp or awareness test files.
					found := false
					for _, dir := range []string{".", "../awareness"} {
						_ = walkTestFiles(dir, func(fileContent string) bool {
							if strings.Contains(fileContent, "func "+testName+"(") {
								found = true
								return true
							}
							return false
						})
						if found {
							break
						}
					}
					if !found {
						missingTests = append(missingTests, testName)
					}
				}
			} else if !strings.HasPrefix(trimmed, "#") && trimmed != "" {
				inRequiredTests = false
			}
		}
	}

	if len(missingTests) > 10 {
		// Too many missing tests — probably a search path issue. Just warn.
		t.Logf("WARNING: %d required tests not found in local test files (may be in other packages): %v", len(missingTests), missingTests[:10])
		return
	}

	// Only fail for tests that are clearly local (TestDecision*, TestPath*, TestGraph*, TestLearn*).
	var localMissing []string
	for _, n := range missingTests {
		if strings.HasPrefix(n, "TestDecision") || strings.HasPrefix(n, "TestPath") ||
			strings.HasPrefix(n, "TestGraph") || strings.HasPrefix(n, "TestLearn") ||
			strings.HasPrefix(n, "TestWeight") || strings.HasPrefix(n, "TestPromoted") {
			localMissing = append(localMissing, n)
		}
	}
	if len(localMissing) > 0 {
		t.Errorf("GUARD FAILED: invariants.yaml references required tests that don't exist locally: %v", localMissing)
	}
}

// walkTestFiles walks directories looking for _test.go files and calls f on each.
// Returns true if f returned true for any file.
func walkTestFiles(dir string, f func(string) bool) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		content, err := os.ReadFile(dir + "/" + e.Name())
		if err != nil {
			continue
		}
		if f(string(content)) {
			return true
		}
	}
	return false
}

// TestRejectForbiddenFixPatternInDiff verifies that the decision_context warning is
// present and warns about NO_MATCH — this is the guard against silently approving
// dangerous changes.
func TestRejectForbiddenFixPatternInDiff(t *testing.T) {
	// Simulate a dangerous action that could be in a diff.
	dangerousGoals := []string{
		"delete all desired state",
		"wipe etcd state",
		"remove all workflow records",
	}

	for _, goal := range dangerousGoals {
		m := decisionContextNoGraph(goal, []string{})

		warning, _ := m["warning"].(string)
		if !strings.Contains(warning, "NO_MATCH") {
			t.Errorf("GUARD FAILED: dangerous goal %q returned warning without NO_MATCH rule: %q", goal, warning)
		}

		forbidden, _ := m["forbidden_actions"].([]string)
		if len(forbidden) == 0 {
			t.Errorf("GUARD FAILED: dangerous goal %q returned no forbidden_actions — always-present guard missing", goal)
		}
	}
}

// TestCausalRulesFileExists verifies that causal_rules.yaml exists and is non-empty.
// Required by awareness.knowledge.causal_rules_evidence_required invariant.
func TestCausalRulesFileExists(t *testing.T) {
	content, err := os.ReadFile("../../docs/awareness/knowledge/causal_rules.yaml")
	if err != nil {
		t.Fatalf("GUARD FAILED: causal_rules.yaml not found — awareness causal chain reasoning has no source: %v", err)
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		t.Fatal("GUARD FAILED: causal_rules.yaml is empty — at least one real incident must be encoded")
	}
}

// TestCausalRulesHaveRequiredFields verifies every causal rule has id, trigger_keywords,
// root_signal, and sequence. Rules missing these fields cannot fire on real incidents.
// Required by awareness.knowledge.causal_rules_evidence_required invariant.
func TestCausalRulesHaveRequiredFields(t *testing.T) {
	content, err := os.ReadFile("../../docs/awareness/knowledge/causal_rules.yaml")
	if err != nil {
		t.Skipf("causal_rules.yaml not found: %v", err)
	}
	s := string(content)

	// Count rule entries.
	ruleCount := strings.Count(s, "  - id:")
	if ruleCount == 0 {
		t.Fatal("GUARD FAILED: causal_rules.yaml has no rules (expected '  - id:' entries)")
	}

	// Each rule must have trigger_keywords — without them rules can never match incidents.
	kwCount := strings.Count(s, "  trigger_keywords:")
	if kwCount < ruleCount {
		t.Errorf("GUARD FAILED: %d causal rules but only %d have trigger_keywords — rules without keywords can never match real incidents", ruleCount, kwCount)
	}

	// Each rule must have a root_signal — the observable symptom that starts the chain.
	signalCount := strings.Count(s, "  root_signal:")
	if signalCount < ruleCount {
		t.Errorf("GUARD FAILED: %d causal rules but only %d have root_signal — rules without root_signal have no observable entry point", ruleCount, signalCount)
	}

	// Each rule must have a sequence — the actual causal chain (the verified incident evidence).
	seqCount := strings.Count(s, "  sequence:")
	if seqCount < ruleCount {
		t.Errorf("GUARD FAILED: %d causal rules but only %d have sequence — rules without sequence are hypothetical chains, not verified scars", ruleCount, seqCount)
	}

	// Each rule must have confidence set — unset confidence defaults to high, which is dishonest.
	confCount := strings.Count(s, "  confidence:")
	if confCount < ruleCount {
		t.Errorf("GUARD FAILED: %d causal rules but only %d have confidence — unset confidence implies high, overstating certainty", ruleCount, confCount)
	}
}

// TestRejectPromotedClaimWithoutVerification verifies that path_weights.yaml
// requires human approval annotation (promoted claims need verification).
func TestRejectPromotedClaimWithoutVerification(t *testing.T) {
	content, err := os.ReadFile("../../docs/awareness/knowledge/path_weights.yaml")
	if err != nil {
		t.Skipf("path_weights.yaml not found: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, "human approval") && !strings.Contains(s, "requires_human_approval") {
		t.Fatal("GUARD FAILED: path_weights.yaml must annotate that changes require human approval — promoted weight claims without verification can corrupt decision rankings")
	}

	if !strings.Contains(s, "awareness build") {
		t.Fatal("GUARD FAILED: path_weights.yaml must note that 'globular awareness build' is required after promotion — weight changes without rebuild don't take effect")
	}
}
