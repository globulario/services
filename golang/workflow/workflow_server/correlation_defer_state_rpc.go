// correlation_defer_state_rpc.go — WF-DEFER B3 read/clear RPCs.
//
// ListCorrelationDeferState surfaces the persistent defer counters so
// cluster_doctor can emit findings, the admin UI can show abandoned
// correlations, and operators can answer "why isn't this re-trying?".
//
// ClearCorrelationDeferState lets an operator reset a row after they've
// addressed (or accepted) the underlying blocker. Resets defer_count
// to 0 and abandoned to false; recorded with the operator's identity.
package main

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/workflow/workflowpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ListCorrelationDeferState returns the persistent across-runs defer
// state, optionally filtered to abandoned-only (the doctor's view).
func (srv *server) ListCorrelationDeferState(ctx context.Context, req *workflowpb.ListCorrelationDeferStateRequest) (*workflowpb.ListCorrelationDeferStateResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req == nil || req.ClusterId == "" {
		return nil, fmt.Errorf("cluster_id is required")
	}
	if srv.deferStore == nil {
		return &workflowpb.ListCorrelationDeferStateResponse{}, nil
	}
	lister, ok := srv.deferStore.(listingDeferStateStore)
	if !ok {
		return nil, fmt.Errorf("defer store does not support listing")
	}
	rows, err := lister.listAll(ctx, req.ClusterId, req.AbandonedOnly)
	if err != nil {
		return nil, err
	}
	out := &workflowpb.ListCorrelationDeferStateResponse{}
	for _, r := range rows {
		out.Records = append(out.Records, correlationDeferStateToProto(r))
	}
	return out, nil
}

// ClearCorrelationDeferState resets the row for one correlation_id.
// Idempotent: clearing a non-existent row is a no-op success.
func (srv *server) ClearCorrelationDeferState(ctx context.Context, req *workflowpb.ClearCorrelationDeferStateRequest) (*workflowpb.ClearCorrelationDeferStateResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req == nil || req.ClusterId == "" || req.CorrelationId == "" {
		return nil, fmt.Errorf("cluster_id and correlation_id are required")
	}
	if srv.deferStore == nil {
		return &workflowpb.ClearCorrelationDeferStateResponse{Cleared: false}, nil
	}
	operator := req.Operator
	if operator == "" {
		operator = "unknown"
	}
	if err := srv.deferStore.ClearByOperator(ctx, req.ClusterId, req.CorrelationId, operator); err != nil {
		return nil, err
	}
	publishWorkflowEvent("workflow.correlation.cleared", map[string]interface{}{
		"correlation_id": req.CorrelationId,
		"operator":       operator,
	})
	return &workflowpb.ClearCorrelationDeferStateResponse{Cleared: true}, nil
}

// correlationDeferStateToProto maps the internal struct to the wire type.
func correlationDeferStateToProto(s *CorrelationDeferState) *workflowpb.CorrelationDeferStateRecord {
	if s == nil {
		return nil
	}
	out := &workflowpb.CorrelationDeferStateRecord{
		ClusterId:        s.ClusterID,
		CorrelationId:    s.CorrelationID,
		DeferCount:       int32(s.DeferCount),
		MaxDefers:        int32(s.MaxDefers),
		LastStepId:       s.LastStepID,
		LastReason:       s.LastReason,
		LastBlockerTags:  append([]string(nil), s.LastBlockerTags...),
		LastDeferUntilMs: s.LastDeferUntilMs,
		Abandoned:        s.Abandoned,
		ClearedBy:        s.ClearedBy,
	}
	if !s.AbandonedAt.IsZero() {
		out.AbandonedAt = timestamppb.New(s.AbandonedAt)
	}
	if !s.ClearedAt.IsZero() {
		out.ClearedAt = timestamppb.New(s.ClearedAt)
	}
	if !s.UpdatedAt.IsZero() {
		out.UpdatedAt = timestamppb.New(s.UpdatedAt)
	}
	return out
}
