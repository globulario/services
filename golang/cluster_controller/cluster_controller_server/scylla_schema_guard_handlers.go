// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.scylla_schema_guard_handlers
// @awareness file_role=typed_grpc_handlers_for_scylla_schema_guard_status_and_enforce_request
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness risk=high
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const scyllaSchemaGuardPrefix = "/globular/scylla/schema_guard/"

// GetScyllaSchemaGuardStatus returns the per-keyspace guard status
// the scylla_schema_guard maintains under
// /globular/scylla/schema_guard/<keyspace>. The cluster_controller
// owns that prefix; CLI + future consumers MUST call this RPC
// instead of scanning etcd directly per
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage.
//
// Read-only; no leader-forwarding. Status records are written by
// the guard's background loop (leader-only); followers serve the
// last-known eventually-consistent view.
func (srv *server) GetScyllaSchemaGuardStatus(ctx context.Context, _ *cluster_controllerpb.GetScyllaSchemaGuardStatusRequest) (*cluster_controllerpb.GetScyllaSchemaGuardStatusResponse, error) {
	kv := srv.kv
	if kv == nil && srv.etcdClient != nil {
		kv = srv.etcdClient
	}
	if kv == nil {
		return nil, status.Error(codes.FailedPrecondition, "scylla kv unavailable")
	}

	resp, err := kv.Get(ctx, scyllaSchemaGuardPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read schema guard status: %v", err)
	}

	out := &cluster_controllerpb.GetScyllaSchemaGuardStatusResponse{}
	for _, ev := range resp.Kvs {
		if ev == nil {
			continue
		}
		keyspace := strings.TrimPrefix(string(ev.Key), scyllaSchemaGuardPrefix)
		// Skip marker/sub-key entries (e.g. enforce_request,
		// bootstrap_marker) — only per-keyspace status leaves
		// surface here. Sub-keys with "/" or known control names
		// are filtered.
		if keyspace == "" || strings.Contains(keyspace, "/") {
			continue
		}
		switch keyspace {
		case "enforce_request", "bootstrap_marker":
			continue
		}
		var raw struct {
			CurrentRF     int32  `json:"current_rf"`
			RequiredRF    int32  `json:"required_rf"`
			Violation     bool   `json:"violation"`
			LastError     string `json:"last_error"`
			UpdatedAtUnix int64  `json:"updated_at_unix"`
		}
		if jErr := json.Unmarshal(ev.Value, &raw); jErr != nil {
			continue
		}
		out.Keyspaces = append(out.Keyspaces, &cluster_controllerpb.ScyllaKeyspaceGuardStatus{
			Keyspace:      keyspace,
			CurrentRf:     raw.CurrentRF,
			RequiredRf:    raw.RequiredRF,
			Violation:     raw.Violation,
			LastError:     raw.LastError,
			UpdatedAtUnix: raw.UpdatedAtUnix,
		})
	}
	return out, nil
}

// RequestScyllaSchemaEnforce stamps the enforce-request signal the
// guard polls. Returns the request_unix timestamp; consumers poll
// GetScyllaSchemaGuardStatus and wait for any keyspace's
// updated_at_unix >= request_unix to confirm the run completed.
//
// Leader-forwarded so the enforce signal is stamped by a single
// authoritative actor.
func (srv *server) RequestScyllaSchemaEnforce(ctx context.Context, req *cluster_controllerpb.RequestScyllaSchemaEnforceRequest) (*cluster_controllerpb.RequestScyllaSchemaEnforceResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.RequestScyllaSchemaEnforceResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/RequestScyllaSchemaEnforce", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	kv := srv.kv
	if kv == nil && srv.etcdClient != nil {
		kv = srv.etcdClient
	}
	if kv == nil {
		return nil, status.Error(codes.FailedPrecondition, "scylla kv unavailable")
	}
	ts := time.Now().Unix()
	if _, err := kv.Put(ctx, scyllaSchemaGuardEnforceRequestKey, fmt.Sprintf("%d", ts)); err != nil {
		return nil, status.Errorf(codes.Internal, "write enforce request: %v", err)
	}
	return &cluster_controllerpb.RequestScyllaSchemaEnforceResponse{
		RequestUnix: ts,
	}, nil
}
