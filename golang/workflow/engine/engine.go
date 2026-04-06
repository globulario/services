// Package engine executes workflow definitions.
//
// It evaluates the step DAG, dispatches actions to actors, handles retries
// and timeouts, and tracks run/step state. Definitions are compiled via
// the compiler package before execution — all parsing, validation, and
// graph construction happens once at compile time.
package engine

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/workflow/compiler"
	"github.com/globulario/services/golang/workflow/v1alpha1"
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
	mu        sync.RWMutex
	handlers  map[string]ActionHandler // key: "actor::action"
	fallbacks map[string]ActionHandler // key: actor — transport-only dispatch
}

func NewRouter() *Router {
	return &Router{
		handlers:  make(map[string]ActionHandler),
		fallbacks: make(map[string]ActionHandler),
	}
}

func (r *Router) Register(actor v1alpha1.ActorType, action string, h ActionHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[string(actor)+"::"+action] = h
}

// RegisterFallback sets a transport-only fallback handler for an actor type.
// Resolve checks exact (actor, action) matches first; only if no exact match
// is found does it try the fallback. This is used by WorkflowService to route
// actions for remote actors through a single gRPC dispatch handler.
//
// Fallback handlers are transport mechanisms, not semantic handlers. The actor
// callback must reject unknown actions — the fallback must not silently accept
// them. See docs/centralized-workflow-execution.md §4.
func (r *Router) RegisterFallback(actor v1alpha1.ActorType, h ActionHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fallbacks[string(actor)] = h
}

func (r *Router) Resolve(actor v1alpha1.ActorType, action string) (ActionHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if h, ok := r.handlers[string(actor)+"::"+action]; ok {
		return h, true
	}
	if h, ok := r.fallbacks[string(actor)]; ok {
		return h, true
	}
	return nil, false
}

// RegisteredActions returns all explicitly registered (actor, action) pairs.
// Used by tests to verify actor capability parity — each actor's local Router
// must cover the same actions the central registry declares.
func (r *Router) RegisteredActions() map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string][]string)
	for key := range r.handlers {
		parts := strings.SplitN(key, "::", 2)
		if len(parts) == 2 {
			result[parts[0]] = append(result[parts[0]], parts[1])
		}
	}
	return result
}

func (r *Router) resolveByName(actor, action string) (ActionHandler, bool) {
	return r.Resolve(v1alpha1.ActorType(actor), action)
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

// Engine executes compiled workflows to completion.
type Engine struct {
	Router       *Router
	EvalCond     ConditionFunc
	OnStepDone   func(run *Run, step *StepState) // optional callback for observability
	PreCompleted map[string]StepStatus            // steps already completed (for resume after crash)
}

// Execute compiles a v1alpha1 definition and executes it. This is the
// convenience entry point — it compiles, then delegates to ExecuteCompiled.
func (e *Engine) Execute(ctx context.Context, def *v1alpha1.WorkflowDefinition, inputs map[string]any) (*Run, error) {
	if def == nil {
		return nil, fmt.Errorf("definition is nil")
	}
	cw, _, err := compiler.Compile(ctx, def)
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", def.Metadata.Name, err)
	}
	return e.ExecuteCompiled(ctx, cw, inputs)
}

