package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

// scyllaHostIDKey is the installed_build_ids key used to propagate the local
// ScyllaDB host UUID to the cluster controller via heartbeat. The controller
// stores it in nodeState so it is available when RemoveNode is called.
const scyllaHostIDKey = "scylla:host_id"

// readScyllaHostID runs "nodetool info" and returns the host UUID reported by
// the local ScyllaDB node. Returns ("", nil) if ScyllaDB is not running or the
// ID line is absent — callers must treat an empty return as "unavailable" and
// not as an error.
func readScyllaHostID(ctx context.Context) string {
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(tctx, "nodetool", "info").Output()
	if err != nil {
		return "" // ScyllaDB not running or nodetool not installed — silent
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// "ID                     : a2d8693e-9877-4f29-9f15-2f744917ea5b"
		if strings.HasPrefix(strings.ToUpper(line), "ID ") || strings.HasPrefix(line, "ID\t") || strings.HasPrefix(line, "ID :") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				id := strings.TrimSpace(parts[1])
				if id != "" {
					return id
				}
			}
		}
	}
	return ""
}

// runScyllaRemoveNode executes "nodetool removenode <host_id>" on this node,
// which must be a healthy ScyllaDB peer. This is called by the cluster
// controller on a healthy node when removing a dead node from the ring.
//
// The dead node's ScyllaDB host UUID is passed via inputs["host_id"].
// If the UUID is missing or invalid, the operation fails immediately.
func (srv *NodeAgentServer) runScyllaRemoveNode(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	inputs := req.GetInputs()
	hostID := strings.TrimSpace(inputs["host_id"])
	if hostID == "" {
		msg := "scylla-remove-node: host_id input is required"
		log.Print(msg)
		return &node_agentpb.RunWorkflowResponse{Status: "FAILED", Error: msg}, nil
	}
	// Basic UUID format guard — prevents shell-injection-style misuse even though
	// exec.Command does not invoke a shell.
	for _, c := range hostID {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == '-') {
			msg := fmt.Sprintf("scylla-remove-node: host_id %q contains invalid characters", hostID)
			log.Print(msg)
			return &node_agentpb.RunWorkflowResponse{Status: "FAILED", Error: msg}, nil
		}
	}

	log.Printf("scylla-remove-node: running nodetool removenode %s", hostID)
	tctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(tctx, "nodetool", "removenode", hostID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := fmt.Sprintf("scylla-remove-node: nodetool removenode %s failed: %v — output: %s", hostID, err, strings.TrimSpace(string(out)))
		log.Print(msg)
		return &node_agentpb.RunWorkflowResponse{Status: "FAILED", Error: msg}, nil
	}
	log.Printf("scylla-remove-node: nodetool removenode %s succeeded: %s", hostID, strings.TrimSpace(string(out)))
	return &node_agentpb.RunWorkflowResponse{Status: "SUCCEEDED"}, nil
}
