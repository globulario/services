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
		"nil_list":         nil, // key present but nil — must NOT equal 0 (fail-closed)
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
		// Key present with nil value must also be treated as -1, not 0.
		// This guards against the release workflow short-circuit bug where
		// selectReleaseTargets returned nil instead of []any{} when all
		// nodes were converged, causing finalize_noop to never execute.
		{"len(nil_list) == 0", false},
		{"len(nil_list) == -1", true},
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

// TestDefaultEvalCond_NegationAndBareBool covers the real condition forms used
// in production definitions (e.g. backup: "!inputs.dry_run") that previously
// fell through to the unknown branch and silently returned true.
func TestDefaultEvalCond_NegationAndBareBool(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		expr    string
		inputs  map[string]any
		want    bool
	}{
		{"inputs.dry_run", map[string]any{"dry_run": true}, true},
		{"inputs.dry_run", map[string]any{"dry_run": false}, false},
		{"inputs.dry_run", map[string]any{}, false},          // undefined -> fail closed
		{"!inputs.dry_run", map[string]any{"dry_run": true}, false},
		{"!inputs.dry_run", map[string]any{"dry_run": false}, true},
		{"!inputs.dry_run", map[string]any{}, true},           // !undefined == !false
	}
	for _, tt := range cases {
		ok, err := DefaultEvalCond(ctx, tt.expr, tt.inputs, nil)
		if err != nil {
			t.Errorf("%q: unexpected error: %v", tt.expr, err)
			continue
		}
		if ok != tt.want {
			t.Errorf("%q (dry_run=%v) = %v, want %v", tt.expr, tt.inputs["dry_run"], ok, tt.want)
		}
	}
}

// TestDefaultEvalCond_UnknownFailsClosed locks in the fix for the AWG re-audit
// finding (meta.silence_is_not_valid_for_unexpected): a genuinely unparseable
// guard must surface an error (fail closed), never silently evaluate to true
// and authorize a side-effecting step. A valid-but-undefined identifier fails
// closed to false (not an error) so optional boolean guards still work.
func TestDefaultEvalCond_UnknownFailsClosed(t *testing.T) {
	ctx := context.Background()

	// Undefined-but-valid identifier: fail closed to false, no error.
	ok, err := DefaultEvalCond(ctx, "some_unknown_flag", nil, nil)
	if err != nil {
		t.Fatalf("undefined identifier should not error, got %v", err)
	}
	if ok {
		t.Error("undefined boolean guard must fail closed to false")
	}

	// Genuine garbage: must error, must not evaluate true.
	ok, err = DefaultEvalCond(ctx, "this is not @ valid expr", nil, nil)
	if err == nil {
		t.Fatal("unparseable expression must return an error, got nil")
	}
	if ok {
		t.Error("unparseable expression must not evaluate true (must fail closed)")
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

// TestDefaultEvalCond_Compound pins the && / || compound-expression
// behavior that the platform.upgrade dispatch_upgrades step depends on.
// Before the fix, "dry_run == false && len(upgrade_targets) > 0" fell
// through every branch and returned true silently (or false from the
// equality check on the unparseable RHS), so the dispatch step was
// always skipped and platform-upgrade was a no-op. Without these
// assertions, the platform.upgrade YAML's when: clause could regress
// undetected.
func TestDefaultEvalCond_Compound(t *testing.T) {
	ctx := context.Background()
	inputs := map[string]any{
		"dry_run": false,
		"mode":    "rolling",
	}
	outputs := map[string]any{
		"upgrade_targets": []any{"a", "b", "c"},
		"empty_list":      []any{},
		"status":          "AVAILABLE",
	}

	tests := []struct {
		expr string
		want bool
	}{
		// Canonical platform.upgrade dispatch guard.
		{"dry_run == false && len(upgrade_targets) > 0", true},
		// Same shape with dry_run flipped.
		{"dry_run == true && len(upgrade_targets) > 0", false},
		// Same shape with empty targets.
		{"dry_run == false && len(empty_list) > 0", false},
		// AND short-circuits on first false.
		{"false && len(upgrade_targets) > 0", false},
		// AND succeeds when all clauses true.
		{"true && true && true", true},
		// OR short-circuits on first true.
		{"true || false", true},
		{"false || true", true},
		// OR fails when all clauses false.
		{"false || false", false},
		// Mixed precedence: && binds tighter than ||.
		{"false && false || true", true},
		{"true || false && false", true},
		// Compound with comparisons.
		{"outputs.status == AVAILABLE && len(upgrade_targets) > 0", true},
		{"outputs.status == FAILED || inputs.mode == rolling", true},
	}

	for _, tt := range tests {
		ok, err := DefaultEvalCond(ctx, tt.expr, inputs, outputs)
		if err != nil {
			t.Errorf("compound(%q): unexpected error: %v", tt.expr, err)
			continue
		}
		if ok != tt.want {
			t.Errorf("compound(%q) = %v, want %v", tt.expr, ok, tt.want)
		}
	}
}