// ExecuteCompiled runs a pre-compiled workflow with the given inputs.
// It blocks until the workflow completes or the context is canceled.
func (e *Engine) ExecuteCompiled(ctx context.Context, cw *compiler.CompiledWorkflow, inputs map[string]any) (*Run, error) {
	if cw == nil {
		return nil, fmt.Errorf("compiled workflow is nil")
	}

	// Merge defaults + inputs.
	merged := make(map[string]any)
	for k, v := range cw.Defaults {
		merged[k] = v
	}
	for k, v := range inputs {
		merged[k] = v
	}

	run := &Run{
		ID:         fmt.Sprintf("run-%d", time.Now().UnixMilli()),
		Definition: cw.Name,
		Status:     RunRunning,
		Inputs:     merged,
		Outputs:    make(map[string]any),
		Steps:      make(map[string]*StepState, len(cw.Steps)),
		StartedAt:  time.Now(),
	}
	for id := range cw.Steps {
		run.Steps[id] = &StepState{ID: id, Status: StepPending}
	}

	// For resume after crash: pre-populate steps that are already complete.
	// The DAG walker skips these because depsReady considers them satisfied,
	// and they are never in StepPending state so they won't be re-dispatched.
	if len(e.PreCompleted) > 0 {
		for stepID, status := range e.PreCompleted {
			if st, ok := run.Steps[stepID]; ok {
				st.Status = status
				log.Printf("workflow: step %s pre-completed (%s) from prior execution", stepID, status)
			}
		}
	}

	// Execute DAG using compiled topo order.
	err := e.executeDAG(ctx, run, cw)

	run.FinishedAt = time.Now()
	if err != nil {
		run.Status = RunFailed
		run.Error = err.Error()
		if cw.OnFailure != nil {
			e.dispatchHook(ctx, run, cw.OnFailure)
		}
	} else {
		run.Status = RunSucceeded
		if cw.OnSuccess != nil {
			e.dispatchHook(ctx, run, cw.OnSuccess)
		}
	}

	return run, err
}

// --------------------------------------------------------------------------
// DAG execution
// --------------------------------------------------------------------------

