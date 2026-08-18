package engine

import (
	"context"
	"testing"
)

// `when: expr: len(x) > 0` is how workflow steps gate on "did the previous step
// find anything". The answer must depend on how many items the collection
// holds, never on the concrete Go type the handler happened to return.
//
// collectionLength used to type-switch on []any / []string / map[string]any and
// return 0 for everything else. Handlers returning a concretely-typed slice —
// []map[string]any is the common shape — were therefore indistinguishable from
// empty ones, and their guards failed closed AND silently.
//
// Real consequence: invariantValidateNodeReachability returns `critical` as
// []map[string]any, so fence_unreachable_nodes never ran. A node stopped for
// 17 minutes was classified CRITICAL (quorum_at_risk=true in the step's own
// log) and still never fenced, because the guard between the two steps could
// not see the list. The neighbouring MinIO step worked only because its
// violations happened to be []any.

func TestCollectionLengthCountsTypedSlices(t *testing.T) {
	twoEntries := []map[string]any{
		{"node_id": "a", "severity": "CRITICAL"},
		{"node_id": "b", "severity": "CRITICAL"},
	}

	if got := collectionLength(twoEntries); got != 2 {
		t.Fatalf("collectionLength([]map[string]any{2 items}) = %d, want 2 — "+
			"a guard like len(report.critical) > 0 would never fire", got)
	}
	if got := collectionLength([]map[string]any{}); got != 0 {
		t.Errorf("empty typed slice should be 0, got %d", got)
	}
}

func TestCollectionLengthHandlesTheShapesHandlersReturn(t *testing.T) {
	cases := []struct {
		name string
		val  any
		want int
	}{
		{"nil", nil, 0},
		{"[]any", []any{1, 2, 3}, 3},
		{"[]string", []string{"a"}, 1},
		{"map[string]any", map[string]any{"k": 1, "j": 2}, 2},
		{"[]map[string]any", []map[string]any{{"a": 1}}, 1},
		{"[]int", []int{1, 2, 3, 4}, 4},
		{"[]struct", []struct{ N int }{{1}, {2}}, 2},
		{"map[string]string", map[string]string{"a": "b"}, 1},
		{"string", "abcd", 4},
		{"scalar has no length", 42, 0},
		{"bool has no length", true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := collectionLength(tc.val); got != tc.want {
				t.Fatalf("collectionLength(%v) = %d, want %d", tc.val, got, tc.want)
			}
		})
	}
}

// The end-to-end shape of the guard that was broken.
func TestLenGuardFiresOnTypedCriticalList(t *testing.T) {
	outputs := map[string]any{
		"reachability_report": map[string]any{
			"critical": []map[string]any{
				{"node_id": "node-4"},
				{"node_id": "node-5"},
			},
			"quorum_at_risk": true,
		},
	}

	ok, err := DefaultEvalCond(context.Background(), "len(reachability_report.critical) > 0", map[string]any{}, outputs)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !ok {
		t.Fatal("fence guard did not fire with 2 CRITICAL nodes — this is exactly " +
			"the condition that left partitioned nodes unfenced")
	}

	// And it must still be false when nothing is critical.
	empty := map[string]any{
		"reachability_report": map[string]any{"critical": []map[string]any{}},
	}
	ok, err = DefaultEvalCond(context.Background(), "len(reachability_report.critical) > 0", map[string]any{}, empty)
	if err != nil {
		t.Fatalf("evaluate empty: %v", err)
	}
	if ok {
		t.Fatal("fence guard fired with an empty critical list — healthy nodes would be fenced")
	}
}
