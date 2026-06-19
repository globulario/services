package main

import (
	"context"
	"testing"
)

// TestReconcileClassifyDrift_EmptyReturnsNonNilSlice locks the contract that a
// clean cluster (no drift) yields an INITIALIZED empty remediation_items slice,
// never nil. remediation_items crosses the actor boundary into the reconcile
// workflow's `when` guards (len(remediation_items) == 0 / > 0). A nil slice
// marshals to JSON null and resolves back as nil, which evalLen treats as
// length -1 (fail-closed) — so short_circuit_clean never finalizes and the
// dispatch_remediations foreach fails on a nil collection, emitting
// workflow.run.failed every reconcile tick (the storm that drove the
// ai_executor token drain). An empty []any{} marshals to [] -> length 0.
func TestReconcileClassifyDrift_EmptyReturnsNonNilSlice(t *testing.T) {
	srv := &server{}
	ctx := context.Background()

	for _, tc := range []struct {
		name        string
		driftReport []any
	}{
		{"nil drift report", nil},
		{"empty drift report", []any{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := srv.reconcileClassifyDrift(ctx, tc.driftReport, 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("remediation_items must be an initialized []any{}, got nil " +
					"(nil marshals to JSON null -> evalLen length -1 -> guard storm)")
			}
			if len(got) != 0 {
				t.Fatalf("expected 0 remediation items for no drift, got %d", len(got))
			}
		})
	}
}
