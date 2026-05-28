package main

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/metadata"
)

// AuditCorrelationMetadataKey is the gRPC metadata key callers (workflow,
// CLI) set to propagate a correlation id through the doctor remediation
// surface. The doctor reads it; if absent, a new id is minted so every
// audit record carries a non-empty correlation id. See
// docs/intent/audit.retention_and_correlation_policy.yaml.
const AuditCorrelationMetadataKey = "x-globular-correlation-id"

// AuditWorkflowRunMetadataKey carries the workflow run id when the doctor
// is invoked from a workflow remediation actor. Empty when the doctor was
// called directly (CLI, MCP).
const AuditWorkflowRunMetadataKey = "x-globular-workflow-run-id"

// correlationIDFromContext extracts the caller-supplied correlation id
// from gRPC metadata, falling back to a deterministic id derived from the
// finding + step so audits remain joinable even when the caller forgot
// to set one.
func correlationIDFromContext(ctx context.Context, findingID string, stepIndex uint32) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get(AuditCorrelationMetadataKey); len(vals) > 0 && vals[0] != "" {
			return vals[0]
		}
	}
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		if vals := md.Get(AuditCorrelationMetadataKey); len(vals) > 0 && vals[0] != "" {
			return vals[0]
		}
	}
	// Fallback: deterministic id so the same (finding, step) issued without
	// a correlation header still groups together across retries.
	return fmt.Sprintf("corr-%s-%d-%d", findingID, stepIndex, time.Now().UnixNano())
}

// workflowRunIDFromContext extracts the workflow run id when present.
// Returns "" when the call did not originate from a workflow.
func workflowRunIDFromContext(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get(AuditWorkflowRunMetadataKey); len(vals) > 0 {
			return vals[0]
		}
	}
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		if vals := md.Get(AuditWorkflowRunMetadataKey); len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}
