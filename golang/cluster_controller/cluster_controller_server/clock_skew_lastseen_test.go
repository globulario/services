package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestReportNodeStatus_LastSeenUsesServerClockNotNodeClock is the ratchet for
// meta.clock_skew_invalidates_cross_node_time_comparison. node.LastSeen drives
// every time.Since(node.LastSeen) heartbeat-staleness decision in the controller
// (leader_liveness, posture, stale_instance_purger, release_pipeline, ...). It
// MUST be stamped with the controller's receipt clock, never the node's
// self-reported time — otherwise a node with a skewed clock is judged stale by
// the skew amount and could be marked down. ReportedAt keeps the node's value
// for diagnostics.
func TestReportNodeStatus_LastSeenUsesServerClockNotNodeClock(t *testing.T) {
	state := newControllerState()
	state.Nodes["abc"] = &nodeState{NodeID: "abc"}
	statePath := filepath.Join(t.TempDir(), "state.json")
	srv := newServer(defaultClusterControllerConfig(), "", statePath, state, nil)
	srv.setLeader(true, "test", "127.0.0.1:1234") // ReportNodeStatus requires leadership

	// The node reports a wildly-skewed clock: one hour in the past.
	skewed := time.Now().Add(-time.Hour)
	_, err := srv.ReportNodeStatus(context.Background(), &cluster_controllerpb.ReportNodeStatusRequest{
		Status: &cluster_controllerpb.NodeStatus{
			NodeId:     "abc",
			ReportedAt: timestamppb.New(skewed),
		},
	})
	if err != nil {
		t.Fatalf("ReportNodeStatus: %v", err)
	}

	node := srv.state.Nodes["abc"]
	if node == nil {
		t.Fatal("node abc missing after ReportNodeStatus")
	}
	// LastSeen must be the controller receipt clock (~now), NOT the node's report.
	if age := time.Since(node.LastSeen); age > 5*time.Second {
		t.Errorf("node.LastSeen is %s old — it adopted the node's skewed clock instead of the "+
			"controller receipt clock; clock skew would wrongly mark this node stale "+
			"(meta.clock_skew_invalidates_cross_node_time_comparison)", age.Truncate(time.Second))
	}
	// ReportedAt must preserve the node's self-reported (skewed) value for diagnostics.
	if diff := node.ReportedAt.Sub(skewed); diff > time.Second || diff < -time.Second {
		t.Errorf("node.ReportedAt = %v, want the node's reported %v preserved for diagnostics",
			node.ReportedAt, skewed)
	}
}
