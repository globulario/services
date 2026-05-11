package semanticdiff

import (
	"context"

	"github.com/globulario/services/golang/awareness/assurance"
)

// Layer names.
const (
	LayerArtifact  = "Artifact"
	LayerDesired   = "Desired"
	LayerInstalled = "Installed"
	LayerRuntime   = "Runtime"
	LayerUnknown   = "Unknown"
)

// Verdict values.
const (
	VerdictAllow             = "allow"
	VerdictAllowWithWarnings = "allow_with_warnings"
	VerdictBlock             = "block"
	VerdictUnknown           = "unknown"
)

// Severity values.
const (
	SeverityInfo      = "info"
	SeverityWarning   = "warning"
	SeverityCritical  = "critical"
	SeverityForbidden = "forbidden"
)

// SemanticDiffRequest is the input to Interpret.
type SemanticDiffRequest struct {
	SessionID    string
	Task         string
	DiffText     string
	DiffSource   string
	GitBase      string
	GitHead      string
	Files        []string
	Components   []string
	RequireClean bool
}

// SemanticDiffReport is the full output of a semantic diff interpretation.
type SemanticDiffReport struct {
	ID              string
	SessionID       string
	Task            string
	DiffSource      string
	GitBase         string
	GitHead         string
	Verdict         string
	Severity        string
	Summary         string
	Fingerprint     string
	Findings        []SemanticDiffFinding
	Atoms           []SemanticDiffAtom
	Transitions     []LayerTransition
	AuthorityChange *AuthorityChange         `json:"authority_change,omitempty"`
	AuthorityBudget *AuthorityBudget         `json:"authority_budget,omitempty"`
	Trust           *assurance.TrustEnvelope `json:"trust,omitempty"`
	CreatedAt       int64
}

// AuthorityChange describes the highest-risk detected movement across
// Globular's state authority layers for this diff.
type AuthorityChange struct {
	Detected       bool   `json:"detected"`
	FromLayer      string `json:"from_layer,omitempty"`
	ToLayer        string `json:"to_layer,omitempty"`
	Risk           string `json:"risk"` // none|medium|high|critical
	RequiresReview bool   `json:"requires_review"`
}

// AuthorityBudget states what minimum trust posture is required before the
// semantic diff can be treated as safe for authority-affecting changes.
type AuthorityBudget struct {
	LayerChanged              bool   `json:"layer_changed"`
	SourceLayer               string `json:"source_layer,omitempty"`
	TargetLayer               string `json:"target_layer,omitempty"`
	AllowedWithoutReview      bool   `json:"allowed_without_review"`
	RequiredAwarenessCoverage string `json:"required_awareness_coverage"` // baseline|sufficient|strong
}

// SemanticDiffFinding is a structured violation or observation.
type SemanticDiffFinding struct {
	ID             string
	Kind           string
	Severity       string
	FilePath       string
	Symbol         string
	LayerFrom      string
	LayerTo        string
	AuthorityFrom  string
	AuthorityTo    string
	InvariantID    string
	Message        string
	Evidence       string
	Recommendation string
}

// SemanticDiffAtom is a single detected semantic change.
type SemanticDiffAtom struct {
	ID            string
	FilePath      string
	Symbol        string
	AtomKind      string
	BeforeSummary string
	AfterSummary  string
	Confidence    string
	Evidence      string
}

// LayerTransition describes a detected state-layer crossing.
type LayerTransition struct {
	FilePath       string
	Symbol         string
	LayerFrom      string
	LayerTo        string
	TransitionKind string
	Allowed        bool
	Reason         string
}

// ParsedDiff is the result of parsing a unified diff.
type ParsedDiff struct {
	Files       []*DiffFile
	Fingerprint string // sha256:<hex>
}

// DiffFile is a single changed file in a diff.
type DiffFile struct {
	Path    string
	OldPath string
	Hunks   []*DiffHunk
}

// DiffHunk is a single change hunk within a file.
type DiffHunk struct {
	Symbol       string // function/method name from @@ context hint
	AddedLines   []string
	RemovedLines []string
}

// ChangedSymbol is a symbol that changed in the diff.
type ChangedSymbol struct {
	FilePath string
	Name     string
	Kind     string
}

// InvariantImpact describes impact on a known invariant.
type InvariantImpact struct {
	InvariantID string
	AtomKind    string
	FilePath    string
	Symbol      string
}

// Interpreter is the main entry point.
type Interpreter struct{}

// New returns a new Interpreter.
func New() *Interpreter { return &Interpreter{} }

// Interpret runs the full semantic diff pipeline.
func (i *Interpreter) Interpret(ctx context.Context, req SemanticDiffRequest) (*SemanticDiffReport, error) {
	return InterpretSemanticDiff(ctx, req)
}
