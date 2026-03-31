// Package adapter bridges the workflow engine to remote node-agents.
//
// The adapter translates engine.ActionRequest into ExecuteStepRequest,
// dispatches it to a remote node-agent via a pluggable transport, and
// converts the ResultEvent back into engine.ActionResult.
//
// The node-agent side receives step requests via StepExecutor, dispatches
// to its local action registry, and reports results via callbacks.
//
// Design rule: workflow-service decides, node-agent executes,
// callbacks report facts — never policy.
package adapter

import "time"

// ExecutionIdentity correlates all messages for a single step attempt.
type ExecutionIdentity struct {
	RunID         string
	StepID        string
	Attempt       uint32
	WorkflowName  string
	NodeID        string
	CorrelationID string
}

// ExecuteStepRequest is sent from workflow-service to node-agent.
type ExecuteStepRequest struct {
	Identity          ExecutionIdentity
	Actor             string
	Action            string
	Inputs            map[string]any
	DispatchTime      time.Time
	Deadline          time.Time
	CancellationToken string
	Labels            map[string]string
}

// ExecuteStepResponse is the synchronous ack from node-agent.
type ExecuteStepResponse struct {
	Accepted         bool
	LocalOperationID string
	Message          string
}

// CancelStepRequest asks node-agent to stop a running step.
type CancelStepRequest struct {
	Identity    ExecutionIdentity
	Reason      string
	RequestedAt time.Time
}

// CancelStepResponse is the synchronous ack for cancellation.
type CancelStepResponse struct {
	Accepted bool
	Message  string
}

// StepTerminalStatus is the terminal outcome of a step attempt.
type StepTerminalStatus string

const (
	StatusSucceeded StepTerminalStatus = "SUCCEEDED"
	StatusFailed    StepTerminalStatus = "FAILED"
	StatusCancelled StepTerminalStatus = "CANCELLED"
)

// StepErrorClass classifies failures into stable categories.
type StepErrorClass string

const (
	ErrInvalidInput       StepErrorClass = "INVALID_INPUT"
	ErrPreconditionFailed StepErrorClass = "PRECONDITION_FAILED"
	ErrNotFound           StepErrorClass = "NOT_FOUND"
	ErrPermissionDenied   StepErrorClass = "PERMISSION_DENIED"
	ErrDownloadFailed     StepErrorClass = "DOWNLOAD_FAILED"
	ErrVerifyFailed       StepErrorClass = "VERIFY_FAILED"
	ErrTimeout            StepErrorClass = "TIMEOUT"
	ErrCancelled          StepErrorClass = "CANCELLED"
	ErrTransient          StepErrorClass = "TRANSIENT"
	ErrInternal           StepErrorClass = "INTERNAL"
)

// StepError is a structured error from node-agent execution.
type StepError struct {
	ErrorClass    StepErrorClass
	ErrorCode     string
	Message       string
	RetryableHint bool
	Details       map[string]any
}

// ResultEvent is the terminal callback from node-agent to workflow-service.
type ResultEvent struct {
	Identity         ExecutionIdentity
	Sequence         uint64
	LocalOperationID string
	StartedAt        time.Time
	FinishedAt       time.Time
	Status           StepTerminalStatus
	Summary          string
	Outputs          map[string]any
	Error            *StepError
}

// ProgressEvent is a non-terminal progress update from node-agent.
type ProgressEvent struct {
	Identity         ExecutionIdentity
	Sequence         uint64
	LocalOperationID string
	ObservedAt       time.Time
	Percent          uint32
	Phase            string
	Message          string
}
