package main

// awareness_validate_cmd.go — Phase 6 graph validator.
//
// `globular awareness validate` walks the YAML sources that feed yaml2nt and
// flags structural problems BEFORE rebuild. Catches the rot we've already
// hit in practice: references to invariant/failure-mode IDs that don't
// exist, expressed_by/affected_files pointing at deleted Go files,
// duplicate IDs across files.
//
// Read-only, no gRPC, no schema, no live store dependency. Pure YAML
// walking. ~300 LOC.
//
// What this does NOT do (deliberately):
//   - SHACL or full ontology validation — overkill for v1
//   - Test reference resolution (required_tests: foo:TestX → does TestX
//     exist in Go) — needs Go discovery, deferred
//   - Symbol-level checks against @awareness annotations — needs AST,
//     deferred
//   - Severity/status enum validation — schema is still firming up;
//     would emit noise

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	validateDirs       []string
	validateRepoRoot   string
	validateFormat     string
	validateFailOnWarn bool
)

var awarenessValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Static check of awareness YAML sources (dangling refs, missing files, duplicate IDs)",
	Long: `Walks docs/awareness/ + docs/intent/ (by default) and flags structural
problems before yaml2nt rebuilds the graph. Read-only — never modifies
files. CI-friendly: exits non-zero when any finding has severity=error.

Checks (v1):
  - dangling related_invariants: references an ID that doesn't exist
  - dangling related_failure_modes: same shape, different class
  - missing source file: expressed_by / affected_files paths must point
    at .go files that exist on disk
  - missing reference file: ImplementationPattern reference_files paths
    must exist on disk
  - duplicate ID: the same id appears in two entity records

What this does NOT do in v1: test-reference resolution, symbol-level
checks, severity/status enum validation. Those land later when the
schema is firmer.`,
	RunE: runAwarenessValidate,
}

func runAwarenessValidate(cmd *cobra.Command, args []string) error {
	repoRoot, err := resolveRepoRoot(validateRepoRoot)
	if err != nil {
		return err
	}

	dirs := validateDirs
	if len(dirs) == 0 {
		dirs = []string{
			filepath.Join(repoRoot, "docs/awareness"),
			filepath.Join(repoRoot, "docs/intent"),
		}
	}

	report, err := runValidate(repoRoot, dirs)
	if err != nil {
		return err
	}

	switch validateFormat {
	case "json":
		printValidateJSON(report)
	default:
		printValidateTable(report)
	}

	errCount := 0
	warnCount := 0
	for _, f := range report.Findings {
		if f.Severity == "error" {
			errCount++
		} else if f.Severity == "warn" {
			warnCount++
		}
	}
	if errCount > 0 || (validateFailOnWarn && warnCount > 0) {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return fmt.Errorf("awareness validate: %d error(s), %d warn(s)", errCount, warnCount)
	}
	return nil
}

// ─── core types ──────────────────────────────────────────────────────────

// validateFinding is one structural problem discovered during the walk.
type validateFinding struct {
	Severity string `json:"severity"` // "error" | "warn"
	Check    string `json:"check"`    // dangling_invariant_ref | ...
	File     string `json:"file"`     // repo-relative path of the offending YAML
	EntityID string `json:"entity_id,omitempty"`
	Ref      string `json:"ref,omitempty"`
	Message  string `json:"message"`
}

// validateReport is the aggregate the JSON/table printers consume.
type validateReport struct {
	RepoRoot string             `json:"repo_root"`
	Scanned  []string           `json:"scanned_dirs"`
	Findings []validateFinding  `json:"findings"`
	Counts   map[string]int     `json:"counts_by_check"`
}

// idIndex maps a class (e.g. "invariant", "failure_mode") to the set of
// known IDs in that class, plus where each ID was defined (for dup
// detection).
type idIndex struct {
	byClass map[string]map[string][]string // class → id → [source files]
}

func newIDIndex() *idIndex {
	return &idIndex{byClass: map[string]map[string][]string{}}
}

func (i *idIndex) record(class, id, source string) {
	if i.byClass[class] == nil {
		i.byClass[class] = map[string][]string{}
	}
	i.byClass[class][id] = append(i.byClass[class][id], source)
}

func (i *idIndex) has(class, id string) bool {
	if i.byClass[class] == nil {
		return false
	}
	_, ok := i.byClass[class][id]
	return ok
}

// ─── walk ────────────────────────────────────────────────────────────────

