// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.get_routing_refresh_test
// @awareness file_role=unit_test_for_get_routing_refresh_typed_rpc
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness risk=medium
package main

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// TestGetRoutingRefresh_ReturnsLeaderEpoch is the contract test for the
// v1.2.177 typed RPC that replaces xDS's /globular/routing/refresh-generation
// etcd watch. The handler MUST return the leader's atomic.Int64 epoch
// converted to the proto's uint64 carrier.
//
// Anchored by invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage:
// the cluster_controller owns the routing-refresh signal; routing-aware
// consumers (xDS, gateway) read it via this RPC instead of from etcd.
func TestGetRoutingRefresh_ReturnsLeaderEpoch(t *testing.T) {
	srv := &server{}
	srv.leader.Store(true) // act as leader
	srv.leaderEpoch.Store(42)

	resp, err := srv.GetRoutingRefresh(context.Background(), &cluster_controllerpb.GetRoutingRefreshRequest{})
	if err != nil {
		t.Fatalf("GetRoutingRefresh: %v", err)
	}
	if resp.GetEpoch() != 42 {
		t.Errorf("epoch = %d, want 42", resp.GetEpoch())
	}
	if resp.GetTimestamp() == nil {
		t.Errorf("timestamp must be set")
	}
}

// TestGetRoutingRefresh_NegativeEpochClampedToZero guards against a
// transient race where atomic.Int64.Load returns a value before the
// store completes — leaderEpoch is monotonically increasing per its
// invariant (never negative under normal operation), but a defensive
// clamp prevents a uint64 wrap-around if the contract is ever broken.
func TestGetRoutingRefresh_NegativeEpochClampedToZero(t *testing.T) {
	srv := &server{}
	srv.leader.Store(true)
	srv.leaderEpoch.Store(-1) // pathological — should not happen in practice

	resp, err := srv.GetRoutingRefresh(context.Background(), &cluster_controllerpb.GetRoutingRefreshRequest{})
	if err != nil {
		t.Fatalf("GetRoutingRefresh: %v", err)
	}
	if resp.GetEpoch() != 0 {
		t.Errorf("epoch = %d, want 0 (negative clamped)", resp.GetEpoch())
	}
}
