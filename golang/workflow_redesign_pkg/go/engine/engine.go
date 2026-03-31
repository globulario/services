// Package engine executes workflow definitions.
//
// It evaluates the step DAG, dispatches actions to actors, handles retries
// and timeouts, and tracks run/step state. The engine is the runtime
// counterpart to the v1alpha1 authoring schema.
package engine

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/workflow_redesign_pkg/go/v1alpha1"
)

// --------------------------------------------------------------------------
// Actor dispatch
// --------------------------------------------------------------------------

// ActionRequest is sent to an actor when a step executes.
type ActionRequest struct {
	RunID   string
	StepID  string
	Actor   v1alpha1.ActorType
	Action  string
	With    map[string]any // step.With merged with resolved expressions
	Inputs  map[string]any // workflow-level inputs
	Outputs map[string]any // accumulated step outputs (exports)
}

// ActionResult is returned by an actor after executing an action.
type ActionResult struct {
	OK      bool
	Output  map[string]any
	Message string
}

// ActionHandler executes a single action. Actors register handlers
// for each action they support.
type ActionHandler func(ctx context.Context, req ActionRequest) (*ActionResult, error)

// Router maps (actor, action) pairs to handlers.
type Router struct {
	mu       sync.RWMutex
	handlers map[string]ActionHandler // key: "actor::action"
}

func NewRouter() *Router {
	return &Router{handlers: make(map[string]ActionHandler)}
}

func (r *Router) Register(actor v1alpha1.ActorType, action string, h ActionHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[string(actor)+"::"+action] = h
}

func (r *Router) Resolve(actor v1alpha1.ActorType, action string) (ActionHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[string(actor)+"::"+action]
	return h, ok
}

// --------------------------------------------------------------------------
// Condition evaluator
// --------------------------------------------------------------------------

// ConditionFunc evaluates a condition expression. Returns true if the step
// should run. Register custom evaluators for expressions like
// "contains(inputs.node_profiles, 'etcd')".
type ConditionFunc func(ctx context.Context, expr string, inputs, outputs map[string]any) (bool, error)

// --------------------------------------------------------------------------
// Step state
// --------------------------------------------------------------------------

type StepStatus string

const (
	StepPending   StepStatus = "PENDING"
	StepRunning   StepStatus = "RUNNING"
	StepSucceeded StepStatus = "SUCCEEDED"
	StepFailed    StepStatus = "FAILED"
	StepSkipped   StepStatus = "SKIPPED"
)

type StepState struct {
	ID         string
	Status     StepStatus
	Attempt    int
	Output     map[string]any
	Error      string
	StartedAt  time.Time
	FinishedAt time.Time
}

// --------------------------------------------------------------------------
// Run state
// --------------------------------------------------------------------------

type RunStatus string

const (
	RunPending   RunStatus = "PENDING"
	RunRunning   RunStatus = "RUNNING"
	RunSucceeded RunStatus = "SUCCEEDED"
	RunFailed    RunStatus = "FAILED"
)

type Run struct {
	ID         string
	Definition string
	Status     RunStatus
	Inputs     map[string]any
	Outputs    map[string]any // accumulated exports
	Steps      map[string]*StepState
	StartedAt  time.Time
	FinishedAt time.Time
	Error      string
}

// --------------------------------------------------------------------------
// Engine
// --------------------------------------------------------------------------

// Engine executes a workflow definition to completion.
type Engine struct {
	Router    *Router
	EvalCond  ConditionFunc
	OnStepDone func(run *Run, step *StepState) // optional callback for observability
}

// Execute runs a workflow definition with the given inputs. It blocks until
// the workflow completes (success or failure) or the context is canceled.
func (e *Engine) Execute(ctx context.Context, def *v1alpha1.WorkflowDefinition, inputs map[string]any) (*Run, error) {
	if def == nil {
		return nil, fmt.Errorf("definition is nil")
	}

	// Apply defaults.
	merged := make(map[string]any)
	for k, v := range def.Spec.Defaults {
		merged[k] = v
	}
	for k, v := range inputs {
		merged[k] = v
	}

	run := &Run{
		ID:         fmt.Sprintf("run-%d", time.Now().UnixMilli()),
		Definition: def.Metadata.Name,
		Status:     RunRunning,
		Inputs:     merged,
		Outputs:    make(map[string]any),
		Steps:      make(map[string]*StepState, len(def.Spec.Steps)),
		StartedAt:  time.Now(),
	}
	for _, s := range def.Spec.Steps {
		run.Steps[s.ID] = &StepState{ID: s.ID, Status: StepPending}
	}

	// Build dependency graph.
	stepsByID := make(map[string]*v1alpha1.WorkflowStepSpec, len(def.Spec.Steps))
	for i := range def.Spec.Steps {
		stepsByID[def.Spec.Steps[i].ID] = &def.Spec.Steps[i]
	}

	// Execute DAG.
	err := e.executeDAG(ctx, run, def.Spec.Steps, stepsByID)

	now := time.Now()
	run.FinishedAt = now
	if err != nil {
		run.Status = RunFailed
		run.Error = err.Error()
		// onFailure hook
		if def.Spec.OnFailure != nil {
			e.dispatchHook(ctx, run, def.Spec.OnFailure)
		}
	} else {
		run.Status = RunSucceeded
		// onSuccess hook
		if def.Spec.OnSuccess != nil {
			e.dispatchHook(ctx, run, def.Spec.OnSuccess)
		}
	}

	return run, err
}