func runValidate(repoRoot string, dirs []string) (*validateReport, error) {
	report := &validateReport{
		RepoRoot: repoRoot,
		Counts:   map[string]int{},
	}

	files, err := collectYAMLFiles(dirs)
	if err != nil {
		return nil, err
	}

	// Pass 1: collect all IDs by class.
	index := newIDIndex()
	docs := make(map[string]*yamlDoc, len(files))
	for _, f := range files {
		doc, err := parseYAMLDoc(f, repoRoot)
		if err != nil {
			report.Findings = append(report.Findings, validateFinding{
				Severity: "warn",
				Check:    "yaml_parse_failed",
				File:     relTo(repoRoot, f),
				Message:  err.Error(),
			})
			continue
		}
		docs[f] = doc
		for _, e := range doc.entities {
			if e.id != "" {
				index.record(e.class, e.id, relTo(repoRoot, f))
			}
		}
		report.Scanned = append(report.Scanned, relTo(repoRoot, f))
	}

	// Pass 2: validate references, file existence, duplicates.
	for _, f := range files {
		doc := docs[f]
		if doc == nil {
			continue
		}
		relFile := relTo(repoRoot, f)
		for _, e := range doc.entities {
			validateEntity(report, index, repoRoot, relFile, e)
		}
	}

	// Duplicate-ID check is global. The "File" field carries the first
	// source for table-output readability; the full list lives in Message.
	for class, ids := range index.byClass {
		for id, sources := range ids {
			if len(sources) > 1 {
				report.Findings = append(report.Findings, validateFinding{
					Severity: "error",
					Check:    "duplicate_id",
					File:     sources[0],
					EntityID: id,
					Message: fmt.Sprintf(
						"%s id %q defined in %d files: %s",
						class, id, len(sources), strings.Join(sources, ", ")),
				})
			}
		}
	}

	// Sort findings for deterministic output (file, then check, then ref).
	sort.SliceStable(report.Findings, func(i, j int) bool {
		a, b := report.Findings[i], report.Findings[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Check != b.Check {
			return a.Check < b.Check
		}
		return a.Ref < b.Ref
	})
	for _, f := range report.Findings {
		report.Counts[f.Check]++
	}
	return report, nil
}

// yamlEntity is one record extracted from a YAML file — generic shape so
// the validator handles collections (invariants.yaml) and single-entity
// files (intent/*.yaml, implementation_patterns/*.yaml) uniformly.
type yamlEntity struct {
	class string
	id    string
	// references this entity makes — IDs we'll verify against the index.
	relatedInvariants   []string
	relatedFailureModes []string
	// file paths referenced (expressed_by, affected_files, reference_files).
	referencedFiles []string
}

type yamlDoc struct {
	entities []yamlEntity
}

// classByCollectionKey maps top-level YAML keys to the entity class they
// hold. Keys not listed here yield empty entities — the file is parsed
// but contributes no IDs.
var classByCollectionKey = map[string]string{
	"invariants":        "invariant",
	"failure_modes":     "failure_mode",
	"incident_patterns": "incident_pattern",
	"forbidden_fixes":   "forbidden_fix",
	"required_tests":    "required_test",
	"decisions":         "decision",
	"guardrails":        "guardrail",
	"patterns":          "pattern",
	"design_patterns":   "pattern",
	"services":          "service",
}

// parseYAMLDoc reads a YAML file and extracts every (class, id) record
// plus the references each record makes. Tolerates the two shapes the
// importer knows: top-level collection (key→[]entities) or single-entity
// (top-level id + level/class).
func parseYAMLDoc(path, repoRoot string) (*yamlDoc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}
	if raw == nil {
		return &yamlDoc{}, nil
	}

	doc := &yamlDoc{}

	// Collection shape: top-level key matches a known class.
	for key, class := range classByCollectionKey {
		v, ok := raw[key]
		if !ok {
			continue
		}
		list, ok := v.([]interface{})
		if !ok {
			continue
		}
		for _, item := range list {
			e := extractEntity(item, class)
			if e.id != "" || len(e.relatedInvariants) > 0 || len(e.relatedFailureModes) > 0 || len(e.referencedFiles) > 0 {
				doc.entities = append(doc.entities, e)
			}
		}
	}

	// Single-entity shape: id + level (intent) or id + class:ImplementationPattern.
	if id, ok := raw["id"].(string); ok && id != "" {
		class := ""
		if _, has := raw["level"]; has {
			class = "intent"
		}
		if cls, ok := raw["class"].(string); ok && cls == "ImplementationPattern" {
			class = "implementation_pattern"
		}
		if class != "" {
			e := extractEntity(raw, class)
			e.id = id
			doc.entities = append(doc.entities, e)
		}
	}

	return doc, nil
}

// extractEntity pulls the validation-relevant fields out of one YAML record.
// All field reads are defensive — missing keys yield empty slices, not panics.
func extractEntity(node interface{}, class string) yamlEntity {
	m, ok := node.(map[string]interface{})
	if !ok {
		return yamlEntity{class: class}
	}
	e := yamlEntity{class: class}
	if id, ok := m["id"].(string); ok {
		e.id = id
	}
	e.relatedInvariants = stringsField(m, "related_invariants")
	e.relatedFailureModes = stringsField(m, "related_failure_modes")
	e.referencedFiles = append(e.referencedFiles, stringsField(m, "expressed_by")...)
	e.referencedFiles = append(e.referencedFiles, stringsField(m, "affected_files")...)
	// implementation_pattern reference_files: list of {path, role} maps.
	if v, ok := m["reference_files"].([]interface{}); ok {
		for _, item := range v {
			if mm, ok := item.(map[string]interface{}); ok {
				if p, ok := mm["path"].(string); ok {
					e.referencedFiles = append(e.referencedFiles, p)
				}
			}
		}
	}
	// invariants.yaml's "protects: { files: [...] }" nested form.
	if v, ok := m["protects"].(map[string]interface{}); ok {
		e.referencedFiles = append(e.referencedFiles, stringsField(v, "files")...)
	}
	return e
}

