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

// YAMLRole classifies a YAML file under docs/awareness/ by how it contributes
// (or not) to the awareness graph. Used by the assurance freshness layer to
// distinguish "I don't know what this file is" from "I know exactly what it
// is and that it doesn't affect graph staleness."
type YAMLRole string

const (
	// YAMLRoleGraph: top-level key is in dispatchTable. The file builds graph
	// nodes and edges. Edits to it must mark the graph stale.
	YAMLRoleGraph YAMLRole = "graph"
	// YAMLRoleConfig: top-level key is in configOnlyKeys. The file is loaded
	// by some subsystem but does not contribute graph nodes/edges. Edits do
	// NOT need to mark the graph stale.
	YAMLRoleConfig YAMLRole = "config"
	// YAMLRoleUnknown: top-level key is neither. We can't classify the file
	// without human input. Treated conservatively (assumes graph-contributing
	// for safety).
	YAMLRoleUnknown YAMLRole = "unknown"
)

// GraphYAMLKeys returns the set of top-level YAML keys whose files are
// graph-contributing (have a registered loader).
func GraphYAMLKeys() map[string]bool {
	out := make(map[string]bool, len(dispatchTable))
	for k := range dispatchTable {
		out[k] = true
	}
	return out
}

// ConfigYAMLKeys returns the set of top-level YAML keys whose files are
// explicitly config-only (loaded by some subsystem but not graph-contributing).
func ConfigYAMLKeys() map[string]bool {
	out := make(map[string]bool, len(configOnlyKeys))
	for k := range configOnlyKeys {
		out[k] = true
	}
	return out
}

// externallyHandledGraphKeys lists top-level YAML keys that are graph-
// contributing but loaded by extractors OUTSIDE the manual package (the
// failuregraph extractor, the doctor mapping extractor, contract loaders,
// etc.). Edits to such files must mark the graph stale, but the manual
// loader's dispatchTable does not know how to load them — by design.
//
// Add a key here when a new subsystem starts shipping graph-contributing
// YAMLs that don't go through the manual loader.
var externallyHandledGraphKeys = map[string]bool{
	"id":                true, // failuregraph_seeds/*.yaml — loaded by failurelearning.RebuildFromSeeds
	"detector_mappings": true, // detector_mapping.yaml — loaded by doctor mapping extractor
}

// ClassifyYAMLByTopKey returns the role for a top-level YAML key. Use this
// when you've already parsed the key (e.g. inside a file walker that needs
// to read content anyway).
func ClassifyYAMLByTopKey(topKey string) YAMLRole {
	if dispatchTable[topKey] != nil {
		return YAMLRoleGraph
	}
	if externallyHandledGraphKeys[topKey] {
		return YAMLRoleGraph
	}
	if configOnlyKeys[topKey] {
		return YAMLRoleConfig
	}
	return YAMLRoleUnknown
}

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
	"version":      true, // contracts/*.yaml — data contract docs, no graph loader
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
			if externallyHandledGraphKeys[key] {
				return nil // graph-contributing but loaded by another extractor — not a coverage gap
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

// TopLevelKey returns the first key in the top-level YAML mapping of data.
// Exported so callers (e.g. the assurance freshness layer) can classify
// already-loaded YAML content without re-reading the file from disk.
func TopLevelKey(data []byte) (string, error) { return topLevelKey(data) }

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
