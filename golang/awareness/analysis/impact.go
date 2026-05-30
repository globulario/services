// Package analysis provides graph-based impact analysis, cycle detection,
// and agent context generation. No LLM calls — all matching is graph traversal
// and keyword matching against manually declared truth.
package analysis

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/integrity"
)

// ImpactResult collects nodes reachable from a source file, partitioned by type.
//
// The Invariants and FailureModes slices are kept for back-compat with existing
// callers; new code should read DirectInvariants / InferredInvariants (and the
// failure_mode pair) separately because the two tiers carry very different
// signal:
//
//   - Direct = the file is the explicit subject of the invariant: it appears
//     in protects.{files, enforces_files, configures_files, observes_files}
//     in docs/awareness/invariants.yaml. These matches are file-specific by
//     construction.
//
//   - Inferred = the invariant is reachable via the 6-hop traversal but the
//     file is not its explicit subject. These matches come from package /
//     symbol / service edges (e.g. file → defines → symbol → ... → invariant
//     or file → owned-by → service → ... → invariant) and they SHARE across
//     siblings in the same package. They are useful broad context but they
//     should not dominate output for an agent that needs to know what's
//     specific to the file it's about to edit.
//
// Invariants and FailureModes are populated as DirectInvariants ++
// InferredInvariants (and the FailureModes equivalent) so the back-compat
// slices are still Direct-first, which fixes ranking for legacy consumers
// without requiring them to know about the partition.
type ImpactResult struct {
	SourceFile     *graph.Node
	Symbols        []*graph.Node
	Services       []*graph.Node
	Invariants     []*graph.Node // back-compat: DirectInvariants ++ InferredInvariants
	FailureModes   []*graph.Node // back-compat: DirectFailureModes ++ InferredFailureModes
	ForbiddenFixes []*graph.Node
	Tests          []*graph.Node
	Other          []*graph.Node

	// DirectInvariants are invariants that explicitly name this file as their
	// subject via a 1-hop implements / enforces / configures / observes edge
	// from the file to the invariant. They are the file-specific signal.
	DirectInvariants []*graph.Node

	// InferredInvariants are invariants reachable through the broader graph
	// walk (package, symbol, service edges) but where the file is not the
	// invariant's explicit subject. They typically bleed across siblings in
	// the same package and should be presented as lower-rank context.
	InferredInvariants []*graph.Node

	// DirectFailureModes are failure_modes related (via 1-hop "affects" edge)
	// to at least one DirectInvariant. They are 2 hops from the file but still
	// file-specific by virtue of their anchor.
	DirectFailureModes []*graph.Node

	// InferredFailureModes are failure_modes reached only via the broader walk.
	InferredFailureModes []*graph.Node
}

// ImpactByFile finds all nodes impacted by changes to the file at filePath,
// then partitions them by type into an ImpactResult. Invariants and
// failure_modes are additionally split into Direct (file-specific anchor) and
// Inferred (reached via broader package / symbol / service walks) tiers — see
// the ImpactResult docstring for the rationale.
func ImpactByFile(ctx context.Context, g *graph.Graph, filePath string) (*ImpactResult, error) {
	res, err := g.ImpactByFile(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("ImpactByFile %s: %w", filePath, err)
	}

	directInvIDs, err := directInvariantIDsForFile(ctx, g, filePath)
	if err != nil {
		return nil, fmt.Errorf("ImpactByFile %s: direct invariants: %w", filePath, err)
	}
	directFMIDs, err := failureModesAffectedByInvariants(ctx, g, directInvIDs)
	if err != nil {
		return nil, fmt.Errorf("ImpactByFile %s: direct failure modes: %w", filePath, err)
	}

	result := &ImpactResult{}
	seen := make(map[string]bool)

	for _, n := range res.Nodes {
		if seen[n.ID] {
			continue
		}
		seen[n.ID] = true

		switch n.Type {
		case graph.NodeTypeSourceFile:
			if n.Path == filePath {
				result.SourceFile = n
			}
		case graph.NodeTypeSymbol:
			result.Symbols = append(result.Symbols, n)
		case graph.NodeTypeGlobularService:
			result.Services = append(result.Services, n)
		case graph.NodeTypeInvariant:
			if directInvIDs[n.ID] {
				result.DirectInvariants = append(result.DirectInvariants, n)
			} else {
				result.InferredInvariants = append(result.InferredInvariants, n)
			}
		case graph.NodeTypeFailureMode:
			if directFMIDs[n.ID] {
				result.DirectFailureModes = append(result.DirectFailureModes, n)
			} else {
				result.InferredFailureModes = append(result.InferredFailureModes, n)
			}
		case graph.NodeTypeForbiddenFix:
			result.ForbiddenFixes = append(result.ForbiddenFixes, n)
		case graph.NodeTypeTest:
			result.Tests = append(result.Tests, n)
		default:
			result.Other = append(result.Other, n)
		}
	}

	// Back-compat: populate the legacy flat slices Direct-first. Existing
	// consumers that read result.Invariants get the better ranking with no
	// code change on their side.
	result.Invariants = append(append([]*graph.Node{}, result.DirectInvariants...), result.InferredInvariants...)
	result.FailureModes = append(append([]*graph.Node{}, result.DirectFailureModes...), result.InferredFailureModes...)

	return result, nil
}

