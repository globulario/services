package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// IntentNode is the schema for a docs/intent/*.yaml node.
type IntentNode struct {
	ID                 string   `yaml:"id"`
	Level              string   `yaml:"level"`
	Title              string   `yaml:"title"`
	Intent             string   `yaml:"intent"`
	AgentGuidance      string   `yaml:"agent_guidance"`
	BadSmells          []string `yaml:"bad_smells"`
	ExpressedBy        []string `yaml:"expressed_by"`
	RelatedInvariants  []string `yaml:"related_invariants"`
	ActivationTriggers []string `yaml:"activation_triggers"`
	ZoomsOutTo         []string `yaml:"zooms_out_to"`
	RelatedTo          []string `yaml:"related_to"`
	ZoomsInTo          []string `yaml:"zooms_in_to"`
	Status             string   `yaml:"status"`
}

// intentMatch is a node plus its match score and reason.
type intentMatch struct {
	node   IntentNode
	score  int
	reason string
}

// resolveIntentDir finds docs/intent/ relative to known root candidates.
func resolveIntentDir() (string, bool) {
	// 1. Explicit env var (dev convenience only).
	if env := os.Getenv("GLOBULAR_INTENT_DIR"); env != "" {
		if dirExists(env) {
			return env, true
		}
	}

	// 2. System-installed path — populated by build/deploy pipeline.
	const systemPath = "/var/lib/globular/intent"
	if dirExists(systemPath) {
		return systemPath, true
	}

	// 3. Try repoRoot from git (same helper used by awareness).
	if repoRoot := awarGitRoot(); repoRoot != "" {
		candidate := filepath.Join(repoRoot, "docs", "intent")
		if dirExists(candidate) {
			return candidate, true
		}
	}

	// 4. Walk up from CWD.
	for _, rel := range []string{"docs/intent", "../docs/intent", "../../docs/intent"} {
		abs, err := filepath.Abs(rel)
		if err == nil && dirExists(abs) {
			return abs, true
		}
	}

	return "", false
}

// loadIntentNodes reads all *.yaml files from the intent directory.
// A parse error in one file is included in the diagnostics section but does
// not prevent other files from loading.
func loadIntentNodes(intentDir string) ([]IntentNode, []string) {
	entries, err := os.ReadDir(intentDir)
	if err != nil {
		return nil, []string{fmt.Sprintf("cannot read intent dir %s: %v", intentDir, err)}
	}

	var nodes []IntentNode
	var diags []string

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(intentDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			diags = append(diags, fmt.Sprintf("cannot read %s: %v", e.Name(), err))
			continue
		}
		var node IntentNode
		if err := yaml.Unmarshal(data, &node); err != nil {
			diags = append(diags, fmt.Sprintf("parse error in %s: %v", e.Name(), err))
			continue
		}
		if node.ID != "" {
			nodes = append(nodes, node)
		}
	}

	return nodes, diags
}

// scoreNode returns a score and a brief match reason for the given query against a node.
func scoreNode(query string, node IntentNode) (int, string) {
	q := strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(query, "./"), "/"))
	score := 0
	var reasons []string

	idLower := strings.ToLower(node.ID)
	titleLower := strings.ToLower(node.Title)

	// +100 exact id match
	if q == idLower {
		score += 100
		reasons = append(reasons, "exact id match")
	} else if strings.Contains(idLower, q) || strings.Contains(q, idLower) {
		// +60 id substring
		score += 60
		reasons = append(reasons, "id substring")
	}

	// +60 title substring (only if id didn't match it already at this score)
	if score < 60 && strings.Contains(titleLower, q) {
		score += 60
		reasons = append(reasons, "title substring")
	}

	// +50 expressed_by path / prefix match
	for _, eb := range node.ExpressedBy {
		norm := strings.TrimPrefix(strings.ToLower(eb), "./")
		if strings.Contains(q, norm) || strings.HasPrefix(q, norm) {
			score += 50
			reasons = append(reasons, fmt.Sprintf("expressed_by path prefix %s", eb))
			break
		}
	}

	// +30 activation_trigger substring
	for _, t := range node.ActivationTriggers {
		if strings.Contains(q, strings.ToLower(t)) || strings.Contains(strings.ToLower(t), q) {
			score += 30
			reasons = append(reasons, fmt.Sprintf("activation_trigger: %s", t))
			break
		}
	}

	// +25 bad_smell substring
	for _, bs := range node.BadSmells {
		if strings.Contains(q, strings.ToLower(bs)) || strings.Contains(strings.ToLower(bs), q) {
			score += 25
			reasons = append(reasons, fmt.Sprintf("bad_smell: %s", bs))
			break
		}
	}

	// +20 related_invariant / zoom / related_to
	for _, ri := range node.RelatedInvariants {
		if strings.Contains(strings.ToLower(ri), q) || strings.Contains(q, strings.ToLower(ri)) {
			score += 20
			reasons = append(reasons, fmt.Sprintf("related_invariant: %s", ri))
			break
		}
	}
	for _, z := range append(append(node.ZoomsOutTo, node.ZoomsInTo...), node.RelatedTo...) {
		if strings.Contains(strings.ToLower(z), q) || strings.Contains(q, strings.ToLower(z)) {
			score += 20
			reasons = append(reasons, fmt.Sprintf("zoom/related: %s", z))
			break
		}
	}

	// +1 per token overlap — fallback for multi-word task descriptions
	tokens := strings.Fields(q)
	if len(tokens) > 1 {
		allText := strings.ToLower(strings.Join([]string{
			node.ID, node.Title,
			strings.Join(node.BadSmells, " "),
			strings.Join(node.ActivationTriggers, " "),
			strings.Join(node.RelatedInvariants, " "),
			strings.Join(node.ZoomsOutTo, " "),
			strings.Join(node.ZoomsInTo, " "),
			strings.Join(node.RelatedTo, " "),
		}, " "))
		for _, tok := range tokens {
			if len(tok) > 2 && strings.Contains(allText, tok) {
				score++
			}
		}
	}

	reason := ""
	if len(reasons) > 0 {
		reason = strings.Join(reasons, "; ")
	}
	return score, reason
}

