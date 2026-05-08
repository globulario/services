// Package preflight composes all awareness capabilities into a single
// agent-facing report. No new graph theory — pure composition.
package preflight

// TaskClass labels the nature of a task for agent routing.
type TaskClass string

const (
	ClassLocalCodeChange       TaskClass = "LOCAL_CODE_CHANGE"
	ClassArchitectureSensitive TaskClass = "ARCHITECTURE_SENSITIVE"
	ClassConvergenceRisk       TaskClass = "CONVERGENCE_RISK"
	ClassPackageAdmission      TaskClass = "PACKAGE_ADMISSION"
	ClassRuntimeIncident       TaskClass = "RUNTIME_INCIDENT"
	ClassRetryLoop             TaskClass = "RETRY_LOOP"
	ClassRestartStorm          TaskClass = "RESTART_STORM"
	ClassStateMismatch         TaskClass = "STATE_MISMATCH"
	ClassDependencyCycle       TaskClass = "DEPENDENCY_CYCLE"
	ClassUnknownImpact         TaskClass = "UNKNOWN_IMPACT"
	ClassStaticFallback        TaskClass = "STATIC_KNOWLEDGE_FALLBACK"
)

// DidWeFixSection summarises the fix-ledger lookup result.
type DidWeFixSection struct {
	Status          string   `json:"status"`
	MatchedPatterns []string `json:"matched_patterns"`
	FixCases        []string `json:"fix_cases"`
	RemainingGaps   []string `json:"remaining_gaps"`
	NextAction      string   `json:"next_action"`
}

// PackageAdmissionSection holds package validation results.
type PackageAdmissionSection struct {
	Status  string   `json:"status"`
	Reasons []string `json:"reasons"`
}

// CycleWarning is a simplified cycle summary for preflight output.
type CycleWarning struct {
	Phase          string   `json:"phase"`
	Classification string   `json:"classification"`
	Path           []string `json:"path"`
	Reason         string   `json:"reason"`
}

// RawKnowledgeMatch is a conservative fallback match from the source YAML files.
// It exists to make NO_MATCH honest: graph lookup can be silent while the
// hand-authored truth files still contain relevant knowledge.
type RawKnowledgeMatch struct {
	Source       string   `json:"source"`
	Kind         string   `json:"kind"`
	ID           string   `json:"id"`
	Score        int      `json:"score"`
	MatchedTerms []string `json:"matched_terms"`
}

