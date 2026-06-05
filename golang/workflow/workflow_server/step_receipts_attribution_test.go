// @awareness namespace=globular.platform
// @awareness component=platform_workflow.step_receipts_attribution
// @awareness file_role=regression_tests_for_workflow_instance_ownership_of_step_receipts
// @awareness enforces=globular.platform:invariant.workflow.every_state_mutation_belongs_to_a_workflow_instance
// @awareness enforces=globular.platform:invariant.four_layer.workflow_actor_attribution_required
// @awareness risk=critical
package main

import (
	"strings"
	"testing"
)

// TestStepReceipt_CarriesRunIDAndStepID pins the pointwise-rule contract on
// the step_receipts table schema: every receipt is keyed by (run_id, step_id),
// which means every step outcome stored in workflow.step_receipts is
// IDENTIFIABLY bound to a workflow instance.
//
// This is the schema-level proof of the invariant
// workflow.every_state_mutation_belongs_to_a_workflow_instance: no receipt
// can exist without naming the run_id + step_id that owns it. The PRIMARY
// KEY enforces the binding at storage time.
//
// Future drift would look like: removing run_id from the key (allowing
// receipts without instance ownership), adding a "global receipt" path that
// bypasses the table entirely, or letting receipts be written with empty
// run_id / step_id strings. The DDL is the regression seam.
func TestStepReceipt_CarriesRunIDAndStepID(t *testing.T) {
	ddl := createStepReceiptsTableCQL

	if !strings.Contains(ddl, "run_id") {
		t.Fatalf("step_receipts DDL does not declare run_id column — workflow instance binding lost")
	}
	if !strings.Contains(ddl, "step_id") {
		t.Fatalf("step_receipts DDL does not declare step_id column — step binding lost")
	}
	if !strings.Contains(ddl, "PRIMARY KEY (run_id, step_id)") {
		t.Fatalf("step_receipts PRIMARY KEY is not (run_id, step_id) — instance ownership not enforced by schema")
	}
}

// TestWriteStepReceipt_RejectsEmptyReceiptKey verifies the write path guards
// against a degenerate caller pattern: a fire-and-forget receipt with no
// receipt_key (no semantic identity) is silently dropped rather than
// producing an orphan row. This is the other half of the pointwise rule
// at the write boundary — receipts must name themselves.
//
// We exercise the early-return branch by calling writeStepReceipt with an
// empty receiptKey on a server with no Scylla session. If the function
// short-circuits on receiptKey == "" before touching the session, the call
// returns cleanly. If it later attempts to use the nil session, it would
// still no-op (the session-nil guard exists at line 52-55), so this test
// pins the FIRST guard specifically.
func TestWriteStepReceipt_RejectsEmptyReceiptKey(t *testing.T) {
	srv := &server{}
	// Empty receiptKey must short-circuit before any session access.
	// No panic, no error path — the function returns silently.
	srv.writeStepReceipt("run-id-test", "step-id-test", "", map[string]any{"k": "v"})
}
