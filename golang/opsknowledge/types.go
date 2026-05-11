// Package opsknowledge loads, validates, and canonicalizes the operational-knowledge YAML
// seed entries that ship in `docs/operational-knowledge/`.
//
// See docs/operational-knowledge/SCHEMA.md for the authoritative schema.
package opsknowledge

// File is one parsed YAML file (one of stages/, runbooks/, service-roles/).
type File struct {
	SchemaVersion int      `yaml:"schema_version"`
	FileKind      string   `yaml:"file_kind"`
	Metadata      Metadata `yaml:"metadata"`
	Entries       []Entry  `yaml:"entries"`

	// Path is the absolute path the file was loaded from. Not part of the YAML
	// schema; populated by the loader for diagnostics and provenance.
	Path string `yaml:"-"`
}

type Metadata struct {
	Title           string   `yaml:"title"`
	Description     string   `yaml:"description"`
	SourceDocuments []string `yaml:"source_documents,omitempty"`
}

// Entry is one operational-knowledge entry that becomes one AI Memory row.
type Entry struct {
	ID          string      `yaml:"id"`
	Type        string      `yaml:"type"` // ARCHITECTURE | DECISION | REFERENCE | SKILL | DEBUG
	Title       string      `yaml:"title"`
	Tags        []string    `yaml:"tags"`
	AppliesWhen AppliesWhen `yaml:"applies_when"`
	Content     string      `yaml:"content"`
	Links       Links       `yaml:"links,omitempty"`
	RelatedIDs  []string    `yaml:"related_ids,omitempty"`
	Provenance  Provenance  `yaml:"provenance"`

	// Runbook-only field. Optional everywhere, expected on file_kind=runbook entries.
	Procedure []ProcedureStep `yaml:"procedure,omitempty"`

	// Free-form sections used by some runbooks (success_criteria, follow_up,
	// known_pitfalls, etc.). Captured generically so we don't break the loader
	// if files add new sections; the validator does not enforce these.
	Extra map[string]any `yaml:",inline"`
}

type AppliesWhen struct {
	ClusterPhases   []string `yaml:"cluster_phases"`
	ServicesPresent []string `yaml:"services_present,omitempty"`
	ServicesHealthy []string `yaml:"services_healthy,omitempty"`
}

type Links struct {
	AwarenessInvariants   []string `yaml:"awareness_invariants,omitempty"`
	AwarenessFailureModes []string `yaml:"awareness_failure_modes,omitempty"`
	Runbooks              []string `yaml:"runbooks,omitempty"`
	CLICommands           []string `yaml:"cli_commands,omitempty"`
}

type Provenance struct {
	Source       string `yaml:"source"` // must be "seed" for entries in this directory
	SeedVersion  string `yaml:"seed_version"`
	SeedSHA256   string `yaml:"seed_sha256"`
	Immutable    bool   `yaml:"immutable"`
}

// ProcedureStep is one phased step in a runbook.
type ProcedureStep struct {
	Phase       string         `yaml:"phase"`
	Description string         `yaml:"description"`
	Warning     string         `yaml:"warning,omitempty"`
	Note        string         `yaml:"note,omitempty"`
	Commands    []CommandStep  `yaml:"commands,omitempty"`
}

type CommandStep struct {
	Cmd     string `yaml:"cmd"`
	Expect  string `yaml:"expect,omitempty"`
	Note    string `yaml:"note,omitempty"`
	Warning string `yaml:"warning,omitempty"`
}

// FileKind constants.
const (
	FileKindStage       = "stage"
	FileKindRunbook     = "runbook"
	FileKindServiceRole = "service-role"
)

// EntryType constants — must match docs/ai/ai-services.md MemoryType values.
const (
	TypeArchitecture = "ARCHITECTURE"
	TypeDecision     = "DECISION"
	TypeReference    = "REFERENCE"
	TypeSkill        = "SKILL"
	TypeDebug        = "DEBUG"
)

// LifecycleStage constants — first tag of every entry MUST be one of these.
const (
	StageDay0   = "day-0"
	StageDay1   = "day-1"
	StageDay2   = "day-2"
	StageAlways = "always"
)

// ProvenanceSource constants — entries in this directory always carry "seed".
const ProvenanceSourceSeed = "seed"

// IDPrefix — every entry id MUST start with this. Prevents collision with
// non-seed memories in AI Memory.
const IDPrefix = "ops."

// MaxEntryContentBytes — per SCHEMA.md, content size cap to keep seed entries concise.
const MaxEntryContentBytes = 16 * 1024