// stringsField returns the []string at m[key], tolerating yaml.v3's interface{} slice form.
func stringsField(m map[string]interface{}, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	list, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// validateEntity emits findings for one entity record.
func validateEntity(report *validateReport, idx *idIndex, repoRoot, file string, e yamlEntity) {
	for _, ref := range e.relatedInvariants {
		if !idx.has("invariant", ref) {
			report.Findings = append(report.Findings, validateFinding{
				Severity: "error",
				Check:    "dangling_invariant_ref",
				File:     file,
				EntityID: e.id,
				Ref:      ref,
				Message:  fmt.Sprintf("related_invariants references invariant %q which does not exist", ref),
			})
		}
	}
	for _, ref := range e.relatedFailureModes {
		if !idx.has("failure_mode", ref) {
			report.Findings = append(report.Findings, validateFinding{
				Severity: "error",
				Check:    "dangling_failure_mode_ref",
				File:     file,
				EntityID: e.id,
				Ref:      ref,
				Message:  fmt.Sprintf("related_failure_modes references failure_mode %q which does not exist", ref),
			})
		}
	}
	for _, path := range e.referencedFiles {
		// Only enforce existence for paths that look like Go source. Other
		// kinds of references (docs, awareness YAML) follow different shapes
		// and are validated elsewhere — out of scope for v1.
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		// Some legacy entries use "services/golang/..." while others use
		// the canonical "golang/...". Accept either.
		candidates := []string{
			filepath.Join(repoRoot, path),
			filepath.Join(repoRoot, strings.TrimPrefix(path, "services/")),
		}
		exists := false
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				exists = true
				break
			}
		}
		if !exists {
			report.Findings = append(report.Findings, validateFinding{
				Severity: "error",
				Check:    "missing_source_file",
				File:     file,
				EntityID: e.id,
				Ref:      path,
				Message:  fmt.Sprintf("path %q referenced from entity %q does not exist on disk", path, e.id),
			})
		}
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────

func collectYAMLFiles(dirs []string) ([]string, error) {
	var files []string
	for _, d := range dirs {
		err := filepath.WalkDir(d, func(p string, info fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip missing dirs without erroring
			}
			if info.IsDir() {
				// Skip generated content — it's machine output, not
				// authored awareness, and rotting refs there are not the
				// validator's problem.
				if filepath.Base(p) == "generated" || filepath.Base(p) == "cache" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(p, ".yaml") || strings.HasSuffix(p, ".yml") {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

func resolveRepoRoot(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// Walk up until we find a docs/awareness directory — that's the
	// services repo root.
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "docs", "awareness")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return cwd, nil // fall back; --dir overrides anyway
}

func relTo(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return r
}

// ─── output ──────────────────────────────────────────────────────────────

func printValidateTable(r *validateReport) {
	if len(r.Findings) == 0 {
		fmt.Printf("awareness validate: scanned %d files in %s — no findings\n",
			len(r.Scanned), r.RepoRoot)
		return
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SEVERITY\tCHECK\tFILE\tENTITY\tREF\tMESSAGE")
	for _, f := range r.Findings {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			strings.ToUpper(f.Severity), f.Check, f.File,
			validateTruncate(f.EntityID, 50), validateTruncate(f.Ref, 60), validateTruncate(f.Message, 100))
	}
	tw.Flush()
	fmt.Printf("\nawareness validate: scanned %d files, %d finding(s):\n",
		len(r.Scanned), len(r.Findings))
	for check, n := range r.Counts {
		fmt.Printf("  %s: %d\n", check, n)
	}
}

func printValidateJSON(r *validateReport) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(r)
}

func validateTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// ─── registration ────────────────────────────────────────────────────────

func init() {
	awarenessValidateCmd.Flags().StringSliceVar(&validateDirs, "dir", nil,
		"directories to scan (default: docs/awareness + docs/intent under repo root)")
	awarenessValidateCmd.Flags().StringVar(&validateRepoRoot, "repo-root", "",
		"repo root for resolving relative paths (default: walk up from cwd to find docs/awareness)")
	awarenessValidateCmd.Flags().StringVar(&validateFormat, "format", "table",
		"output format: table | json")
	awarenessValidateCmd.Flags().BoolVar(&validateFailOnWarn, "fail-on-warn", false,
		"exit non-zero when any warning is found (default: only errors trigger failure)")
	awarenessCmd.AddCommand(awarenessValidateCmd)
}
