// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.node_removal_requests
// @awareness file_role=processes_explicit_removal_requests_from_clean_node_flows_with_audit_trail
// @awareness enforces=globular.platform:invariant.destructive_actions.require_explicit_guard
// @awareness risk=critical
package main

// node_removal_requests.go — drains explicit removal requests
// written under /globular/controller/node_removals/requests/.
// These requests are the SAFE channel for a node that wants to
// leave the cluster when its local CLI/gateway auth is unavailable
// (e.g. mid-clean-node).
//
// Every removal MUST be backed by an explicit request record — the
// controller MUST NOT infer "remove this node" from absence,
// heartbeat staleness, or a failed health probe. That asymmetry is
// what prevents a network partition or a slow disk from cascading
// into a destructive cluster-shrink. See also etcd_stale_member.go
// in cluster_doctor/rules, which surfaces stale-membership as a
// finding but never auto-evicts.

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

// enqueueNodeRemovalRequest writes an explicit removal request record to the
// queue (drained by processNodeRemovalRequests, which dispatches the node.remove
// workflow). This is the sanctioned trigger for the scylla-join FSM to roll back
// a FAILED FRESH-JOIN CANDIDATE: the destructive mutation stays owned by the
// node.remove workflow, never done inline
// (invariant:workflow.every_state_mutation_belongs_to_a_workflow_instance).
//
// EXPLICIT GUARD (invariant:destructive_actions.require_explicit_guard): callers
// MUST only enqueue for a node that was never a verified ScyllaDB member
// (ScyllaWasEverVerified==false). The "controller MUST NOT infer removal from a
// failed probe" rule protects established members from transient-failure
// cluster-shrink; a never-verified candidate is not a member, so its cleanup is
// out of that rule's scope. Writing an explicit request record here satisfies
// intent:delete_requires_explicit_intent_marker. Idempotent: re-writing the same
// node's key is safe (the consumer is idempotent and deletes the key).
func (srv *server) enqueueNodeRemovalRequest(ctx context.Context, nodeID, hostname, ip, agentEndpoint string) error {
	if srv == nil || srv.etcdClient == nil {
		return fmt.Errorf("enqueue node removal: no etcd client")
	}
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return fmt.Errorf("enqueue node removal: node_id is required")
	}
	payload, err := json.Marshal(nodeRemovalRequest{
		NodeID:        nodeID,
		Hostname:      strings.TrimSpace(hostname),
		IP:            strings.TrimSpace(ip),
		AgentEndpoint: strings.TrimSpace(agentEndpoint),
	})
	if err != nil {
		return fmt.Errorf("enqueue node removal: marshal: %w", err)
	}
	putCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := srv.etcdClient.Put(putCtx, nodeRemovalRequestPrefix+nodeID, string(payload)); err != nil {
		return fmt.Errorf("enqueue node removal: put %q: %w", nodeID, err)
	}
	log.Printf("node-removal-requests: enqueued removal request for node %s (source: scylla fresh-join rollback)", nodeID)
	return nil
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
