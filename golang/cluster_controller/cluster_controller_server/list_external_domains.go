// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.list_external_domains
// @awareness file_role=typed_grpc_handler_for_external_domain_specs_consumed_by_xds
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness risk=high
package main

import (
	"context"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListExternalDomains returns every external-domain spec the
// controller's embedded domain reconciler manages, paired with the
// reconciliation status. xDS (and other future consumers) MUST call
// this RPC instead of scanning /globular/domains/v1/* in etcd —
// anchored by
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage.
//
// The cluster_controller IS the owner of /globular/domains/v1/*: the
// embedded domain reconciler (services/golang/domain) writes specs +
// statuses via its EtcdDomainStore. Inside this handler we use that
// same typed Store — the store is the owner's canonical typed read
// path, even though it happens to read from etcd.
//
// Read-only; no leader-forwarding needed. Specs are eventually
// consistent across followers via etcd; xDS callers can hit any
// controller instance.
func (srv *server) ListExternalDomains(ctx context.Context, _ *cluster_controllerpb.ListExternalDomainsRequest) (*cluster_controllerpb.ListExternalDomainsResponse, error) {
	if srv.etcdClient == nil {
		return nil, status.Error(codes.FailedPrecondition, "etcd client unavailable")
	}

	store := domain.NewEtcdDomainStore(srv.etcdClient)
	specs, err := store.ListSpecs(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list domain specs: %v", err)
	}

	resp := &cluster_controllerpb.ListExternalDomainsResponse{
		Domains: make([]*cluster_controllerpb.ExternalDomainEntry, 0, len(specs)),
	}
	for _, spec := range specs {
		if spec == nil {
			continue
		}
		entry := &cluster_controllerpb.ExternalDomainEntry{
			Fqdn:           spec.FQDN,
			IngressEnabled: spec.Ingress.Enabled,
			IngressService: spec.Ingress.Service,
			IngressPort:    int32(spec.Ingress.Port),
			AcmeEnabled:    spec.ACME.Enabled,
		}
		// Status is stored under a separate etcd key. Best-effort: a
		// missing status is not a failure — the entry surfaces with
		// status_phase="" and the consumer (xDS) filters those out.
		if st, _, sErr := store.GetStatus(ctx, spec.FQDN); sErr == nil && st != nil {
			entry.StatusPhase = st.Phase
		}
		resp.Domains = append(resp.Domains, entry)
	}
	return resp, nil
}
