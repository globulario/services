// @awareness namespace=globular.platform
// @awareness component=platform_controller.join_lifecycle
// @awareness file_role=node_removal_request_processing
// @awareness implements=globular.platform:intent.delete_requires_explicit_intent_marker
// @awareness risk=high
package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	NodeID        string `json:"node_id,omitempty"`
	Hostname      string `json:"hostname,omitempty"`
	IP            string `json:"ip,omitempty"`
	AgentEndpoint string `json:"agent_endpoint,omitempty"`
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
// Returns handled=true for all parsed requests (including malformed payloads)
// so the queue entry is consumed and cannot wedge reconcile forever.
func (srv *server) handleNodeRemovalRequest(ctx context.Context, key string, value []byte) (handled bool, err error) {
	req := nodeRemovalRequest{NodeID: strings.TrimSpace(strings.TrimPrefix(key, nodeRemovalRequestPrefix))}
	if len(value) > 0 {
		var payload nodeRemovalRequest
		if uerr := json.Unmarshal(value, &payload); uerr == nil {
			if req.NodeID == "" {
				req.NodeID = strings.TrimSpace(payload.NodeID)
			}
			req.Hostname = strings.TrimSpace(payload.Hostname)
			req.IP = strings.TrimSpace(payload.IP)
			req.AgentEndpoint = strings.TrimSpace(payload.AgentEndpoint)
		}
	}

	nodeID, rerr := srv.resolveNodeRemovalTarget(req)
	if rerr != nil {
		return true, rerr
	}
	if nodeID == "" {
		log.Printf("node-removal-requests: consumed malformed request key=%q", key)
		return true, nil
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

func (srv *server) resolveNodeRemovalTarget(req nodeRemovalRequest) (string, error) {
	if srv == nil {
		return "", nil
	}
	// Priority 1: exact node_id match.
	if req.NodeID != "" {
		srv.lock("resolve-node-removal:id")
		_, ok := srv.state.Nodes[req.NodeID]
		srv.unlock()
		if ok {
			return req.NodeID, nil
		}
		// Fall through to selector-based resolution if possible. If no selector
		// resolves, return original node_id so RemoveNode emits NotFound and the
		// stale request is still consumed.
	}

	selector := strings.ToLower(strings.TrimSpace(req.Hostname))
	matches := make([]string, 0, 1)
	srv.lock("resolve-node-removal:selectors")
	for id, n := range srv.state.Nodes {
		if n == nil {
			continue
		}
		if selector != "" && strings.EqualFold(strings.TrimSpace(n.Identity.Hostname), selector) {
			matches = append(matches, id)
			continue
		}
		if req.IP != "" {
			for _, ip := range n.Identity.Ips {
				if strings.TrimSpace(ip) == req.IP {
					matches = append(matches, id)
					goto nextNode
				}
			}
		}
		if req.AgentEndpoint != "" && strings.TrimSpace(n.AgentEndpoint) == req.AgentEndpoint {
			matches = append(matches, id)
		}
	nextNode:
	}
	srv.unlock()

	switch len(matches) {
	case 0:
		if req.NodeID != "" {
			return req.NodeID, nil
		}
		return "", nil
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("node-removal-requests: ambiguous target hostname=%q ip=%q endpoint=%q matches=%v",
			req.Hostname, req.IP, req.AgentEndpoint, matches)
	}
}
