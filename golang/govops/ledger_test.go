package govops

import (
	"context"
	"testing"

	pb "github.com/globulario/services/golang/govops/governed_operationpb"
)

func entry(id, ts, actor, owner, result string, invariants ...string) *pb.OperationLedgerEntry {
	var r pb.OperationResult
	switch result {
	case "refused":
		r = pb.OperationResult_REFUSED
	case "completed":
		r = pb.OperationResult_COMPLETED
	case "break_glass":
		r = pb.OperationResult_BREAK_GLASS_COMPLETED
	}
	return &pb.OperationLedgerEntry{
		OperationId: id, Timestamp: ts, Actor: actor, TargetOwner: owner,
		AwgInvariants: invariants, Result: r,
	}
}

func ids(es []*pb.OperationLedgerEntry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.GetOperationId()
	}
	return out
}

func TestMemLedgerStore_PutList(t *testing.T) {
	s := NewMemLedgerStore()
	ctx := context.Background()
	_ = s.Put(ctx, entry("a", "2026-06-24T00:00:00Z", "sa", "controller", "completed"))
	_ = s.Put(ctx, entry("b", "2026-06-24T01:00:00Z", "sa", "controller", "refused"))
	got, err := s.List(ctx)
	if err != nil || len(got) != 2 {
		t.Fatalf("List = %v, %v; want 2 entries", ids(got), err)
	}
}

func TestQueryLedger_Filters(t *testing.T) {
	all := []*pb.OperationLedgerEntry{
		entry("a", "2026-06-24T00:00:00Z", "alice", "controller", "completed", "desired.keyed_by_kind_and_name"),
		entry("b", "2026-06-24T02:00:00Z", "bob", "node_agent", "refused", "desired.no_regression_all_paths"),
		entry("c", "2026-06-24T01:00:00Z", "alice", "controller", "break_glass"),
		entry("d", "2026-06-24T03:00:00Z", "alice", "repository", "completed", "desired.keyed_by_kind_and_name"),
	}

	cases := []struct {
		name   string
		filter LedgerFilter
		want   []string
	}{
		{"all (zero filter) newest-first", LedgerFilter{}, []string{"d", "b", "c", "a"}},
		{"by actor", LedgerFilter{Actor: "alice"}, []string{"d", "c", "a"}},
		{"by owner", LedgerFilter{Owner: "controller"}, []string{"c", "a"}},
		{"by invariant", LedgerFilter{Invariant: "desired.keyed_by_kind_and_name"}, []string{"d", "a"}},
		{"only refused", LedgerFilter{OnlyRefused: true}, []string{"b"}},
		{"only break-glass", LedgerFilter{OnlyBreakGlass: true}, []string{"c"}},
		{"by result completed", LedgerFilter{Result: pb.OperationResult_COMPLETED}, []string{"d", "a"}},
		{"by operation id", LedgerFilter{OperationID: "b"}, []string{"b"}},
		{"time window", LedgerFilter{Since: "2026-06-24T01:00:00Z", Until: "2026-06-24T02:00:00Z"}, []string{"b", "c"}},
		{"actor + owner combined", LedgerFilter{Actor: "alice", Owner: "controller"}, []string{"c", "a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ids(QueryLedger(all, tc.filter))
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// An entry with an empty/unparseable timestamp is excluded by a time window but
// included by an unbounded filter (sorted last).
func TestQueryLedger_UnparseableTimestamp(t *testing.T) {
	all := []*pb.OperationLedgerEntry{
		entry("good", "2026-06-24T00:00:00Z", "sa", "controller", "completed"),
		entry("notime", "", "sa", "controller", "completed"),
	}
	if got := ids(QueryLedger(all, LedgerFilter{})); len(got) != 2 || got[0] != "good" || got[1] != "notime" {
		t.Errorf("unbounded: got %v, want [good notime]", got)
	}
	if got := ids(QueryLedger(all, LedgerFilter{Since: "2026-06-24T00:00:00Z"})); len(got) != 1 || got[0] != "good" {
		t.Errorf("windowed: got %v, want [good]", got)
	}
}
