package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testIntentYAML = `id: cluster.membership.earned_trust
level: principle
title: Cluster membership is earned, not assumed
intent: >
  A node becomes a cluster member only after phase-ordered, observable proof.
agent_guidance: >
  When changing join or admission code, never shortcut proof phases.
bad_smells:
  - reachability treated as identity
  - stale etcd membership record used as admission proof
expressed_by:
  - services/golang/cluster_controller/
activation_triggers:
  - node join retry
  - stale membership
  - ghost member
related_invariants:
  - join.token.validated.before.phase
zooms_out_to:
  - globular.security.ceremony_over_configuration
zooms_in_to:
  - join.token.validation
status: seed
`

const testDegradedYAML = `id: degraded_is_explicit_not_hidden
level: principle
title: Degraded state must be visible
intent: >
  When a service dependency is missing or unhealthy, the service must report degraded explicitly.
agent_guidance: >
  Never swallow errors from dependencies. Return a visible degraded or partial status.
bad_smells:
  - health check returns green even when dependency missing
  - error swallowed and status not updated
activation_triggers:
  - health check passes when it should fail
  - silent fallback hiding dependency outage
status: seed
`

func writeTempIntentDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("setup: write %s: %v", name, err)
		}
	}
	return dir
}

func runIntentExplain(t *testing.T, intentDir, query string, limit int) string {
	t.Helper()
	t.Setenv("GLOBULAR_INTENT_DIR", intentDir)

	nodes, _ := loadIntentNodes(intentDir)
	var matches []intentMatch
	for _, n := range nodes {
		sc, reason := scoreNode(query, n)
		if sc > 0 {
			matches = append(matches, intentMatch{node: n, score: sc, reason: reason})
		}
	}

	if limit <= 0 {
		limit = 5
	}
	if len(matches) > limit {
		matches = matches[:limit]
	}

	if len(matches) == 0 {
		return "No intent node matched this query."
	}

	var parts []string
	for i, m := range matches {
		parts = append(parts, formatNode(i+1, m))
	}
	return strings.Join(parts, "\n---\n")
}

func TestIntentExplain_ExactConceptID(t *testing.T) {
	dir := writeTempIntentDir(t, map[string]string{
		"cluster.membership.earned_trust.yaml": testIntentYAML,
	})
	out := runIntentExplain(t, dir, "cluster.membership.earned_trust", 5)
	if !strings.Contains(out, "cluster.membership.earned_trust") {
		t.Errorf("expected node id in output, got:\n%s", out)
	}
}

func TestIntentExplain_FilePath(t *testing.T) {
	dir := writeTempIntentDir(t, map[string]string{
		"cluster.membership.earned_trust.yaml": testIntentYAML,
	})
	out := runIntentExplain(t, dir, "services/golang/cluster_controller/join_authorize.go", 5)
	if !strings.Contains(out, "cluster.membership.earned_trust") {
		t.Errorf("expected earned_trust node from file path match, got:\n%s", out)
	}
}

func TestIntentExplain_ActivationTrigger(t *testing.T) {
	dir := writeTempIntentDir(t, map[string]string{
		"cluster.membership.earned_trust.yaml": testIntentYAML,
	})
	out := runIntentExplain(t, dir, "stale membership after node rejoin", 5)
	if !strings.Contains(out, "cluster.membership.earned_trust") {
		t.Errorf("expected earned_trust node from activation_trigger match, got:\n%s", out)
	}
}

func TestIntentExplain_BadSmell(t *testing.T) {
	dir := writeTempIntentDir(t, map[string]string{
		"degraded_is_explicit_not_hidden.yaml": testDegradedYAML,
	})
	out := runIntentExplain(t, dir, "health check returns green even when dependency missing", 5)
	if !strings.Contains(out, "degraded_is_explicit_not_hidden") {
		t.Errorf("expected degraded node from bad_smell match, got:\n%s", out)
	}
}

func TestIntentExplain_NoMatch(t *testing.T) {
	dir := writeTempIntentDir(t, map[string]string{
		"cluster.membership.earned_trust.yaml": testIntentYAML,
	})
	out := runIntentExplain(t, dir, "frontend button color", 5)
	if !strings.Contains(out, "No intent node matched") {
		t.Errorf("expected no-match message, got:\n%s", out)
	}
}

func TestIntentExplain_BadYAMLContinues(t *testing.T) {
	dir := writeTempIntentDir(t, map[string]string{
		"bad.yaml":                             "id: bad\n  broken: [\n",
		"cluster.membership.earned_trust.yaml": testIntentYAML,
	})
	nodes, diags := loadIntentNodes(dir)
	if len(diags) == 0 {
		t.Error("expected diagnostics for broken YAML")
	}
	found := false
	for _, n := range nodes {
		if n.ID == "cluster.membership.earned_trust" {
			found = true
		}
	}
	if !found {
		t.Error("expected good node to be loaded despite bad YAML in another file")
	}
}
