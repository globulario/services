package v1alpha1

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

const (
	APIVersion = "workflow.globular.io/v1alpha1"
	Kind       = "WorkflowDefinition"
)

// WorkflowDefinition is the authoring-time schema loaded from YAML/JSON.
// It is intentionally more flexible than the runtime model because authoring
// files may use expressions such as $.max_parallel_nodes or $.execute_timeout.
type WorkflowDefinition struct {
	APIVersion string                 `json:"apiVersion" yaml:"apiVersion"`
	Kind       string                 `json:"kind" yaml:"kind"`
	Metadata   WorkflowMetadata       `json:"metadata" yaml:"metadata"`
	Spec       WorkflowDefinitionSpec `json:"spec" yaml:"spec"`
}

type WorkflowMetadata struct {
	Name        string            `json:"name" yaml:"name"`
	DisplayName string            `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

type WorkflowDefinitionSpec struct {
	InputSchema map[string]any     `json:"inputSchema,omitempty" yaml:"inputSchema,omitempty"`
	Defaults    map[string]any     `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	Strategy    ExecutionStrategy  `json:"strategy" yaml:"strategy"`
	Steps       []WorkflowStepSpec `json:"steps" yaml:"steps"`
	OnFailure   *WorkflowHook      `json:"onFailure,omitempty" yaml:"onFailure,omitempty"`
	OnSuccess   *WorkflowHook      `json:"onSuccess,omitempty" yaml:"onSuccess,omitempty"`
}

type ExecutionStrategy struct {
	Mode        StrategyMode  `json:"mode" yaml:"mode"`
	Collection  *ScalarString `json:"collection,omitempty" yaml:"collection,omitempty"`
	Concurrency *ScalarInt    `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
	ItemName    *ScalarString `json:"itemName,omitempty" yaml:"itemName,omitempty"`
}

type StrategyMode string

const (
	StrategySingle  StrategyMode = "single"
	StrategyForeach StrategyMode = "foreach"
	StrategyDAG     StrategyMode = "dag"
)

type WorkflowStepSpec struct {
	ID        string         `json:"id" yaml:"id"`
	Title     string         `json:"title,omitempty" yaml:"title,omitempty"`
	Actor     ActorType      `json:"actor" yaml:"actor"`
	Action    string         `json:"action" yaml:"action"`
	DependsOn []string       `json:"dependsOn,omitempty" yaml:"dependsOn,omitempty"`
	When      *StepCondition `json:"when,omitempty" yaml:"when,omitempty"`
	Foreach   *ScalarString    `json:"foreach,omitempty" yaml:"foreach,omitempty"`
	Steps     []WorkflowStepSpec `json:"steps,omitempty" yaml:"steps,omitempty"` // nested sub-DAG for foreach groups
	With      map[string]any `json:"with,omitempty" yaml:"with,omitempty"`
	Retry     *RetryPolicy   `json:"retry,omitempty" yaml:"retry,omitempty"`
	Timeout   *ScalarString  `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	WaitFor    *WaitPolicy    `json:"waitFor,omitempty" yaml:"waitFor,omitempty"`
	Export     *ScalarString  `json:"export,omitempty" yaml:"export,omitempty"`
	OnFailure  *WorkflowHook  `json:"onFailure,omitempty" yaml:"onFailure,omitempty"` // per-item failure hook for foreach groups
	Strategy   *ExecutionStrategy `json:"strategy,omitempty" yaml:"strategy,omitempty"` // per-step strategy override (foreach groups)
	ItemName   *ScalarString  `json:"itemName,omitempty" yaml:"itemName,omitempty"` // variable name for foreach current item

	// Workflow hardening fields (WH-1). Optional — steps without these
	// fields behave exactly as before (backward compatible).
	Execution    *StepExecution    `json:"execution,omitempty" yaml:"execution,omitempty"`
	Verification *StepVerification `json:"verification,omitempty" yaml:"verification,omitempty"`
	Compensation *StepCompensation `json:"compensation,omitempty" yaml:"compensation,omitempty"`
}

// ── Workflow hardening types (WH-1) ──────────────────────────────────────────
//
// These add engine-visible execution semantics to workflow steps:
//   - Idempotency classifies the step's side-effect profile
//   - Resume policy tells the engine what to do on crash recovery
//   - Verification proves the effect already exists (tri-state: present/absent/inconclusive)
//   - Compensation defines optional rollback actions
//
// See docs/architecture/workflow-hardening.md and workflow-hardening-implementation.md.

// StepExecution describes how a step should be handled during normal
// execution and on resume after executor crash.
type StepExecution struct {
	// Idempotency classifies the step's replay safety.
	//   safe_retry:           replay is safe without checks
	//   verify_then_continue: verify effect first, then decide
	//   manual_approval:      require approval on uncertain resume
	//   compensatable:        step can be rolled back via compensation
	Idempotency string `json:"idempotency,omitempty" yaml:"idempotency,omitempty"`

	// ResumePolicy tells the engine what to do when this step was in-progress
	// and the executor crashed.
	//   retry:               re-execute unconditionally
	//   verify_effect:       run verification first
	//   rerun_if_no_receipt: check receipt, verify if absent, then execute
	//   pause_for_approval:  block run and wait for operator
	//   fail:                fail the step conservatively
	ResumePolicy string `json:"resume_policy,omitempty" yaml:"resume_policy,omitempty"`

	// ReceiptKey is an optional durable breadcrumb key for ambiguous steps.
	// When set, the engine stores a completion receipt after the step succeeds.
	ReceiptKey string `json:"receipt_key,omitempty" yaml:"receipt_key,omitempty"`

	// ReceiptRequired means the step should not be considered safely completed
	// without a receipt present.
	ReceiptRequired bool `json:"receipt_required,omitempty" yaml:"receipt_required,omitempty"`
}

