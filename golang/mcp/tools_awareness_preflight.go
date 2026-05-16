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
		Description: "Run a compact architecture preflight before editing Globular code. Returns a bounded decision envelope: safety status, risk tier, confidence, top forbidden fixes, required tests, and agent context. Use output_profile=forensic or full_json=true only when deep diagnosis is truly needed.",
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
					Description: "Collect live runtime snapshot and merge into preflight. Ignored unless runtime_policy is also set to auto or required.",
					Default:     false,
				},
				"runtime_policy": {
					Type:        "string",
					Description: "Runtime collection policy: never (default — skip runtime), auto (collect if live), required (collect, fail if unavailable), offline (use saved snapshot).",
					Default:     "never",
				},
				"runtime_window": {
					Type:        "string",
					Description: "Lookback window for runtime events/workflows (e.g. 15m, 1h)",
					Default:     "15m",
				},
				"output_profile": {
					Type:        "string",
					Description: "Output profile: compact (default) | standard | deep | forensic. Compact returns only essential safety fields. Forensic returns the full report.",
					Default:     "compact",
				},
				"format": {
					Type:        "string",
					Description: "Output format: agent (default) | json. JSON full report requires output_profile=forensic or full_json=true.",
					Default:     "agent",
				},
				"max_items": {
					Type:        "number",
					Description: "Maximum items per compact list (forbidden_fixes, required_tests). Default: 5.",
					Default:     5,
				},
				"max_bytes": {
					Type:        "number",
					Description: "Maximum serialized response size in bytes. Response is truncated with truncated=true if exceeded. Default: 12000.",
					Default:     12000,
				},
				"include_runtime_detail": {
					Type:        "boolean",
					Description: "Include detailed runtime section in the response. Default: false. Has no effect unless runtime was collected.",
					Default:     false,
				},
				"include_raw_matches": {
					Type:        "boolean",
					Description: "Include raw YAML match details (filtered_matches). Default: false.",
					Default:     false,
				},
				"include_decision_traces": {
					Type:        "boolean",
					Description: "Include decision traces in agent_context. Default: false for compact, true for deep/forensic.",
					Default:     false,
				},
				"full_json": {
					Type:        "boolean",
					Description: "Return canonical full JSON report. Default: false. Only use for debugging or forensic runs.",
					Default:     false,
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

		// Default runtime_policy is now "never" — agents must opt in to runtime collection.
		runtimePolicy := strArg(args, "runtime_policy")
		if runtimePolicy == "" {
			runtimePolicy = "never"
		}

		switch runtimePolicy {
		case "required", "auto":
			opts.IncludeRuntime = true
			opts.Bridge = newLiveBridge(st)
		case "never", "offline":
			opts.IncludeRuntime = false
		default:
			// Legacy: include_runtime flag still works when runtime_policy is unrecognized.
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

		// Resolve output profile → budget.
		profile := strArg(args, "output_profile")
		if profile == "" {
			profile = "compact"
		}
		budget := preflight.BudgetCompact
		switch profile {
		case "standard":
			budget = preflight.BudgetStandard
		case "deep":
			budget = preflight.BudgetDeep
		case "forensic":
			budget = preflight.BudgetForensic
		}

		// Resolve format: only allow full JSON for forensic or explicit full_json=true.
		fullJSON := getBool(args, "full_json", false)
		formatArg := strArg(args, "format")
		format := preflight.FormatAgent
		if fullJSON || (formatArg == "json" && profile == "forensic") {
			format = preflight.FormatJSON
		}

		// Override budget to include decision traces when explicitly requested.
		renderOpts := preflight.RenderOptions{Budget: budget}
		if getBool(args, "include_decision_traces", false) && budget == preflight.BudgetCompact {
			renderOpts.Budget = preflight.BudgetStandard
		}

		out, err := preflight.RenderWithOptions(r, format, renderOpts)
		if err != nil {
			return nil, fmt.Errorf("render: %w", err)
		}

		maxItems := intArgDefault(args, "max_items", 5)
		maxBytes := intArgDefault(args, "max_bytes", 12000)

		// Forensic full-JSON mode: return raw object with profile metadata.
		if fullJSON && format == preflight.FormatJSON {
			var raw map[string]interface{}
			if err := json.Unmarshal([]byte(out), &raw); err == nil {
				raw["output_profile"] = profile
				raw["full_json"] = true
				raw["runtime_policy"] = runtimePolicy
				return raw, nil
			}
		}

		// Compact envelope: filtering happens before serialization so the agent
		// never receives a giant context window. The agent cannot save tokens by
		// ignoring large MCP results after receiving them.
		result := map[string]interface{}{
			"output_profile":    profile,
			"format":            string(format),
			"runtime_policy":    runtimePolicy,
			"safety_status":     string(r.SafetyStatus),
			"risk_tier":         string(r.RiskTier),
			"confidence":        string(r.Confidence),
			"confidence_reason": r.ConfidenceReason,
			"graph_available":   r.GraphAvailable,
			"graph_match_count": r.GraphMatchCount,
			"raw_yaml_match_count": r.RawYAMLMatchCount,
			"agent_context":        out,
			"next_context_handles": buildPreflightContextHandles(r, maxItems, budget),
		}

		// Ranked forbidden fixes and required tests (capped at max_items).
		if len(r.ForbiddenFixes) > 0 {
			cap := maxItems
			if cap > len(r.ForbiddenFixes) {
				cap = len(r.ForbiddenFixes)
			}
			result["forbidden_fixes"] = r.ForbiddenFixes[:cap]
		}
		if len(r.RequiredTests) > 0 {
			cap := maxItems
			if cap > len(r.RequiredTests) {
				cap = len(r.RequiredTests)
			}
			result["required_tests"] = r.RequiredTests[:cap]
		}
		if len(r.Warnings) > 0 {
			result["warnings"] = r.Warnings
		}

		// Trust summary from graph if available.
		if st.g != nil {
			result["trust_summary"] = buildTrustSummaryFromReport(r)
		}

		// Runtime detail only when explicitly requested.
		if getBool(args, "include_runtime_detail", false) {
			result["runtime"] = r.Runtime
		}

		// Raw filtered matches only when explicitly requested.
		if getBool(args, "include_raw_matches", false) && len(r.FilteredMatches) > 0 {
			result["filtered_matches"] = r.FilteredMatches
		}

		return enforceResponseBudget(result, maxBytes), nil
	})

	s.register(toolDef{
		Name: "awareness.agent_context",
		Description: "Secondary context tool — use awareness.preflight (compact) first for most tasks. " +
			"Returns invariants, failure modes, forbidden fixes, and required tests from the awareness graph. " +
			"Do NOT call this in the same turn as awareness.preflight unless preflight explicitly listed it as a required pivot. " +
			"Response is hard-capped at max_bytes (default 8000) to protect context budget.",
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
				"max_bytes": {
					Type:        "number",
					Description: "Maximum response size in bytes. Default: 8000.",
					Default:     8000,
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
				"trust":    awarenessTrustMap(st, false),
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

		// Attach relevant failure knowledge — summary-only (category + summary)
		// to keep tokens bounded. Use awareness.failure_explain_category for detail.
		failureKnowledge := buildFailureKnowledgeSummary(ctx, st, task, getStrSlice(args, "files"))

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
			"trust": awarenessTrustMap(st, len(result.InvariantIDs)+len(result.FailureModeIDs)+len(result.ForbiddenFixes)+len(result.RequiredTests) > 0),
		}
		if len(failureKnowledge) > 0 {
			out["relevant_failure_knowledge"] = failureKnowledge
		}

		maxBytes := intArgDefault(args, "max_bytes", 8000)
		return enforceAgentContextBudget(out, maxBytes), nil
	})
}

