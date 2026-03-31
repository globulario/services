package adapter

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// StepExecutor runs on the node-agent side. It receives ExecuteStepRequest
// from the workflow-service, dispatches to the local action registry, and
// returns a ResultEvent.
type StepExecutor struct {
	Router *engine.Router
	NodeID string

	mu       sync.Mutex
	running  map[string]context.CancelFunc // key: runID/stepID/attempt
}

// NewStepExecutor creates an executor wired to the local action router.
func NewStepExecutor(router *engine.Router, nodeID string) *StepExecutor {
	return &StepExecutor{
		Router:  router,
		NodeID:  nodeID,
		running: make(map[string]context.CancelFunc),
	}
}

// Execute handles an incoming step request. It dispatches to the local
// action registry and returns the terminal result.
func (se *StepExecutor) Execute(ctx context.Context, req ExecuteStepRequest) *ResultEvent {
	started := time.Now()
	ident := req.Identity

	// Apply deadline if set.
	if !req.Deadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, req.Deadline)
		defer cancel()
	}

	// Track running step for cancellation.
	key := fmt.Sprintf("%s/%s/%d", ident.RunID, ident.StepID, ident.Attempt)
	execCtx, execCancel := context.WithCancel(ctx)
	se.mu.Lock()
	se.running[key] = execCancel
	se.mu.Unlock()
	defer func() {
		se.mu.Lock()
		delete(se.running, key)
		se.mu.Unlock()
		execCancel()
	}()

	log.Printf("executor: step %s action=%s::%s (run=%s attempt=%d)",
		ident.StepID, req.Actor, req.Action, ident.RunID, ident.Attempt)

	// Resolve handler from local router.
	handler, ok := se.Router.Resolve(v1alpha1.ActorType(req.Actor), req.Action)
	if !ok {
		return &ResultEvent{
			Identity:   ident,
			StartedAt:  started,
			FinishedAt: time.Now(),
			Status:     StatusFailed,
			Summary:    fmt.Sprintf("no handler for %s::%s", req.Actor, req.Action),
			Error: &StepError{
				ErrorClass: ErrInvalidInput,
				ErrorCode:  "no_handler",
				Message:    fmt.Sprintf("action %s::%s not registered", req.Actor, req.Action),
			},
		}
	}

	// Build engine request from adapter request.
	engineReq := engine.ActionRequest{
		RunID:   ident.RunID,
		StepID:  ident.StepID,
		Actor:   v1alpha1.ActorType(req.Actor),
		Action:  req.Action,
		With:    req.Inputs,
		Inputs:  req.Inputs,
		Outputs: make(map[string]any),
	}

	// Dispatch.
	result, err := handler(execCtx, engineReq)
	finished := time.Now()

	if err != nil {
		errClass := classifyError(err)
		return &ResultEvent{
			Identity:   ident,
			StartedAt:  started,
			FinishedAt: finished,
			Status:     StatusFailed,
			Summary:    err.Error(),
			Error: &StepError{
				ErrorClass:    errClass,
				ErrorCode:     "execution_error",
				Message:       err.Error(),
				RetryableHint: errClass == ErrTransient || errClass == ErrTimeout,
			},
		}
	}

	if result != nil && !result.OK {
		return &ResultEvent{
			Identity:   ident,
			StartedAt:  started,
			FinishedAt: finished,
			Status:     StatusFailed,
			Summary:    result.Message,
			Error: &StepError{
				ErrorClass:    ErrInternal,
				ErrorCode:     "not_ok",
				Message:       result.Message,
				RetryableHint: false,
			},
		}
	}

	var outputs map[string]any
	if result != nil {
		outputs = result.Output
	}

	return &ResultEvent{
		Identity:   ident,
		StartedAt:  started,
		FinishedAt: finished,
		Status:     StatusSucceeded,
		Summary:    resultMessage(result),
		Outputs:    outputs,
	}
}

// Cancel attempts to cancel a running step.
func (se *StepExecutor) Cancel(req CancelStepRequest) bool {
	key := fmt.Sprintf("%s/%s/%d", req.Identity.RunID, req.Identity.StepID, req.Identity.Attempt)
	se.mu.Lock()
	cancel, ok := se.running[key]
	se.mu.Unlock()
	if ok {
		log.Printf("executor: cancelling step %s (reason: %s)", req.Identity.StepID, req.Reason)
		cancel()
		return true
	}
	return false
}

// classifyError maps Go errors into stable error classes.
func classifyError(err error) StepErrorClass {
	if err == nil {
		return ErrInternal
	}
	msg := err.Error()
	switch {
	case ctx_timeout(err):
		return ErrTimeout
	case ctx_canceled(err):
		return ErrCancelled
	case contains(msg, "not found", "no such file"):
		return ErrNotFound
	case contains(msg, "permission denied", "access denied"):
		return ErrPermissionDenied
	case contains(msg, "download", "fetch"):
		return ErrDownloadFailed
	case contains(msg, "verify", "checksum", "digest"):
		return ErrVerifyFailed
	case contains(msg, "transient", "temporary"):
		return ErrTransient
	default:
		return ErrInternal
	}
}

func ctx_timeout(err error) bool {
	return err == context.DeadlineExceeded
}

func ctx_canceled(err error) bool {
	return err == context.Canceled
}

func contains(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

func resultMessage(r *engine.ActionResult) string {
	if r == nil {
		return "completed"
	}
	if r.Message != "" {
		return r.Message
	}
	return "completed"
}
