package opsknowledge

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Severity for a validation finding.
type Severity string

const (
	SevError Severity = "error" // blocks build / CI; YAML must be fixed
	SevWarn  Severity = "warn"  // surface but don't block
)

// Finding is one validation issue.
type Finding struct {
	Path     string   // file path
	EntryID  string   // entry id, or "" if file-level finding
	Severity Severity
	Code     string   // stable identifier (e.g. "id_missing", "tag_first_must_be_lifecycle")
	Message  string
}

func (f Finding) String() string {
	loc := f.Path
	if f.EntryID != "" {
		loc = fmt.Sprintf("%s [%s]", f.Path, f.EntryID)
	}
	return fmt.Sprintf("%s %s: %s — %s", strings.ToUpper(string(f.Severity)), f.Code, loc, f.Message)
}

// Refs holds the cross-file reference targets the validator needs to verify
// link integrity. Pre-loaded once and passed into Validate so we don't re-read
// the awareness YAMLs for every entry.
type Refs struct {
	InvariantIDs   map[string]bool // ids found in docs/awareness/invariants.yaml
	FailureModeIDs map[string]bool // ids found in docs/awareness/failure_modes.yaml
	RunbookPaths   map[string]bool // relative paths under runbooks/ that exist
	SeenEntryIDs   map[string]string // running set of entry ids → owning file path (for uniqueness check)
}

// LoadRefsFromAwareness reads the awareness YAML files needed for link
// integrity checks and the on-disk runbooks/ tree to build the Refs.
//
// Awareness IDs may live in invariants.yaml, failure_modes.yaml,
// convergence_rules.yaml, patterns.yaml, or forbidden_fixes.yaml depending on
// their kind. The validator treats invariant-style references and
// failure-mode-style references as separate categories, but accepts an id
// from ANY of those files into the matching category — operational-knowledge
// authors may not always know which awareness file an id lives in, and a
// shipped seed YAML is more useful than a strict mismatch error.
//
// awarenessDir = path to docs/awareness/
// opsKnowledgeDir = path to docs/operational-knowledge/
func LoadRefsFromAwareness(awarenessDir, opsKnowledgeDir string) (*Refs, error) {
	refs := &Refs{
		InvariantIDs:   map[string]bool{},
		FailureModeIDs: map[string]bool{},
		RunbookPaths:   map[string]bool{},
		SeenEntryIDs:   map[string]string{},
	}

	// Scan every yaml under awarenessDir for "id:" entries. Some awareness
	// files (proposals, decisions/) intentionally hold local-scope ids; we
	// pool them all into both buckets and let the validator surface only
	// genuinely dangling references.
	allIDs := map[string]bool{}
	awarenessYAMLs, err := filepath.Glob(filepath.Join(awarenessDir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("glob awareness yamls: %w", err)
	}
	for _, p := range awarenessYAMLs {
		if err := loadAwarenessIDs(p, allIDs); err != nil {
			slog.Warn("opsknowledge: LoadRefsFromAwareness failed to load awareness YAML", "path", p, "error", err)
		}
	}
	// Pool the same set into both categories — see doc above.
	for id := range allIDs {
		refs.InvariantIDs[id] = true
		refs.FailureModeIDs[id] = true
	}

	runbookDir := filepath.Join(opsKnowledgeDir, "runbooks")
	if entries, err := os.ReadDir(runbookDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.HasSuffix(strings.ToLower(e.Name()), ".yaml") {
				refs.RunbookPaths["runbooks/"+e.Name()] = true
			}
		}
	}
	return refs, nil
}

// loadAwarenessIDs walks an awareness YAML file and pulls every "- id: <foo>"
// entry id (matches both invariants.yaml and failure_modes.yaml shapes).
func loadAwarenessIDs(path string, into map[string]bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	// Both files share the shape: top-level list per category, each containing
	// items with `id`. We don't need typed structs — walk the generic tree.
	var generic any
	if err := yaml.Unmarshal(data, &generic); err != nil {
		return err
	}
	walkForIDs(generic, into)
	return nil
}

func walkForIDs(node any, into map[string]bool) {
	switch t := node.(type) {
	case map[string]any:
		if id, ok := t["id"].(string); ok && id != "" {
			into[id] = true
		}
		for _, v := range t {
			walkForIDs(v, into)
		}
	case []any:
		for _, v := range t {
			walkForIDs(v, into)
		}
	}
}

// Validate runs all schema rules on a single File. Returns all findings.
// The caller should track refs.SeenEntryIDs across files to catch duplicates.
func Validate(f *File, refs *Refs) []Finding {
	var findings []Finding

	// File-level rules
	if f.SchemaVersion != 1 {
		findings = append(findings, Finding{
			Path: f.Path, Severity: SevError, Code: "schema_version_unsupported",
			Message: fmt.Sprintf("schema_version=%d, only 1 is supported", f.SchemaVersion),
		})
	}
	switch f.FileKind {
	case FileKindStage, FileKindRunbook, FileKindServiceRole:
		// ok
	default:
		findings = append(findings, Finding{
			Path: f.Path, Severity: SevError, Code: "file_kind_invalid",
			Message: fmt.Sprintf("file_kind=%q, must be one of stage|runbook|service-role", f.FileKind),
		})
	}
	if f.Metadata.Title == "" {
		findings = append(findings, Finding{
			Path: f.Path, Severity: SevError, Code: "metadata_title_missing",
			Message: "metadata.title is required",
		})
	}
	if f.Metadata.Description == "" {
		findings = append(findings, Finding{
			Path: f.Path, Severity: SevWarn, Code: "metadata_description_missing",
			Message: "metadata.description is recommended",
		})
	}
	if len(f.Entries) == 0 {
		findings = append(findings, Finding{
			Path: f.Path, Severity: SevError, Code: "no_entries",
			Message: "file has no entries",
		})
	}

	// Per-entry rules
	for i := range f.Entries {
		findings = append(findings, validateEntry(f.Path, &f.Entries[i], refs)...)
	}

	return findings
}

