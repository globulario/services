// Package preflight composes all awareness capabilities into a single
// agent-facing report. No new graph theory — pure composition.
package preflight

import "github.com/globulario/services/golang/awareness/assurance"

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

// CollectorHealthSummary is a compact view of a single collector's outcome.
// Populated from the most recent graph_builds.collector_health_json record.
type CollectorHealthSummary struct {
	CollectorID  string `json:"collector_id"`
	Status       string `json:"status"` // "ok" | "skipped" | "error"
	NodesEmitted int    `json:"nodes_emitted"`
	Error        string `json:"error,omitempty"`
}

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
	Kind       string `json:"kind"`   // invariant, failure_mode, forbidden_fix
	Reason     string `json:"reason"` // stale, inferred, invalid, proposal, missing_provenance
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
	WorkflowRuntime     *WorkflowRuntimeSection  `json:"workflow_runtime,omitempty"`
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

// WorkflowRuntimeSection summarises live workflow execution coverage in the preflight.
type WorkflowRuntimeSection struct {
	Coverage        string `json:"coverage"`  // checked_clean | checked_with_matches | failed | stale | disabled | not_checked
	Freshness       string `json:"freshness"` // fresh | stale | unknown
	Source          string `json:"source"`    // workflow_service_grpc | graph_cache | none
	RunsSeen        int    `json:"runs_seen"`
	FailedRuns      int    `json:"failed_runs"`
	BlockedRuns     int    `json:"blocked_runs"`
	RetryStorms     int    `json:"retry_storms"`
	CollectedAt     string `json:"collected_at,omitempty"`
	TTLSeconds      int    `json:"ttl_seconds,omitempty"`
	CollectorStatus string `json:"collector_status,omitempty"` // ok | partial | failed | disabled
}

// Coverage tracks which evidence layers were checked and their result.
type Coverage struct {
	Graph           CoverageState `json:"graph"`
	RawYAML         CoverageState `json:"raw_yaml"`
	Runtime         CoverageState `json:"runtime"`
	Metrics         CoverageState `json:"metrics"`
	CodeScan        CoverageState `json:"code_scan"`
	IncidentStore   CoverageState `json:"incident_store"`
	WorkflowRuntime CoverageState `json:"workflow_runtime"`
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
	LastBuildDurationMs int64   `json:"last_build_duration_ms,omitempty"`
}

// GoFileCoverageReport holds the graph Go-file coverage metrics as reported by
// preflight. It mirrors enforce.GoFileCoverageResult but lives here to avoid a
// circular import (enforce imports preflight via strict.go).
type GoFileCoverageReport struct {
	EligibleGoFilesTotal   int      `json:"eligible_go_files_total"`
	IndexedGoFilesTotal    int      `json:"indexed_go_files_total"`
	CoveragePercentGoFiles float64  `json:"coverage_percent_go_files"`
	EligibleNonTestGoFiles int      `json:"eligible_non_test_go_files_total"`
	IndexedNonTestGoFiles  int      `json:"indexed_non_test_go_files_total"`
	MissingFiles           []string `json:"missing_files,omitempty"`
	BlindSpots             []string `json:"blind_spots,omitempty"`
	// ConfidenceImpact: none | low | medium | high
	ConfidenceImpact string `json:"confidence_impact"`
}

// ConfidenceFactors explains why a confidence level was assigned.
type ConfidenceFactors struct {
	Coverage        CoverageState `json:"coverage"`
	Provenance      string        `json:"provenance"`
	GraphFreshness  CoverageState `json:"graph_freshness"`
	PathQuality     string        `json:"path_quality"`
	RuntimeEvidence CoverageState `json:"runtime_evidence"`
}

// SafetyStatus indicates whether the current evidence quality is safe enough
// to proceed without escalation.
type SafetyStatus string

const (
	SafetyStatusProceed        SafetyStatus = "PROCEED"
	SafetyStatusUnknownNotSafe SafetyStatus = "UNKNOWN_NOT_SAFE"
)

type RiskTier string

const (
	RiskLow    RiskTier = "low"
	RiskMedium RiskTier = "medium"
	RiskHigh   RiskTier = "high"
)

