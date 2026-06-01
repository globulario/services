// correlation_defer_state_rpc.go — WF-DEFER B3/B4 RPCs.
//
// B3:
//   ListCorrelationDeferState   — read counters (doctor / admin UI)
//   ClearCorrelationDeferState  — operator reset
//
// B4:
//   WakeDeferredRunsByBlockerTag — event-driven cooldown shortcut
//
// Cooldown remains the floor (B2). Wake just collapses backoff_until_ms
// to "now" so the next dispatch attempt proceeds without waiting out
// the rest of the window. defer_count is preserved.
// @awareness namespace=globular.platform
// @awareness component=platform_workflow.server
// @awareness file_role=workflow_correlation_defer_rpc_handler
// @awareness implements=globular.platform:intent.workflow.terminal_runs_must_be_bounded
// @awareness risk=high
package main

import (
	"context"
	"fmt"
	"time"

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

// WakeDeferredRunsByBlockerTag finds non-abandoned correlations whose
// last_blocker_tags contains the given tag and clears the cooldown
// floor on each one's active deferred run. Idempotent: callable
// repeatedly with no extra side-effects.
//
// What this does NOT do:
//   - Touch defer_count (B3 budget continues to count down).
//   - Wake abandoned correlations (operator must clear first).
//   - Restart the run itself — the next ExecuteWorkflow caller picks
//     it back up. We just remove the "still in cooldown" reason to
//     skip dispatch.
func (srv *server) WakeDeferredRunsByBlockerTag(ctx context.Context, req *workflowpb.WakeDeferredRunsByBlockerTagRequest) (*workflowpb.WakeDeferredRunsByBlockerTagResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req == nil || req.ClusterId == "" || req.BlockerTag == "" {
		return nil, fmt.Errorf("cluster_id and blocker_tag are required")
	}
	source := req.Source
	if source == "" {
		source = "unknown"
	}
	resp := &workflowpb.WakeDeferredRunsByBlockerTagResponse{}
	if srv.deferStore == nil {
		return resp, nil
	}
	waker, ok := srv.deferStore.(wakingDeferStateStore)
	if !ok {
		return nil, fmt.Errorf("defer store does not support wake-by-tag")
	}
	rows, err := waker.FindByBlockerTag(ctx, req.ClusterId, req.BlockerTag)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for _, r := range rows {
		runID, woke, werr := srv.wakeActiveDeferredRun(ctx, req.ClusterId, r.CorrelationID, now)
		if werr != nil {
			logger.Warn("executor: wake-by-tag failed for correlation",
				"correlation_id", r.CorrelationID, "blocker_tag", req.BlockerTag, "err", werr)
			continue
		}
		if !woke {
			// No active deferred run — counter row exists but the run
			// already advanced past cooldown. Nothing to do; skip.
			continue
		}
		resp.Woken++
		resp.CorrelationIds = append(resp.CorrelationIds, r.CorrelationID)
		logger.Info("executor: woke deferred correlation by blocker tag",
			"correlation_id", r.CorrelationID,
			"blocker_tag", req.BlockerTag,
			"run_id", runID,
			"source", source,
		)
		publishWorkflowEvent("workflow.run_woken", map[string]interface{}{
			"correlation_id": r.CorrelationID,
			"blocker_tag":    req.BlockerTag,
			"run_id":         runID,
			"source":         source,
			"defer_count":    r.DeferCount,
			"max_defers":     r.MaxDefers,
		})
	}
	return resp, nil
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