// executeDAG runs all steps respecting their dependency order.
// Steps with satisfied dependencies execute in parallel.
func (e *Engine) executeDAG(ctx context.Context, run *Run, steps []v1alpha1.WorkflowStepSpec, byID map[string]*v1alpha1.WorkflowStepSpec) error {
	// Keep looping until all steps are terminal (succeeded/failed/skipped).
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Find ready steps: all dependencies are terminal.
		var ready []*v1alpha1.WorkflowStepSpec
		allDone := true
		for _, s := range steps {
			st := run.Steps[s.ID]
			if st.Status == StepPending {
				allDone = false
				if e.depsReady(run, s.DependsOn) {
					ready = append(ready, byID[s.ID])
				}
			} else if st.Status == StepRunning {
				allDone = false
			}
		}

		if allDone {
			break
		}
		if len(ready) == 0 {
			// No steps ready but not all done — wait a tick for running steps.
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Execute ready steps in parallel.
		var wg sync.WaitGroup
		var mu sync.Mutex
		var firstErr error

		for _, spec := range ready {
			spec := spec
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := e.executeStep(ctx, run, spec)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
				}
			}()
		}
		wg.Wait()

		if firstErr != nil {
			return firstErr
		}
	}

	// Check for any failed steps.
	for _, st := range run.Steps {
		if st.Status == StepFailed {
			return fmt.Errorf("step %s failed: %s", st.ID, st.Error)
		}
	}
	return nil
}

// depsReady returns true if all dependencies are in a terminal success state.
func (e *Engine) depsReady(run *Run, deps []string) bool {
	for _, dep := range deps {
		st, ok := run.Steps[dep]
		if !ok {
			return false
		}
		if st.Status != StepSucceeded && st.Status != StepSkipped {
			return false
		}
	}
	return true
}

// executeStep runs a single step with retry and timeout.
func (e *Engine) executeStep(ctx context.Context, run *Run, spec *v1alpha1.WorkflowStepSpec) error {
	st := run.Steps[spec.ID]

	// Evaluate when condition.
	if spec.When != nil {
		ok, err := e.evaluateCondition(ctx, spec.When, run.Inputs, run.Outputs)
		if err != nil {
			st.Status = StepFailed
			st.Error = fmt.Sprintf("condition eval: %v", err)
			return nil // condition failure doesn't fail the workflow
		}
		if !ok {
			st.Status = StepSkipped
			log.Printf("workflow: step %s skipped (condition not met)", spec.ID)
			if e.OnStepDone != nil {
				e.OnStepDone(run, st)
			}
			return nil
		}
	}

	// Resolve handler.
	handler, ok := e.Router.Resolve(spec.Actor, spec.Action)
	if !ok {
		st.Status = StepFailed
		st.Error = fmt.Sprintf("no handler for %s::%s", spec.Actor, spec.Action)
		e.notifyStep(run, st)
		return fmt.Errorf("step %s: no handler for %s::%s", spec.ID, spec.Actor, spec.Action)
	}

	// Timeout.
	stepCtx := ctx
	if spec.Timeout != nil && !spec.Timeout.IsExpression() {
		if d, err := time.ParseDuration(spec.Timeout.String()); err == nil {
			var cancel context.CancelFunc
			stepCtx, cancel = context.WithTimeout(ctx, d)
			defer cancel()
		}
	}

	// Retry loop.
	maxAttempts := 1
	backoff := 2 * time.Second
	if spec.Retry != nil {
		maxAttempts = spec.Retry.MaxAttempts
		if spec.Retry.Backoff != nil && !spec.Retry.Backoff.IsExpression() {
			if d, err := time.ParseDuration(spec.Retry.Backoff.String()); err == nil {
				backoff = d
			}
		}
	}

	st.Status = StepRunning
	st.StartedAt = time.Now()
	log.Printf("workflow: step %s starting (actor=%s action=%s)", spec.ID, spec.Actor, spec.Action)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		st.Attempt = attempt

		req := ActionRequest{
			RunID:   run.ID,
			StepID:  spec.ID,
			Actor:   spec.Actor,
			Action:  spec.Action,
			With:    resolveWith(spec.With, run.Inputs, run.Outputs),
			Inputs:  run.Inputs,
			Outputs: run.Outputs,
		}

		result, err := handler(stepCtx, req)
		if err != nil {
			lastErr = err
			if attempt < maxAttempts {
				log.Printf("workflow: step %s attempt %d/%d failed: %v — retrying in %s",
					spec.ID, attempt, maxAttempts, err, backoff)
				select {
				case <-stepCtx.Done():
					st.Status = StepFailed
					st.Error = fmt.Sprintf("timeout after %d attempts: %v", attempt, err)
					st.FinishedAt = time.Now()
					e.notifyStep(run, st)
					return fmt.Errorf("step %s timed out: %v", spec.ID, err)
				case <-time.After(backoff):
				}
				continue
			}
			st.Status = StepFailed
			st.Error = err.Error()
			st.FinishedAt = time.Now()
			log.Printf("workflow: step %s FAILED after %d attempts: %v", spec.ID, attempt, err)
			e.notifyStep(run, st)
			return fmt.Errorf("step %s failed: %v", spec.ID, lastErr)
		}

		if result != nil && !result.OK {
			lastErr = fmt.Errorf("%s", result.Message)
			if attempt < maxAttempts {
				log.Printf("workflow: step %s attempt %d/%d not OK: %s — retrying",
					spec.ID, attempt, maxAttempts, result.Message)
				select {
				case <-stepCtx.Done():
					st.Status = StepFailed
					st.Error = result.Message
					st.FinishedAt = time.Now()
					e.notifyStep(run, st)
					return fmt.Errorf("step %s timed out", spec.ID)
				case <-time.After(backoff):
				}
				continue
			}
			st.Status = StepFailed
			st.Error = result.Message
			st.FinishedAt = time.Now()
			e.notifyStep(run, st)
			return fmt.Errorf("step %s failed: %s", spec.ID, result.Message)
		}

		// Success.
		st.Status = StepSucceeded
		st.FinishedAt = time.Now()
		if result != nil {
			st.Output = result.Output
		}

		// Export output to run-level.
		if spec.Export != nil && spec.Export.String() != "" {
			run.Outputs[spec.Export.String()] = st.Output
		}

		log.Printf("workflow: step %s SUCCEEDED (attempt %d)", spec.ID, attempt)
		e.notifyStep(run, st)
		return nil
	}

	return lastErr
}