// FilteredMatch is a graph match that has low-trust provenance.
// The match is still present in the main invariants/failure_modes lists
// (with lower confidence) but is also reported here so callers understand
// why graph_filtered_by_trust_count > 0.
type FilteredMatch struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`       // invariant, failure_mode, forbidden_fix
	Reason     string `json:"reason"`     // stale, inferred, invalid, proposal, missing_provenance
	TrustLevel string `json:"trust_level"`
}

// RuntimeSection holds live cluster evidence included in the preflight.
type RuntimeSection struct {
	Included            bool                     `json:"included"`
	CapturedAt          string                   `json:"captured_at,omitempty"`
	DoctorFindings      []DoctorFindingSummary   `json:"doctor_findings"`
	ServiceStatuses     []ServiceStatusSummary   `json:"service_statuses"`
	WorkflowReceipts    []WorkflowReceiptSummary `json:"workflow_receipts"`
	StateDeltas         []StateDeltaSummary      `json:"state_deltas"`
	MatchedInvariants   []string                 `json:"matched_invariants"`
	MatchedFailureModes []string                 `json:"matched_failure_modes"`
	MetricWarnings      []string                 `json:"metric_warnings,omitempty"`
	Warnings            []string                 `json:"warnings"`
}

// DoctorFindingSummary is a compact view of a DoctorFinding.
type DoctorFindingSummary struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
}

// ServiceStatusSummary is a compact view of a ServiceStatus.
type ServiceStatusSummary struct {
	ServiceID string `json:"service_id"`
	State     string `json:"state"`
	NodeID    string `json:"node_id"`
}

// WorkflowReceiptSummary is a compact view of a WorkflowReceipt.
type WorkflowReceiptSummary struct {
	WorkflowType string `json:"workflow_type"`
	Status       string `json:"status"`
	ErrorMsg     string `json:"error_msg,omitempty"`
}

// StateDeltaSummary is a compact view of a StateDelta.
type StateDeltaSummary struct {
	ServiceID string `json:"service_id"`
	DeltaType string `json:"delta_type"`
	Desired   string `json:"desired_version,omitempty"`
	Installed string `json:"installed_version,omitempty"`
}

// Confidence describes how much the preflight knows about the task's safety.
type Confidence string

const (
	ConfidenceHigh    Confidence = "high"    // graph + raw YAML + runtime + metrics all ran
	ConfidenceMedium  Confidence = "medium"  // graph + raw YAML ran; runtime partial
	ConfidenceLow     Confidence = "low"     // only static analysis; runtime unavailable
	ConfidenceUnknown Confidence = "unknown" // nothing ran (no graph, no docs dir)
)

// CoverageState describes the result of a single coverage layer check.
type CoverageState string

const (
	CoverageNotChecked       CoverageState = "not_checked"
	CoverageNotApplicable    CoverageState = "not_applicable"
	CoverageCheckedClean     CoverageState = "checked_clean"
	CoverageCheckedWithMatch CoverageState = "checked_with_matches"
	CoverageFailed           CoverageState = "failed"
	CoverageNoop             CoverageState = "noop"
	CoverageStale            CoverageState = "stale"
)

// Coverage tracks which evidence layers were checked and their result.
type Coverage struct {
	Graph         CoverageState `json:"graph"`
	RawYAML       CoverageState `json:"raw_yaml"`
	Runtime       CoverageState `json:"runtime"`
	Metrics       CoverageState `json:"metrics"`
	CodeScan      CoverageState `json:"code_scan"`
	IncidentStore CoverageState `json:"incident_store"`
}

// GraphFreshnessReport summarises graph staleness for the report.
type GraphFreshnessReport struct {
	BuiltAt             string  `json:"built_at,omitempty"`
	AgeSeconds          float64 `json:"age_seconds,omitempty"`
	Stale               bool    `json:"stale"`
	StaleReason         string  `json:"stale_reason,omitempty"`
	KnowledgeMtime      string  `json:"knowledge_mtime,omitempty"`
	KnowledgeSourceHash string  `json:"knowledge_source_hash,omitempty"`
	RebuildRecommended  bool    `json:"rebuild_recommended"`
}

// Report is the complete output of a preflight run.
type Report struct {
	Task                string                   `json:"task"`
	Classification      []TaskClass              `json:"classification"`
	MatchedAliases      []string                 `json:"matched_aliases"`
	Services            []string                 `json:"services"`
	Packages            []string                 `json:"packages"`
	Files               []string                 `json:"files"`
	Invariants          []string                 `json:"invariants"`
	FailureModes        []string                 `json:"failure_modes"`
	ForbiddenFixes      []string                 `json:"forbidden_fixes"`
	CodeSmells          []string                 `json:"code_smells,omitempty"`
	DesignPatterns      []string                 `json:"design_patterns,omitempty"`
	AntiPatterns        []string                 `json:"anti_patterns,omitempty"`
	HashSchemas         []string                 `json:"hash_schemas,omitempty"`
	StateTransitions    []string                 `json:"state_transitions,omitempty"`
	DidWeFix            *DidWeFixSection         `json:"did_we_fix"`
	PackageAdmission    *PackageAdmissionSection `json:"package_admission,omitempty"`
	Cycles              []CycleWarning           `json:"cycles"`
	RequiredTests       []string                 `json:"required_tests"`
	RequiredSearches    []string                 `json:"required_searches"`
	RecommendedOrder    []string                 `json:"recommended_investigation_order"`
	AgentInstruction    string                   `json:"agent_instruction"`
	Warnings            []string                 `json:"warnings"`
	RawKnowledgeMatches []RawKnowledgeMatch      `json:"raw_knowledge_matches,omitempty"`
	Runtime             *RuntimeSection          `json:"runtime,omitempty"`
	Confidence          Confidence               `json:"confidence"`
	ConfidenceReason    string                   `json:"confidence_reason"`
	Coverage            Coverage                 `json:"coverage"`
	BlindSpots          []string                 `json:"blind_spots,omitempty"`
	GraphFreshness      *GraphFreshnessReport    `json:"graph_freshness,omitempty"`

	// Graph coverage detail — tells callers WHY a result has no/few matches.
	GraphAvailable            bool            `json:"graph_available"`
	GraphDBPath               string          `json:"graph_db_path,omitempty"`
	GraphMatchCount           int             `json:"graph_match_count"`
	GraphFilteredByTrustCount int             `json:"graph_filtered_by_trust_count"`
	RawYAMLMatchCount         int             `json:"raw_yaml_match_count"`
	FilteredMatches           []FilteredMatch `json:"filtered_matches,omitempty"`
}