func validateEntry(filePath string, e *Entry, refs *Refs) []Finding {
	var fs []Finding
	emit := func(sev Severity, code, msg string) {
		fs = append(fs, Finding{Path: filePath, EntryID: e.ID, Severity: sev, Code: code, Message: msg})
	}

	// id required + namespaced + globally unique
	if e.ID == "" {
		emit(SevError, "id_missing", "entry id is required")
		return fs // nothing else makes sense without an id
	}
	if !strings.HasPrefix(e.ID, IDPrefix) {
		emit(SevError, "id_namespace_invalid",
			fmt.Sprintf("id %q must start with %q", e.ID, IDPrefix))
	}
	if refs != nil && refs.SeenEntryIDs != nil {
		if existing, dup := refs.SeenEntryIDs[e.ID]; dup {
			emit(SevError, "id_duplicate",
				fmt.Sprintf("id %q already declared in %s", e.ID, existing))
		} else {
			refs.SeenEntryIDs[e.ID] = filePath
		}
	}

	// type required + valid
	switch e.Type {
	case TypeArchitecture, TypeDecision, TypeReference, TypeSkill, TypeDebug:
		// ok
	case "":
		emit(SevError, "type_missing", "entry type is required")
	default:
		emit(SevError, "type_invalid",
			fmt.Sprintf("type=%q, must be one of ARCHITECTURE|DECISION|REFERENCE|SKILL|DEBUG", e.Type))
	}

	// title required
	if strings.TrimSpace(e.Title) == "" {
		emit(SevError, "title_missing", "entry title is required")
	}

	// tags: first must be a lifecycle stage
	if len(e.Tags) == 0 {
		emit(SevError, "tags_missing", "entry must have at least one tag (lifecycle stage)")
	} else {
		first := e.Tags[0]
		switch first {
		case StageDay0, StageDay1, StageDay2, StageAlways:
			// ok
		default:
			emit(SevError, "tag_first_must_be_lifecycle",
				fmt.Sprintf("first tag %q must be one of day-0|day-1|day-2|always", first))
		}
	}

	// applies_when.cluster_phases required and non-empty
	if len(e.AppliesWhen.ClusterPhases) == 0 {
		emit(SevError, "applies_when_cluster_phases_empty",
			"applies_when.cluster_phases must be non-empty")
	} else {
		for _, p := range e.AppliesWhen.ClusterPhases {
			switch p {
			case StageDay0, StageDay1, StageDay2:
				// ok
			default:
				emit(SevError, "applies_when_cluster_phase_invalid",
					fmt.Sprintf("cluster_phase %q must be one of day-0|day-1|day-2", p))
			}
		}
	}

	// services_healthy must be a subset of services_present
	if len(e.AppliesWhen.ServicesHealthy) > 0 {
		present := map[string]bool{}
		for _, s := range e.AppliesWhen.ServicesPresent {
			present[s] = true
		}
		for _, h := range e.AppliesWhen.ServicesHealthy {
			if !present[h] {
				emit(SevError, "services_healthy_not_in_present",
					fmt.Sprintf("services_healthy contains %q which is not in services_present", h))
			}
		}
	}

	// content size cap
	if len(e.Content) > MaxEntryContentBytes {
		emit(SevError, "content_too_large",
			fmt.Sprintf("content is %d bytes, exceeds max %d", len(e.Content), MaxEntryContentBytes))
	}
	if strings.TrimSpace(e.Content) == "" {
		emit(SevError, "content_missing", "entry content is required")
	}

	// provenance.source must be "seed"
	if e.Provenance.Source != ProvenanceSourceSeed {
		// Empty Source is treated as a warning — the build tool will stamp the
		// canonical defaults. In source YAML the operator may omit provenance
		// entirely; that's fine.
		if e.Provenance.Source == "" {
			emit(SevWarn, "provenance_source_unset",
				"provenance.source is unset; the build tool will set it to 'seed'")
		} else {
			emit(SevError, "provenance_source_invalid",
				fmt.Sprintf("provenance.source=%q, must be 'seed'", e.Provenance.Source))
		}
	}

	// link integrity
	if refs != nil {
		for _, id := range e.Links.AwarenessInvariants {
			if !refs.InvariantIDs[id] {
				emit(SevError, "link_invariant_not_found",
					fmt.Sprintf("links.awareness_invariants references %q which does not exist in docs/awareness/invariants.yaml", id))
			}
		}
		for _, id := range e.Links.AwarenessFailureModes {
			if !refs.FailureModeIDs[id] {
				emit(SevError, "link_failure_mode_not_found",
					fmt.Sprintf("links.awareness_failure_modes references %q which does not exist in docs/awareness/failure_modes.yaml", id))
			}
		}
		for _, p := range e.Links.Runbooks {
			// allow either "runbooks/foo.yaml" or "foo.yaml"
			lookup := p
			if !strings.HasPrefix(p, "runbooks/") {
				lookup = "runbooks/" + p
			}
			if !refs.RunbookPaths[lookup] {
				emit(SevError, "link_runbook_not_found",
					fmt.Sprintf("links.runbooks references %q which does not exist under docs/operational-knowledge/runbooks/", p))
			}
		}
	}

	return fs
}

// HasErrors reports whether any finding is severity=error.
func HasErrors(findings []Finding) bool {
	for _, f := range findings {
		if f.Severity == SevError {
			return true
		}
	}
	return false
}
