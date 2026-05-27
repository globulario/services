// Package intentaudit loads intent YAML nodes and checks them against
// source code for violations, test coverage, and change-risk classification.
//
// This is the executable layer for the intent-as-testable-architecture model.
// It does NOT require a live cluster — all checks are source-only.
package intentaudit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Node is the full schema for a docs/intent/*.yaml file, including the
// new audit metadata fields added in the intent evolution pass.
type Node struct {
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

	// Audit metadata (added in intent evolution pass).
	RequiredTests     []string    `yaml:"required_tests"`
	ChangeRisk        []string    `yaml:"change_risk"`
	ViolationPatterns []string    `yaml:"violation_patterns"`
	Exceptions        []Exception `yaml:"exceptions"`
	RuntimeEvidence   interface{} `yaml:"runtime_evidence"` // schema varies per node
}

// Exception is a named, bounded deviation from the intent.
type Exception struct {
	Name        string   `yaml:"name"`
	ID          string   `yaml:"id"`
	Description string   `yaml:"description"`
	Status      string   `yaml:"status"`
	Bounded     bool     `yaml:"bounded"`
	Permanent   bool     `yaml:"permanent"`
	Files       []string `yaml:"files"` // file path substrings that match this exception
}

// ExceptionID returns the stable identifier for the exception.
func (e Exception) ExceptionID() string {
	if e.ID != "" {
		return e.ID
	}
	return e.Name
}

// LoadDir loads all *.yaml files from the given intent directory.
// Returns nodes indexed by ID. Files that fail to parse produce an error
// in the returned error slice but do not prevent other nodes from loading.
func LoadDir(dir string) (map[string]*Node, []error) {
	nodes := make(map[string]*Node)
	var errs []error

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{fmt.Errorf("read intent dir %s: %w", dir, err)}
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", path, err))
			continue
		}
		var node Node
		if err := yaml.Unmarshal(data, &node); err != nil {
			errs = append(errs, fmt.Errorf("parse %s: %w", path, err))
			continue
		}
		if node.ID == "" {
			errs = append(errs, fmt.Errorf("%s: missing id field", path))
			continue
		}
		nodes[node.ID] = &node
	}
	return nodes, errs
}

// LoadFile loads a single intent YAML file.
func LoadFile(path string) (*Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var node Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if node.ID == "" {
		return nil, fmt.Errorf("%s: missing id field", path)
	}
	return &node, nil
}
