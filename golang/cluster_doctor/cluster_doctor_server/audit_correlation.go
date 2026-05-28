package main

import (
	"context"
	"fmt"
	"time"

	"github.com/globulario/services/golang/remediation"
)

// Re-exported here for backward compatibility with existing callers; the
// canonical constants live in golang/remediation/correlation.go so the
// workflow engine can set them without importing cluster_doctor.
const (
	AuditCorrelationMetadataKey = remediation.CorrelationMetadataKey
	AuditWorkflowRunMetadataKey = remediation.WorkflowRunMetadataKey
)

// correlationIDFromContext extracts the caller-supplied correlation id
// from gRPC metadata, falling back to a deterministic id derived from the
// finding + step so audits remain joinable even when the caller forgot
// to set one.
func correlationIDFromContext(ctx context.Context, findingID string, stepIndex uint32) string {
	if cid := remediation.CorrelationFromContext(ctx); cid != "" {
		return cid
	}
	// Fallback: deterministic id so the same (finding, step) issued without
	// a correlation header still groups together across retries.
	return fmt.Sprintf("corr-%s-%d-%d", findingID, stepIndex, time.Now().UnixNano())
}

// workflowRunIDFromContext extracts the workflow run id when present.
// Returns "" when the call did not originate from a workflow.
func workflowRunIDFromContext(ctx context.Context) string {
	return remediation.WorkflowRunFromContext(ctx)
}