// directInvariantIDsForFile returns the set of invariant node IDs that name
// the file at filePath as their explicit subject. "Explicit subject" means
// the file appears in protects.{files, enforces_files, configures_files,
// observes_files} of the invariant — the loader translates each of those
// into a 1-hop edge from the file to the invariant using the matching edge
// kind (implements / enforces / configures / observes). Returns an empty set
// (not nil) when no source_file node exists for the path so callers can
// always check membership safely.
func directInvariantIDsForFile(ctx context.Context, g *graph.Graph, filePath string) (map[string]bool, error) {
	out := map[string]bool{}
	fileID := "source_file:" + filePath
	edges, err := g.Neighbors(ctx, fileID, "out")
	if err != nil {
		return nil, err
	}
	for _, e := range edges {
		switch e.Kind {
		case graph.EdgeImplements, graph.EdgeEnforces, graph.EdgeConfigures, graph.EdgeObserves:
			if strings.HasPrefix(e.Dst, "invariant:") || strings.HasPrefix(e.Dst, "inv:") {
				out[e.Dst] = true
			}
		}
	}
	return out, nil
}

// failureModesAffectedByInvariants walks 1-hop affects edges from each
// invariant in invIDs and returns the set of failure_mode node IDs they
// reach. The invariant→failure_mode "affects" edge is how
// related_failure_modes entries land in the graph, so this is exactly the
// "failure_modes named on a direct invariant" set — 2 hops total from the
// file but still file-specific by construction.
func failureModesAffectedByInvariants(ctx context.Context, g *graph.Graph, invIDs map[string]bool) (map[string]bool, error) {
	out := map[string]bool{}
	for invID := range invIDs {
		edges, err := g.Neighbors(ctx, invID, "out")
		if err != nil {
			return nil, err
		}
		for _, e := range edges {
			if e.Kind == graph.EdgeAffects && (strings.HasPrefix(e.Dst, "failure_mode:") || strings.HasPrefix(e.Dst, "fm:")) {
				out[e.Dst] = true
			}
		}
	}
	return out, nil
}

// ExplainedFinding is a single impacted node with its graph path trace.
type ExplainedFinding struct {
	NodeID     string
	NodeType   string
	NodeName   string
	Severity   string
	Mandatory  bool     // true if reached via required/enforces/implements edges or is a ForbiddenFix
	EdgePath   []string // human-readable hops: "file → implements → invariant → forbids → forbidden_fix"
	Confidence string   // high/medium/low
	Source     string   // "invariant:xxx" or "" if direct
}

// ExplainedImpactResult is the full explained impact output.
type ExplainedImpactResult struct {
	File           string
	Invariants     []ExplainedFinding
	ForbiddenFixes []ExplainedFinding
	RequiredTests  []ExplainedFinding
	FailureModes   []ExplainedFinding
	MissingLinks   []string // suggested edges not yet in graph
}

