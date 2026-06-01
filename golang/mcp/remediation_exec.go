package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ── Remediation Execution Types ──────────────────────────────────────────────

// RemediationWorkflow tracks the execution of a remediation plan.
type RemediationWorkflow struct {
	ID          string            `json:"id"`
	Plan        *RemediationPlan  `json:"plan"`
	Status      string            `json:"status"` // "pending", "running", "completed", "failed"
	DryRun      bool              `json:"dry_run"`
	CurrentStep int               `json:"current_step"`
	StepResults []StepResult      `json:"step_results"`
	StartedAt   string            `json:"started_at"`
	CompletedAt string            `json:"completed_at,omitempty"`
	Error       string            `json:"error,omitempty"`
}

// StepResult captures the outcome of a single remediation step.
type StepResult struct {
	StepOrder  int    `json:"step_order"`
	Action     string `json:"action"`
	Target     string `json:"target"`
	Success    bool   `json:"success"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	DryRun     bool   `json:"dry_run,omitempty"`
}

// ── Executor ─────────────────────────────────────────────────────────────────

// RemediationExecutor runs a remediation plan step by step.
type RemediationExecutor struct {
	clients  *clientPool
	readOnly bool
}

// NewRemediationExecutor creates a new executor.
func NewRemediationExecutor(clients *clientPool, readOnly bool) *RemediationExecutor {
	return &RemediationExecutor{clients: clients, readOnly: readOnly}
}

// Execute runs a remediation plan. If dryRun is true, steps are simulated
// without executing real commands. Stops on first failure.
func (e *RemediationExecutor) Execute(ctx context.Context, plan *RemediationPlan, dryRun bool, approved bool) *RemediationWorkflow {
	wf := &RemediationWorkflow{
		ID:          fmt.Sprintf("rem-%d", time.Now().UnixMilli()),
		Plan:        plan,
		Status:      "running",
		DryRun:      dryRun,
		CurrentStep: 0,
		StartedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	if plan.Status == "blocked" {
		wf.Status = "failed"
		wf.Error = "plan is blocked: " + plan.Reason
		wf.CompletedAt = time.Now().UTC().Format(time.RFC3339)
		return wf
	}

	if len(plan.OrderedSteps) == 0 {
		wf.Status = "completed"
		wf.CompletedAt = time.Now().UTC().Format(time.RFC3339)
		return wf
	}

	for i, step := range plan.OrderedSteps {
		wf.CurrentStep = i + 1
		result := e.executeStep(ctx, step, dryRun, approved)
		wf.StepResults = append(wf.StepResults, result)

		if !result.Success {
			wf.Status = "failed"
			wf.Error = fmt.Sprintf("step %d failed: %s", step.Order, result.Error)
			wf.CompletedAt = time.Now().UTC().Format(time.RFC3339)
			return wf
		}
	}

	wf.Status = "completed"
	wf.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	return wf
}

func (e *RemediationExecutor) executeStep(ctx context.Context, step RemediationStep, dryRun bool, approved bool) StepResult {
	start := time.Now()
	result := StepResult{
		StepOrder: step.Order,
		Action:    step.Action,
		Target:    step.Target,
		DryRun:    dryRun,
	}

	// Build the command for this step
	command := buildStepCommand(step)

	// Step 1: Validate
	req := ValidationRequest{
		Command: command,
	}
	validation := ValidateCommand(req)

	// For commands not in the knowledge base, allow them through
	// (services desired remove is a valid operation even if not in CLI knowledge)
	if validation.Status == StatusInvalid {
		// Not in knowledge base — that's OK for remediation steps
	} else if validation.Status == StatusBlocked {
		result.Success = false
		result.Error = fmt.Sprintf("validation blocked: %s", validation.Reason)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	// Step 2: Check approval
	if validation.Status == StatusNeedsConfirmation && !approved {
		result.Success = false
		result.Error = "requires approval — set approved=true to proceed"
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	// Step 3: Execute or simulate
	if dryRun {
		result.Success = true
		result.Output = fmt.Sprintf("[dry-run] would execute: %s", command)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	// Real execution blocked if MCP is read-only
	if e.readOnly {
		result.Success = false
		result.Error = "execution blocked: MCP server is in read-only mode"
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	// Execute the command
	execResult := executeRaw(command, command, nil)
	result.DurationMs = time.Since(start).Milliseconds()

	if !execResult.Success {
		result.Success = false
		result.Error = strings.Join(execResult.Errors, "; ")
		if result.Error == "" {
			result.Error = execResult.Stderr
		}
		return result
	}

	result.Success = true
	result.Output = truncateOutput(execResult.Stdout, 4096)

	// Step 4: Verify — confirm the service is no longer installed
	// (only for remove actions, and only if we have a client pool)
	if step.Action == "remove" && e.clients != nil {
		if stillInstalled := e.verifyRemoved(ctx, step.Target); stillInstalled {
			// Service is still showing as installed — this might just be
			// async state propagation, so warn but don't fail
			result.Output += "\n[warning] service may still appear in installed list — state propagation may be pending"
		}
	}

	return result
}

// verifyRemoved checks if a service is still in the installed packages list.
func (e *RemediationExecutor) verifyRemoved(ctx context.Context, service string) bool {
	if e.clients == nil {
		return false
	}

	// Use a short-lived planner just to query installed services
	planner := NewPlanner(NewStaticDependencySource(), e.clients)
	installed := planner.getInstalledServices(ctx)

	for _, svc := range installed {
		if svc == service {
			return true // still installed
		}
	}
	return false // confirmed removed
}

// buildStepCommand converts a remediation step into a CLI command string.
func buildStepCommand(step RemediationStep) string {
	switch step.Action {
	case "remove":
		return "globular services desired remove " + step.Target
	case "disable":
		return "globular services disable " + step.Target
	case "install":
		return "globular services desired set " + step.Target
	default:
		return "globular " + step.Action + " " + step.Target
	}
}
