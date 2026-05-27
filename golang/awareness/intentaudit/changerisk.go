package intentaudit

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// RiskCategory maps file patterns and symbols to intent nodes.
type RiskCategory struct {
	Description    string   `yaml:"description"`
	FilePatterns   []string `yaml:"file_patterns"`
	SymbolPatterns []string `yaml:"symbol_patterns"`
	IntentNodes    []string `yaml:"intent_nodes"`
}

// RiskClassifier holds the full change-risk classification config.
type RiskClassifier struct {
	Categories map[string]RiskCategory `yaml:"categories"`
}

// PreflightResult is the output of a change-risk preflight.
type PreflightResult struct {
	ChangedFile    string   `json:"changed_file" yaml:"changed_file"`
	RiskCategories []string `json:"risk_categories" yaml:"risk_categories"`
	IntentsToAudit []string `json:"intents_to_audit" yaml:"intents_to_audit"`
	RequiredTests  []string `json:"required_tests" yaml:"required_tests"`
}

// LoadClassifier loads the change-risk classifier from a YAML file.
func LoadClassifier(path string) (*RiskClassifier, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read classifier %s: %w", path, err)
	}
	var wrapper struct {
		Categories map[string]RiskCategory `yaml:"categories"`
	}
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("parse classifier %s: %w", path, err)
	}
	return &RiskClassifier{Categories: wrapper.Categories}, nil
}

// Classify returns the risk categories and intent nodes relevant to a
// changed file. Matching is by substring on file path.
func (rc *RiskClassifier) Classify(changedFile string) PreflightResult {
	result := PreflightResult{ChangedFile: changedFile}
	seen := make(map[string]bool)

	for catName, cat := range rc.Categories {
		matched := false
		for _, fp := range cat.FilePatterns {
			if strings.Contains(changedFile, fp) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		result.RiskCategories = append(result.RiskCategories, catName)
		for _, intentID := range cat.IntentNodes {
			if !seen[intentID] {
				seen[intentID] = true
				result.IntentsToAudit = append(result.IntentsToAudit, intentID)
			}
		}
	}
	return result
}

// ClassifyFiles runs classification for multiple changed files and merges
// the results into a single deduplicated list.
func (rc *RiskClassifier) ClassifyFiles(files []string) []PreflightResult {
	results := make([]PreflightResult, len(files))
	for i, f := range files {
		results[i] = rc.Classify(f)
	}
	return results
}

// MergedPreflight returns a single merged result for multiple files.
func (rc *RiskClassifier) MergedPreflight(files []string, nodes map[string]*Node) PreflightResult {
	merged := PreflightResult{}
	riskSet := make(map[string]bool)
	intentSet := make(map[string]bool)
	testSet := make(map[string]bool)

	for _, f := range files {
		r := rc.Classify(f)
		for _, cat := range r.RiskCategories {
			if !riskSet[cat] {
				riskSet[cat] = true
				merged.RiskCategories = append(merged.RiskCategories, cat)
			}
		}
		for _, id := range r.IntentsToAudit {
			if !intentSet[id] {
				intentSet[id] = true
				merged.IntentsToAudit = append(merged.IntentsToAudit, id)
			}
		}
	}

	// Collect required tests from matched intent nodes.
	for _, id := range merged.IntentsToAudit {
		if n, ok := nodes[id]; ok {
			for _, t := range n.RequiredTests {
				name := extractTestName(t)
				if !testSet[name] {
					testSet[name] = true
					merged.RequiredTests = append(merged.RequiredTests, name)
				}
			}
		}
	}

	return merged
}
