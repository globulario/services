package main

// awareness_promote_cmd.go — `globular awareness promote`
//
// Promotes a candidate from docs/awareness/candidates/ into the matching
// canonical YAML file. Validates against the learning rules, transforms
// the entry to canonical form, appends it, removes from the candidate file,
// and triggers a rebuild.
//
// Reimplements scripts/promote-awareness-candidate.py in Go so the full
// pipeline (promote + rebuild + Oxigraph reload) is one command.
//
// Usage:
//
//	globular awareness promote <candidate-id>
//	globular awareness promote <candidate-id> --dry-run
//	globular awareness promote <candidate-id> --target docs/awareness/failure_modes.yaml
//	globular awareness promote <candidate-id> --no-rebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ── flags ────────────────────────────────────────────────────────────────

var (
	promoteTarget    string // optional: override auto-detected target file
	promoteDryRun    bool
	promoteNoRebuild bool
)

var awarenessPromoteCmd = &cobra.Command{
	Use:   "promote <candidate-id>",
	Short: "Promote a candidate into canonical awareness YAML",
	Long: `Moves a candidate entry from docs/awareness/candidates/ into the
matching canonical YAML file (invariants.yaml, failure_modes.yaml,
incident_patterns.yaml, or intents.yaml).

The target file is auto-detected from the candidate's class field.
Override with --target if needed.

Validation rules (any failure blocks promotion):
  - ID must match canonical naming: <namespace>.<bare_id> (lowercase, dots, underscores)
  - Status must be "candidate"
  - Confidence must not be "low"
  - Evidence field must be non-empty
  - Discovered_from field must be non-empty
  - ID must not already exist in any canonical YAML file
  - Class must match the target file's expected class

After promotion, the candidate is removed from its source file and
the awareness graph is rebuilt automatically (unless --no-rebuild).`,
	Args: cobra.ExactArgs(1),
	RunE: runAwarenessPromote,
}

// ── constants ────────────────────────────────────────────────────────────

// canonicalIDPattern matches <namespace>.<bare_id> — lowercase ASCII, digits,
// dots, underscores. No spaces, no uppercase, no slashes.
var canonicalIDPattern = regexp.MustCompile(`^[a-z0-9_]+(\.[a-z0-9_]+)+$`)

// classToTarget maps candidate class → canonical filename.
var classToTarget = map[string]string{
	"invariant":        "invariants.yaml",
	"failure_mode":     "failure_modes.yaml",
	"incident_pattern": "incident_patterns.yaml",
	"intent":           "intents.yaml",
}

// targetToListKey maps canonical filename → top-level YAML list key.
var targetToListKey = map[string]string{
	"invariants.yaml":        "invariants",
	"failure_modes.yaml":     "failure_modes",
	"incident_patterns.yaml": "incident_patterns",
	"intents.yaml":           "intents",
}

// targetToClass maps canonical filename → expected class (reverse of classToTarget).
var targetToClass = map[string]string{
	"invariants.yaml":        "invariant",
	"failure_modes.yaml":     "failure_mode",
	"incident_patterns.yaml": "incident_pattern",
	"intents.yaml":           "intent",
}

// ── entry point ──────────────────────────────────────────────────────────

func runAwarenessPromote(cmd *cobra.Command, args []string) error {
	candidateID := args[0]

	svcRepo, err := resolveServicesRepo()
	if err != nil {
		return err
	}
	candidatesDir := filepath.Join(svcRepo, "docs", "awareness", "candidates")

	// ── find candidate ───────────────────────────────────────────────────
	candidatePath, candidate, err := findCandidate(candidatesDir, candidateID)
	if err != nil {
		return err
	}
	fmt.Printf("candidate found: %s\n", relPath(svcRepo, candidatePath))

	// ── resolve target ───────────────────────────────────────────────────
	targetFilename, err := resolveTarget(candidate)
	if err != nil {
		return err
	}
	targetPath := filepath.Join(svcRepo, "docs", "awareness", targetFilename)
	listKey := targetToListKey[targetFilename]

	// ── validate ─────────────────────────────────────────────────────────
	if err := validateCandidate(candidate, targetFilename, svcRepo); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	fmt.Println("validation: OK")

	// ── transform ────────────────────────────────────────────────────────
	canonical := toCanonical(candidate)

	if promoteDryRun {
		out, _ := yaml.Marshal(map[string]interface{}{listKey: []interface{}{canonical}})
		fmt.Println()
		fmt.Println("[dry-run] would append to", relPath(svcRepo, targetPath)+":")
		fmt.Println(string(out))
		return nil
	}

	// ── append to canonical file ─────────────────────────────────────────
	if err := appendToCanonical(targetPath, listKey, canonical); err != nil {
		return err
	}
	fmt.Printf("appended to %s\n", relPath(svcRepo, targetPath))

	// ── remove from candidate file ───────────────────────────────────────
	if err := removeCandidate(candidatePath, candidateID); err != nil {
		return err
	}
	fmt.Printf("removed %s from %s\n", candidateID, relPath(svcRepo, candidatePath))

	// ── rebuild ──────────────────────────────────────────────────────────
	if promoteNoRebuild {
		fmt.Println("\nnext step: globular awareness rebuild")
		return nil
	}
	fmt.Println()
	fmt.Println("Triggering rebuild...")
	rebuildNoReload = false
	rebuildCheck = false
	rebuildStrict = false
	return runAwarenessRebuild(nil, nil)
}

