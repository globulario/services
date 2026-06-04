// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.list_services_test
// @awareness file_role=unit_test_for_list_services_typed_rpc
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness risk=medium
package main

import (
	"context"
	"encoding/json"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// TestListServices_DegradesGracefullyWhenRegistryUnavailable confirms
// the handler returns an error (rather than panicking) when the
// service registry cannot be fetched. Operating without an etcd
// connection in the test environment, the underlying
// config.GetServicesConfigurations either returns an error or an
// empty list; either outcome is acceptable as long as the handler
// does not crash.
func TestListServices_DegradesGracefullyWhenRegistryUnavailable(t *testing.T) {
	srv := &server{}
	resp, err := srv.ListServices(context.Background(), &cluster_controllerpb.ListServicesRequest{})
	if err != nil {
		// Acceptable: registry truly unavailable in this scaffold.
		return
	}
	// Acceptable: empty list. We just need to confirm no panic and
	// the response payload, if present, is a slice of valid JSON.
	if resp == nil {
		t.Fatalf("nil response with nil error")
	}
	for i, s := range resp.GetServicesJson() {
		var m map[string]any
		if jerr := json.Unmarshal([]byte(s), &m); jerr != nil {
			t.Errorf("services_json[%d] is not valid JSON: %v\npayload: %s", i, jerr, s)
		}
	}
}
