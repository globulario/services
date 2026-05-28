package remediation

import (
	"context"

	"google.golang.org/grpc/metadata"
)

// gRPC metadata keys used to propagate remediation correlation across
// service boundaries (workflow engine → doctor → node_agent → verifier).
// The doctor's audit writer reads these and stamps every audit record.
// See docs/intent/audit.retention_and_correlation_policy.yaml.
const (
	// CorrelationMetadataKey carries the cross-service correlation id.
	// When absent, the doctor derives a deterministic id from
	// (finding, step) so audits still join.
	CorrelationMetadataKey = "x-globular-correlation-id"

	// WorkflowRunMetadataKey carries the workflow run id when the call
	// originates from a workflow remediation actor. Absent when the
	// doctor is invoked directly (CLI, MCP, etc).
	WorkflowRunMetadataKey = "x-globular-workflow-run-id"
)

// WithCorrelation returns a new context carrying correlationID and
// optionally workflowRunID as outgoing-gRPC metadata. Either can be
// empty; empty values are not set so receivers don't see blank strings
// that look intentional.
func WithCorrelation(ctx context.Context, correlationID, workflowRunID string) context.Context {
	pairs := []string{}
	if correlationID != "" {
		pairs = append(pairs, CorrelationMetadataKey, correlationID)
	}
	if workflowRunID != "" {
		pairs = append(pairs, WorkflowRunMetadataKey, workflowRunID)
	}
	if len(pairs) == 0 {
		return ctx
	}
	// Merge with any existing outgoing metadata so we don't clobber
	// upstream callers' headers.
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		merged := md.Copy()
		for i := 0; i < len(pairs); i += 2 {
			merged.Set(pairs[i], pairs[i+1])
		}
		return metadata.NewOutgoingContext(ctx, merged)
	}
	return metadata.AppendToOutgoingContext(ctx, pairs...)
}

// WithCorrelationAsIncoming is the same as WithCorrelation but for
// in-process callbacks (e.g. workflow engine → doctor actor handler in
// the same process) where the receiver reads incoming-context metadata.
func WithCorrelationAsIncoming(ctx context.Context, correlationID, workflowRunID string) context.Context {
	pairs := []string{}
	if correlationID != "" {
		pairs = append(pairs, CorrelationMetadataKey, correlationID)
	}
	if workflowRunID != "" {
		pairs = append(pairs, WorkflowRunMetadataKey, workflowRunID)
	}
	if len(pairs) == 0 {
		return ctx
	}
	md := metadata.Pairs(pairs...)
	if existing, ok := metadata.FromIncomingContext(ctx); ok {
		md = metadata.Join(existing, md)
	}
	return metadata.NewIncomingContext(ctx, md)
}

// CorrelationFromContext returns the correlation id propagated by either
// the incoming or outgoing gRPC metadata, preferring incoming (the
// receiver's natural read path). Returns "" when absent.
func CorrelationFromContext(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vs := md.Get(CorrelationMetadataKey); len(vs) > 0 {
			return vs[0]
		}
	}
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		if vs := md.Get(CorrelationMetadataKey); len(vs) > 0 {
			return vs[0]
		}
	}
	return ""
}

// WorkflowRunFromContext returns the workflow run id when present.
func WorkflowRunFromContext(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vs := md.Get(WorkflowRunMetadataKey); len(vs) > 0 {
			return vs[0]
		}
	}
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		if vs := md.Get(WorkflowRunMetadataKey); len(vs) > 0 {
			return vs[0]
		}
	}
	return ""
}
