package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/awareness/analysis"
	"github.com/globulario/services/golang/awareness/failuregraph"
	"github.com/globulario/services/golang/awareness/integrity"
	"github.com/globulario/services/golang/awareness/learning"
	"github.com/globulario/services/golang/awareness/preflight"
)

func registerAwarenessPreflightTools(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.preflight",
		Description: "Run a full architecture preflight before editing Globular code. Returns invariants, failure modes, forbidden fixes, did-we-fix status, required tests, and agent instruction.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"task": {
					Type:        "string",
					Description: "Task description (required)",
				},
				"files": {
					Type:        "array",
					Description: "Files you plan to edit (run impact analysis)",
					Items:       &propSchema{Type: "string"},
				},
				"package_path": {
					Type:        "string",
					Description: "Path to package directory containing awareness.yaml",
				},
				"phase": {
					Type:        "string",
					Description: "Dependency phase for cycle detection (recovery, bootstrap, package_install, reconcile)",
				},
				"include_runtime": {
					Type:        "boolean",
					Description: "Collect live runtime snapshot and merge into preflight",
					Default:     false,
				},
				"runtime_policy": {
					Type:        "string",
					Description: "Runtime collection policy: auto (collect if live), never (skip), required (fail if unavailable), offline (use saved snapshot). Default: auto.",
					Default:     "auto",
				},
				"runtime_window": {
					Type:        "string",
					Description: "Lookback window for runtime events/workflows (e.g. 15m, 1h)",
					Default:     "15m",
				},
			},
			Required: []string{"task"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		task := strArg(args, "task")
		if task == "" {
			return nil, fmt.Errorf("task is required")
		}

		opts := preflight.Options{
			Task:        task,
			Files:       getStrSlice(args, "files"),
			PackagePath: strArg(args, "package_path"),
			Phase:       strArg(args, "phase"),
			DocsDir:     st.docsDir,
			RepoRoot:    st.repoRoot,
		}

		runtimePolicy := strArg(args, "runtime_policy")
		if runtimePolicy == "" {
			runtimePolicy = "auto"
		}

		// Resolve runtime_policy to include_runtime flag.
		// "auto": collect if cluster config is detectable; "required": collect, fail if unavailable;
		// "never": skip; "offline": use saved snapshot (treated as never for now — offline_diagnose is separate).
		switch runtimePolicy {
		case "required", "auto":
			opts.IncludeRuntime = true
			opts.Bridge = newLiveBridge(st)
		case "never", "offline":
			opts.IncludeRuntime = false
		default:
			if getBool(args, "include_runtime", false) {
				opts.IncludeRuntime = true
				opts.Bridge = newLiveBridge(st)
			}
		}

		if opts.IncludeRuntime {
			if ws := strArg(args, "runtime_window"); ws != "" {
				if d, err := time.ParseDuration(ws); err == nil {
					opts.RuntimeWindow = d
				}
			}
			if opts.RuntimeWindow == 0 {
				opts.RuntimeWindow = 15 * time.Minute
			}
		}

		r, err := preflight.Run(ctx, opts, st.g)
		if err != nil {
			return nil, fmt.Errorf("preflight: %w", err)
		}

		out, err := preflight.Render(r, preflight.FormatJSON)
		if err != nil {
			return nil, fmt.Errorf("render: %w", err)
		}

		// Return as a raw JSON object (not a string) for clean MCP consumption.
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(out), &result); err == nil {
			// Inject runtime_policy and trust_summary into the result.
			result["runtime_policy"] = runtimePolicy

			// Build trust_summary from matched graph nodes if graph is available.
			if st.g != nil {
				invIDs := getStrSlice(args, "invariants")
				_ = invIDs // main IDs are in the rendered JSON, not re-available here.
				// Instead, include trust_summary by looking at filtered_matches from the report.
				// The trust distribution is pre-computed in the report; expose it directly.
				trustSum := buildTrustSummaryFromReport(r)
				result["trust_summary"] = trustSum
			}
		}
		return result, nil
	})

	s.register(toolDef{
		Name:        "awareness.agent_context",
		Description: "Generate architectural context for a task — invariants, failure modes, forbidden fixes, and required tests from the awareness graph.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"task": {
					Type:        "string",
					Description: "Task description",
				},
				"files": {
					Type:        "array",
					Description: "Files being edited (narrows context)",
					Items:       &propSchema{Type: "string"},
				},
				"services": {
					Type:        "array",
					Description: "Service names to include (narrows context)",
					Items:       &propSchema{Type: "string"},
				},
			},
			Required: []string{"task"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		task := strArg(args, "task")
		if task == "" {
			return nil, fmt.Errorf("task is required")
		}
		if st.g == nil {
			return map[string]interface{}{
				"status":          "UNKNOWN_IMPACT",
				"invariants":      []string{},
				"failure_modes":   []string{},
				"forbidden_fixes": []string{},
				"required_tests":  []string{},
				"graph_available": false,
				"coverage": map[string]interface{}{
					"graph":          "not_checked",
					"raw_yaml":       "not_checked",
					"runtime":        "noop",
					"metrics":        "noop",
					"code_scan":      "not_checked",
					"incident_store": "not_checked",
				},
				"confidence":        "unknown",
				"confidence_reason": "graph DB not available — no architecture facts can be matched; run 'globular awareness build' first",
				"blind_spots": []string{
					"graph unavailable — static pattern matching only",
					"runtime snapshot not collected — no live cluster evidence",
					"metrics not available — resource saturation not assessed",
					"code violation scan not run — use awareness.scan_violations",
				},
				"warnings": []string{"no graph DB — run 'globular awareness build' first"},
			}, nil
		}

		var aliasMap learning.ContextAliasMap
		if st.docsDir != "" {
			aliasMap, _ = learning.LoadContextAliases(st.docsDir + "/context_aliases.yaml")
		}

		hints := analysis.AgentContextHints{
			Files:    getStrSlice(args, "files"),
			Services: getStrSlice(args, "services"),
		}

		_, result, err := analysis.GenerateAgentContext(ctx, st.g, task, hints, analysis.AgentContextAliases(aliasMap))
		if err != nil {
			return nil, err
		}

		// Include a lightweight proposal queue health summary so agents know
		// whether there are stale/pending proposals before starting code changes.
		queueSection, _ := buildQueueSection(st.docsDir, 24.0)

		// Attach relevant failure knowledge for this task.
		failureKnowledge := buildFailureKnowledgeForTask(ctx, st, task, getStrSlice(args, "files"))

		out := map[string]interface{}{
			"invariants":        result.InvariantIDs,
			"failure_modes":     result.FailureModeIDs,
			"forbidden_fixes":   result.ForbiddenFixes,
			"required_tests":    result.RequiredTests,
			"required_searches": result.RequiredSearches,
			"services":          result.ServiceNames,
			"proposal_queue": map[string]interface{}{
				"pending_proposals": queueSection.PendingProposals,
				"stale_proposals":   queueSection.StaleProposals,
				"queue_status":      queueSection.QueueStatus,
				"status":            queueSection.Status,
			},
		}
		if len(failureKnowledge) > 0 {
			out["relevant_failure_knowledge"] = failureKnowledge
		}
		return out, nil
	})
}