// ExplainImpactByFile returns an explained impact result with full graph path traces.
// It calls TraverseImpactPaths and builds ExplainedFinding entries for each high-value
// terminal node, deduplicating by NodeID (keeping shortest path).
func ExplainImpactByFile(ctx context.Context, g *graph.Graph, filePath string) (*ExplainedImpactResult, error) {
	q := integrity.ImpactPathQuery{
		ChangedFiles: []string{filePath},
		MaxDepth:     6,
	}
	paths, err := integrity.TraverseImpactPaths(ctx, g, q)
	if err != nil {
		return nil, fmt.Errorf("ExplainImpactByFile %s: %w", filePath, err)
	}

	result := &ExplainedImpactResult{File: filePath}

	// Track best (shortest) path per NodeID.
	bestByID := make(map[string]ExplainedFinding)

	for _, p := range paths {
		if len(p.Steps) == 0 {
			continue
		}

		terminal := p.Steps[len(p.Steps)-1]

		// Build human-readable edge path.
		edgePath := buildEdgePath(filePath, p.Steps)

		// Determine mandatory: ForbiddenFix is always mandatory;
		// paths containing enforces/requires_test/implements are also mandatory.
		mandatory := terminal.NodeType == graph.NodeTypeForbiddenFix
		if !mandatory {
			for _, s := range p.Steps {
				if s.Predicate == graph.EdgeEnforces ||
					s.Predicate == graph.EdgeRequiresTest ||
					s.Predicate == graph.EdgeImplements {
					mandatory = true
					break
				}
			}
		}

		// Find the invariant source (first invariant in the path).
		source := ""
		for _, s := range p.Steps {
			if s.NodeType == graph.NodeTypeInvariant {
				source = "invariant:" + s.NodeName
				break
			}
		}

		// Resolve severity for invariant nodes from the graph record.
		severity := ""
		if terminal.NodeType == graph.NodeTypeInvariant {
			// Strip "invariant:" prefix to get the record ID.
			invID := strings.TrimPrefix(terminal.NodeID, "invariant:")
			if inv, _ := g.FindInvariant(ctx, invID); inv != nil {
				severity = inv.Severity
			}
		}

		finding := ExplainedFinding{
			NodeID:     terminal.NodeID,
			NodeType:   terminal.NodeType,
			NodeName:   terminal.NodeName,
			Severity:   severity,
			Mandatory:  mandatory,
			EdgePath:   edgePath,
			Confidence: p.Confidence,
			Source:     source,
		}

		// Deduplicate — keep the shortest path (fewer hops = clearer).
		if existing, ok := bestByID[terminal.NodeID]; !ok || len(p.Steps) < len(existing.EdgePath) {
			bestByID[terminal.NodeID] = finding
		}
	}

	// Partition findings by node type.
	for _, f := range bestByID {
		switch f.NodeType {
		case graph.NodeTypeInvariant:
			result.Invariants = append(result.Invariants, f)
		case graph.NodeTypeForbiddenFix:
			result.ForbiddenFixes = append(result.ForbiddenFixes, f)
		case graph.NodeTypeTest:
			result.RequiredTests = append(result.RequiredTests, f)
		case graph.NodeTypeFailureMode:
			result.FailureModes = append(result.FailureModes, f)
		}
	}

	// Phase 2: rank each partition so mandatory/high-severity items surface first.
	rankFindings(result.Invariants)
	rankFindings(result.ForbiddenFixes)
	rankFindings(result.RequiredTests)
	rankFindings(result.FailureModes)

	// Phase 6: missing-link detection.
	// When no findings were produced (no paths, or only paths with zero steps),
	// explain the coverage gap. This is important: an empty result must NOT be
	// interpreted as "no rules apply" (NO_MATCH ≠ safe to proceed).
	if len(bestByID) == 0 {
		result.MissingLinks = detectMissingLinks(filePath)
	}

	return result, nil
}

