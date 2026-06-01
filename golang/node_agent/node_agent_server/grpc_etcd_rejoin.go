package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

const etcdMemberDir = "/var/lib/globular/etcd/member"
const etcdUnit = "globular-etcd.service"

// runWipeEtcdAndRejoin stops globular-etcd, wipes the member data directory,
// and restarts etcd so it can join the cluster with its new MemberAdd config.
//
// The controller calls this workflow on a node in EtcdJoinRejoinInProgress after
// performing MemberAdd and re-rendering etcd.yaml with the updated initial-cluster.
// The node agent then performs the destructive wipe locally — the controller
// (security constraint) must never use os/exec or direct file operations.
func (srv *NodeAgentServer) runWipeEtcdAndRejoin(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	start := time.Now()
	log.Printf("wipe-etcd-and-rejoin: starting etcd data wipe and rejoin")

	// Step 1: stop etcd so we don't wipe a running process's data.
	stopCtx, stopCancel := context.WithTimeout(ctx, 30*time.Second)
	defer stopCancel()
	if err := supervisor.Stop(stopCtx, etcdUnit); err != nil {
		msg := fmt.Sprintf("wipe-etcd-and-rejoin: failed to stop %s: %v", etcdUnit, err)
		log.Print(msg)
		return &node_agentpb.RunWorkflowResponse{Status: "FAILED", Error: msg}, nil
	}
	log.Printf("wipe-etcd-and-rejoin: stopped %s", etcdUnit)

	// Step 2: wipe the member directory. This contains the WAL and snapshots
	// that record the old (removed) member identity. etcd will re-create it
	// from scratch when it joins the cluster as a new member.
	if err := os.RemoveAll(etcdMemberDir); err != nil {
		msg := fmt.Sprintf("wipe-etcd-and-rejoin: failed to remove %s: %v", etcdMemberDir, err)
		log.Print(msg)
		return &node_agentpb.RunWorkflowResponse{Status: "FAILED", Error: msg}, nil
	}
	log.Printf("wipe-etcd-and-rejoin: wiped %s", etcdMemberDir)

	// Step 3: start etcd with the new initial-cluster config rendered by
	// the controller (reconcileServiceConfigs writes /var/lib/globular/config/etcd.yaml).
	startCtx, startCancel := context.WithTimeout(ctx, 60*time.Second)
	defer startCancel()
	if err := supervisor.Start(startCtx, etcdUnit); err != nil {
		msg := fmt.Sprintf("wipe-etcd-and-rejoin: failed to start %s: %v", etcdUnit, err)
		log.Print(msg)
		return &node_agentpb.RunWorkflowResponse{Status: "FAILED", Error: msg}, nil
	}
	if err := supervisor.WaitActive(startCtx, etcdUnit, 30*time.Second); err != nil {
		msg := fmt.Sprintf("wipe-etcd-and-rejoin: %s did not become active: %v", etcdUnit, err)
		log.Print(msg)
		return &node_agentpb.RunWorkflowResponse{Status: "FAILED", Error: msg}, nil
	}

	elapsed := time.Since(start).Round(time.Millisecond)
	log.Printf("wipe-etcd-and-rejoin: completed successfully in %s", elapsed)
	return &node_agentpb.RunWorkflowResponse{
		Status:         "SUCCEEDED",
		StepsTotal:     3,
		StepsSucceeded: 3,
	}, nil
}
