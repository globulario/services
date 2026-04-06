// Package compiler transforms v1alpha1 workflow definitions into
// runtime-ready compiled workflows with precomputed DAG indexes,
// resolved static values, and validated structure.
package compiler

import "time"

// CompiledWorkflow is the output of the compiler — a frozen, deterministic
// artifact ready for execution without re-parsing or re-validating.
type CompiledWorkflow struct {
	Name        string                   `json:"name"`
	SourceHash  string                   `json:"source_hash"`
	Steps       map[string]*CompiledStep `json:"steps"`
	TopoOrder   []string                 `json:"topo_order"`
	EntryPoints []string                 `json:"entry_points"`
	Dependents  map[string][]string      `json:"dependents"`
	Defaults    map[string]any           `json:"defaults,omitempty"`
	Metadata    CompiledMetadata         `json:"metadata,omitempty"`
	Strategy    CompiledStrategy         `json:"strategy"`
	OnFailure   *CompiledHook            `json:"on_failure,omitempty"`
	OnSuccess   *CompiledHook            `json:"on_success,omitempty"`
}

// CompiledMetadata holds display and labeling info.
type CompiledMetadata struct {
	DisplayName string            `json:"display_name,omitempty"`
	Description string            `json:"description,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// CompiledStrategy is the resolved execution strategy.
type CompiledStrategy struct {
	Mode        string     `json:"mode"`
	Collection  *ValueExpr `json:"collection,omitempty"`
	Concurrency *ValueExpr `json:"concurrency,omitempty"`
	ItemName    string     `json:"item_name,omitempty"`
}

// CompiledStep is a single step with all values resolved or classified.
type CompiledStep struct {
	ID         string               `json:"id"`
	Title      string               `json:"title,omitempty"`
	Actor      string               `json:"actor"`
	Action     string               `json:"action"`
	With       map[string]ValueExpr `json:"with,omitempty"`
	DependsOn  []string             `json:"depends_on,omitempty"`
	Dependents []string             `json:"dependents,omitempty"`
	Retry      CompiledRetry        `json:"retry"`
	Timeout    time.Duration        `json:"timeout,omitempty"`
	Foreach    *ValueExpr           `json:"foreach,omitempty"`
	SubSteps   *CompiledWorkflow    `json:"sub_steps,omitempty"` // nested DAG for foreach-with-steps
	ItemName   string               `json:"item_name,omitempty"` // variable name for foreach current item
	OnFailure  *CompiledHook        `json:"on_failure,omitempty"` // per-item failure hook
	Export     string               `json:"export,omitempty"`
	When       *CompiledCondition   `json:"when,omitempty"`

	// Workflow hardening (WH-1). Optional — nil means legacy behavior.
	Execution    *CompiledExecution    `json:"execution,omitempty"`
	Verification *CompiledVerification `json:"verification,omitempty"`
	Compensation *CompiledCompensation `json:"compensation,omitempty"`
}

// ── Workflow hardening compiled types ─────────────────────────────────────────

// CompiledExecution holds the resolved execution metadata for a step.
type CompiledExecution struct {
	Idempotency     string `json:"idempotency,omitempty"`
	ResumePolicy    string `json:"resume_policy,omitempty"`
	ReceiptKey      string `json:"receipt_key,omitempty"`
	ReceiptRequired bool   `json:"receipt_required,omitempty"`
}

// CompiledVerification holds the resolved verification action for a step.
type CompiledVerification struct {
	Actor       string               `json:"actor"`
	Action      string               `json:"action"`
	With        map[string]ValueExpr `json:"with,omitempty"`
	SuccessExpr string               `json:"success_expr"`
}

// CompiledCompensation holds the resolved compensation action for a step.
type CompiledCompensation struct {
	Enabled bool                 `json:"enabled"`
	Actor   string               `json:"actor,omitempty"`
	Action  string               `json:"action,omitempty"`
	With    map[string]ValueExpr `json:"with,omitempty"`
}

// CompiledRetry holds resolved retry policy.
type CompiledRetry struct {
	MaxAttempts int           `json:"max_attempts"`
	Backoff     time.Duration `json:"backoff,omitempty"`
}

// CompiledHook is a resolved onSuccess/onFailure hook.
type CompiledHook struct {
	Actor  string               `json:"actor"`
	Action string               `json:"action"`
	With   map[string]ValueExpr `json:"with,omitempty"`
}

// CompiledCondition preserves the when clause for runtime evaluation.
type CompiledCondition struct {
	Expr  string              `json:"expr,omitempty"`
	AnyOf []CompiledCondition `json:"any_of,omitempty"`
	AllOf []CompiledCondition `json:"all_of,omitempty"`
	Not   *CompiledCondition  `json:"not,omitempty"`
}

// ValueExpr classifies a value as either a static literal or a runtime expression.
type ValueExpr struct {
	Raw    string `json:"raw,omitempty"`
	IsExpr bool   `json:"is_expr,omitempty"`
	Static any    `json:"static,omitempty"`
}

// Severity for diagnostics.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Diagnostic reports a validation issue.
type Diagnostic struct {
	Severity Severity `json:"severity"`
	Path     string   `json:"path"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
}

// HasErrors returns true if any diagnostic is an error.
func HasErrors(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}
