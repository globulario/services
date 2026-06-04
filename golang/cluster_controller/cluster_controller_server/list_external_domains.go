// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.list_external_domains
// @awareness file_role=typed_grpc_handler_for_external_domain_specs_consumed_by_xds
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness risk=high
package main

import (
	"context"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/dnsprovider"
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

// CreateExternalDomain writes a new external-domain spec via the
// controller's embedded domain.EtcdDomainStore — the owner's typed
// path. Replaces the prior CLI putting raw JSON into
// /globular/domains/v1/{fqdn} via etcdClient.Put.
//
// Validation: the request fields are mapped into a
// domain.ExternalDomainSpec and validated with spec.Validate()
// before persistence. The status field is seeded with a "Pending"
// phase; the reconciler will overwrite it on the next pass.
func (srv *server) CreateExternalDomain(ctx context.Context, req *cluster_controllerpb.CreateExternalDomainRequest) (*cluster_controllerpb.CreateExternalDomainResponse, error) {
	if srv.etcdClient == nil {
		return nil, status.Error(codes.FailedPrecondition, "etcd client unavailable")
	}
	if req.GetFqdn() == "" {
		return nil, status.Error(codes.InvalidArgument, "fqdn is required")
	}

	spec := &domain.ExternalDomainSpec{
		FQDN:            req.GetFqdn(),
		Zone:            req.GetZone(),
		NodeID:          req.GetNodeId(),
		TargetIP:        req.GetTargetIp(),
		ProviderRef:     req.GetProviderRef(),
		TTL:             int(req.GetTtl()),
		PublishExternal: req.GetPublishExternal(),
		UseWildcardCert: req.GetUseWildcardCert(),
	}
	if a := req.GetAcme(); a != nil {
		spec.ACME = domain.ACMEConfig{
			Enabled:       a.GetEnabled(),
			ChallengeType: a.GetChallengeType(),
			Email:         a.GetEmail(),
			Directory:     a.GetDirectory(),
		}
	}
	if i := req.GetIngress(); i != nil {
		spec.Ingress = domain.IngressConfig{
			Enabled: i.GetEnabled(),
			Service: i.GetService(),
			Port:    int(i.GetPort()),
		}
	}
	spec.Status = domain.ExternalDomainStatus{
		Phase:   "Pending",
		Message: "Awaiting reconciliation",
	}

	if err := spec.Validate(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid domain spec: %v", err)
	}

	store := domain.NewEtcdDomainStore(srv.etcdClient)
	if err := store.PutSpec(ctx, spec); err != nil {
		return nil, status.Errorf(codes.Internal, "save spec: %v", err)
	}
	return &cluster_controllerpb.CreateExternalDomainResponse{}, nil
}

// DeleteExternalDomain removes an external-domain spec via the
// owner's typed store. Replaces the prior CLI etcdClient.Delete.
func (srv *server) DeleteExternalDomain(ctx context.Context, req *cluster_controllerpb.DeleteExternalDomainRequest) (*cluster_controllerpb.DeleteExternalDomainResponse, error) {
	if srv.etcdClient == nil {
		return nil, status.Error(codes.FailedPrecondition, "etcd client unavailable")
	}
	fqdn := req.GetFqdn()
	if fqdn == "" {
		return nil, status.Error(codes.InvalidArgument, "fqdn is required")
	}
	store := domain.NewEtcdDomainStore(srv.etcdClient)
	if err := store.DeleteSpec(ctx, fqdn); err != nil {
		return nil, status.Errorf(codes.Internal, "delete spec: %v", err)
	}
	return &cluster_controllerpb.DeleteExternalDomainResponse{}, nil
}

// CreateDNSProvider writes a DNS-provider config via the controller's
// embedded domain store — the owner's typed surface. Replaces the
// prior CLI raw JSON Put into /globular/providers/v1/{name}.
func (srv *server) CreateDNSProvider(ctx context.Context, req *cluster_controllerpb.CreateDNSProviderRequest) (*cluster_controllerpb.CreateDNSProviderResponse, error) {
	if srv.etcdClient == nil {
		return nil, status.Error(codes.FailedPrecondition, "etcd client unavailable")
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.GetType() == "" {
		return nil, status.Error(codes.InvalidArgument, "type is required")
	}
	cfg := &dnsprovider.Config{
		Type:        req.GetType(),
		Zone:        req.GetZone(),
		DefaultTTL:  int(req.GetDefaultTtl()),
		Credentials: req.GetCredentials(),
	}
	store := domain.NewEtcdDomainStore(srv.etcdClient)
	if err := store.PutProviderConfig(ctx, req.GetName(), cfg); err != nil {
		return nil, status.Errorf(codes.Internal, "save provider config: %v", err)
	}
	return &cluster_controllerpb.CreateDNSProviderResponse{}, nil
}

// ListDNSProviders returns the projected provider entries with
// credential VALUES intentionally redacted — only the key count is
// surfaced. Replaces the prior CLI etcd prefix scan.
func (srv *server) ListDNSProviders(ctx context.Context, _ *cluster_controllerpb.ListDNSProvidersRequest) (*cluster_controllerpb.ListDNSProvidersResponse, error) {
	if srv.etcdClient == nil {
		return nil, status.Error(codes.FailedPrecondition, "etcd client unavailable")
	}
	store := domain.NewEtcdDomainStore(srv.etcdClient)
	named, err := store.ListNamedProviderConfigs(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list provider configs: %v", err)
	}
	resp := &cluster_controllerpb.ListDNSProvidersResponse{
		Providers: make([]*cluster_controllerpb.DNSProviderEntry, 0, len(named)),
	}
	for _, n := range named {
		if n.Config == nil {
			continue
		}
		resp.Providers = append(resp.Providers, &cluster_controllerpb.DNSProviderEntry{
			Name:               n.Name,
			Type:               n.Config.Type,
			Zone:               n.Config.Zone,
			DefaultTtl:         int32(n.Config.DefaultTTL),
			CredentialKeyCount: int32(len(n.Config.Credentials)),
		})
	}
	return resp, nil
}
