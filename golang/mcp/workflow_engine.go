package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ── Workflow Session ─────────────────────────────────────────────────────────

// WorkflowSession tracks the execution state of a multi-step workflow.
type WorkflowSession struct {
	ID          string               `json:"id"`
	Task        string               `json:"task"`
	StartedAt   string               `json:"started_at"`
	Status      WorkflowSessionStatus `json:"status"`
	CurrentStep int                  `json:"current_step"`
	TotalSteps  int                  `json:"total_steps"`
	Steps       []WorkflowStepStatus `json:"steps"`
	Context     map[string]string    `json:"context,omitempty"`
}

// WorkflowSessionStatus describes the overall workflow state.
type WorkflowSessionStatus string

const (
	WorkflowPending    WorkflowSessionStatus = "pending"
	WorkflowInProgress WorkflowSessionStatus = "in_progress"
	WorkflowCompleted  WorkflowSessionStatus = "completed"
	WorkflowFailed     WorkflowSessionStatus = "failed"
	WorkflowBlocked    WorkflowSessionStatus = "blocked"
	WorkflowAborted    WorkflowSessionStatus = "aborted"
)

// WorkflowStepStatus tracks the status of a single step.
type WorkflowStepStatus struct {
	StepName    string    `json:"step_name"`
	Order       int       `json:"order"`
	Status      StepStatus `json:"status"`
	StartedAt   string    `json:"started_at,omitempty"`
	CompletedAt string    `json:"completed_at,omitempty"`
	Result      string    `json:"result,omitempty"`
	Error       string    `json:"error,omitempty"`
	Skipped     bool      `json:"skipped,omitempty"`
}

// StepStatus describes the state of a single step.
type StepStatus string

const (
	StepPending    StepStatus = "pending"
	StepInProgress StepStatus = "in_progress"
	StepCompleted  StepStatus = "completed"
	StepFailed     StepStatus = "failed"
	StepSkipped    StepStatus = "skipped"
	StepBlocked    StepStatus = "blocked"
)

// ── Approval ─────────────────────────────────────────────────────────────────

// ApprovalRequirement describes an approval gate.
type ApprovalRequirement struct {
	Action                   string `json:"action"`
	Reason                   string `json:"reason"`
	Scope                    string `json:"scope"`
	RequiresUserConfirmation bool   `json:"requires_user_confirmation"`
}

// ApprovalPolicy maps command patterns to approval requirements.
type ApprovalPolicy struct {
	CommandPattern string `json:"command_pattern"`
	Scope          string `json:"scope"`      // "production", "destructive", "publish"
	Reason         string `json:"reason"`
	AlwaysRequire  bool   `json:"always_require"`
}

// approvalPolicies defines all approval gates.
var approvalPolicies = []ApprovalPolicy{
	{CommandPattern: "services repair", Scope: "destructive", Reason: "Modifies installed services to match desired state — may restart or remove services", AlwaysRequire: true},
	{CommandPattern: "cluster bootstrap", Scope: "destructive", Reason: "Initializes cluster state — cannot be undone, creates PKI and etcd", AlwaysRequire: true},
	{CommandPattern: "pkg publish", Scope: "publish", Reason: "Publishes package to repository — visible to all cluster nodes", AlwaysRequire: true},
	{CommandPattern: "services desired set", Scope: "production", Reason: "Sets desired release state — triggers reconciliation on next repair", AlwaysRequire: true},
	{CommandPattern: "dns a set", Scope: "production", Reason: "Modifies DNS records — affects service discovery cluster-wide", AlwaysRequire: false},
}

// CheckApproval evaluates whether a command requires approval.
func CheckApproval(cmdPath string) *ApprovalRequirement {
	for _, p := range approvalPolicies {
		if p.CommandPattern == cmdPath || strings.HasPrefix(cmdPath, p.CommandPattern) {
			return &ApprovalRequirement{
				Action:                   cmdPath,
				Reason:                   p.Reason,
				Scope:                    p.Scope,
				RequiresUserConfirmation: p.AlwaysRequire,
			}
		}
	}
	return nil
}