// DegradedModePlaybook provides deterministic guidance when evidence quality
// is degraded and preflight cannot safely produce high-confidence decisions.
type DegradedModePlaybook struct {
	Enabled          bool     `json:"enabled"`
	Reason           string   `json:"reason,omitempty"`
	AllowedNextSteps []string `json:"allowed_next_steps,omitempty"`
	BlockedActions   []string `json:"blocked_actions,omitempty"`
	StopConditions   []string `json:"stop_conditions,omitempty"`
}

// LiveOverlayFreshness reports the current freshness of the live mirror overlay.
// It is separate from graph freshness: the graph can be fresh (recently built)
// while the live overlay is stale (no recent live-snapshot run).
type LiveOverlayFreshness struct {
	Status      string                   `json:"status"` // "fresh" | "stale" | "absent" | "failed" | "partial"
	AgeSeconds  float64                  `json:"age_seconds"`
	CollectedAt string                   `json:"collected_at,omitempty"`
	Collectors  []CollectorHealthSummary `json:"collectors,omitempty"`
}

// LiveOverlayTTLSeconds is how long a live snapshot stays "fresh".
const LiveOverlayTTLSeconds = 300 // 5 minutes
// LiveOverlayStaleSeconds is when a live snapshot is considered "stale" but not absent.
const LiveOverlayStaleSeconds = 900 // 15 minutes

// FindingType labels the kind of awareness object a decision trace describes.
// The enum mirrors the four canonical Report match buckets so callers can
// route on type without re-classifying the underlying id.
type FindingType string

const (
	FindingInvariant    FindingType = "invariant"
	FindingFailureMode  FindingType = "failure_mode"
	FindingForbiddenFix FindingType = "forbidden_fix"
	FindingRawKnowledge FindingType = "raw_knowledge"
	FindingRuntime      FindingType = "runtime"
	FindingExperience   FindingType = "experience"
)

// EvidenceRef is one strand of why a finding fired. A DecisionTrace can carry
// multiple — graph match, raw-yaml fallback, alias hit, runtime observation —
// and the verdict reader can see exactly how the match landed instead of
// having to re-derive it from prose. The Source classification is critical:
// agents must distinguish "graph proved this" from "alias guessed this".
type EvidenceRef struct {
	Source      string  `json:"source"` // graph | raw_yaml | runtime | alias | experience
	NodeID      string  `json:"node_id,omitempty"`
	EdgeKind    string  `json:"edge_kind,omitempty"`
	PathSummary string  `json:"path_summary,omitempty"`
	Confidence  float64 `json:"confidence"`
	Freshness   string  `json:"freshness,omitempty"` // fresh | stale | unknown | absent
	Reason      string  `json:"reason,omitempty"`
}

// OwnerContext names which layer owns a finding. Phase 1 leaves most fields
// empty; later phases will infer Layer/Service/Package from graph edges and
// task hints. The type is stable from day one so MCP consumers don't have
// to chase shape changes when Phase 3 lands.
type OwnerContext struct {
	Layer    string   `json:"layer,omitempty"` // repository | desired | installed | runtime | workflow | pki | dns | rbac | unknown
	Service  string   `json:"service,omitempty"`
	Package  string   `json:"package,omitempty"`
	Files    []string `json:"files,omitempty"`
	Symbols  []string `json:"symbols,omitempty"`
	StateIDs []string `json:"state_ids,omitempty"`
}

// ContextPivot is one ranked next-hop a reader can navigate to from a
// finding. Kind discriminates the destination (source_invariant, required_test,
// forbidden_fix, runtime_evidence, etc.) so agents render or follow pivots
// without parsing the ID.
type ContextPivot struct {
	Kind        string  `json:"kind"`
	ID          string  `json:"id"`
	Title       string  `json:"title,omitempty"`
	WhyRelevant string  `json:"why_relevant,omitempty"`
	Command     string  `json:"command,omitempty"`
	Confidence  float64 `json:"confidence,omitempty"`
}

// DiagnosticAction is a safe-to-run remediation command suggested for a
// finding. SafeToRun=true means read-only / test / build only. Anything
// cluster-mutating MUST set RequiresAck=true and explain why in Reason.
type DiagnosticAction struct {
	Kind        string `json:"kind"` // inspect | test | rebuild | runtime_collect | grep | runbook | stop
	Command     string `json:"command,omitempty"`
	Reason      string `json:"reason"`
	SafeToRun   bool   `json:"safe_to_run"`
	RequiresAck bool   `json:"requires_ack,omitempty"`
}

