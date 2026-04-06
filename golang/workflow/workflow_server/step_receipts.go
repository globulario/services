// step_receipts.go implements durable step completion receipts (MC-1/WH-8).
//
// A receipt is a breadcrumb left by the executor after a step completes
// successfully. During resume, the engine checks for a receipt before
// running verification — if a receipt exists, the step is known-complete
// without re-querying the world.
//
// Receipts are especially valuable for:
//   - Steps where verification is expensive (health probes, etcd scans)
//   - Steps where the world is noisy (partial installs, flapping services)
//   - Steps where the side effect happened but state sync didn't complete
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

// ── Schema ───────────────────────────────────────────────────────────────────

const createStepReceiptsTableCQL = `
CREATE TABLE IF NOT EXISTS workflow.step_receipts (
    run_id      text,
    step_id     text,
    receipt_key text,
    result_json text,
    created_at  bigint,
    PRIMARY KEY (run_id, step_id)
)`

// ── Write ────────────────────────────────────────────────────────────────────

// writeStepReceipt persists a receipt after a step completes successfully.
// Called by the executor's OnStepDone callback when the step has a receipt_key.
// Fire-and-forget: errors are logged but never block execution.
func (srv *server) writeStepReceipt(runID, stepID, receiptKey string, result map[string]any) {
	if srv.session == nil || receiptKey == "" {
		return
	}

	resultJSON := "{}"
	if result != nil {
		if b, err := json.Marshal(result); err == nil {
			resultJSON = string(b)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := srv.session.Query(`
		INSERT INTO workflow.step_receipts (run_id, step_id, receipt_key, result_json, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		runID, stepID, receiptKey, resultJSON, time.Now().UnixMilli(),
	).WithContext(ctx).Exec(); err != nil {
		slog.Warn("step receipt: write failed",
			"run_id", runID, "step_id", stepID, "receipt_key", receiptKey, "err", err)
	}
}

// ── Read ─────────────────────────────────────────────────────────────────────

// readStepReceipt checks if a receipt exists for the given run/step.
// Returns the result JSON and true if found, or empty string and false.
func (srv *server) readStepReceipt(runID, stepID string) (string, bool) {
	if srv.session == nil {
		return "", false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var resultJSON string
	if err := srv.session.Query(`
		SELECT result_json FROM workflow.step_receipts
		WHERE run_id = ? AND step_id = ? LIMIT 1`,
		runID, stepID,
	).WithContext(ctx).Scan(&resultJSON); err != nil {
		return "", false
	}
	return resultJSON, true
}
