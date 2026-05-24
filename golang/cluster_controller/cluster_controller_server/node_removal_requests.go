package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const nodeRemovalRequestPrefix = "/globular/controller/node_removals/requests/"

type nodeRemovalRequest struct {
	NodeID string `json:"node_id,omitempty"`
}

// processNodeRemovalRequests drains explicit removal requests written by
// clean-node flows running on target nodes. This guarantees that a node which
// explicitly asks to leave the cluster is removed authoritatively even when
// gateway/CLI auth on the target node is unavailable.
//
// Request format:
//
//	key:   /globular/controller/node_removals/requests/<node_id>
//	value: JSON (optional), may include {"node_id":"..."}
//
// Behavior:
//   - leader-only, idempotent
//   - runs force=true, drain=false removal via controller RPC path
//   - removes processed request keys (including already-not-found nodes)
func (srv *server) processNodeRemovalRequests(ctx context.Context) {
	if srv == nil || srv.etcdClient == nil {
		return
	}

	getCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := srv.etcdClient.Get(getCtx, nodeRemovalRequestPrefix, clientv3.WithPrefix())
	if err != nil {
		log.Printf("node-removal-requests: list failed: %v", err)
		return
	}
	if len(resp.Kvs) == 0 {
		return
	}

	for _, kv := range resp.Kvs {
		handled, rmErr := srv.handleNodeRemovalRequest(ctx, string(kv.Key), kv.Value)
		if !handled {
			continue
		}
		if rmErr != nil {
			log.Printf("node-removal-requests: %v", rmErr)
			continue
		}

		delCtx, delCancel := context.WithTimeout(ctx, 3*time.Second)
		if _, err := srv.etcdClient.Delete(delCtx, string(kv.Key)); err != nil {
			log.Printf("node-removal-requests: delete %q failed: %v", string(kv.Key), err)
		}
		delCancel()
	}
}

// handleNodeRemovalRequest applies one queued node removal request.
// Returns handled=false for malformed requests that should be skipped.
func (srv *server) handleNodeRemovalRequest(ctx context.Context, key string, value []byte) (handled bool, err error) {
	nodeID := strings.TrimPrefix(key, nodeRemovalRequestPrefix)
	if nodeID == "" {
		var req nodeRemovalRequest
		if uerr := json.Unmarshal(value, &req); uerr == nil {
			nodeID = strings.TrimSpace(req.NodeID)
		}
	}
	if nodeID == "" {
		log.Printf("node-removal-requests: skip malformed request key=%q", key)
		return false, nil
	}

	_, rmErr := srv.RemoveNode(ctx, &cluster_controllerpb.RemoveNodeRequest{
		NodeId: nodeID,
		Force:  true,
		Drain:  false,
	})
	if rmErr != nil && status.Code(rmErr) != codes.NotFound {
		return true, rmErr
	}
	if rmErr == nil {
		log.Printf("node-removal-requests: removed node %s from queued request", nodeID)
	} else {
		log.Printf("node-removal-requests: node %s already absent, clearing stale request", nodeID)
	}
	return true, nil
}