// executeDAG runs all steps respecting their dependency order.
// Steps with satisfied dependencies execute in parallel.
func (e *Engine) executeDAG(ctx context.Context, run *Run, cw *compiler.CompiledWorkflow) error {
	// Use topo order for deterministic iteration; parallelize independent steps.
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		var ready []*compiler.CompiledStep
		allDone := true
		hasBlockedByFailure := false
		for _, id := range cw.TopoOrder {
			st := run.Steps[id]
			if st.Status == StepPending {
				allDone = false
				if depsReady(run, cw.Steps[id].DependsOn) {
					ready = append(ready, cw.Steps[id])
				} else if depsFailed(run, cw.Steps[id].DependsOn) {
					// This step is blocked by a failed dependency — it
					// will never become ready. Mark it so the DAG walker
					// can exit instead of looping forever.
					hasBlockedByFailure = true
				}
			} else if st.Status == StepRunning {
				allDone = false
			}
		}

		if allDone {
			break
		}
		if len(ready) == 0 {
			if hasBlockedByFailure {
				// All remaining pending steps are blocked by failed deps.
				// Exit the DAG loop — the failed-step check below will
				// produce the appropriate error.
				break
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}

		var wg sync.WaitGroup
		var mu sync.Mutex
		var firstErr error

		for _, step := range ready {
			step := step
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := e.executeStep(ctx, run, step); err != nil {
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

	for _, st := range run.Steps {
		if st.Status == StepFailed {
			return fmt.Errorf("step %s failed: %s", st.ID, st.Error)
		}
	}
	return nil
}

// depsFailed returns true if any dependency is in FAILED state, meaning
// the dependent step can never become ready.
func depsFailed(run *Run, deps []string) bool {
	for _, dep := range deps {
		st, ok := run.Steps[dep]
		if !ok {
			continue
		}
		if st.Status == StepFailed {
			return true
		}
	}
	return false
}

func depsReady(run *Run, deps []string) bool {
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

// --------------------------------------------------------------------------
// Step execution
// --------------------------------------------------------------------------

func (e *Engine) executeStep(ctx context.Context, run *Run, step *compiler.CompiledStep) error {
	st := run.Steps[step.ID]

	// Foreach expansion.
	if step.Foreach != nil {
		return e.executeForeach(ctx, run, step)
	}

	// Evaluate when condition.
	if step.When != nil {
		ok, err := e.evalCondition(ctx, step.When, run.Inputs, run.Outputs)
		if err != nil {
			st.Status = StepFailed
			st.Error = fmt.Sprintf("condition eval: %v", err)
			return nil
		}
		if !ok {
			st.Status = StepSkipped
			log.Printf("workflow: step %s skipped (condition not met)", step.ID)
			e.notifyStep(run, st)
			return nil
		}
	}

	// Resolve handler.
	handler, ok := e.Router.resolveByName(step.Actor, step.Action)
	if !ok {
		st.Status = StepFailed
		st.Error = fmt.Sprintf("no handler for %s::%s", step.Actor, step.Action)
		e.notifyStep(run, st)
		return fmt.Errorf("step %s: no handler for %s::%s", step.ID, step.Actor, step.Action)
	}

	// Pre-resolved timeout from compiler.
	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
	}

	// Pre-resolved retry from compiler.
	maxAttempts := step.Retry.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	backoff := step.Retry.Backoff
	if backoff == 0 {
		backoff = 2 * time.Second
	}

	st.Status = StepRunning
	st.StartedAt = time.Now()
	log.Printf("workflow: step %s starting (actor=%s action=%s)", step.ID, step.Actor, step.Action)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		st.Attempt = attempt

		req := ActionRequest{
			RunID:   run.ID,
			StepID:  step.ID,
			Actor:   v1alpha1.ActorType(step.Actor),
			Action:  step.Action,
			With:    resolveCompiledWith(step.With, run.Inputs, run.Outputs),
			Inputs:  run.Inputs,
			Outputs: run.Outputs,
		}

		result, err := handler(stepCtx, req)
		if err != nil {
			lastErr = err
			if attempt < maxAttempts {
				log.Printf("workflow: step %s attempt %d/%d failed: %v — retrying in %s",
					step.ID, attempt, maxAttempts, err, backoff)
				select {
				case <-stepCtx.Done():
					st.Status = StepFailed
					st.Error = fmt.Sprintf("timeout after %d attempts: %v", attempt, err)
					st.FinishedAt = time.Now()
					e.notifyStep(run, st)
					return fmt.Errorf("step %s timed out: %v", step.ID, err)
				case <-time.After(backoff):
				}
				continue
			}
			st.Status = StepFailed
			st.Error = err.Error()
			st.FinishedAt = time.Now()
			log.Printf("workflow: step %s FAILED after %d attempts: %v", step.ID, attempt, err)
			e.notifyStep(run, st)
			return fmt.Errorf("step %s failed: %v", step.ID, lastErr)
		}

		if result != nil && !result.OK {
			lastErr = fmt.Errorf("%s", result.Message)
			if attempt < maxAttempts {
				log.Printf("workflow: step %s attempt %d/%d not OK: %s — retrying",
					step.ID, attempt, maxAttempts, result.Message)
				select {
				case <-stepCtx.Done():
					st.Status = StepFailed
					st.Error = result.Message
					st.FinishedAt = time.Now()
					e.notifyStep(run, st)
					return fmt.Errorf("step %s timed out", step.ID)
				case <-time.After(backoff):
				}
				continue
			}
			st.Status = StepFailed
			st.Error = result.Message
			st.FinishedAt = time.Now()
			e.notifyStep(run, st)
			return fmt.Errorf("step %s failed: %s", step.ID, result.Message)
		}

		// Success.
		st.Status = StepSucceeded
		st.FinishedAt = time.Now()
		if result != nil {
			st.Output = result.Output
		}
		if step.Export != "" {
			run.Outputs[step.Export] = st.Output
		}

		log.Printf("workflow: step %s SUCCEEDED (attempt %d)", step.ID, attempt)
		e.notifyStep(run, st)
		return nil
	}

	return lastErr
}

// --------------------------------------------------------------------------
// Foreach expansion
// --------------------------------------------------------------------------

func (e *Engine) executeForeach(ctx context.Context, run *Run, step *compiler.CompiledStep) error {
	st := run.Steps[step.ID]

	// Resolve collection from inputs/outputs.
	var items []any
	if step.Foreach.IsExpr {
		raw := resolveValue(step.Foreach.Raw, run.Inputs, run.Outputs)
		if list, ok := raw.([]any); ok {
			items = list
		}
	} else if step.Foreach.Raw != "" {
		raw := resolveValue(step.Foreach.Raw, run.Inputs, run.Outputs)
		if list, ok := raw.([]any); ok {
			items = list
		}
	}

	if len(items) == 0 {
		st.Status = StepSkipped
		log.Printf("workflow: step %s skipped (foreach collection empty)", step.ID)
		e.notifyStep(run, st)
		return nil
	}

	// Evaluate when condition once before iterating.
	if step.When != nil {
		ok, err := e.evalCondition(ctx, step.When, run.Inputs, run.Outputs)
		if err != nil {
			st.Status = StepFailed
			st.Error = fmt.Sprintf("condition eval: %v", err)
			return nil
		}
		if !ok {
			st.Status = StepSkipped
			e.notifyStep(run, st)
			return nil
		}
	}

	// Nested sub-steps: execute a sub-DAG per item.
	if step.SubSteps != nil {
		return e.executeForeachWithSubSteps(ctx, run, step, items)
	}

	handler, ok := e.Router.resolveByName(step.Actor, step.Action)
	if !ok {
		st.Status = StepFailed
		st.Error = fmt.Sprintf("no handler for %s::%s", step.Actor, step.Action)
		e.notifyStep(run, st)
		return fmt.Errorf("step %s: no handler for %s::%s", step.ID, step.Actor, step.Action)
	}

	st.Status = StepRunning
	st.StartedAt = time.Now()
	log.Printf("workflow: step %s starting foreach (%d items, actor=%s action=%s)",
		step.ID, len(items), step.Actor, step.Action)

	var results []any
	for i, item := range items {
		iterInputs := make(map[string]any, len(run.Inputs)+2)
		for k, v := range run.Inputs {
			iterInputs[k] = v
		}
		iterInputs["item"] = item
		iterInputs["item_index"] = i
		if s, ok := item.(string); ok {
			iterInputs["node_id"] = s
		}

		req := ActionRequest{
			RunID:   run.ID,
			StepID:  fmt.Sprintf("%s[%d]", step.ID, i),
			Actor:   v1alpha1.ActorType(step.Actor),
			Action:  step.Action,
			With:    resolveCompiledWith(step.With, iterInputs, run.Outputs),
			Inputs:  iterInputs,
			Outputs: run.Outputs,
		}

		result, err := handler(ctx, req)
		if err != nil {
			st.Status = StepFailed
			st.Error = fmt.Sprintf("item %d: %v", i, err)
			st.FinishedAt = time.Now()
			e.notifyStep(run, st)
			return fmt.Errorf("step %s item %d: %v", step.ID, i, err)
		}
		if result != nil && !result.OK {
			st.Status = StepFailed
			st.Error = fmt.Sprintf("item %d: %s", i, result.Message)
			st.FinishedAt = time.Now()
			e.notifyStep(run, st)
			return fmt.Errorf("step %s item %d: %s", step.ID, i, result.Message)
		}
		if result != nil && result.Output != nil {
			results = append(results, result.Output)
		}
	}

	st.Status = StepSucceeded
	st.FinishedAt = time.Now()
	st.Output = map[string]any{"results": results, "count": len(items)}
	if step.Export != "" {
		run.Outputs[step.Export] = results
	}

	log.Printf("workflow: step %s foreach SUCCEEDED (%d/%d items)", step.ID, len(items), len(items))
	e.notifyStep(run, st)
	return nil
}

// executeForeachWithSubSteps runs a nested sub-DAG per foreach item.
// Unlike flat foreach, this does NOT short-circuit on first failure —
// all items run to completion so aggregate steps can compute DEGRADED vs FAILED.
func (e *Engine) executeForeachWithSubSteps(ctx context.Context, run *Run, step *compiler.CompiledStep, items []any) error {
	st := run.Steps[step.ID]
	st.Status = StepRunning
	st.StartedAt = time.Now()

	itemName := step.ItemName
	if itemName == "" {
		itemName = "target"
	}

	log.Printf("workflow: step %s starting foreach-with-substeps (%d items, %d sub-steps)",
		step.ID, len(items), len(step.SubSteps.Steps))

	var allResults []any
	succeeded := 0
	failed := 0

	for i, item := range items {
		// Build per-item inputs.
		itemInputs := make(map[string]any, len(run.Inputs)+3)
		for k, v := range run.Inputs {
			itemInputs[k] = v
		}
		// Merge parent outputs so sub-steps can see prior step exports.
		for k, v := range run.Outputs {
			itemInputs[k] = v
		}
		itemInputs["item"] = item
		itemInputs["item_index"] = i
		itemInputs[itemName] = item
		if s, ok := item.(string); ok {
			itemInputs["node_id"] = s
		}
		// If item is a map, flatten its fields into inputs for $.target.node_id etc.
		if m, ok := item.(map[string]any); ok {
			for k, v := range m {
				itemInputs[itemName+"."+k] = v
				// Also expose as node_id if present.
				if k == "node_id" {
					itemInputs["node_id"] = v
				}
			}
		}

		// Create a child run for this item's sub-DAG.
		childRun := &Run{
			ID:         fmt.Sprintf("%s[%d]", run.ID, i),
			Definition: run.Definition,
			Status:     RunRunning,
			Inputs:     itemInputs,
			Outputs:    make(map[string]any),
			Steps:      make(map[string]*StepState, len(step.SubSteps.Steps)),
			StartedAt:  time.Now(),
		}
		for id := range step.SubSteps.Steps {
			childRun.Steps[id] = &StepState{ID: id, Status: StepPending}
		}

		// Also register child step states in parent run for observability.
		for id := range step.SubSteps.Steps {
			qualID := fmt.Sprintf("%s[%d].%s", step.ID, i, id)
			run.Steps[qualID] = childRun.Steps[id]
		}

		log.Printf("workflow: %s[%d] starting sub-DAG", step.ID, i)
		err := e.executeDAG(ctx, childRun, step.SubSteps)
		childRun.FinishedAt = time.Now()

		if err != nil {
			childRun.Status = RunFailed
			childRun.Error = err.Error()
			failed++
			log.Printf("workflow: %s[%d] sub-DAG FAILED: %v", step.ID, i, err)
			// Fire per-item onFailure hook.
			if step.OnFailure != nil {
				e.dispatchHook(ctx, childRun, step.OnFailure)
			}
			allResults = append(allResults, map[string]any{
				"index": i, "status": "FAILED", "error": err.Error(),
			})
			// Do NOT return — continue with remaining items.
		} else {
			childRun.Status = RunSucceeded
			succeeded++
			log.Printf("workflow: %s[%d] sub-DAG SUCCEEDED", step.ID, i)
			allResults = append(allResults, map[string]any{
				"index": i, "status": "SUCCEEDED", "outputs": childRun.Outputs,
			})
		}
	}

	st.FinishedAt = time.Now()
	st.Output = map[string]any{
		"results":   allResults,
		"count":     len(items),
		"succeeded": succeeded,
		"failed":    failed,
	}

	if step.Export != "" {
		run.Outputs[step.Export] = allResults
	}

	if failed > 0 {
		st.Status = StepFailed
		st.Error = fmt.Sprintf("%d/%d items failed", failed, len(items))
		e.notifyStep(run, st)
		return fmt.Errorf("step %s: %d/%d items failed", step.ID, failed, len(items))
	}

	st.Status = StepSucceeded
	log.Printf("workflow: step %s foreach-with-substeps SUCCEEDED (%d/%d items)",
		step.ID, succeeded, len(items))
	e.notifyStep(run, st)
	return nil
}

// --------------------------------------------------------------------------
// Condition evaluation
// --------------------------------------------------------------------------

func (e *Engine) evalCondition(ctx context.Context, cond *compiler.CompiledCondition, inputs, outputs map[string]any) (bool, error) {
	if cond == nil {
		return true, nil
	}
	if cond.Expr != "" {
		if e.EvalCond != nil {
			return e.EvalCond(ctx, cond.Expr, inputs, outputs)
		}
		// Fall back to built-in expression evaluator.
		return DefaultEvalCond(ctx, cond.Expr, inputs, outputs)
	}
	if len(cond.AnyOf) > 0 {
		for _, child := range cond.AnyOf {
			ok, err := e.evalCondition(ctx, &child, inputs, outputs)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	}
	if len(cond.AllOf) > 0 {
		for _, child := range cond.AllOf {
			ok, err := e.evalCondition(ctx, &child, inputs, outputs)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil
	}
	if cond.Not != nil {
		ok, err := e.evalCondition(ctx, cond.Not, inputs, outputs)
		if err != nil {
			return false, err
		}
		return !ok, nil
	}
	return true, nil
}

// --------------------------------------------------------------------------
// Hooks
// --------------------------------------------------------------------------

func (e *Engine) dispatchHook(ctx context.Context, run *Run, hook *compiler.CompiledHook) {
	handler, ok := e.Router.resolveByName(hook.Actor, hook.Action)
	if !ok {
		log.Printf("workflow: hook %s::%s has no handler, skipping", hook.Actor, hook.Action)
		return
	}
	req := ActionRequest{
		RunID:   run.ID,
		Actor:   v1alpha1.ActorType(hook.Actor),
		Action:  hook.Action,
		With:    resolveCompiledWith(hook.With, run.Inputs, run.Outputs),
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

// resolveCompiledWith resolves ValueExpr maps into concrete values.
func resolveCompiledWith(with map[string]compiler.ValueExpr, inputs, outputs map[string]any) map[string]any {
	if len(with) == 0 {
		return nil
	}
	resolved := make(map[string]any, len(with))
	for k, ve := range with {
		if ve.IsExpr {
			resolved[k] = resolveValue(ve.Raw, inputs, outputs)
		} else if ve.Static != nil {
			resolved[k] = ve.Static
		} else {
			resolved[k] = ve.Raw
		}
	}
	return resolved
}

// resolveWith substitutes $.field references in step.With values.
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

// resolveDotPath navigates a dotted path (e.g. "foo.bar.baz") into nested maps.
func resolveDotPath(path string, data map[string]any) (any, bool) {
	parts := strings.SplitN(path, ".", 2)
	if len(parts) < 2 {
		return nil, false
	}
	root, ok := data[parts[0]]
	if !ok {
		return nil, false
	}
	m, ok := root.(map[string]any)
	if !ok {
		return nil, false
	}
	rest := parts[1]
	if val, ok := m[rest]; ok {
		return val, true
	}
	// Recurse for deeper paths.
	return resolveDotPath(rest, m)
}

func resolveValue(v any, inputs, outputs map[string]any) any {
	switch val := v.(type) {
	case string:
		if strings.HasPrefix(val, "$.") {
			path := val[2:]
			// Direct key lookup first.
			if result, ok := outputs[path]; ok {
				return result
			}
			if result, ok := inputs[path]; ok {
				return result
			}
			// Dot-path navigation into nested maps (e.g. "workflow_choice.workflow_name").
			if result, ok := resolveDotPath(path, outputs); ok {
				return result
			}
			if result, ok := resolveDotPath(path, inputs); ok {
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
