package engine

import (
	"context"
	"testing"
)

func TestDefaultEvalCond_Literals(t *testing.T) {
	ctx := context.Background()
	inputs := map[string]any{}
	outputs := map[string]any{}

	ok, err := DefaultEvalCond(ctx, "true", inputs, outputs)
	if err != nil || !ok {
		t.Errorf("expected true, got %v (err=%v)", ok, err)
	}

	ok, err = DefaultEvalCond(ctx, "false", inputs, outputs)
	if err != nil || ok {
		t.Errorf("expected false, got %v (err=%v)", ok, err)
	}
}

func TestDefaultEvalCond_Contains(t *testing.T) {
	ctx := context.Background()
	inputs := map[string]any{
		"node_profiles": []any{"control-plane", "gateway", "etcd"},
	}
	outputs := map[string]any{}

	tests := []struct {
		expr string
		want bool
	}{
		{"contains(inputs.node_profiles, 'etcd')", true},
		{"contains(inputs.node_profiles, 'storage')", false},
		{"contains(inputs.node_profiles, 'gateway')", true},
		{"contains(node_profiles, 'control-plane')", true},
		{"contains(node_profiles, 'missing')", false},
	}

	for _, tt := range tests {
		ok, err := DefaultEvalCond(ctx, tt.expr, inputs, outputs)
		if err != nil {
			t.Errorf("contains(%q): unexpected error: %v", tt.expr, err)
			continue
		}
		if ok != tt.want {
			t.Errorf("contains(%q) = %v, want %v", tt.expr, ok, tt.want)
		}
	}
}

func TestDefaultEvalCond_Len(t *testing.T) {
	ctx := context.Background()
	inputs := map[string]any{}
	outputs := map[string]any{
		"selected_targets": []any{"node-1", "node-2", "node-3"},
		"empty_list":       []any{},
	}

	tests := []struct {
		expr string
		want bool
	}{
		{"len(selected_targets) == 3", true},
		{"len(selected_targets) == 0", false},
		{"len(selected_targets) > 0", true},
		{"len(selected_targets) > 5", false},
		{"len(selected_targets) >= 3", true},
		{"len(selected_targets) >= 4", false},
		{"len(selected_targets) < 5", true},
		{"len(selected_targets) <= 3", true},
		{"len(selected_targets) != 0", true},
		{"len(empty_list) == 0", true},
		{"len(empty_list) > 0", false},
		// Undefined variables return length -1 (fail-closed: undefined ≠ empty).
		// This prevents accidental short-circuit when a prior step was skipped.
		{"len(missing_var) == 0", false},
		{"len(missing_var) == -1", true},
	}

	for _, tt := range tests {
		ok, err := DefaultEvalCond(ctx, tt.expr, inputs, outputs)
		if err != nil {
			t.Errorf("len(%q): unexpected error: %v", tt.expr, err)
			continue
		}
		if ok != tt.want {
			t.Errorf("len(%q) = %v, want %v", tt.expr, ok, tt.want)
		}
	}
}

func TestDefaultEvalCond_Equality(t *testing.T) {
	ctx := context.Background()
	inputs := map[string]any{
		"restart_required": true,
		"mode":             "rolling",
	}
	outputs := map[string]any{
		"status": "AVAILABLE",
	}

	tests := []struct {
		expr string
		want bool
	}{
		{"inputs.restart_required == true", true},
		{"inputs.restart_required == false", false},
		{"inputs.mode == rolling", true},
		{"inputs.mode == canary", false},
		{"outputs.status == AVAILABLE", true},
		{"outputs.status == FAILED", false},
	}

	for _, tt := range tests {
		ok, err := DefaultEvalCond(ctx, tt.expr, inputs, outputs)
		if err != nil {
			t.Errorf("eq(%q): unexpected error: %v", tt.expr, err)
			continue
		}
		if ok != tt.want {
			t.Errorf("eq(%q) = %v, want %v", tt.expr, ok, tt.want)
		}
	}
}

func TestDefaultEvalCond_Inequality(t *testing.T) {
	ctx := context.Background()
	inputs := map[string]any{
		"restart_policy": "auto",
		"mode":           "rolling",
	}
	outputs := map[string]any{}

	tests := []struct {
		expr string
		want bool
	}{
		{"inputs.restart_policy != 'never'", true},
		{"inputs.restart_policy != 'auto'", false},
		{"inputs.mode != 'canary'", true},
		{"inputs.mode != 'rolling'", false},
	}

	for _, tt := range tests {
		ok, err := DefaultEvalCond(ctx, tt.expr, inputs, outputs)
		if err != nil {
			t.Errorf("ineq(%q): unexpected error: %v", tt.expr, err)
			continue
		}
		if ok != tt.want {
			t.Errorf("ineq(%q) = %v, want %v", tt.expr, ok, tt.want)
		}
	}
}

func TestDefaultEvalCond_UnknownDefaultsTrue(t *testing.T) {
	ctx := context.Background()
	ok, err := DefaultEvalCond(ctx, "some_unknown_expression", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("unknown expression should default to true")
	}
}

func TestDefaultEvalCond_LenWithMap(t *testing.T) {
	ctx := context.Background()
	outputs := map[string]any{
		"results": map[string]any{"a": 1, "b": 2},
	}
	ok, err := DefaultEvalCond(ctx, "len(results) == 2", nil, outputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("len(map) == 2 should be true")
	}
}

func TestDefaultEvalCond_ContainsMalformed(t *testing.T) {
	ctx := context.Background()
	_, err := DefaultEvalCond(ctx, "contains(broken", nil, nil)
	if err == nil {
		t.Error("expected error for malformed contains")
	}
}
