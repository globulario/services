package main

import (
	"context"
	"fmt"
	"testing"

	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
)

// F3: the remediation CAP selects what is remediated this cycle. It must never
// redefine what was OBSERVED.
//
// reconcileClassifyDrift used to truncate `items` at maxRemediations and then
// build the "current scan" ref set from the TRUNCATED slice. clearResolvedDrift
// clears any persisted drift_unresolved row absent from that set, so with 75
// observations and max_remediations=50 rows 51-75 were read as "no longer
// drifting" and CLEARED — unresolved drift falsely marked resolved purely for
// sorting below a cap. The cap keeps highest priority first, so the rows
// destroyed were always the lowest-priority ones, and their
// RecordDriftObservation call was skipped too, resetting consecutive-cycle
// history every cycle so persistent low-priority drift could never reach
// workflow.drift_stuck.

func driftItem(kind, pkg, node string) map[string]any {
	return map[string]any{"type": kind, "package_name": pkg, "node_id": node}
}

// THE critical proof: 75 observed, cap 50 — only 50 selected, but all 75 must
// remain observed so cleanup cannot clear rows 51-75.
func TestClassifyDrift_CapDoesNotShrinkObservedSet(t *testing.T) {
	report := make([]any, 0, 75)
	// Mixed priorities so the cap truncates by priority, as production does.
	for i := 0; i < 75; i++ {
		kind := "unmanaged_package" // lowest priority — first to be cut
		if i < 10 {
			kind = "infra_unhealthy"
		}
		report = append(report, driftItem(kind, fmt.Sprintf("pkg%02d", i), "node-a"))
	}
	srv := &server{}
	got, err := srv.reconcileClassifyDrift(context.Background(), report, 50)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if len(got) != 50 {
		t.Fatalf("cap must select exactly 50; got %d", len(got))
	}

	// Rebuild the observed set exactly as production does, and prove cleanup
	// would NOT clear the 25 unselected rows.
	observed := map[string]map[string]bool{}
	for _, it := range report {
		m := it.(map[string]any)
		d := fmt.Sprint(m["type"])
		if observed[d] == nil {
			observed[d] = map[string]bool{}
		}
		observed[d][driftEntityRef(m)] = true
	}
	persisted := make([]*workflowpb.DriftUnresolved, 0, 75)
	for _, it := range report {
		m := it.(map[string]any)
		persisted = append(persisted, &workflowpb.DriftUnresolved{
			DriftType: fmt.Sprint(m["type"]), EntityRef: driftEntityRef(m),
		})
	}
	cleared := 0
	clearResolvedDriftItems(observed, persisted, func(string, string) { cleared++ })
	if cleared != 0 {
		t.Errorf("cleanup cleared %d still-observed rows; rows 51-75 must survive the cap", cleared)
	}
}

// The capped SELECTION must never be used as the cleanup oracle.
func TestClassifyDrift_CappedSelectionIsNotTheCleanupOracle(t *testing.T) {
	report := []any{}
	for i := 0; i < 10; i++ {
		report = append(report, driftItem("version_drift", fmt.Sprintf("p%d", i), "node-a"))
	}
	persisted := make([]*workflowpb.DriftUnresolved, 0, 10)
	for _, it := range report {
		m := it.(map[string]any)
		persisted = append(persisted, &workflowpb.DriftUnresolved{
			DriftType: "version_drift", EntityRef: driftEntityRef(m),
		})
	}
	// Simulate the OLD behaviour: oracle built from a cap of 3.
	capped := map[string]map[string]bool{"version_drift": {}}
	for i := 0; i < 3; i++ {
		capped["version_drift"][driftEntityRef(report[i].(map[string]any))] = true
	}
	clearedByCapped := 0
	clearResolvedDriftItems(capped, persisted, func(string, string) { clearedByCapped++ })
	if clearedByCapped != 7 {
		t.Fatalf("sanity: a capped oracle should clear the 7 unselected rows; got %d", clearedByCapped)
	}
	// Complete oracle clears nothing.
	full := map[string]map[string]bool{"version_drift": {}}
	for _, it := range report {
		full["version_drift"][driftEntityRef(it.(map[string]any))] = true
	}
	clearedByFull := 0
	clearResolvedDriftItems(full, persisted, func(string, string) { clearedByFull++ })
	if clearedByFull != 0 {
		t.Errorf("complete observed set must clear nothing; got %d", clearedByFull)
	}
}

// A genuinely absent row must still clear — the fix must not disable cleanup.
func TestClassifyDrift_GenuinelyAbsentRowStillClears(t *testing.T) {
	observed := map[string]map[string]bool{"version_drift": {"still@node-a": true}}
	persisted := []*workflowpb.DriftUnresolved{
		{DriftType: "version_drift", EntityRef: "still@node-a"},
		{DriftType: "version_drift", EntityRef: "gone@node-a"},
	}
	var clearedRefs []string
	clearResolvedDriftItems(observed, persisted, func(_, ref string) { clearedRefs = append(clearedRefs, ref) })
	if len(clearedRefs) != 1 || clearedRefs[0] != "gone@node-a" {
		t.Errorf("only the genuinely absent row may clear; cleared %v", clearedRefs)
	}
}

// Structural ratchet: the cleanup oracle must be the complete observed set.
func TestClassifyDrift_CleanupUsesObservedNotSelected(t *testing.T) {
	src := readSourceFile(t, "reconcile_actions.go")
	if !containsAll(src, "observedAll := make(map[string]map[string]bool)",
		"go srv.clearResolvedDrift(context.Background(), observedAll)") {
		t.Error("clearResolvedDrift must be passed the COMPLETE observed set (observedAll)")
	}
	// The cap must not rebind `items` — that is what silently shrank the oracle.
	if containsAll(src, "items = items[:maxRemediations]") {
		t.Error("cap must not truncate `items` in place; select into a separate slice " +
			"so the observed set cannot be narrowed by the cap")
	}
}
