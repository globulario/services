package compiler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// Compile transforms a v1alpha1 WorkflowDefinition into a CompiledWorkflow.
// It normalizes defaults, validates the DAG, resolves static values, and
// preserves runtime expressions for later evaluation.
func Compile(ctx context.Context, def *v1alpha1.WorkflowDefinition) (*CompiledWorkflow, []Diagnostic, error) {
	_ = ctx
	if def == nil {
		return nil, []Diagnostic{{SeverityError, "", "nil_definition", "workflow definition is nil"}}, fmt.Errorf("workflow definition is nil")
	}

	diags := validate(def)
	if HasErrors(diags) {
		return nil, diags, fmt.Errorf("workflow definition has validation errors")
	}

	cw := &CompiledWorkflow{
		Name:     def.Metadata.Name,
		Defaults: def.Spec.Defaults,
		Metadata: CompiledMetadata{
			DisplayName: def.Metadata.DisplayName,
			Description: def.Metadata.Description,
			Labels:      def.Metadata.Labels,
		},
		Strategy: compileStrategy(def.Spec.Strategy),
		Steps:    make(map[string]*CompiledStep, len(def.Spec.Steps)),
		Dependents: make(map[string][]string),
	}

	if def.Spec.OnFailure != nil {
		cw.OnFailure = compileHook(def.Spec.OnFailure)
	}
	if def.Spec.OnSuccess != nil {
		cw.OnSuccess = compileHook(def.Spec.OnSuccess)
	}

	// Compile each step.
	for _, s := range def.Spec.Steps {
		cs := &CompiledStep{
			ID:        s.ID,
			Title:     s.Title,
			Actor:     string(s.Actor),
			Action:    s.Action,
			DependsOn: append([]string(nil), s.DependsOn...),
			With:      compileWith(s.With),
			Retry:     compileRetry(s.Retry),
			Timeout:   compileDuration(s.Timeout),
		}
		if s.Foreach != nil {
			ve := toValueExpr(s.Foreach.String())
			cs.Foreach = &ve
		}
		if s.ItemName != nil && s.ItemName.String() != "" {
			cs.ItemName = s.ItemName.String()
		}
		// Nested sub-steps for foreach groups.
		if len(s.Steps) > 0 {
			sub, subDiags, err := compileSubSteps(s.Steps)
			diags = append(diags, subDiags...)
			if err != nil {
				return nil, diags, fmt.Errorf("compile sub-steps for %s: %w", s.ID, err)
			}
			cs.SubSteps = sub
		}
		if s.OnFailure != nil {
			cs.OnFailure = compileHook(s.OnFailure)
		}
		if s.Export != nil && s.Export.String() != "" {
			cs.Export = s.Export.String()
		}
		if s.When != nil {
			cc := compileCondition(s.When)
			cs.When = &cc
		}
		// Workflow hardening fields (WH-1) — pass through if present.
		if s.Execution != nil {
			cs.Execution = &CompiledExecution{
				Idempotency:     s.Execution.Idempotency,
				ResumePolicy:    s.Execution.ResumePolicy,
				ReceiptKey:      s.Execution.ReceiptKey,
				ReceiptRequired: s.Execution.ReceiptRequired,
			}
		}
		if s.Verification != nil {
			cs.Verification = &CompiledVerification{
				Actor:       string(s.Verification.Actor),
				Action:      s.Verification.Action,
				With:        compileWith(s.Verification.With),
				SuccessExpr: s.Verification.Success.Expr,
			}
		}
		if s.Compensation != nil {
			cs.Compensation = &CompiledCompensation{
				Enabled: s.Compensation.Enabled,
				Actor:   string(s.Compensation.Actor),
				Action:  s.Compensation.Action,
				With:    compileWith(s.Compensation.With),
			}
		}
		cw.Steps[cs.ID] = cs
	}

	// Build dependency graph indexes.
	for id, step := range cw.Steps {
		for _, dep := range step.DependsOn {
			cw.Dependents[dep] = append(cw.Dependents[dep], id)
		}
	}
	for id, step := range cw.Steps {
		if len(step.DependsOn) == 0 {
			cw.EntryPoints = append(cw.EntryPoints, id)
		}
		step.Dependents = append([]string(nil), cw.Dependents[id]...)
		sort.Strings(step.Dependents)
	}
	sort.Strings(cw.EntryPoints)

	// Topological sort.
	order, err := topoSort(cw.Steps)
	if err != nil {
		diags = append(diags, Diagnostic{SeverityError, "spec.steps", "cycle_detected", err.Error()})
		return nil, diags, err
	}
	cw.TopoOrder = order

	// Source hash for caching.
	cw.SourceHash = sourceHash(def)

	return cw, diags, nil
}

