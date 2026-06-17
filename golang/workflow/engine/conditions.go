// @awareness namespace=globular.platform
// @awareness component=platform_workflow.engine_conditions
// @awareness file_role=condition_evaluator_null_safe_fallback_to_true_on_missing_field
// @awareness implements=globular.platform:intent.workflow.source_of_operational_truth
// @awareness risk=medium
package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// DefaultEvalCond is a built-in condition evaluator that handles common
// expressions without requiring a custom EvalCond function:
//
//   - contains(inputs.X, 'val')    — check if array contains value
//   - len(X) == N / len(X) > N     — check collection length
//   - inputs.X == val              — simple equality
//   - A && B / A || B              — compound conjunction/disjunction
//   - true / false                 — literal booleans
//
// Compound operators short-circuit: && stops at first false, || stops at
// first true. Sub-expressions are recursively evaluated by DefaultEvalCond.
// Operator precedence: && binds tighter than ||. To override grouping,
// restructure the YAML condition as AllOf/AnyOf.
func DefaultEvalCond(ctx context.Context, expr string, inputs, outputs map[string]any) (bool, error) {
	expr = strings.TrimSpace(expr)

	// Literal booleans.
	if expr == "true" {
		return true, nil
	}
	if expr == "false" {
		return false, nil
	}

	// Compound disjunction: split on `||` first (lower precedence).
	if parts := splitTopLevel(expr, "||"); len(parts) > 1 {
		for _, p := range parts {
			ok, err := DefaultEvalCond(ctx, strings.TrimSpace(p), inputs, outputs)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	}

	// Compound conjunction: split on `&&` (higher precedence than ||).
	if parts := splitTopLevel(expr, "&&"); len(parts) > 1 {
		for _, p := range parts {
			ok, err := DefaultEvalCond(ctx, strings.TrimSpace(p), inputs, outputs)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil
	}

	// Unary negation: !<expr>  (e.g. !inputs.dry_run). Must come before the
	// "!=" inequality check below, which matches an embedded "!=" not a
	// leading "!".
	if strings.HasPrefix(expr, "!") && !strings.HasPrefix(expr, "!=") {
		ok, err := DefaultEvalCond(ctx, strings.TrimSpace(expr[1:]), inputs, outputs)
		if err != nil {
			return false, err
		}
		return !ok, nil
	}

	// contains(inputs.X, 'val') or contains(X, 'val')
	if strings.HasPrefix(expr, "contains(") {
		return evalContains(expr, inputs, outputs)
	}

	// len(X) OP N
	if strings.HasPrefix(expr, "len(") {
		return evalLen(expr, inputs, outputs)
	}

	// inputs.X != val / outputs.X != val  (must come before == check)
	if strings.Contains(expr, "!=") {
		return evalInequality(expr, inputs, outputs)
	}

	// inputs.X == val / outputs.X == val
	if strings.Contains(expr, "==") {
		return evalEquality(expr, inputs, outputs)
	}

	// Bare boolean reference: inputs.X / outputs.X / X used directly as a guard
	// (e.g. "inputs.dry_run", "all_installed"). Resolve and interpret as bool.
	// An undefined identifier fails CLOSED to false (consistent with evalLen:
	// undefined != truthy) — many guards are optional booleans — but a
	// non-boolean value cannot be used as a condition and must error.
	if isIdentifierPath(expr) {
		switch v := resolveVar(expr, inputs, outputs).(type) {
		case nil:
			return false, nil
		case bool:
			return v, nil
		case string:
			switch v {
			case "true":
				return true, nil
			case "false", "":
				return false, nil
			default:
				return false, fmt.Errorf("non-boolean string %q for condition %q", v, expr)
			}
		default:
			return false, fmt.Errorf("non-boolean value for condition %q", expr)
		}
	}

	// Genuinely unrecognized / unparseable expression — fail closed with an
	// error. Silently returning true (the prior behavior) let a side-effecting
	// step run when its guard could not be understood — a typo, a new
	// operator, a malformed condition. The engine is fail-closed elsewhere
	// (evalLen returns -1 for undefined vars; preflight rejects unresolvable
	// handlers); an unrecognized guard must surface an error, not authorize
	// execution. (meta.silence_is_not_valid_for_unexpected)
	return false, fmt.Errorf("unrecognized condition expression %q", expr)
}

// isIdentifierPath reports whether expr is a bare variable path such as
// "inputs.dry_run", "outputs.repair_plan.ready", or "ok": letters, digits,
// underscores and dots only, starting with a letter or underscore. Used to
// distinguish an undefined-but-valid boolean guard (fail closed to false)
// from genuinely unparseable garbage (error).
func isIdentifierPath(expr string) bool {
	if expr == "" {
		return false
	}
	for i := 0; i < len(expr); i++ {
		c := expr[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
		case (c >= '0' && c <= '9') || c == '.':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// splitTopLevel splits `expr` on the given operator at top-level,
// respecting nested parentheses. Returns a single-element slice if no
// top-level split is found, so callers can distinguish "no operator" from
// "operator present but unbalanced parens" with a len() check.
func splitTopLevel(expr, op string) []string {
	var parts []string
	depth := 0
	last := 0
	for i := 0; i < len(expr); i++ {
		c := expr[i]
		switch c {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && i+len(op) <= len(expr) && expr[i:i+len(op)] == op {
			parts = append(parts, expr[last:i])
			last = i + len(op)
			i += len(op) - 1
		}
	}
	parts = append(parts, expr[last:])
	return parts
}

// evalContains handles: contains(inputs.node_profiles, 'etcd')
func evalContains(expr string, inputs, outputs map[string]any) (bool, error) {
	// Extract the collection path and target value.
	inner := expr[len("contains("):]
	if !strings.HasSuffix(inner, ")") {
		return false, fmt.Errorf("malformed contains expression: %s", expr)
	}
	inner = inner[:len(inner)-1]

	parts := strings.SplitN(inner, ",", 2)
	if len(parts) != 2 {
		return false, nil
	}

	collPath := strings.TrimSpace(parts[0])
	targetRaw := strings.TrimSpace(parts[1])
	target := strings.Trim(targetRaw, "'\"")

	// Resolve collection.
	coll := resolveVar(collPath, inputs, outputs)
	items, ok := coll.([]any)
	if !ok {
		return false, nil
	}

	for _, item := range items {
		if fmt.Sprint(item) == target {
			return true, nil
		}
	}
	return false, nil
}

// evalLen handles: len(selected_targets) == 0, len(X) > 0
//
// SAFETY: if the variable is undefined (nil), len() returns -1 so that
// "len(X) == 0" evaluates to false. This prevents the clean-path
// short-circuit when a prior step was skipped and never populated the
// variable. Fail-closed: undefined ≠ empty.
func evalLen(expr string, inputs, outputs map[string]any) (bool, error) {
	// Parse: len(VARNAME) OP NUMBER
	closeParen := strings.Index(expr, ")")
	if closeParen < 0 {
		return false, fmt.Errorf("malformed len expression: %s", expr)
	}

	varName := strings.TrimSpace(expr[len("len("):closeParen])
	rest := strings.TrimSpace(expr[closeParen+1:])

	// Resolve variable. If undefined, use -1 (fail-closed: not zero).
	val := resolveVar(varName, inputs, outputs)
	length := -1
	if val != nil {
		length = collectionLength(val)
	}

	// Parse operator and number.
	if strings.HasPrefix(rest, "==") {
		n, err := strconv.Atoi(strings.TrimSpace(rest[2:]))
		if err != nil {
			return false, err
		}
		return length == n, nil
	}
	if strings.HasPrefix(rest, "!=") {
		n, err := strconv.Atoi(strings.TrimSpace(rest[2:]))
		if err != nil {
			return false, err
		}
		return length != n, nil
	}
	if strings.HasPrefix(rest, ">=") {
		n, err := strconv.Atoi(strings.TrimSpace(rest[2:]))
		if err != nil {
			return false, err
		}
		return length >= n, nil
	}
	if strings.HasPrefix(rest, "<=") {
		n, err := strconv.Atoi(strings.TrimSpace(rest[2:]))
		if err != nil {
			return false, err
		}
		return length <= n, nil
	}
	if strings.HasPrefix(rest, ">") {
		n, err := strconv.Atoi(strings.TrimSpace(rest[1:]))
		if err != nil {
			return false, err
		}
		return length > n, nil
	}
	if strings.HasPrefix(rest, "<") {
		n, err := strconv.Atoi(strings.TrimSpace(rest[1:]))
		if err != nil {
			return false, err
		}
		return length < n, nil
	}

	return false, fmt.Errorf("unsupported len operator in: %s", expr)
}

// evalEquality handles: inputs.restart_required == true, X == Y
func evalEquality(expr string, inputs, outputs map[string]any) (bool, error) {
	parts := strings.SplitN(expr, "==", 2)
	if len(parts) != 2 {
		return false, nil
	}

	lhs := strings.TrimSpace(parts[0])
	rhs := strings.TrimSpace(parts[1])

	lVal := resolveVar(lhs, inputs, outputs)
	rVal := rhs

	// Compare as strings.
	return fmt.Sprint(lVal) == rVal, nil
}

// evalInequality handles: inputs.restart_policy != 'never', X != Y
func evalInequality(expr string, inputs, outputs map[string]any) (bool, error) {
	parts := strings.SplitN(expr, "!=", 2)
	if len(parts) != 2 {
		return false, nil
	}
	lhs := strings.TrimSpace(parts[0])
	rhs := strings.TrimSpace(parts[1])
	rhs = strings.Trim(rhs, "'\"")
	lVal := resolveVar(lhs, inputs, outputs)
	return fmt.Sprint(lVal) != rhs, nil
}

// resolveVar resolves a variable path from inputs/outputs.
// Supports: "inputs.X", "outputs.X", "X" (searches outputs then inputs).
func resolveVar(path string, inputs, outputs map[string]any) any {
	if strings.HasPrefix(path, "inputs.") {
		key := path[len("inputs."):]
		return inputs[key]
	}
	if strings.HasPrefix(path, "outputs.") {
		key := path[len("outputs."):]
		return outputs[key]
	}
	// Search outputs first, then inputs.
	if v, ok := outputs[path]; ok {
		return v
	}
	if v, ok := inputs[path]; ok {
		return v
	}
	return nil
}

func collectionLength(v any) int {
	if v == nil {
		return 0
	}
	switch c := v.(type) {
	case []any:
		return len(c)
	case []string:
		return len(c)
	case map[string]any:
		return len(c)
	default:
		return 0
	}
}
