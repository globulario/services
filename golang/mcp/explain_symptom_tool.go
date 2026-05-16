package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/awareness/preflight"
	"gopkg.in/yaml.v3"
)

func registerExplainSymptomTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.explain_symptom",
		Description: "Maps a raw error message, log line, or symptom description to known failure modes, invariants, and forbidden fixes. For quick diagnosis without opening a full incident.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"text":    {Type: "string", Description: "Raw error, log line, gRPC error, systemd output, or test failure."},
				"service": {Type: "string", Description: "Optional: service name/id the error came from."},
				"file":    {Type: "string", Description: "Optional: source file path associated with the error."},
				"node":    {Type: "string", Description: "Optional: node ID where the error occurred."},
			},
			Required: []string{"text"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		_ = ctx
		text := strArg(args, "text")
		if text == "" {
			return nil, fmt.Errorf("text is required")
		}
		service := strArg(args, "service")
		file := strArg(args, "file")

		docsDir := st.docsDir
		if docsDir == "" {
			return nil, fmt.Errorf("docs dir not configured")
		}

		// Build a synthetic task string from all input signals.
		task := text
		if service != "" {
			task += " service=" + service
		}
		if file != "" {
			task += " file=" + file
		}

		var files []string
		if file != "" {
			files = []string{file}
		}

		matches := explainSymptomScan(task, files, docsDir)
		confidence := symptomConfidence(matches)

		return map[string]interface{}{
			"confidence": confidence,
			"matches":    matches,
			"note":       "Run awareness.runtime_snapshot for live cluster confirmation.",
		}, nil
	})
}

func explainSymptomScan(task string, files []string, docsDir string) []map[string]interface{} {
	raw := preflight.RawKnowledgeFallback(task, files, docsDir)

	// Also load full failure mode details to enrich matches.
	fmDetails := loadFailureModeDetails(filepath.Join(docsDir, "failure_modes.yaml"))
	invDetails := loadInvariantDetails(filepath.Join(docsDir, "invariants.yaml"))

	results := make([]map[string]interface{}, 0, len(raw))
	for _, m := range raw {
		entry := map[string]interface{}{
			"id":            m.ID,
			"type":          m.Kind,
			"score":         m.Score,
			"matched_terms": m.MatchedTerms,
		}
		switch m.Kind {
		case "failure_mode":
			if detail, ok := fmDetails[m.ID]; ok {
				if rc, ok := detail["root_cause"].(string); ok && rc != "" {
					entry["root_cause"] = strings.TrimSpace(rc)
				}
				if ff, ok := detail["forbidden_fixes"].([]interface{}); ok {
					fixes := make([]string, 0, len(ff))
					for _, f := range ff {
						if s, ok := f.(string); ok {
							fixes = append(fixes, s)
						}
					}
					entry["forbidden_fixes"] = fixes
				}
				if rt, ok := detail["required_tests"].([]interface{}); ok {
					tests := make([]string, 0, len(rt))
					for _, t := range rt {
						if s, ok := t.(string); ok {
							tests = append(tests, s)
						}
					}
					entry["required_tests"] = tests
				}
				if arch, ok := detail["architecture_fix"].(string); ok && arch != "" {
					lines := strings.Split(strings.TrimSpace(arch), "\n")
					if len(lines) > 0 {
						entry["recommended_next_diagnostic"] = lines[0]
					}
				}
			}
		case "invariant":
			if detail, ok := invDetails[m.ID]; ok {
				if rc, ok := detail["description"].(string); ok && rc != "" {
					entry["root_cause"] = strings.TrimSpace(rc)
				}
			}
		}
		results = append(results, entry)
	}
	return results
}

func symptomConfidence(matches []map[string]interface{}) string {
	if len(matches) == 0 {
		return "unknown"
	}
	if len(matches) >= 3 {
		return "high"
	}
	return "medium"
}

func loadFailureModeDetails(path string) map[string]map[string]interface{} {
	return loadYAMLListByID(path, "failure_modes")
}

func loadInvariantDetails(path string) map[string]map[string]interface{} {
	return loadYAMLListByID(path, "invariants")
}

func loadYAMLListByID(path, listKey string) map[string]map[string]interface{} {
	out := make(map[string]map[string]interface{})
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return out
	}
	items, _ := root[listKey].([]interface{})
	for _, item := range items {
		m, _ := item.(map[string]interface{})
		if id, ok := m["id"].(string); ok && id != "" {
			out[id] = m
		}
	}
	return out
}
