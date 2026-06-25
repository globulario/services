// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.acc_config_handlers
// @awareness file_role=typed_grpc_mutation_gate_for_acc_config
// @awareness implements=globular.platform:intent.controller.leader_election_gates_all_writes
// @awareness risk=high
package main

// acc_config_handlers.go — typed gRPC handlers for SetAccConfig / ResetAccConfig.
//
// The adaptive-concurrency-control (ACC) tuning blob lives at
// /globular/system/acc/config and is owned by the cluster-controller (system
// config). It used to be written/cleared by `globular cluster acc set|reset`
// with a raw clientv3 Put/Delete straight to etcd — an owner-owned write that
// bypassed the controller, its leader gate, and the interceptor audit chain
// (RT-2 direct-write surface). These handlers move that mutation onto the
// owner's typed RPC: leader-gated (controller.leader_election_gates_all_writes,
// meta.competing_writers_must_converge_or_be_fenced), written through the
// governed critical-write seam (config.PutRuntimeWithClass /
// DeleteRuntimeWithClass), and audited by the auth→RBAC→audit interceptor chain
// via the (globular.auth.authz) annotation on the proto.
//
// The config blob itself is opaque to the controller: the CLI marshals the ACC
// struct and ships the JSON bytes; the controller validates only that the
// payload is well-formed JSON before committing it. Interceptors apply the ACC
// config; the controller never interprets the fields.

import (
	"context"
	"encoding/json"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// accConfigKey is the etcd location of the ACC tuning blob. It mirrors the
// constant the CLI used for its former raw write; the controller is now the
// single writer of this key.
const accConfigKey = "/globular/system/acc/config"

// SetAccConfig commits the ACC tuning blob to /globular/system/acc/config.
// Leader-gated (controller.leader_election_gates_all_writes); the payload must
// be well-formed JSON; the write goes through the critical-write seam so a
// failure is propagated rather than silently dropped.
func (srv *server) SetAccConfig(ctx context.Context, req *cluster_controllerpb.SetAccConfigRequest) (*cluster_controllerpb.SetAccConfigResponse, error) {
	cfg := req.GetConfigJson()
	if len(cfg) == 0 {
		return nil, status.Error(codes.InvalidArgument, "config_json is required")
	}
	if !json.Valid(cfg) {
		return nil, status.Error(codes.InvalidArgument, "config_json is not valid JSON")
	}
	if !srv.isLeader() {
		resp := &cluster_controllerpb.SetAccConfigResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/SetAccConfig", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if err := config.PutRuntimeWithClass(ctx, accConfigKey, cfg, config.CriticalWrite); err != nil {
		return nil, status.Errorf(codes.Internal, "write acc config: %v", err)
	}
	return &cluster_controllerpb.SetAccConfigResponse{Ok: true}, nil
}

// ResetAccConfig removes /globular/system/acc/config so interceptors revert to
// their compiled defaults. Leader-gated; deletes through the critical-write
// seam. Reports whether a key was actually removed (deleted=false when the key
// was already absent — a no-op reset is not an error, and the explicit bool
// keeps the absent-vs-removed distinction visible to the caller).
func (srv *server) ResetAccConfig(ctx context.Context, req *cluster_controllerpb.ResetAccConfigRequest) (*cluster_controllerpb.ResetAccConfigResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.ResetAccConfigResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/ResetAccConfig", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	deleted, err := config.DeleteRuntimeWithClass(ctx, accConfigKey, config.CriticalWrite)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "delete acc config: %v", err)
	}
	return &cluster_controllerpb.ResetAccConfigResponse{Deleted: deleted}, nil
}