// formatNode renders a single intent node as readable text.
func formatNode(rank int, m intentMatch) string {
	n := m.node
	var sb strings.Builder

	fmt.Fprintf(&sb, "%d. %s\n", rank, n.ID)
	fmt.Fprintf(&sb, "Level: %s | Status: %s\n", n.Level, n.Status)
	if m.reason != "" {
		fmt.Fprintf(&sb, "Match: %s\n", m.reason)
	}

	if n.Title != "" {
		fmt.Fprintf(&sb, "\nTitle: %s\n", n.Title)
	}
	if n.Intent != "" {
		fmt.Fprintf(&sb, "\nIntent:\n%s\n", strings.TrimSpace(n.Intent))
	}
	if n.AgentGuidance != "" {
		fmt.Fprintf(&sb, "\nAgent guidance:\n%s\n", strings.TrimSpace(n.AgentGuidance))
	}
	if len(n.BadSmells) > 0 {
		fmt.Fprintf(&sb, "\nBad smells:\n")
		for _, s := range n.BadSmells {
			fmt.Fprintf(&sb, "  - %s\n", s)
		}
	}
	if len(n.ExpressedBy) > 0 {
		fmt.Fprintf(&sb, "\nExpressed by:\n")
		for _, s := range n.ExpressedBy {
			fmt.Fprintf(&sb, "  - %s\n", s)
		}
	}
	if len(n.RelatedInvariants) > 0 {
		fmt.Fprintf(&sb, "\nRelated invariants:\n")
		for _, s := range n.RelatedInvariants {
			fmt.Fprintf(&sb, "  - %s\n", s)
		}
	}
	if len(n.ActivationTriggers) > 0 {
		fmt.Fprintf(&sb, "\nActivation triggers:\n")
		for _, s := range n.ActivationTriggers {
			fmt.Fprintf(&sb, "  - %s\n", s)
		}
	}
	if len(n.ZoomsOutTo) > 0 {
		fmt.Fprintf(&sb, "\nZooms out to:\n")
		for _, s := range n.ZoomsOutTo {
			fmt.Fprintf(&sb, "  - %s\n", s)
		}
	}
	if len(n.RelatedTo) > 0 {
		fmt.Fprintf(&sb, "\nRelated to:\n")
		for _, s := range n.RelatedTo {
			fmt.Fprintf(&sb, "  - %s\n", s)
		}
	}
	if len(n.ZoomsInTo) > 0 {
		fmt.Fprintf(&sb, "\nZooms in to:\n")
		for _, s := range n.ZoomsInTo {
			fmt.Fprintf(&sb, "  - %s\n", s)
		}
	}

	return sb.String()
}

func registerIntentTools(s *server) {
	s.register(toolDef{
		Name:        "intent_explain",
		Description: "Explain architectural intent for a file, concept id, or task description using docs/intent/*.yaml.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"query": {
					Type:        "string",
					Description: "File path, concept id, title fragment, or task description to explain.",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of matching intent nodes to return. Defaults to 5.",
				},
			},
			Required: []string{"query"},
		},
	}, func(_ context.Context, args map[string]interface{}) (interface{}, error) {
		query, _ := args["query"].(string)
		query = strings.TrimSpace(query)
		if query == "" {
			return "query is required", nil
		}

		limit := 5
		if v, ok := args["limit"]; ok {
			switch n := v.(type) {
			case float64:
				if int(n) > 0 {
					limit = int(n)
				}
			case int:
				if n > 0 {
					limit = n
				}
			}
		}

		intentDir, found := resolveIntentDir()
		if !found {
			return "Intent directory not found: docs/intent", nil
		}

		nodes, diags := loadIntentNodes(intentDir)

		var matches []intentMatch
		for _, n := range nodes {
			sc, reason := scoreNode(query, n)
			if sc > 0 {
				matches = append(matches, intentMatch{node: n, score: sc, reason: reason})
			}
		}

		sort.Slice(matches, func(i, j int) bool {
			return matches[i].score > matches[j].score
		})
		if len(matches) > limit {
			matches = matches[:limit]
		}

		var sb strings.Builder

		if len(diags) > 0 {
			sb.WriteString("Diagnostics:\n")
			for _, d := range diags {
				fmt.Fprintf(&sb, "  - %s\n", d)
			}
			sb.WriteString("\n")
		}

		if len(matches) == 0 {
			sb.WriteString("No intent node matched this query.\n")
			sb.WriteString("Proceed cautiously and consider adding a seed node if this area has recurring architectural meaning.\n")
			return sb.String(), nil
		}

		fmt.Fprintf(&sb, "Relevant intent nodes: %d\n\n", len(matches))
		for i, m := range matches {
			sb.WriteString(formatNode(i+1, m))
			if i < len(matches)-1 {
				sb.WriteString("\n---\n\n")
			}
		}

		return sb.String(), nil
	})
}
