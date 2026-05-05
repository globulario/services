package main

import (
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

