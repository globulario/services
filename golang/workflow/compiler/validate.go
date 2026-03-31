package compiler

import (
	"fmt"
	"sort"
	"time"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// validate checks the definition for structural errors.
func validate(def *v1alpha1.WorkflowDefinition) []Diagnostic {
	var diags []Diagnostic

	if def.APIVersion == "" {
		diags = append(diags, Diagnostic{SeverityError, "apiVersion", "required", "apiVersion is required"})
	}
	if def.Kind == "" {
		diags = append(diags, Diagnostic{SeverityError, "kind", "required", "kind is required"})
	}
	if def.Metadata.Name == "" {
		diags = append(diags, Diagnostic{SeverityError, "metadata.name", "required", "workflow name is required"})
	}

	seen := make(map[string]struct{}, len(def.Spec.Steps))
	for i, s := range def.Spec.Steps {
		path := fmt.Sprintf("spec.steps[%d]", i)
		if s.ID == "" {
			diags = append(diags, Diagnostic{SeverityError, path + ".id", "required", "step id is required"})
			continue
		}
		if _, ok := seen[s.ID]; ok {
			diags = append(diags, Diagnostic{SeverityError, path + ".id", "duplicate_id", "duplicate step id: " + s.ID})
		}
		seen[s.ID] = struct{}{}
		if s.Actor == "" {
			diags = append(diags, Diagnostic{SeverityError, path + ".actor", "required", "actor is required"})
		}
		if s.Action == "" {
			diags = append(diags, Diagnostic{SeverityError, path + ".action", "required", "action is required"})
		}
		if s.Timeout != nil && !s.Timeout.IsExpression() && s.Timeout.String() != "" {
			if _, err := time.ParseDuration(s.Timeout.String()); err != nil {
				diags = append(diags, Diagnostic{SeverityError, path + ".timeout", "invalid_duration", err.Error()})
			}
		}
		if s.Retry != nil && s.Retry.Backoff != nil && !s.Retry.Backoff.IsExpression() {
			if _, err := time.ParseDuration(s.Retry.Backoff.String()); err != nil {
				diags = append(diags, Diagnostic{SeverityError, path + ".retry.backoff", "invalid_duration", err.Error()})
			}
		}
	}

	// Check dependency references.
	for i, s := range def.Spec.Steps {
		for _, dep := range s.DependsOn {
			if dep == s.ID {
				diags = append(diags, Diagnostic{SeverityError, fmt.Sprintf("spec.steps[%d].dependsOn", i), "self_dependency", "step cannot depend on itself"})
			}
			if _, ok := seen[dep]; !ok {
				diags = append(diags, Diagnostic{SeverityError, fmt.Sprintf("spec.steps[%d].dependsOn", i), "missing_dependency", "unknown dependency: " + dep})
			}
		}
	}

	// Cycle detection via topo sort.
	tmpSteps := make(map[string]*CompiledStep, len(def.Spec.Steps))
	for _, s := range def.Spec.Steps {
		if s.ID != "" {
			tmpSteps[s.ID] = &CompiledStep{ID: s.ID, DependsOn: s.DependsOn}
		}
	}
	if _, err := topoSort(tmpSteps); err != nil {
		diags = append(diags, Diagnostic{SeverityError, "spec.steps", "cycle_detected", err.Error()})
	}

	sort.SliceStable(diags, func(i, j int) bool {
		if diags[i].Path == diags[j].Path {
			return diags[i].Code < diags[j].Code
		}
		return diags[i].Path < diags[j].Path
	})
	return diags
}
