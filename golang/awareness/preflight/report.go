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

// Report is the complete output of a preflight run.
type Report struct {
	Task             string                   `json:"task"`
	Classification   []TaskClass              `json:"classification"`
	MatchedAliases   []string                 `json:"matched_aliases"`
	Services         []string                 `json:"services"`
	Packages         []string                 `json:"packages"`
	Files            []string                 `json:"files"`
	Invariants       []string                 `json:"invariants"`
	FailureModes     []string                 `json:"failure_modes"`
	ForbiddenFixes   []string                 `json:"forbidden_fixes"`
	CodeSmells       []string                 `json:"code_smells,omitempty"`
	HashSchemas      []string                 `json:"hash_schemas,omitempty"`
	StateTransitions []string                 `json:"state_transitions,omitempty"`
	DidWeFix         *DidWeFixSection         `json:"did_we_fix"`
	PackageAdmission *PackageAdmissionSection `json:"package_admission,omitempty"`
	Cycles           []CycleWarning           `json:"cycles"`
	RequiredTests    []string                 `json:"required_tests"`
	RequiredSearches []string                 `json:"required_searches"`
	RecommendedOrder []string                 `json:"recommended_investigation_order"`
	AgentInstruction string                   `json:"agent_instruction"`
	Warnings         []string                 `json:"warnings"`
	Runtime          *RuntimeSection          `json:"runtime,omitempty"`
}
