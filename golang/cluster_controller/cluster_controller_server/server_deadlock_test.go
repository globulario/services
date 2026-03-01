package main

import (
	"context"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

func TestReportNodeStatusPanicDoesNotLockMutex(t *testing.T) {
	state := newControllerState()
	state.Nodes["n1"] = &nodeState{NodeID: "n1"}
	srv := newServer(defaultClusterControllerConfig(), "", "", state, nil)

	testHookBeforeReportNodeStatusApply = func() { panic("boom") }
	defer func() { testHookBeforeReportNodeStatusApply = nil }()

	done := make(chan struct{})
	go func() {
		defer func() {
			_ = recover()
			close(done)
		}()
		_, _ = srv.ReportNodeStatus(context.Background(), &cluster_controllerpb.ReportNodeStatusRequest{
			Status: &cluster_controllerpb.NodeStatus{NodeId: "n1"},
		})
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("ReportNodeStatus panic did not return in time")
	}

	lockCh := make(chan struct{})
	go func() {
		srv.lock("test")
		srv.unlock()
		close(lockCh)
	}()

	select {
	case <-lockCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("mutex remained locked after panic")
	}
}