// compileSubSteps recursively compiles nested steps into a sub-CompiledWorkflow.
func compileSubSteps(steps []v1alpha1.WorkflowStepSpec) (*CompiledWorkflow, []Diagnostic, error) {
	var diags []Diagnostic
	sub := &CompiledWorkflow{
		Steps:      make(map[string]*CompiledStep, len(steps)),
		Dependents: make(map[string][]string),
	}

	for _, s := range steps {
		cs := &CompiledStep{
			ID:        s.ID,
			Title:     s.Title,
			Actor:     string(s.Actor),
			Action:    s.Action,
			DependsOn: append([]string(nil), s.DependsOn...),
			With:      compileWith(s.With),
			Retry:     compileRetry(s.Retry),
			Timeout:   compileDuration(s.Timeout),
		}
		if s.Foreach != nil {
			ve := toValueExpr(s.Foreach.String())
			cs.Foreach = &ve
		}
		if s.Export != nil && s.Export.String() != "" {
			cs.Export = s.Export.String()
		}
		if s.When != nil {
			cc := compileCondition(s.When)
			cs.When = &cc
		}
		sub.Steps[cs.ID] = cs
	}

	// Build sub-DAG indexes.
	for id, step := range sub.Steps {
		for _, dep := range step.DependsOn {
			sub.Dependents[dep] = append(sub.Dependents[dep], id)
		}
	}
	for id, step := range sub.Steps {
		if len(step.DependsOn) == 0 {
			sub.EntryPoints = append(sub.EntryPoints, id)
		}
		step.Dependents = append([]string(nil), sub.Dependents[id]...)
		sort.Strings(step.Dependents)
	}
	sort.Strings(sub.EntryPoints)

	order, err := topoSort(sub.Steps)
	if err != nil {
		diags = append(diags, Diagnostic{SeverityError, "sub_steps", "cycle_detected", err.Error()})
		return nil, diags, err
	}
	sub.TopoOrder = order

	return sub, diags, nil
}

// MustCompile compiles a definition and panics on error.
func MustCompile(def *v1alpha1.WorkflowDefinition) *CompiledWorkflow {
	cw, _, err := Compile(context.Background(), def)
	if err != nil {
		panic(fmt.Sprintf("compiler: %v", err))
	}
	return cw
}

func compileStrategy(s v1alpha1.ExecutionStrategy) CompiledStrategy {
	cs := CompiledStrategy{Mode: string(s.Mode)}
	if cs.Mode == "" {
		cs.Mode = "single"
	}
	if s.Collection != nil {
		ve := toValueExpr(s.Collection.String())
		cs.Collection = &ve
	}
	if s.Concurrency != nil {
		ve := ValueExpr{}
		if v, ok := s.Concurrency.IntValue(); ok {
			ve.Static = v
		} else if s.Concurrency.IsExpression() {
			ve.Raw = s.Concurrency.String()
			ve.IsExpr = true
		}
		cs.Concurrency = &ve
	}
	if s.ItemName != nil {
		cs.ItemName = s.ItemName.String()
	}
	return cs
}

func compileHook(h *v1alpha1.WorkflowHook) *CompiledHook {
	return &CompiledHook{
		Actor:  string(h.Actor),
		Action: h.Action,
		With:   compileWith(h.With),
	}
}

func compileWith(with map[string]any) map[string]ValueExpr {
	if len(with) == 0 {
		return nil
	}
	out := make(map[string]ValueExpr, len(with))
	for k, v := range with {
		out[k] = toValueExprAny(v)
	}
	return out
}

func compileRetry(r *v1alpha1.RetryPolicy) CompiledRetry {
	cr := CompiledRetry{MaxAttempts: 1}
	if r == nil {
		return cr
	}
	if r.MaxAttempts > 0 {
		cr.MaxAttempts = r.MaxAttempts
	}
	if r.Backoff != nil && !r.Backoff.IsExpression() {
		if d, err := time.ParseDuration(r.Backoff.String()); err == nil {
			cr.Backoff = d
		}
	}
	return cr
}

func compileDuration(s *v1alpha1.ScalarString) time.Duration {
	if s == nil || s.IsExpression() {
		return 0
	}
	d, _ := time.ParseDuration(s.String())
	return d
}

func compileCondition(c *v1alpha1.StepCondition) CompiledCondition {
	cc := CompiledCondition{Expr: c.Expr}
	for _, child := range c.AnyOf {
		cc.AnyOf = append(cc.AnyOf, compileCondition(&child))
	}
	for _, child := range c.AllOf {
		cc.AllOf = append(cc.AllOf, compileCondition(&child))
	}
	if c.Not != nil {
		not := compileCondition(c.Not)
		cc.Not = &not
	}
	return cc
}

func toValueExpr(s string) ValueExpr {
	if strings.HasPrefix(s, "$.") {
		return ValueExpr{Raw: s, IsExpr: true}
	}
	return ValueExpr{Raw: s, Static: s}
}

func toValueExprAny(v any) ValueExpr {
	if s, ok := v.(string); ok {
		return toValueExpr(s)
	}
	return ValueExpr{Static: v}
}

func sourceHash(def *v1alpha1.WorkflowDefinition) string {
	b, err := json.Marshal(def)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func topoSort(steps map[string]*CompiledStep) ([]string, error) {
	inDeg := make(map[string]int, len(steps))
	adj := make(map[string][]string, len(steps))
	for id := range steps {
		inDeg[id] = 0
	}
	for id, step := range steps {
		for _, dep := range step.DependsOn {
			adj[dep] = append(adj[dep], id)
			inDeg[id]++
		}
	}

	queue := make([]string, 0, len(steps))
	for id, deg := range inDeg {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	order := make([]string, 0, len(steps))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)
		next := append([]string(nil), adj[id]...)
		sort.Strings(next)
		for _, child := range next {
			inDeg[child]--
			if inDeg[child] == 0 {
				queue = append(queue, child)
				sort.Strings(queue)
			}
		}
	}
	if len(order) != len(steps) {
		return nil, fmt.Errorf("dependency cycle detected")
	}
	return order, nil
}