// ── find candidate ───────────────────────────────────────────────────────

func findCandidate(candidatesDir, id string) (filePath string, entry map[string]interface{}, err error) {
	type match struct {
		path  string
		entry map[string]interface{}
	}
	var matches []match

	entries, err := os.ReadDir(candidatesDir)
	if err != nil {
		return "", nil, fmt.Errorf("cannot read candidates dir: %w", err)
	}

	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(candidatesDir, de.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var doc map[string]interface{}
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			continue
		}
		candidates, ok := doc["candidates"].([]interface{})
		if !ok {
			continue
		}
		for _, c := range candidates {
			m, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			if cid, _ := m["id"].(string); cid == id {
				matches = append(matches, match{path: path, entry: m})
			}
		}
	}

	if len(matches) == 0 {
		return "", nil, fmt.Errorf("candidate %q not found in %s", id, candidatesDir)
	}
	if len(matches) > 1 {
		var paths []string
		for _, m := range matches {
			paths = append(paths, m.path)
		}
		return "", nil, fmt.Errorf("candidate %q found in multiple files: %s", id, strings.Join(paths, ", "))
	}
	return matches[0].path, matches[0].entry, nil
}

// ── validation ───────────────────────────────────────────────────────────

func validateCandidate(candidate map[string]interface{}, targetFilename, svcRepo string) error {
	id, _ := candidate["id"].(string)

	// Rule: ID matches canonical naming.
	if !canonicalIDPattern.MatchString(id) {
		return fmt.Errorf("id %q does not match canonical naming: <namespace>.<bare_id> (segments: [a-z0-9_]+, joined by dots)", id)
	}

	// Rule: class matches target.
	expectedClass := targetToClass[targetFilename]
	candidateClass, _ := candidate["class"].(string)
	if candidateClass != expectedClass {
		return fmt.Errorf("class mismatch: candidate.class=%q but target %q expects class=%q", candidateClass, targetFilename, expectedClass)
	}

	// Rule: status must be "candidate".
	status, _ := candidate["status"].(string)
	if status != "candidate" {
		return fmt.Errorf("status=%q, expected 'candidate' — promotion is the ONLY way to change status", status)
	}

	// Rule: confidence must not be "low".
	confidence, _ := candidate["confidence"].(string)
	if confidence == "low" {
		return fmt.Errorf("confidence=low — gather more evidence before promoting")
	}

	// Rule: evidence must be non-empty.
	evidence, _ := candidate["evidence"].(string)
	if strings.TrimSpace(evidence) == "" {
		return fmt.Errorf("evidence is empty — reviewers need provenance")
	}

	// Rule: discovered_from must be non-empty.
	discoveredFrom, _ := candidate["discovered_from"].(string)
	if strings.TrimSpace(discoveredFrom) == "" {
		return fmt.Errorf("discovered_from is empty — provenance is required")
	}

	// Rule: no duplicate IDs in canonical files.
	existing := allCanonicalIDs(svcRepo)
	if existing[id] {
		return fmt.Errorf("duplicate: %q already exists in canonical YAML", id)
	}

	return nil
}

func allCanonicalIDs(svcRepo string) map[string]bool {
	ids := make(map[string]bool)
	awarenessDir := filepath.Join(svcRepo, "docs", "awareness")

	entries, err := os.ReadDir(awarenessDir)
	if err != nil {
		return ids
	}

	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(awarenessDir, de.Name()))
		if err != nil {
			continue
		}
		var doc map[string]interface{}
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			continue
		}
		for _, listKey := range []string{"invariants", "failure_modes", "incident_patterns", "intents", "forbidden_fixes", "required_tests"} {
			entries, ok := doc[listKey].([]interface{})
			if !ok {
				continue
			}
			for _, e := range entries {
				m, ok := e.(map[string]interface{})
				if !ok {
					continue
				}
				if id, ok := m["id"].(string); ok {
					ids[id] = true
				}
			}
		}
	}
	return ids
}

