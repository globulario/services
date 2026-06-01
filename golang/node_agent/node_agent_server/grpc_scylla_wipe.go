package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/user"
	"strconv"
	"time"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

const scyllaDataDir = "/var/lib/scylla/data"
const scyllaUnit = "scylla-server.service"

// runWipeScyllaData stops ScyllaDB, wipes the Raft data directory, and restarts.
//
// ScyllaDB 2025.3+ uses Raft topology. When multiple nodes try to join
// concurrently, a node can get stuck in "join cluster" state — its local
// Raft group ID doesn't match the cluster's. CQL port 9042 never comes up.
// A simple restart doesn't help because the stale Raft state persists on disk.
//
// The controller calls this after a restart alone failed to unstick the join.
// The wipe removes /var/lib/scylla/data (including system/raft_*), allowing
// ScyllaDB to re-bootstrap into the existing cluster's Raft group.
func (srv *NodeAgentServer) runWipeScyllaData(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	start := time.Now()
	log.Printf("wipe-scylla-data: starting ScyllaDB data wipe for Raft re-bootstrap")

	// Step 1: stop ScyllaDB.
	stopCtx, stopCancel := context.WithTimeout(ctx, 30*time.Second)
	defer stopCancel()
	if err := supervisor.Stop(stopCtx, scyllaUnit); err != nil {
		msg := fmt.Sprintf("wipe-scylla-data: failed to stop %s: %v", scyllaUnit, err)
		log.Print(msg)
		return &node_agentpb.RunWorkflowResponse{Status: "FAILED", Error: msg}, nil
	}
	log.Printf("wipe-scylla-data: stopped %s", scyllaUnit)

	// Step 2: wipe the data directory. This contains the Raft group state
	// (system/raft_*) that prevents re-joining. ScyllaDB will re-create it
	// from scratch when it bootstraps into the cluster's Raft group.
	for _, dir := range []string{
		"/var/lib/scylla/data",
		"/var/lib/scylla/commitlog",
		"/var/lib/scylla/hints",
		"/var/lib/scylla/view_hints",
	} {
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("wipe-scylla-data: warning: failed to remove %s: %v", dir, err)
		}
	}
	// Re-create required directories with correct ownership.
	for _, dir := range []string{"/var/lib/scylla/data", "/var/lib/scylla/commitlog"} {
		os.MkdirAll(dir, 0755)
		// chown to scylla user — best effort.
		chownToUser(dir, "scylla")
	}
	log.Printf("wipe-scylla-data: wiped ScyllaDB data directories")

	// Step 3: restart ScyllaDB for a clean Raft bootstrap.
	startCtx, startCancel := context.WithTimeout(ctx, 60*time.Second)
	defer startCancel()
	if err := supervisor.Start(startCtx, scyllaUnit); err != nil {
		msg := fmt.Sprintf("wipe-scylla-data: failed to start %s: %v", scyllaUnit, err)
		log.Print(msg)
		return &node_agentpb.RunWorkflowResponse{Status: "FAILED", Error: msg}, nil
	}
	log.Printf("wipe-scylla-data: started %s (Raft re-bootstrap in progress)", scyllaUnit)

	elapsed := time.Since(start)
	log.Printf("wipe-scylla-data: completed in %s", elapsed.Round(time.Millisecond))
	return &node_agentpb.RunWorkflowResponse{
		Status: "SUCCEEDED",
	}, nil
}

// chownToUser sets ownership of a path to the given user (best-effort).
func chownToUser(path, username string) {
	u, err := user.Lookup(username)
	if err != nil {
		log.Printf("wipe-scylla-data: user %q not found, skipping chown: %v", username, err)
		return
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	os.Chown(path, uid, gid)
}