// StepVerification defines how to prove a step's intended effect already
// exists in the world. Used during resume to decide skip vs re-execute.
type StepVerification struct {
	Actor   ActorType      `json:"actor" yaml:"actor"`
	Action  string         `json:"action" yaml:"action"`
	With    map[string]any `json:"with,omitempty" yaml:"with,omitempty"`
	Success VerifySuccess  `json:"success" yaml:"success"`
}

// VerifySuccess defines the expression that proves the effect exists.
// Evaluated against the verification action's result output.
type VerifySuccess struct {
	Expr string `json:"expr" yaml:"expr"`
}

// StepCompensation defines an optional rollback action for recoverable steps.
type StepCompensation struct {
	Enabled bool           `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Actor   ActorType      `json:"actor,omitempty" yaml:"actor,omitempty"`
	Action  string         `json:"action,omitempty" yaml:"action,omitempty"`
	With    map[string]any `json:"with,omitempty" yaml:"with,omitempty"`
}

// Idempotency class constants.
const (
	IdempotencySafeRetry           = "safe_retry"
	IdempotencyVerifyThenContinue  = "verify_then_continue"
	IdempotencyManualApproval      = "manual_approval"
	IdempotencyCompensatable       = "compensatable"
)

// Resume policy constants.
const (
	ResumePolicyRetry            = "retry"
	ResumePolicyVerifyEffect     = "verify_effect"
	ResumePolicyRerunIfNoReceipt = "rerun_if_no_receipt"
	ResumePolicyPauseForApproval = "pause_for_approval"
	ResumePolicyFail             = "fail"
)

type StepCondition struct {
	Expr  string          `json:"expr,omitempty" yaml:"expr,omitempty"`
	AnyOf []StepCondition `json:"anyOf,omitempty" yaml:"anyOf,omitempty"`
	AllOf []StepCondition `json:"allOf,omitempty" yaml:"allOf,omitempty"`
	Not   *StepCondition  `json:"not,omitempty" yaml:"not,omitempty"`
}

type RetryPolicy struct {
	MaxAttempts int           `json:"maxAttempts" yaml:"maxAttempts"`
	Backoff     *ScalarString `json:"backoff,omitempty" yaml:"backoff,omitempty"`
}

type WaitPolicy struct {
	Condition string        `json:"condition" yaml:"condition"`
	Timeout   *ScalarString `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

type WorkflowHook struct {
	Actor  ActorType      `json:"actor" yaml:"actor"`
	Action string         `json:"action" yaml:"action"`
	With   map[string]any `json:"with,omitempty" yaml:"with,omitempty"`
}

type ActorType string

const (
	ActorWorkflowService   ActorType = "workflow-service"
	ActorClusterController ActorType = "cluster-controller"
	ActorClusterDoctor     ActorType = "cluster-doctor"
	ActorNodeAgent         ActorType = "node-agent"
	ActorInstaller         ActorType = "installer"
	ActorRepository        ActorType = "repository"
	ActorOperator          ActorType = "operator"
	ActorCompute           ActorType = "compute"
)

// ScalarString accepts either a literal string or an expression-like string.
type ScalarString struct {
	Raw string
}

func (s *ScalarString) UnmarshalJSON(data []byte) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case string:
		s.Raw = x
		return nil
	default:
		return fmt.Errorf("expected string scalar, got %T", x)
	}
}

func (s ScalarString) MarshalJSON() ([]byte, error) { return json.Marshal(s.Raw) }

func (s *ScalarString) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected scalar string node, got kind=%d", node.Kind)
	}
	s.Raw = node.Value
	return nil
}

func (s *ScalarString) String() string {
	if s == nil {
		return ""
	}
	return s.Raw
}
func (s *ScalarString) IsExpression() bool { return s != nil && len(s.Raw) > 2 && s.Raw[0:2] == "$." }

// ScalarInt accepts either a literal integer or an expression string.
type ScalarInt struct {
	Value *int
	Expr  string
}

func (s *ScalarInt) UnmarshalJSON(data []byte) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case float64:
		n := int(x)
		if float64(n) != x {
			return fmt.Errorf("expected integer scalar, got %v", x)
		}
		s.Value = &n
		s.Expr = ""
		return nil
	case string:
		s.Expr = x
		s.Value = nil
		return nil
	default:
		return fmt.Errorf("expected int or string scalar, got %T", x)
	}
}

func (s *ScalarInt) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected scalar int node, got kind=%d", node.Kind)
	}
	var n int
	if err := node.Decode(&n); err == nil {
		s.Value = &n
		s.Expr = ""
		return nil
	}
	var expr string
	if err := node.Decode(&expr); err == nil {
		s.Value = nil
		s.Expr = expr
		return nil
	}
	return fmt.Errorf("expected int or string scalar, got %q", node.Value)
}

func (s ScalarInt) MarshalJSON() ([]byte, error) {
	if s.Value != nil {
		return json.Marshal(*s.Value)
	}
	return json.Marshal(s.Expr)
}

func (s *ScalarInt) IntValue() (int, bool) {
	if s == nil || s.Value == nil {
		return 0, false
	}
	return *s.Value, true
}

func (s *ScalarInt) IsExpression() bool { return s != nil && s.Value == nil && s.Expr != "" }
func (s *ScalarInt) String() string {
	if s == nil {
		return ""
	}
	if s.Value != nil {
		return fmt.Sprintf("%d", *s.Value)
	}
	return s.Expr
}