// buildPreflightContextHandles returns follow-up tool handles so agents can
// request deeper context without receiving the full graph in the initial call.
// In compact budget mode, only impact_file handles are included — agent_context
// and explain handles are omitted to prevent redundant chaining that wastes tokens.
func buildPreflightContextHandles(r *preflight.Report, maxItems int, budget preflight.Budget) []map[string]interface{} {
	var handles []map[string]interface{}

	// Impact analysis handles for files being edited.
	limit := 3
	for i, f := range r.Files {
		if i >= limit {
			break
		}
		handles = append(handles, map[string]interface{}{
			"tool":      "awareness.impact_file",
			"available": true,
			"args":      map[string]interface{}{"file": f, "limit": maxItems},
		})
	}

	// Compact mode: stop here — don't suggest agent_context or explain handles.
	// Agents using compact preflight should not chain secondary awareness calls
	// unless a specific finding warrants it.
	if budget == preflight.BudgetCompact {
		return handles
	}

	// Explain handles for top invariants — only for standard/deep/forensic.
	for i, inv := range r.Invariants {
		if i >= 2 {
			break
		}
		handles = append(handles, map[string]interface{}{
			"tool":      "awareness.explain",
			"available": true,
			"args":      map[string]interface{}{"id": inv},
		})
	}

	// Agent context handle — only for standard/deep/forensic.
	handles = append(handles, map[string]interface{}{
		"tool":      "awareness.agent_context",
		"available": true,
		"args":      map[string]interface{}{"task": r.Task, "files": r.Files},
	})

	return handles
}