// buildTrustSummaryFromReport derives a trust level distribution from a preflight
// Report's FilteredMatches. Matches not in FilteredMatches are assumed "declared"
// (the most common source for YAML-authored knowledge).
func buildTrustSummaryFromReport(r *preflight.Report) map[string]int {
	counts := map[string]int{
		integrity.TrustStrictVerified: 0,
		integrity.TrustVerified:       0,
		integrity.TrustDeclared:       0,
		integrity.TrustInferred:       0,
		integrity.TrustProposal:       0,
		integrity.TrustStale:          0,
		integrity.TrustInvalid:        0,
	}

	// Count low-trust filtered matches.
	filteredIDs := make(map[string]bool)
	for _, fm := range r.FilteredMatches {
		filteredIDs[fm.ID] = true
		if _, ok := counts[fm.TrustLevel]; ok {
			counts[fm.TrustLevel]++
		}
	}

	// Remaining matched nodes are assumed declared (YAML-authored).
	totalMatched := r.GraphMatchCount
	filteredCount := len(r.FilteredMatches)
	declaredCount := totalMatched - filteredCount
	if declaredCount < 0 {
		declaredCount = 0
	}
	counts[integrity.TrustDeclared] += declaredCount

	return counts
}

// buildFailureKnowledgeForTask returns failure categories relevant to the task
// by matching task keywords against category names and summaries.
// At most 3 categories are returned to keep agent-context concise.
func buildFailureKnowledgeForTask(ctx context.Context, st *awarenessState, task string, files []string) []map[string]interface{} {
	if st.g == nil {
		return nil
	}
	store := failuregraph.New(st.g)
	cats, err := store.ListCategories(ctx)
	if err != nil || len(cats) == 0 {
		return nil
	}

	// Build a set of candidate keywords from task + file paths.
	taskLower := strings.ToLower(task)
	fileLower := strings.ToLower(strings.Join(files, " "))
	combined := taskLower + " " + fileLower

	type scored struct {
		cat   failuregraph.FailureNode
		score int
	}
	var candidates []scored
	for _, cat := range cats {
		s := 0
		name := strings.ToLower(cat.Name)
		words := strings.FieldsFunc(name, func(r rune) bool { return r == '_' })
		for _, w := range words {
			if len(w) > 3 && strings.Contains(combined, w) {
				s++
			}
		}
		sum := strings.ToLower(cat.Summary)
		sumWords := strings.Fields(sum)
		for _, w := range sumWords {
			if len(w) > 5 && strings.Contains(combined, w) {
				s++
			}
		}
		if s > 0 {
			candidates = append(candidates, scored{cat, s})
		}
	}

	// Sort by score descending, cap at 3.
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}

	var result []map[string]interface{}
	for _, c := range candidates {
		exp, err := failuregraph.ExplainCategory(ctx, store, c.cat.ID)
		if err != nil {
			continue
		}
		causes := nodeSummaries(exp.LikelyCauses)
		wrongFixes := nodeSummaries(exp.WrongFixes)
		tests := nodeSummaries(exp.RequiredTests)
		result = append(result, map[string]interface{}{
			"category":    c.cat.Name,
			"summary":     c.cat.Summary,
			"causes":      causes,
			"wrong_fixes": wrongFixes,
			"tests":       tests,
		})
	}
	return result
}