// ── Preconditions ────────────────────────────────────────────────────────────

// Precondition is a state check that must pass before a command can execute.
type Precondition struct {
	Check       string `json:"check"`
	Description string `json:"description"`
}

// PreconditionResult is the outcome of evaluating a precondition.
type PreconditionResult struct {
	Check   string `json:"check"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

// commandPreconditions maps command paths to required preconditions.
var commandPreconditions = map[string][]Precondition{
	"pkg publish": {
		{Check: "build_compiles", Description: "Code must compile before publishing"},
		{Check: "tests_pass", Description: "Tests must pass before publishing"},
	},
	"services desired set": {
		{Check: "build_compiles", Description: "Code must compile before setting desired release"},
	},
	"generate service": {
		{Check: "proto_exists", Description: "Proto file must exist"},
	},
	"generate all": {
		{Check: "proto_exists", Description: "Proto file must exist"},
	},
}

// EvaluatePreconditions checks all preconditions for a command.
func EvaluatePreconditions(cmdPath string, args []string, state *StateSnapshot) []PreconditionResult {
	preconds, ok := commandPreconditions[cmdPath]
	if !ok {
		return nil
	}

	var results []PreconditionResult
	for _, pc := range preconds {
		result := PreconditionResult{Check: pc.Check}
		switch pc.Check {
		case "build_compiles":
			if state != nil {
				result.Passed = state.BuildState.Compiles
				if !result.Passed {
					result.Message = fmt.Sprintf("Build failed: %s", state.BuildState.Error)
				}
			} else {
				result.Passed = false
				result.Message = "State not available — run globular_cli.state first"
			}
		case "tests_pass":
			if state != nil {
				result.Passed = state.BuildState.TestsPassed
				if !result.Passed {
					result.Message = fmt.Sprintf("Tests failed: %s", state.BuildState.Error)
				}
			} else {
				result.Passed = false
				result.Message = "State not available — run globular_cli.state first"
			}
		case "proto_exists":
			result.Passed = false
			for _, a := range args {
				if strings.HasSuffix(a, ".proto") {
					result.Passed = true
					break
				}
			}
			// Also check after --proto flag
			for i, a := range args {
				if a == "--proto" && i+1 < len(args) {
					result.Passed = true
					break
				}
			}
			if !result.Passed {
				result.Message = "No proto file specified — add --proto <file>"
			}
		default:
			result.Passed = true // Unknown checks pass by default
		}
		results = append(results, result)
	}
	return results
}

// ── Workflow Store ────────────────────────────────────────────────────────────

// workflowStore is an in-memory store for active workflow sessions.
type workflowStore struct {
	mu       sync.RWMutex
	sessions map[string]*WorkflowSession
	counter  int
}

var activeWorkflows = &workflowStore{
	sessions: make(map[string]*WorkflowSession),
}

// StartWorkflow creates a new workflow session from a known workflow definition.
func (ws *workflowStore) StartWorkflow(task string) (*WorkflowSession, error) {
	wf, ok := lookupWorkflow(task)
	if !ok {
		available := make([]string, 0, len(cliWorkflows))
		for k := range cliWorkflows {
			available = append(available, k)
		}
		return nil, fmt.Errorf("unknown workflow %q — available: %s", task, strings.Join(available, ", "))
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.counter++
	id := fmt.Sprintf("wf-%s-%d", task, ws.counter)

	session := &WorkflowSession{
		ID:          id,
		Task:        task,
		StartedAt:   time.Now().UTC().Format(time.RFC3339),
		Status:      WorkflowInProgress,
		CurrentStep: 1,
		TotalSteps:  len(wf.Steps),
		Context:     make(map[string]string),
	}

	for _, step := range wf.Steps {
		session.Steps = append(session.Steps, WorkflowStepStatus{
			StepName: step.Description,
			Order:    step.Order,
			Status:   StepPending,
		})
	}
	// Mark first step as in_progress
	if len(session.Steps) > 0 {
		session.Steps[0].Status = StepInProgress
		session.Steps[0].StartedAt = time.Now().UTC().Format(time.RFC3339)
	}

	ws.sessions[id] = session
	return session, nil
}

// StartCustomWorkflow creates a workflow session from arbitrary steps (e.g. from a skill).
// This allows skills to reuse the workflow engine's tracking, approval, and branching.
func (ws *workflowStore) StartCustomWorkflow(name string, steps []WorkflowStepStatus, ctx map[string]string) (*WorkflowSession, error) {
	if len(steps) == 0 {
		return nil, fmt.Errorf("at least one step is required")
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.counter++
	id := fmt.Sprintf("wf-skill-%s-%d", name, ws.counter)

	session := &WorkflowSession{
		ID:          id,
		Task:        name,
		StartedAt:   time.Now().UTC().Format(time.RFC3339),
		Status:      WorkflowInProgress,
		CurrentStep: 1,
		TotalSteps:  len(steps),
		Steps:       steps,
		Context:     ctx,
	}

	// Mark first step as in_progress.
	session.Steps[0].Status = StepInProgress
	session.Steps[0].StartedAt = time.Now().UTC().Format(time.RFC3339)

	ws.sessions[id] = session
	return session, nil
}

// GetSession returns a workflow session by ID.
func (ws *workflowStore) GetSession(id string) (*WorkflowSession, bool) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	s, ok := ws.sessions[id]
	return s, ok
}

// AdvanceStep marks the current step as completed and moves to the next.
func (ws *workflowStore) AdvanceStep(id string, result string) (*WorkflowSession, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	session, ok := ws.sessions[id]
	if !ok {
		return nil, fmt.Errorf("workflow session %q not found", id)
	}
	if session.Status != WorkflowInProgress {
		return nil, fmt.Errorf("workflow %q is %s, cannot advance", id, session.Status)
	}

	idx := session.CurrentStep - 1
	if idx < 0 || idx >= len(session.Steps) {
		return nil, fmt.Errorf("invalid step index %d", idx)
	}

	// Complete current step
	session.Steps[idx].Status = StepCompleted
	session.Steps[idx].CompletedAt = time.Now().UTC().Format(time.RFC3339)
	session.Steps[idx].Result = result

	// Advance to next step
	session.CurrentStep++
	if session.CurrentStep > session.TotalSteps {
		session.Status = WorkflowCompleted
	} else {
		nextIdx := session.CurrentStep - 1
		session.Steps[nextIdx].Status = StepInProgress
		session.Steps[nextIdx].StartedAt = time.Now().UTC().Format(time.RFC3339)
	}

	return session, nil
}

// FailStep marks the current step as failed and blocks the workflow.
func (ws *workflowStore) FailStep(id string, errMsg string) (*WorkflowSession, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	session, ok := ws.sessions[id]
	if !ok {
		return nil, fmt.Errorf("workflow session %q not found", id)
	}

	idx := session.CurrentStep - 1
	if idx >= 0 && idx < len(session.Steps) {
		session.Steps[idx].Status = StepFailed
		session.Steps[idx].CompletedAt = time.Now().UTC().Format(time.RFC3339)
		session.Steps[idx].Error = errMsg
	}
	session.Status = WorkflowFailed

	return session, nil
}

// AbortWorkflow marks the workflow as aborted.
func (ws *workflowStore) AbortWorkflow(id string) (*WorkflowSession, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	session, ok := ws.sessions[id]
	if !ok {
		return nil, fmt.Errorf("workflow session %q not found", id)
	}
	session.Status = WorkflowAborted
	return session, nil
}

// ListSessions returns all active workflow sessions.
func (ws *workflowStore) ListSessions() []*WorkflowSession {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	out := make([]*WorkflowSession, 0, len(ws.sessions))
	for _, s := range ws.sessions {
		out = append(out, s)
	}
	return out
}

// ── Workflow-Aware Validation ────────────────────────────────────────────────

// ValidateWorkflowStep checks if a command is valid for the current workflow step.
func ValidateWorkflowStep(sessionID string, cmdPath string) ValidationResult {
	session, ok := activeWorkflows.GetSession(sessionID)
	if !ok {
		return ValidationResult{
			Status: StatusInvalid,
			Reason: fmt.Sprintf("workflow session %q not found", sessionID),
		}
	}

	if session.Status != WorkflowInProgress {
		return ValidationResult{
			Status: StatusBlocked,
			Reason: fmt.Sprintf("workflow is %s — cannot execute commands", session.Status),
		}
	}

	// Get the current workflow definition
	wf, ok := lookupWorkflow(session.Task)
	if !ok {
		return ValidationResult{
			Status: StatusInvalid,
			Reason: "workflow definition not found",
		}
	}

	idx := session.CurrentStep - 1
	if idx < 0 || idx >= len(wf.Steps) {
		return ValidationResult{
			Status: StatusInvalid,
			Reason: "workflow step out of bounds",
		}
	}

	step := wf.Steps[idx]

	// Check if the command matches what this step expects
	if step.Command != "" {
		// Extract command name from the step's command template
		stepCmd := extractCommandPath(step.Command, nil)
		if stepCmd != "" && stepCmd != cmdPath {
			return ValidationResult{
				Status:            StatusOutOfOrder,
				Reason:            fmt.Sprintf("current workflow step %d expects %q, got %q", step.Order, step.Command, cmdPath),
				SuggestedNextStep: fmt.Sprintf("Execute: %s", step.Command),
			}
		}
	}

	return ValidationResult{
		Status:         StatusAllowed,
		MatchedCommand: cmdPath,
	}
}

// ── Branching Logic ──────────────────────────────────────────────────────────

// BranchDecision describes a workflow branching outcome.
type BranchDecision struct {
	Action      string `json:"action"`       // "continue", "retry", "rollback", "stop", "remediate", "branch", "request_approval"
	Reason      string `json:"reason"`
	NextCommand string `json:"next_command,omitempty"`
	WorkflowID  string `json:"workflow_id,omitempty"`
}

// DecideBranch evaluates an execution result and determines the next action.
func DecideBranch(result ExecutionResult, sessionID string) BranchDecision {
	if result.Success {
		return BranchDecision{
			Action: "continue",
			Reason: "command succeeded",
		}
	}

	// Analyze failure type
	stderr := strings.ToLower(result.Stderr)
	errors := strings.ToLower(strings.Join(result.Errors, " "))
	combined := stderr + " " + errors

	switch {
	case strings.Contains(combined, "compile") || strings.Contains(combined, "build"):
		return BranchDecision{
			Action:      "remediate",
			Reason:      "build failure — fix compilation errors before proceeding",
			NextCommand: "go build ./...",
		}
	case strings.Contains(combined, "fail") && strings.Contains(combined, "test"):
		return BranchDecision{
			Action:      "remediate",
			Reason:      "test failure — fix failing tests",
			NextCommand: "go test ./... -v",
		}
	case strings.Contains(combined, "permission") || strings.Contains(combined, "denied"):
		return BranchDecision{
			Action: "stop",
			Reason: "permission denied — check RBAC configuration or authentication token",
		}
	case strings.Contains(combined, "not found") || strings.Contains(combined, "no such"):
		return BranchDecision{
			Action: "stop",
			Reason: "resource not found — verify the target exists before retrying",
		}
	case strings.Contains(combined, "timeout") || strings.Contains(combined, "deadline"):
		return BranchDecision{
			Action:      "retry",
			Reason:      "timeout — service may be temporarily unavailable",
			NextCommand: result.Command,
		}
	case strings.Contains(combined, "connection refused") || strings.Contains(combined, "unavailable"):
		return BranchDecision{
			Action: "stop",
			Reason: "service unavailable — verify the target service is running",
		}
	default:
		return BranchDecision{
			Action: "stop",
			Reason: fmt.Sprintf("unrecognized failure (exit code %d) — investigate manually", result.ExitCode),
		}
	}
}
