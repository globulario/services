// Package manual loads hand-authored awareness truth files into the graph.
// Files are discovered by walking docsDir recursively. Each YAML file's
// top-level key determines which loader handles it; unknown keys are skipped
// so config, proposal, and incident files can coexist in the same tree.
package manual

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/globulario/services/golang/awareness/graph"
)

// dispatchTable maps top-level YAML keys to the loader that handles them.
var dispatchTable = map[string]func(context.Context, *graph.Graph, string) error{
	"invariants":      LoadInvariants,
	"failure_modes":   LoadFailureModes,
	"forbidden_fixes": LoadForbiddenFixes,
	"services":        LoadServices,
	"patterns":        LoadPatterns,
	"design_patterns": LoadDesignPatterns,
	"decision_rules":  LoadDecisionRules,
	"causal_rules":    LoadCausalRules,
}

// configOnlyKeys are top-level YAML keys that identify files intentionally not
// loaded into the graph. These are config, pipeline state, or operational data
// files that coexist in the awareness tree but have no graph representation.
// Files with these keys are excluded from the WalkUnindexed report so that
// health_pulse does not flag them as coverage gaps.
var configOnlyKeys = map[string]bool{
	"aliases":      true, // context_aliases.yaml — loaded by alias matcher
	"suppressions": true, // audit_suppressions.yaml — suppression config
	"fix_cases":    true, // fix_cases.yaml — loaded by fixledger
	"guardrails":   true, // guardrails.yaml — loaded by enforcement
	"files":        true, // high_risk_files.yaml — hook config
	"rules":        true, // learning_rules.yaml — pipeline config
	"trust":        true, // knowledge/path_weights.yaml — scoring config
	"allowlist":    true, // knowledge/scan_allowlist.yaml — scanner config
	"queries":      true, // knowledge/metric_queries.yaml — Prometheus query templates
	"thresholds":   true, // knowledge/metric_thresholds.yaml — metric thresholds
	"playbooks":    true, // knowledge/agent_playbooks.yaml — agent playbooks
	"dns_zones":    true, // knowledge/dns_zones.yaml — read by dns extractor, not manual loader
	"incident_id":  true, // incidents/*.yaml — dynamic incident records
	"proposal":     true, // proposals/*.yaml — proposal pipeline records
	"last_updated": true, // status_tracker.yaml — operational status
}

// UnindexedFile describes a YAML file whose top-level key has no registered loader.
// These files are silently skipped by LoadAll and their content does not reach the graph.
type UnindexedFile struct {
	Path   string // path relative to docsDir, or absolute if docsDir is empty
	TopKey string // the unrecognized top-level YAML key
}

// WalkUnindexed returns all YAML files under docsDir whose top-level key is not
// registered in the dispatch table. The caller can inspect these to decide whether
// new loaders are needed or whether the files are intentionally config-only.
func WalkUnindexed(docsDir string) ([]UnindexedFile, error) {
	var out []UnindexedFile
	err := filepath.WalkDir(docsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		key, err := topLevelKey(data)
		if err != nil || key == "" {
			return nil
		}
		if _, known := dispatchTable[key]; !known {
			if configOnlyKeys[key] {
				return nil // intentional config-only file — not a coverage gap
			}
			rel, err := filepath.Rel(docsDir, path)
			if err != nil {
				rel = path
			}
			out = append(out, UnindexedFile{Path: rel, TopKey: key})
		}
		return nil
	})
	return out, err
}

// LoadAll walks docsDir recursively and loads every YAML file whose top-level
// key matches a known loader. Unknown top-level keys are silently skipped.
// Missing docsDir is silently skipped.
func LoadAll(ctx context.Context, g *graph.Graph, docsDir string) error {
	return filepath.WalkDir(docsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entry: skip
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		if loadErr := dispatchFile(ctx, g, path); loadErr != nil {
			return fmt.Errorf("manual loader %s: %w", path, loadErr)
		}
		return nil
	})
}

// dispatchFile reads path, detects its top-level YAML key, and calls the
// matching loader. Files with unknown keys are silently skipped.
func dispatchFile(ctx context.Context, g *graph.Graph, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // missing or unreadable: skip
	}

	key, err := topLevelKey(data)
	if err != nil || key == "" {
		return nil // unparseable or empty: skip
	}

	loader, ok := dispatchTable[key]
	if !ok {
		return nil // unknown type: skip
	}
	return loader(ctx, g, path)
}

// topLevelKey returns the first key in the top-level YAML mapping of data.
func topLevelKey(data []byte) (string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return "", nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode || len(root.Content) == 0 {
		return "", nil
	}
	return root.Content[0].Value, nil
}
