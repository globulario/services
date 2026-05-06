package mcp

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/globulario/services/golang/awareness/fixledger"
	"github.com/globulario/services/golang/awareness/learning"
)

func registerFixledgerTools(s *Server) {
	registerDidWeFixTool(s)
	registerPatternStatusTool(s)
	registerFixStatusTool(s)
}

func registerDidWeFixTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.did_we_fix",
		Description: "Query the fix ledger to determine if this class of problem has been fixed before. Returns status, matched fix cases, remaining gaps, and next action.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"task": {
					Type:        "string",
					Description: "Task or problem description to match against known fix cases",
				},
			},
			Required: []string{"task"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		task := strArg(args, "task")
		if task == "" {
			return nil, fmt.Errorf("task is required")
		}

		fixCases, aliases := s.loadFixLedger()
		result := fixledger.DidWeFix(task, fixCases, fixledger.ContextAliasMap(aliases))

		caseIDs := make([]string, 0, len(result.MatchedFixCases))
		for _, fc := range result.MatchedFixCases {
			caseIDs = append(caseIDs, fc.ID)
		}

		gaps := result.RemainingFiles
		if gaps == nil {
			gaps = []string{}
		}

		regressions := fixledger.ListRegressions(fixCases)
		regressionIDs := make([]string, 0, len(regressions))
		for _, r := range regressions {
			regressionIDs = append(regressionIDs, r.ID)
		}

		return map[string]interface{}{
			"status":           string(result.OverallStatus),
			"matched_patterns": orEmpty([]string{result.MatchedPattern}),
			"fix_cases":        caseIDs,
			"remaining_gaps":   gaps,
			"regressions":      regressionIDs,
			"required_tests":   result.RequiredTests,
			"next_action":      result.NextAction,
		}, nil
	})
}

func registerPatternStatusTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.pattern_status",
		Description: "Return all fix cases matching a pattern keyword.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"pattern": {
					Type:        "string",
					Description: "Pattern keyword to search fix cases (e.g. 'desired_hash', 'restart')",
				},
			},
			Required: []string{"pattern"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		pattern := strArg(args, "pattern")
		if pattern == "" {
			return nil, fmt.Errorf("pattern is required")
		}

		fixCases, _ := s.loadFixLedger()
		matched := fixledger.PatternStatus(pattern, fixCases)

		out := make([]map[string]interface{}, 0, len(matched))
		for _, fc := range matched {
			out = append(out, fixCaseToMap(fc))
		}
		return map[string]interface{}{"fix_cases": out}, nil
	})
}

func registerFixStatusTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.fix_status",
		Description: "Return the fix status for a given fix case ID or pattern.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"id": {
					Type:        "string",
					Description: "Fix case ID (exact match)",
				},
				"pattern": {
					Type:        "string",
					Description: "Pattern substring (returns all matching cases if id is not provided)",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		id := strArg(args, "id")
		pattern := strArg(args, "pattern")

		fixCases, _ := s.loadFixLedger()

		var matched []fixledger.FixCase
		if id != "" {
			for _, fc := range fixCases {
				if fc.ID == id {
					matched = append(matched, fc)
					break
				}
			}
		} else if pattern != "" {
			matched = fixledger.PatternStatus(pattern, fixCases)
		} else {
			matched = fixCases
		}

		out := make([]map[string]interface{}, 0, len(matched))
		for _, fc := range matched {
			out = append(out, fixCaseToMap(fc))
		}
		return map[string]interface{}{"fix_cases": out}, nil
	})
}

// loadFixLedger loads fix cases and context aliases from the docs dir.
func (s *Server) loadFixLedger() ([]fixledger.FixCase, learning.ContextAliasMap) {
	docsDir := s.resolvedDocsDir()
	var fixCases []fixledger.FixCase
	if docsDir != "" {
		fixCases, _ = fixledger.LoadFixCases(filepath.Join(docsDir, "fix_cases.yaml"))
	}
	var aliases learning.ContextAliasMap
	if docsDir != "" {
		aliases, _ = learning.LoadContextAliases(filepath.Join(docsDir, "context_aliases.yaml"))
	}
	return fixCases, aliases
}

func fixCaseToMap(fc fixledger.FixCase) map[string]interface{} {
	return map[string]interface{}{
		"id":                fc.ID,
		"title":             fc.Title,
		"status":            string(fc.Status),
		"pattern":           fc.Pattern,
		"target_invariants": fc.TargetInvariants,
		"fixed_files":       fc.FixedFiles,
		"remaining_files":   fc.RemainingFiles,
		"required_tests":    fc.RequiredTests,
	}
}

func orEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
