// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.ingress_handlers
// @awareness file_role=typed_grpc_handlers_for_ingress_status_and_republish_request
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

const ingressStatusPrefix = "/globular/ingress/v1/status/"

// GetIngressStatus returns the ingress spec summary and per-node
// ingress status aggregated from the keepalived controller reports.
// Consumers (CLI, gateway, future operators) MUST call this RPC
// instead of reading the keys directly per
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage.
//
// Spec is read via the canonical srv.loadIngressSpec helper.
// Per-node statuses are read by prefix scan; the controller is the
// authoritative aggregator of those reports for cluster-wide
// queries.
//
// Read-only; no leader-forwarding. Status reports are eventually
// consistent across followers via etcd.
func (srv *server) GetIngressStatus(ctx context.Context, _ *cluster_controllerpb.GetIngressStatusRequest) (*cluster_controllerpb.GetIngressStatusResponse, error) {
	resp := &cluster_controllerpb.GetIngressStatusResponse{}

	spec, present, err := srv.loadIngressSpec(ctx)
	if err != nil {
		// Surface read errors but keep going with status — the
		// spec being unreadable does not block per-node visibility.
		spec = nil
	}
	if spec != nil && present {
		resp.SpecPresent = true
		resp.Generation = spec.Generation
		resp.Mode = string(spec.Mode)
		resp.ExplicitDisabled = spec.ExplicitDisabled
		resp.WriterLeaderId = spec.WriterLeaderID
		resp.WrittenAtUnix = spec.WrittenAtUnix
	}

	kv := srv.kv
	if kv == nil && srv.etcdClient != nil {
		kv = srv.etcdClient
	}
	if kv != nil {
		statusResp, sErr := kv.Get(ctx, ingressStatusPrefix, clientv3.WithPrefix())
		if sErr == nil {
			for _, ev := range statusResp.Kvs {
				if ev == nil {
					continue
				}
				nodeID := strings.TrimPrefix(string(ev.Key), ingressStatusPrefix)
				if nodeID == "" {
					continue
				}
				var raw struct {
					Phase     string `json:"phase"`
					VrrpState string `json:"vrrp_state"`
					HasVip    bool   `json:"has_vip"`
					LastError string `json:"last_error"`
				}
				if jErr := json.Unmarshal(ev.Value, &raw); jErr != nil {
					continue
				}
				resp.Nodes = append(resp.Nodes, &cluster_controllerpb.IngressNodeStatus{
					NodeId:    nodeID,
					Phase:     raw.Phase,
					VrrpState: raw.VrrpState,
					HasVip:    raw.HasVip,
					LastError: raw.LastError,
				})
			}
		}
	}
	return resp, nil
}

// RequestIngressRepublish writes the republish-request signal that
// the ingress spec guard watches for. Returns the unix-seconds
// timestamp the controller stamped on the request so consumers can
// poll GetIngressStatus and wait for written_at_unix >= request_unix
// to confirm the republish.
//
// Leader-forwarded so the actor identity reflects the node that
// will service the request.
func (srv *server) RequestIngressRepublish(ctx context.Context, req *cluster_controllerpb.RequestIngressRepublishRequest) (*cluster_controllerpb.RequestIngressRepublishResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.RequestIngressRepublishResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/RequestIngressRepublish", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	kv := srv.kv
	if kv == nil && srv.etcdClient != nil {
		kv = srv.etcdClient
	}
	if kv == nil {
		return nil, status.Error(codes.FailedPrecondition, "ingress kv unavailable")
	}
	ts := time.Now().Unix()
	if _, err := kv.Put(ctx, ingressRepublishRequestKey, fmt.Sprintf("%d", ts)); err != nil {
		return nil, status.Errorf(codes.Internal, "write republish request: %v", err)
	}
	return &cluster_controllerpb.RequestIngressRepublishResponse{
		RequestUnix: ts,
	}, nil
}