func (e *Engine) evaluateCondition(ctx context.Context, cond *v1alpha1.StepCondition, inputs, outputs map[string]any) (bool, error) {
	if cond == nil {
		return true, nil
	}

	// Simple expression.
	if cond.Expr != "" {
		if e.EvalCond != nil {
			return e.EvalCond(ctx, cond.Expr, inputs, outputs)
		}
		// Default: treat non-empty expression as true (pass-through).
		return true, nil
	}

	// anyOf: at least one must be true.
	if len(cond.AnyOf) > 0 {
		for _, child := range cond.AnyOf {
			ok, err := e.evaluateCondition(ctx, &child, inputs, outputs)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	}

	// allOf: all must be true.
	if len(cond.AllOf) > 0 {
		for _, child := range cond.AllOf {
			ok, err := e.evaluateCondition(ctx, &child, inputs, outputs)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil
	}

	// not: invert child.
	if cond.Not != nil {
		ok, err := e.evaluateCondition(ctx, cond.Not, inputs, outputs)
		if err != nil {
			return false, err
		}
		return !ok, nil
	}

	return true, nil
}

func (e *Engine) dispatchHook(ctx context.Context, run *Run, hook *v1alpha1.WorkflowHook) {
	handler, ok := e.Router.Resolve(hook.Actor, hook.Action)
	if !ok {
		log.Printf("workflow: hook %s::%s has no handler, skipping", hook.Actor, hook.Action)
		return
	}
	req := ActionRequest{
		RunID:   run.ID,
		Actor:   hook.Actor,
		Action:  hook.Action,
		With:    hook.With,
		Inputs:  run.Inputs,
		Outputs: run.Outputs,
	}
	if _, err := handler(ctx, req); err != nil {
		log.Printf("workflow: hook %s::%s failed: %v", hook.Actor, hook.Action, err)
	}
}

func (e *Engine) notifyStep(run *Run, st *StepState) {
	if e.OnStepDone != nil {
		e.OnStepDone(run, st)
	}
}

// --------------------------------------------------------------------------
// Expression resolution
// --------------------------------------------------------------------------

// resolveWith substitutes $.field references in step.With values with
// actual values from inputs or outputs.
func resolveWith(with map[string]any, inputs, outputs map[string]any) map[string]any {
	if len(with) == 0 {
		return with
	}
	resolved := make(map[string]any, len(with))
	for k, v := range with {
		resolved[k] = resolveValue(v, inputs, outputs)
	}
	return resolved
}

func resolveValue(v any, inputs, outputs map[string]any) any {
	switch val := v.(type) {
	case string:
		if strings.HasPrefix(val, "$.") {
			path := val[2:]
			if result, ok := outputs[path]; ok {
				return result
			}
			if result, ok := inputs[path]; ok {
				return result
			}
		}
		return val
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = resolveValue(item, inputs, outputs)
		}
		return out
	case map[string]any:
		return resolveWith(val, inputs, outputs)
	default:
		return v
	}
}