// ── transformation ───────────────────────────────────────────────────────

func toCanonical(candidate map[string]interface{}) map[string]interface{} {
	entry := map[string]interface{}{
		"id":       candidate["id"],
		"title":    strField(candidate, "label"),
		"severity": strField(candidate, "risk"),
		"status":   "active",
	}
	if entry["severity"] == "" {
		entry["severity"] = "medium"
	}

	// Carry forward applicable fields.
	for _, key := range []string{
		"summary", "protects", "symptoms", "root_cause", "architecture_fix",
		"forbidden_fixes", "related_invariants", "related_services",
		"required_tests", "failure_mode", "lesson", "edit_shapes",
		"wrong_fixes", "files", "related_symbols", "enforcement",
	} {
		if v, ok := candidate[key]; ok {
			entry[key] = v
		}
	}

	// If there's a proposed_required_test, carry it as a comment-like field.
	if prt := strField(candidate, "proposed_required_test"); prt != "" {
		entry["proposed_required_test"] = prt
	}

	// Provenance block.
	entry["provenance"] = map[string]interface{}{
		"promoted_from":            "candidate",
		"discovered_from":          strField(candidate, "discovered_from"),
		"confidence_at_promotion":  strField(candidate, "confidence"),
	}

	return entry
}

func strField(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return strings.TrimSpace(v)
}

// ── file operations ──────────────────────────────────────────────────────

func appendToCanonical(targetPath, listKey string, newEntry map[string]interface{}) error {
	raw, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", targetPath, err)
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		doc = make(map[string]interface{})
	}

	entries, _ := doc[listKey].([]interface{})
	entries = append(entries, newEntry)

	// Sort by ID for deterministic output.
	sort.SliceStable(entries, func(i, j int) bool {
		a, _ := entries[i].(map[string]interface{})
		b, _ := entries[j].(map[string]interface{})
		ai, _ := a["id"].(string)
		bi, _ := b["id"].(string)
		return ai < bi
	})
	doc[listKey] = entries

	out, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(targetPath, out, 0o644)
}

func removeCandidate(candidatePath, id string) error {
	raw, err := os.ReadFile(candidatePath)
	if err != nil {
		return fmt.Errorf("read candidate file: %w", err)
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse candidate file: %w", err)
	}

	candidates, ok := doc["candidates"].([]interface{})
	if !ok {
		return nil
	}

	var remaining []interface{}
	for _, c := range candidates {
		m, ok := c.(map[string]interface{})
		if !ok {
			remaining = append(remaining, c)
			continue
		}
		if cid, _ := m["id"].(string); cid != id {
			remaining = append(remaining, c)
		}
	}
	doc["candidates"] = remaining

	out, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(candidatePath, out, 0o644)
}

// ── target resolution ────────────────────────────────────────────────────

func resolveTarget(candidate map[string]interface{}) (string, error) {
	if promoteTarget != "" {
		// User specified explicit target.
		base := filepath.Base(promoteTarget)
		if _, ok := targetToListKey[base]; !ok {
			return "", fmt.Errorf("target %q is not a recognized canonical file; supported: %v",
				base, []string{"invariants.yaml", "failure_modes.yaml", "incident_patterns.yaml", "intents.yaml"})
		}
		return base, nil
	}

	// Auto-detect from class field.
	class, _ := candidate["class"].(string)
	target, ok := classToTarget[class]
	if !ok {
		return "", fmt.Errorf("cannot auto-detect target: unknown class %q; use --target to specify", class)
	}
	return target, nil
}

// ── helpers ──────────────────────────────────────────────────────────────

func relPath(base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return rel
}

// ── wiring ───────────────────────────────────────────────────────────────

func init() {
	awarenessPromoteCmd.Flags().StringVar(&promoteTarget, "target", "",
		"Target canonical YAML file (auto-detected from class if omitted)")
	awarenessPromoteCmd.Flags().BoolVar(&promoteDryRun, "dry-run", false,
		"Validate and show the resulting entry; do not modify files")
	awarenessPromoteCmd.Flags().BoolVar(&promoteNoRebuild, "no-rebuild", false,
		"Skip the automatic rebuild after promotion")

	awarenessCmd.AddCommand(awarenessPromoteCmd)
}