// enforceAgentContextBudget applies a byte cap to agent_context responses.
// Drops optional sections in priority order: relevant_failure_knowledge → required_searches → trust.
// Essential fields (invariants, failure_modes, forbidden_fixes, required_tests) are never dropped.
func enforceAgentContextBudget(result map[string]interface{}, maxBytes int) map[string]interface{} {
	b, _ := json.Marshal(result)
	if maxBytes <= 0 || len(b) <= maxBytes {
		result["bytes"] = len(b)
		result["truncated"] = false
		return result
	}
	// Drop failure knowledge first — the caller can use awareness.failure_explain_category.
	delete(result, "relevant_failure_knowledge")
	b, _ = json.Marshal(result)
	if len(b) <= maxBytes {
		result["bytes"] = len(b)
		result["truncated"] = true
		result["truncation_reason"] = "failure_knowledge dropped; use awareness.failure_explain_category for detail"
		return result
	}
	// Drop required_searches — lowest priority list.
	delete(result, "required_searches")
	b, _ = json.Marshal(result)
	if len(b) <= maxBytes {
		result["bytes"] = len(b)
		result["truncated"] = true
		result["truncation_reason"] = "failure_knowledge and required_searches dropped to fit budget"
		return result
	}
	// Cap lists to 5 items each.
	for _, key := range []string{"forbidden_fixes", "required_tests", "invariants", "failure_modes"} {
		if sl, ok := result[key].([]string); ok && len(sl) > 5 {
			result[key] = sl[:5]
		}
	}
	b, _ = json.Marshal(result)
	result["bytes"] = len(b)
	result["truncated"] = true
	result["truncation_reason"] = "lists capped at 5 items; use output_profile=deep for full detail"
	return result
}

// buildFailureKnowledgeSummary returns category+summary only (no graph expansion)
// to keep agent_context tokens bounded. Use awareness.failure_explain_category for causes/fixes.
func buildFailureKnowledgeSummary(ctx context.Context, st *awarenessState, task string, files []string) []map[string]interface{} {
	if st.g == nil {
		return nil
	}
	store := failuregraph.New(st.g)
	cats, err := store.ListCategories(ctx)
	if err != nil || len(cats) == 0 {
		return nil
	}

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
		for _, w := range strings.Fields(sum) {
			if len(w) > 5 && strings.Contains(combined, w) {
				s++
			}
		}
		if s > 0 {
			candidates = append(candidates, scored{cat, s})
		}
	}

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
		result = append(result, map[string]interface{}{
			"category": c.cat.Name,
			"summary":  c.cat.Summary,
			"hint":     "use awareness.failure_explain_category for causes/wrong_fixes/tests",
		})
	}
	return result
}

// enforceResponseBudget ensures the MCP response never exceeds maxBytes.
// Filtering happens here before serialization — the agent cannot recover
// tokens by ignoring data after it has already been injected into context.
// Returns a valid JSON-serializable map with truncated=false/true and bytes set.
//
// Fallback order (per spec):
//  1. Truncate agent_context text (the primary large section).
//  2. Drop low-priority optional sections (filtered_matches, trust_summary).
//  3. Essential fields only — keeps safety_status, risk_tier, confidence,
//     forbidden_fixes, required_tests, and next_context_handles.
func enforceResponseBudget(result map[string]interface{}, maxBytes int) map[string]interface{} {
	b, _ := json.Marshal(result)
	if maxBytes <= 0 || len(b) <= maxBytes {
		result["truncated"] = false
		result["bytes"] = len(b)
		return result
	}

	// First fallback: truncate agent_context — it is always the largest section.
	const agentContextTruncLen = 2000
	if ac, ok := result["agent_context"].(string); ok && len(ac) > agentContextTruncLen {
		result["agent_context"] = ac[:agentContextTruncLen] + "\n... [truncated; use output_profile=deep for full context]"
	}
	b, _ = json.Marshal(result)
	if len(b) <= maxBytes {
		result["truncated"] = true
		result["truncation_reason"] = "response exceeded max_bytes; use output_profile=deep or forensic for full detail"
		result["bytes"] = len(b)
		return result
	}

	// Second fallback: drop large optional sections that were not the
	// primary request target. Do not drop "runtime" here since the caller
	// may have explicitly requested include_runtime_detail=true; it will be
	// omitted only if the essential-only fallback is reached.
	for _, key := range []string{"filtered_matches", "trust_summary"} {
		delete(result, key)
	}
	b, _ = json.Marshal(result)
	if len(b) <= maxBytes {
		result["truncated"] = true
		result["truncation_reason"] = "response exceeded max_bytes; use output_profile=deep or forensic for full detail"
		result["bytes"] = len(b)
		return result
	}

	// Final fallback: return only essential safety fields.
	essential := map[string]interface{}{
		"output_profile":    result["output_profile"],
		"safety_status":     result["safety_status"],
		"risk_tier":         result["risk_tier"],
		"confidence":        result["confidence"],
		"confidence_reason": result["confidence_reason"],
		"truncated":         true,
		"truncation_reason": "response exceeded max_bytes; use output_profile=forensic for full detail",
	}
	if ff, ok := result["forbidden_fixes"]; ok {
		essential["forbidden_fixes"] = ff
	}
	if tests, ok := result["required_tests"]; ok {
		essential["required_tests"] = tests
	}
	if handles, ok := result["next_context_handles"]; ok {
		essential["next_context_handles"] = handles
	}
	b, _ = json.Marshal(essential)
	essential["bytes"] = len(b)
	return essential
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

