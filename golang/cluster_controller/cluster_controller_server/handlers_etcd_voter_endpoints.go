// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.etcd_members
// @awareness file_role=controller_rpc_serving_authoritative_etcd_voter_endpoints_to_joining_nodes
// @awareness implements=globular.platform:intent.etcd.is_source_of_truth
// @awareness risk=high
package main

import (
	"context"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetEtcdVoterEndpoints returns the authoritative client endpoints (https://IP:2379)
// of the current healthy, NON-learner (voting) etcd members.
//
// A joining node whose own local etcd member is a non-voting learner during
// buildout calls this over the trusted controller endpoint (from its join plan) to
// discover a voting endpoint to use as its desired-state authority — it must never
// depend on MemberList from its own learner (which refuses client RPCs), and must
// never guess endpoints. If no voter can be resolved this returns an error so the
// caller stays degraded/pending with a precise reason rather than fabricating a
// list (contract: controller-issued config is the owner path, not guessed).
func (srv *server) GetEtcdVoterEndpoints(ctx context.Context, req *cluster_controllerpb.GetEtcdVoterEndpointsRequest) (*cluster_controllerpb.GetEtcdVoterEndpointsResponse, error) {
	if srv.etcdMembers == nil {
		return nil, status.Errorf(codes.Unavailable, "etcd member manager not ready")
	}
	eps, err := srv.etcdMembers.voterClientEndpoints(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "resolve etcd voter endpoints: %v", err)
	}
	if len(eps) == 0 {
		return nil, status.Errorf(codes.Unavailable, "no healthy etcd voter endpoints available")
	}
	return &cluster_controllerpb.GetEtcdVoterEndpointsResponse{Endpoints: eps}, nil
}