// rankFindings sorts a slice of ExplainedFindings in priority order:
//  1. Mandatory items first (implements/enforces path or ForbiddenFix node).
//  2. By severity: critical > high > medium > low > "".
//  3. By confidence: high > medium > low.
//  4. By path length: shorter paths (clearer evidence) ranked higher.
func rankFindings(findings []ExplainedFinding) {
	sort.SliceStable(findings, func(i, j int) bool {
		fi, fj := findings[i], findings[j]
		// Mandatory before non-mandatory.
		if fi.Mandatory != fj.Mandatory {
			return fi.Mandatory
		}
		si, sj := severityRank(fi.Severity), severityRank(fj.Severity)
		if si != sj {
			return si > sj
		}
		ci, cj := confidenceRank(fi.Confidence), confidenceRank(fj.Confidence)
		if ci != cj {
			return ci > cj
		}
		return len(fi.EdgePath) < len(fj.EdgePath)
	})
}

// severityRank maps severity strings to sortable integers (higher = more severe).
func severityRank(s string) int {
	switch strings.ToLower(s) {
	case "critical":
		return 4
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

// confidenceRank maps confidence strings to sortable integers (higher = more confident).
func confidenceRank(c string) int {
	switch strings.ToLower(c) {
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

// detectMissingLinks suggests why a file has no graph edges and what to add.
// These are recommendations only — they do not mutate the graph.
func detectMissingLinks(filePath string) []string {
	var links []string

	// File not indexed at all — most common cause.
	links = append(links, fmt.Sprintf(
		"no graph edges found from %q — run 'globular awareness build --clean' to index this file", filePath))

	// Pattern-based suggestions for well-known high-risk areas.
	type suggestion struct {
		pattern  string
		edgeKind string
		suggest  string
	}

	suggestions := []suggestion{
		{"golang/awareness/", "implements", "This file is in the awareness package. Add it to an invariant's 'files:' list in docs/awareness/invariants.yaml so impact traversal can reach the rules it implements."},
		{"golang/mcp/tools_awareness", "implements", "This is an awareness MCP tool. Add it to awareness.mcp.* invariants in docs/awareness/invariants.yaml with 'files:' so decision_context changes are governed."},
		{"golang/globularcli/awareness", "implements", "This is the awareness CLI. Add it to an awareness invariant's 'files:' list in docs/awareness/invariants.yaml."},
		{"docs/awareness/knowledge/", "configures", "This is an awareness knowledge YAML file. Add it to an invariant's 'configures_files:' list in docs/awareness/invariants.yaml so edits trigger the graph rebuild requirement."},
		{"docs/awareness/invariants.yaml", "configures", "invariants.yaml configures the whole awareness graph. It should appear in 'configures_files:' of awareness.knowledge.graph_rebuild_after_yaml_edit."},
		{"golang/cluster_controller/", "implements", "This is a cluster controller file. Check docs/awareness/invariants.yaml for controller invariants and add this file to the relevant 'files:' or 'enforces_files:' list."},
		{"golang/node_agent/", "implements", "This is a node agent file. Check docs/awareness/invariants.yaml for node agent invariants and add this file to the relevant 'files:' list."},
		{"golang/workflow/", "implements", "This is a workflow engine file. Check for workflow-related invariants and add this file to their 'files:' list."},
	}

	for _, s := range suggestions {
		if strings.Contains(filePath, s.pattern) {
			links = append(links, fmt.Sprintf(
				"missing_link: %q appears to %s an invariant but has no '%s' edge. Suggestion: %s",
				filePath, s.edgeKind, s.edgeKind, s.suggest))
			break
		}
	}

	return links
}

// buildEdgePath constructs a human-readable path string from the impact steps.
// Format: "file →[edge]→ node1 →[edge]→ node2"
func buildEdgePath(startFile string, steps []integrity.ImpactStep) []string {
	if len(steps) == 0 {
		return nil
	}
	parts := make([]string, 0, len(steps)+1)
	prev := startFile
	for _, s := range steps {
		parts = append(parts, fmt.Sprintf("%s →[%s]→ %s", prev, s.Predicate, s.NodeName))
		prev = s.NodeName
	}
	// Return the full chain as a single string (split into hops for clarity).
	return []string{strings.Join(parts, " | ")}
}
