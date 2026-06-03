// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.delete_approval
// @awareness file_role=shared_tombstone_helpers_for_audited_critical_key_deletion
// @awareness enforces=globular.platform:invariant.critical_state.deletion_requires_audited_intent
// @awareness enforces=globular.platform:invariant.destructive_actions.require_explicit_guard
// @awareness risk=critical
package main

// delete_approval.go — shared delete-approval tombstone helpers.
//
// Critical key deletion requires an audited intent marker written to an etcd
// approval prefix before the key is deleted. Guards read the approval prefix
// to decide whether to restore a missing key or honour the operator's intent.
//
// This file contains only pure validation functions (testable without etcd)
// and the server method that queries etcd. Each domain guard (ingress,
// objectstore, PKI) uses these shared helpers and its own approval prefix.
//
// Invariant: critical_state.deletion_requires_audited_intent

import (
	"context"
	"encoding/json"
	"log"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	// approvalMaxAge is the maximum age of a delete-approval record that a
	// guard will accept. Records older than 24 hours are treated as stale
	// and the key is restored unconditionally.
	approvalMaxAge = int64(86400) // 24 h
)

// criticalKeyDeleteApproval is the payload written to a delete-approval key.
// All domain guards share this schema. The ingress guard uses the equivalent
// ingressDeleteApproval struct defined in ingress_spec_guard.go; new domains
// use this shared form.
type criticalKeyDeleteApproval struct {
	Generation     int64  `json:"generation"`
	ActorIdentity  string `json:"actor_identity"`
	Reason         string `json:"reason"`
	ApprovedAtUnix int64  `json:"approved_at_unix"`
}

// isValidDeleteApproval returns true when the record is well-formed (non-empty
// actor and reason) and fresh (approved within maxAge seconds of now).
// Pure function — testable without a real etcd client.
func isValidDeleteApproval(a criticalKeyDeleteApproval, now, maxAge int64) bool {
	if a.ActorIdentity == "" || a.Reason == "" {
		return false
	}
	age := now - a.ApprovedAtUnix
	return age >= 0 && age <= maxAge
}

// hasDeleteApprovalFromKVs returns true when at least one of the raw JSON
// values is a valid delete approval as of now. Used directly in tests and
// called by hasDeleteApproval after querying etcd.
func hasDeleteApprovalFromKVs(kvValues [][]byte, now int64) bool {
	for _, data := range kvValues {
		var a criticalKeyDeleteApproval
		if err := json.Unmarshal(data, &a); err != nil {
			continue
		}
		if isValidDeleteApproval(a, now, approvalMaxAge) {
			return true
		}
	}
	return false
}

// hasDeleteApproval queries etcd for approval keys under approvalPrefix and
// returns true if at least one valid (non-stale, well-formed) approval exists.
// Uses the server's kv client or etcdClient as a fallback.
func (srv *server) hasDeleteApproval(ctx context.Context, approvalPrefix string) bool {
	kv := srv.kv
	if kv == nil {
		kv = srv.etcdClient
	}
	if kv == nil {
		return false
	}
	rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	resp, err := kv.Get(rctx, approvalPrefix, clientv3.WithPrefix())
	if err != nil {
		log.Printf("delete-approval-guard: query %s failed: %v", approvalPrefix, err)
		return false
	}
	if len(resp.Kvs) == 0 {
		return false
	}
	var values [][]byte
	for _, kv := range resp.Kvs {
		values = append(values, kv.Value)
	}
	return hasDeleteApprovalFromKVs(values, time.Now().Unix())
}
