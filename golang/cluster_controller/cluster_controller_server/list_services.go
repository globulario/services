// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.list_services
// @awareness file_role=typed_grpc_handler_for_service_registry_consumed_by_xds
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness risk=high
package main

import (
	"context"
	"encoding/json"
	"log"

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
	skipped := 0
	for _, svc := range services {
		if svc == nil {
			skipped++
			continue
		}
		data, jErr := json.Marshal(svc)
		if jErr != nil {
			// Per-entry marshal failure → log with enough context that an
			// operator can find and repair the corrupt registry entry.
			// Previously this was silently dropped, so xDS received a list
			// shorter than reality (interpreted "missing" as "removed" and
			// drained traffic from a healthy service). The proto cannot
			// yet surface SkippedCount on the wire — adding that field is
			// tracked as defense-in-depth. For now: ensure the diagnostic
			// is preserved in logs.
			// Enforces meta.authority_must_express_uncertainty +
			// forbidden.silent_drop_unparseable_authority_entry.
			var id, name string
			if v, ok := svc["Id"].(string); ok {
				id = v
			}
			if v, ok := svc["Name"].(string); ok {
				name = v
			}
			log.Printf("ListServices: skipping corrupt registry entry id=%q name=%q: %v", id, name, jErr)
			skipped++
			continue
		}
		resp.ServicesJson = append(resp.ServicesJson, string(data))
	}
	if skipped > 0 {
		log.Printf("ListServices: returned %d services; %d corrupt entries skipped (consumer cannot distinguish from 'removed' until SkippedCount field is added to proto)",
			len(resp.ServicesJson), skipped)
	}
	return resp, nil
}
