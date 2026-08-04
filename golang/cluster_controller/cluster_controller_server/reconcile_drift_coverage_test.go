package main

import (
	"testing"

	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
)

// F3b: cleanup must be restricted to the coverage the scan actually had.
//
// clearResolvedDrift lists drift_unresolved CLUSTER-WIDE. With a node-subset
// scan (include_nodes=[node-a]) no observation exists for node-b — not because
// node-b is healthy, but because nobody looked. Clearing on that absence
// converts ignorance into "resolved" and silently discards real drift.

func unresolved(kind, ref string) *workflowpb.DriftUnresolved {
	return &workflowpb.DriftUnresolved{DriftType: kind, EntityRef: ref}
}

func clearedBy(t *testing.T, observed map[string]map[string]bool, coverage map[string]bool,
	items []*workflowpb.DriftUnresolved) []string {
	t.Helper()
	var got []string
	clearResolvedDriftItems(observed, coverage, items, func(_, ref string) { got = append(got, ref) })
	return got
}

// A node-A-only scan may resolve node-A rows and must not touch node-B.
func TestDriftCoverage_PartialScanClearsOnlyCoveredNode(t *testing.T) {
	observed := map[string]map[string]bool{"version_drift": {"kept@node-a": true}}
	coverage := map[string]bool{"node-a": true}
	items := []*workflowpb.DriftUnresolved{
		unresolved("version_drift", "kept@node-a"),  // still observed -> keep
		unresolved("version_drift", "gone@node-a"),  // absent, IN coverage -> clear
		unresolved("version_drift", "other@node-b"), // absent, OUTSIDE coverage -> keep
	}
	got := clearedBy(t, observed, coverage, items)
	if len(got) != 1 || got[0] != "gone@node-a" {
		t.Errorf("partial scan must clear only covered-node absences; cleared %v", got)
	}
}

// Cluster-scoped rows carry no node and must survive any partial scan.
func TestDriftCoverage_ClusterScopedRowsSurvivePartialScan(t *testing.T) {
	observed := map[string]map[string]bool{}
	coverage := map[string]bool{"node-a": true}
	items := []*workflowpb.DriftUnresolved{
		unresolved("infra_unhealthy", "etcd"),     // cluster/package scoped, no node
		unresolved("version_drift", "pkg@node-a"), // node-scoped, in coverage
	}
	got := clearedBy(t, observed, coverage, items)
	if len(got) != 1 || got[0] != "pkg@node-a" {
		t.Errorf("cluster-scoped rows must survive a partial scan; cleared %v", got)
	}
}

// A full cluster scan (empty coverage) retains the complete-scan semantics.
func TestDriftCoverage_FullScanClearsAnyAbsentRow(t *testing.T) {
	observed := map[string]map[string]bool{"version_drift": {"kept@node-a": true}}
	items := []*workflowpb.DriftUnresolved{
		unresolved("version_drift", "kept@node-a"),
		unresolved("version_drift", "gone@node-b"),
		unresolved("infra_unhealthy", "etcd"),
	}
	got := clearedBy(t, observed, nil, items)
	if len(got) != 2 {
		t.Errorf("a full scan must still clear genuinely absent rows; cleared %v", got)
	}
}

// History for an excluded node must be untouched, so workflow.drift_stuck can
// still advance once that node is scanned again.
func TestDriftCoverage_ExcludedNodeHistoryPreserved(t *testing.T) {
	observed := map[string]map[string]bool{}
	coverage := map[string]bool{"node-a": true}
	items := []*workflowpb.DriftUnresolved{unresolved("missing_package", "svc@node-b")}
	if got := clearedBy(t, observed, coverage, items); len(got) != 0 {
		t.Errorf("an excluded node's row must be fully preserved so drift_stuck progress survives; cleared %v", got)
	}
}

// coverageNodeSet: empty means full coverage; entries normalize.
func TestDriftCoverage_NodeSetNormalization(t *testing.T) {
	if coverageNodeSet(nil) != nil || coverageNodeSet([]any{}) != nil {
		t.Error("empty coverage must mean FULL scan (nil set)")
	}
	got := coverageNodeSet([]any{"node-a", " node-b ", ""})
	if len(got) != 2 || !got["node-a"] || !got["node-b"] {
		t.Errorf("coverage set normalization wrong: %v", got)
	}
}

func TestDriftCoverage_NodeOfEntityRef(t *testing.T) {
	for ref, want := range map[string]string{
		"pkg@node-a": "node-a", "etcd": "", "node-a": "", "a@b@node-c": "node-c",
	} {
		if got := nodeOfEntityRef(ref); got != want {
			t.Errorf("nodeOfEntityRef(%q) = %q, want %q", ref, got, want)
		}
	}
}
