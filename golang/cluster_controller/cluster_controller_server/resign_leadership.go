package main

import (
	"context"
	"fmt"
	"log"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// ResignLeadership causes the current leader to resign its etcd election lease,
// allowing a different controller instance to become leader. Used by leader-aware
// deploy workflows to safely update control-plane binaries.
func (srv *server) ResignLeadership(ctx context.Context, req *cluster_controllerpb.ResignLeadershipRequest) (*cluster_controllerpb.ResignLeadershipResponse, error) {
	if !srv.isLeader() {
		return &cluster_controllerpb.ResignLeadershipResponse{
			Ok:      false,
			Message: "this instance is not the leader",
		}, nil
	}

	formerID, _ := srv.leaderID.Load().(string)
	reason := req.GetReason()
	if reason == "" {
		reason = "RPC request"
	}
	log.Printf("leader election: ResignLeadership called (reason=%s, leader_id=%s)", reason, formerID)

	// Signal the leader election goroutine to resign.
	select {
	case srv.resignCh <- struct{}{}:
	default:
		return &cluster_controllerpb.ResignLeadershipResponse{
			Ok:      false,
			Message: "resign already in progress",
		}, nil
	}

	// Wait for the leader flag to actually clear. Do NOT return OK
	// until leadership is confirmed dropped — callers depend on this
	// postcondition to safely proceed with updating the old leader.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !srv.isLeader() {
			log.Printf("leader election: leadership confirmed dropped (former_id=%s)", formerID)
			return &cluster_controllerpb.ResignLeadershipResponse{
				Ok:             true,
				Message:        fmt.Sprintf("leadership resigned (reason: %s)", reason),
				FormerLeaderId: formerID,
			}, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Leadership did not drop within timeout — return failure so the
	// workflow does not proceed with a stale leader still active.
	log.Printf("leader election: resign timeout — leader flag still set after 5s")
	return &cluster_controllerpb.ResignLeadershipResponse{
		Ok:             false,
		Message:        "resign signal sent but leadership not dropped within 5s",
		FormerLeaderId: formerID,
	}, nil
}
