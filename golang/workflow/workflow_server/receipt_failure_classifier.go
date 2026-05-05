package main

import (
	"strings"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

type stepReceiptMeta struct {
	Status         string         `json:"status"`
	FailureClass   string         `json:"failure_class,omitempty"`
	ReasonCode     string         `json:"reason_code,omitempty"`
	RetryPolicy    string         `json:"retry_policy,omitempty"`
	AutoRetry      bool           `json:"auto_retry,omitempty"`
	UnblockSignals []string       `json:"unblock_signals,omitempty"`
	Evidence       map[string]any `json:"evidence,omitempty"`
}

func classifyStepFailureForReceipt(stepErr string) (stepReceiptMeta, bool) {
	lower := strings.ToLower(stepErr)

	// Transient failures: retrying by time/backoff is meaningful.
	switch {
	case strings.Contains(lower, "network timeout"),
		strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "deadlineexceeded"),
		strings.Contains(lower, "temporarily unavailable"),
		strings.Contains(lower, "node-agent busy"),
		strings.Contains(lower, "workflow_unavailable"),
		strings.Contains(lower, "workflow_deadline"),
		strings.Contains(lower, "workflow_circuit_open"),
		strings.Contains(lower, "scylla"),
		strings.Contains(lower, "repository unavailable"),
		strings.Contains(lower, "etcd conflict"):
		return stepReceiptMeta{
			Status:       "RETRY_LATER",
			FailureClass: workflowpb.FailureClass_FAILURE_CLASS_NETWORK.String(),
			ReasonCode:   "TRANSIENT_ERROR",
			RetryPolicy:  "BACKOFF",
			AutoRetry:    true,
		}, true
	}

	// Deterministic blocks: retrying by timer is useless.
	switch {
	case strings.Contains(stepErr, "NATIVE_LIBRARY_DEPENDENCY_MISSING"):
		meta := stepReceiptMeta{
			Status:         "BLOCKED",
			FailureClass:   workflowpb.FailureClass_FAILURE_CLASS_DEPENDENCY.String(),
			ReasonCode:     "NATIVE_DEPENDENCY_MISSING",
			RetryPolicy:    "ON_UNBLOCK_SIGNAL",
			AutoRetry:      false,
			UnblockSignals: []string{"native_dependency_present", "operator_resume", "policy_changed"},
		}
		if strings.Contains(stepErr, "libodbc.so.2") {
			meta.Evidence = map[string]any{
				"missing_libraries": []string{"libodbc.so.2"},
				"provider":          "debian:unixodbc",
				"manual_action":     "sudo apt-get install -y unixodbc",
			}
		}
		return meta, true
	case strings.Contains(lower, "missing secret"),
		strings.Contains(lower, "secret not found"):
		return stepReceiptMeta{
			Status:         "BLOCKED",
			FailureClass:   workflowpb.FailureClass_FAILURE_CLASS_CONFIG.String(),
			ReasonCode:     "MISSING_SECRET",
			RetryPolicy:    "ON_UNBLOCK_SIGNAL",
			AutoRetry:      false,
			UnblockSignals: []string{"secret_created", "operator_resume", "generation_changed"},
		}, true
	case strings.Contains(lower, "manual approval required"),
		strings.Contains(lower, "requires manual approval"),
		strings.Contains(lower, "requires operator approval"):
		return stepReceiptMeta{
			Status:         "BLOCKED",
			FailureClass:   workflowpb.FailureClass_FAILURE_CLASS_VALIDATION.String(),
			ReasonCode:     "MANUAL_APPROVAL_REQUIRED",
			RetryPolicy:    "ON_UNBLOCK_SIGNAL",
			AutoRetry:      false,
			UnblockSignals: []string{"operator_resume"},
		}, true
	case strings.Contains(lower, "unsupported platform"):
		return stepReceiptMeta{
			Status:         "BLOCKED",
			FailureClass:   workflowpb.FailureClass_FAILURE_CLASS_PACKAGE.String(),
			ReasonCode:     "UNSUPPORTED_PLATFORM",
			RetryPolicy:    "ON_UNBLOCK_SIGNAL",
			AutoRetry:      false,
			UnblockSignals: []string{"generation_changed", "policy_changed", "operator_resume"},
		}, true
	case strings.Contains(lower, "checksum mismatch"):
		return stepReceiptMeta{
			Status:         "BLOCKED",
			FailureClass:   workflowpb.FailureClass_FAILURE_CLASS_REPOSITORY.String(),
			ReasonCode:     "CHECKSUM_MISMATCH",
			RetryPolicy:    "ON_UNBLOCK_SIGNAL",
			AutoRetry:      false,
			UnblockSignals: []string{"artifact_changed", "operator_resume"},
		}, true
	}

	return stepReceiptMeta{}, false
}

func buildStepReceiptPayload(step *engine.StepState) map[string]any {
	payload := map[string]any{
		"step":   step.ID,
		"status": string(step.Status),
	}
	if step.Output != nil {
		payload["output"] = step.Output
	}
	if step.Error != "" {
		payload["error"] = step.Error
		if meta, ok := classifyStepFailureForReceipt(step.Error); ok {
			payload["status"] = meta.Status
			payload["failure_class"] = meta.FailureClass
			payload["reason_code"] = meta.ReasonCode
			payload["retry_policy"] = meta.RetryPolicy
			payload["auto_retry"] = meta.AutoRetry
			if len(meta.UnblockSignals) > 0 {
				payload["unblock_signals"] = meta.UnblockSignals
			}
			if len(meta.Evidence) > 0 {
				payload["evidence"] = meta.Evidence
			}
		}
	}
	return payload
}

