package main

import (
	"strings"
	"testing"

	"github.com/globulario/services/golang/workflow/engine"
)

func TestClassifyStepFailureForReceipt_NativeDependencyMissing(t *testing.T) {
	meta, ok := classifyStepFailureForReceipt("NATIVE_LIBRARY_DEPENDENCY_MISSING: libodbc.so.2 not found")
	if !ok {
		t.Fatal("expected classification")
	}
	if meta.Status != "BLOCKED" {
		t.Fatalf("status=%q, want BLOCKED", meta.Status)
	}
	if meta.ReasonCode != "NATIVE_DEPENDENCY_MISSING" {
		t.Fatalf("reason=%q", meta.ReasonCode)
	}
	if meta.AutoRetry {
		t.Fatal("auto_retry must be false for deterministic block")
	}
}

func TestClassifyStepFailureForReceipt_TransientTimeout(t *testing.T) {
	meta, ok := classifyStepFailureForReceipt("network timeout while contacting repository")
	if !ok {
		t.Fatal("expected classification")
	}
	if meta.Status != "RETRY_LATER" {
		t.Fatalf("status=%q, want RETRY_LATER", meta.Status)
	}
	if !meta.AutoRetry {
		t.Fatal("auto_retry must be true for transient error")
	}
}

func TestBuildStepReceiptPayload_BlockedFields(t *testing.T) {
	step := &engine.StepState{
		ID:     "install_package",
		Status: engine.StepFailed,
		Error:  "NATIVE_LIBRARY_DEPENDENCY_MISSING: libodbc.so.2 not found",
	}
	payload := buildStepReceiptPayload(step)
	if payload["status"] != "BLOCKED" {
		t.Fatalf("payload status=%v, want BLOCKED", payload["status"])
	}
	if payload["reason_code"] != "NATIVE_DEPENDENCY_MISSING" {
		t.Fatalf("reason_code=%v", payload["reason_code"])
	}
	if payload["retry_policy"] != "ON_UNBLOCK_SIGNAL" {
		t.Fatalf("retry_policy=%v", payload["retry_policy"])
	}
}

// Pins workflow.receipt_classification_monotonic_and_conservative —
// rule 1 (monotonic): a SUCCEEDED step with no error must NOT be
// reclassified by buildStepReceiptPayload. The classifier branch
// only fires when step.Error is non-empty; this test guards against
// a future change that runs the classifier unconditionally and
// retroactively flips a success into a failure shape (e.g. via a
// late retry observation or orphan resume).
func TestReceiptClassificationMonotonic_SucceededNeverReclassified(t *testing.T) {
	step := &engine.StepState{
		ID:     "install_package",
		Status: engine.StepSucceeded,
		Error:  "",
	}
	payload := buildStepReceiptPayload(step)

	if payload["status"] != string(engine.StepSucceeded) {
		t.Fatalf("payload status=%v, want %v — SUCCEEDED must not be reclassified",
			payload["status"], string(engine.StepSucceeded))
	}
	for _, k := range []string{"failure_class", "reason_code", "retry_policy", "auto_retry", "unblock_signals", "evidence"} {
		if _, ok := payload[k]; ok {
			t.Errorf("succeeded step received %q=%v — classifier ran on a non-failed step",
				k, payload[k])
		}
	}
}

// Pins workflow.receipt_classification_monotonic_and_conservative —
// rule 2 (conservative): an unknown/ambiguous error must never be
// classified as terminal success. The classifier's contract is
// failure-side only: if it cannot map the error to a known
// transient or blocked shape, it returns ok=false and the caller
// keeps the raw status. Any classified result MUST carry a retry
// hint (auto_retry=true OR a non-empty unblock_signals list) — a
// classified-but-silent block would be a permanent abandonment.
func TestReceiptClassificationConservative_UnknownIsRetryNotSuccess(t *testing.T) {
	unknownErrors := []string{
		"some weird error nobody mapped",
		"ENOENT: file vanished",
		"0xDEADBEEF",
		"panic: runtime error",
		"i/o error 17",
		"",
	}
	for _, e := range unknownErrors {
		meta, ok := classifyStepFailureForReceipt(e)
		if !ok {
			// Conservative response: classifier refuses to claim
			// certainty about an unknown error. This is correct.
			continue
		}
		if meta.Status == "SUCCEEDED" || strings.EqualFold(meta.Status, "succeeded") {
			t.Errorf("error %q classified as terminal SUCCEEDED — violates conservative rule",
				e)
		}
		if !meta.AutoRetry && len(meta.UnblockSignals) == 0 {
			t.Errorf("error %q classified without any retry path (auto_retry=false and no unblock_signals) — would silently abandon the run",
				e)
		}
	}

	// Direct invariant: any branch added to the classifier in the
	// future must surface either a known transient (BACKOFF) or a
	// known block (ON_UNBLOCK_SIGNAL). Iterate the known mapped
	// errors to ensure every classified shape carries a non-empty
	// RetryPolicy — defending against a "silent default" branch
	// being added without an unblock contract.
	knownErrors := []string{
		"network timeout",
		"connection refused",
		"NATIVE_LIBRARY_DEPENDENCY_MISSING: x",
		"missing secret",
		"manual approval required",
		"unsupported platform",
		"checksum mismatch",
	}
	for _, e := range knownErrors {
		meta, ok := classifyStepFailureForReceipt(e)
		if !ok {
			t.Errorf("known error %q failed to classify — regression in classifier coverage", e)
			continue
		}
		if meta.RetryPolicy == "" {
			t.Errorf("error %q classified with empty RetryPolicy — every classified shape must declare its retry policy",
				e)
		}
		if meta.Status == "SUCCEEDED" {
			t.Errorf("error %q classified as SUCCEEDED — classifier is failure-side only", e)
		}
	}
}

