// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.list_services
// @awareness file_role=typed_grpc_handler_for_service_registry_consumed_by_xds
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness risk=high
package main

import (
	"context"
	"encoding/json"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListServices returns the merged service-registry view as a list of
// JSON strings. xDS (and any future consumer that needs the
// service-registry shape) MUST call this RPC instead of scanning
// /globular/services/* in etcd directly. Anchored by
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage:
// the cluster_controller is the owner of /globular/services/*
// (writers PutInstance / PutConfig in config/etcd_service_config.go),
// so the handler reads the registry via the same canonical typed
// helper (config.GetServicesConfigurations) that other in-controller
// code uses.
//
// Wire format is JSON-per-service rather than a typed proto message
// because Globular service configs have dynamic schemas — different
// services contribute different keys. A typed proto would either
// bloat or constrain that. Consumers (xDS) json.Unmarshal each string
// into map[string]any and continue as before.
//
// Read-only; no leader-forwarding. The registry is eventually
// consistent across followers via etcd; any controller instance
// returns the same data.
func (srv *server) ListServices(ctx context.Context, _ *cluster_controllerpb.ListServicesRequest) (*cluster_controllerpb.ListServicesResponse, error) {
	if srv == nil {
		return nil, status.Error(codes.FailedPrecondition, "server unavailable")
	}

	services, err := config.GetServicesConfigurations()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list service configurations: %v", err)
	}

	resp := &cluster_controllerpb.ListServicesResponse{
		ServicesJson: make([]string, 0, len(services)),
	}
	for _, svc := range services {
		if svc == nil {
			continue
		}
		data, jErr := json.Marshal(svc)
		if jErr != nil {
			// Skip individual broken entries rather than failing the
			// whole RPC — matches the consumer's prior tolerance for
			// per-entry unmarshal failures.
			continue
		}
		resp.ServicesJson = append(resp.ServicesJson, string(data))
	}
	return resp, nil
}
