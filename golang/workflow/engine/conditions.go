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
//   - true / false                 — literal booleans
func DefaultEvalCond(ctx context.Context, expr string, inputs, outputs map[string]any) (bool, error) {
	expr = strings.TrimSpace(expr)

	// Literal booleans.
	if expr == "true" {
		return true, nil
	}
	if expr == "false" {
		return false, nil
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

	// Unknown expression — default to true (pass-through).
	return true, nil
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
func evalLen(expr string, inputs, outputs map[string]any) (bool, error) {
	// Parse: len(VARNAME) OP NUMBER
	closeParen := strings.Index(expr, ")")
	if closeParen < 0 {
		return false, fmt.Errorf("malformed len expression: %s", expr)
	}

	varName := strings.TrimSpace(expr[len("len("):closeParen])
	rest := strings.TrimSpace(expr[closeParen+1:])

	// Resolve variable.
	val := resolveVar(varName, inputs, outputs)
	length := collectionLength(val)

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