// Falsifier records "what evidence would prove this diagnosis wrong". Forces
// the agent to think in falsifiable claims, not just match output. Phase 1
// emits a generic fallback; later phases ship per-failure_mode templates.
type Falsifier struct {
	Claim      string `json:"claim"`
	HowToCheck string `json:"how_to_check"`
	Command    string `json:"command,omitempty"`
}

// DecisionTrace is the per-finding "why did this fire, and what now?" record.
// One trace per matched invariant / failure_mode / forbidden_fix / raw_yaml
// fallback / runtime match. Phase 1 populates MatchedBy + Pivots + a generic
// Falsifier from data preflight already collects; Owner, NextActions, and
// per-failure_mode Falsifiers are reserved for later phases.
//
// IMPORTANT: when no findings match, DecisionTraces stays empty (length 0),
// NOT nil — the trust envelope is the single source of safety verdicts under
// NO_MATCH, and a fabricated trace would compete with it.
type DecisionTrace struct {
	FindingID       string             `json:"finding_id"`
	FindingType     FindingType        `json:"finding_type"`
	Summary         string             `json:"summary,omitempty"`
	Confidence      Confidence         `json:"confidence"`
	ConfidenceScore float64            `json:"confidence_score,omitempty"`
	MatchedBy       []EvidenceRef      `json:"matched_by"`
	Owner           OwnerContext       `json:"owner"`
	Pivots          []ContextPivot     `json:"pivots"`
	NextActions     []DiagnosticAction `json:"next_actions"`
	Falsifiers      []Falsifier        `json:"falsifiers"`
	Warnings        []string           `json:"warnings,omitempty"`
}

// ExperienceHint is a compact similar-experience suggestion shown during preflight.
type ExperienceHint struct {
	ExperienceID  string   `json:"experience_id"`
	Score         float64  `json:"score"`
	Strategy      string   `json:"strategy"`
	Hint          string   `json:"hint"`
	Status        string   `json:"status"`
	Summary       string   `json:"summary"`
	Verdict       string   `json:"verdict,omitempty"`
	FinalScore    float64  `json:"final_score,omitempty"`
	Reasons       []string `json:"reasons,omitempty"`
	WorkedPaths   []string `json:"worked_paths,omitempty"`
	FailedPaths   []string `json:"failed_paths,omitempty"`
	EvidenceTypes []string `json:"evidence_types,omitempty"`
}

// Report is the complete output of a preflight run.
type Report struct {
	Task                string                   `json:"task"`
	CollectorHealth     []CollectorHealthSummary `json:"collector_health,omitempty"`
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
	GraphAvailable            bool                     `json:"graph_available"`
	GraphDBPath               string                   `json:"graph_db_path,omitempty"`
	GraphMatchCount           int                      `json:"graph_match_count"`
	GraphFilteredByTrustCount int                      `json:"graph_filtered_by_trust_count"`
	RawYAMLMatchCount         int                      `json:"raw_yaml_match_count"`
	FilteredMatches           []FilteredMatch          `json:"filtered_matches,omitempty"`
	ConfidenceFactors         ConfidenceFactors        `json:"confidence_factors"`
	SafetyStatus              SafetyStatus             `json:"safety_status"`
	DegradedMode              DegradedModePlaybook     `json:"degraded_mode"`
	RiskTier                  RiskTier                 `json:"risk_tier"`
	FastPathApplied           bool                     `json:"fast_path_applied"`
	GoFileCoverage            *GoFileCoverageReport    `json:"go_file_coverage,omitempty"`
	LiveOverlay               *LiveOverlayFreshness    `json:"live_overlay,omitempty"`
	ExperienceHints           []ExperienceHint         `json:"experience_hints,omitempty"`
	Trust                     *assurance.TrustEnvelope `json:"trust,omitempty"`

	// DecisionTraces is one record per matched finding describing why it
	// fired, ranked pivots into related awareness state, and (eventually)
	// safe next actions and falsifiers. Empty (not nil) when no findings
	// match — the trust envelope is the authority on NO_MATCH safety, and a
	// fabricated trace would only compete with it. See
	// claude_codex_awareness_context_navigation_improvement.md for the full
	// (multi-phase) design; Phase 1 emits MatchedBy + Pivots + generic
	// Falsifier only.
	DecisionTraces []DecisionTrace `json:"decision_traces"`
}
