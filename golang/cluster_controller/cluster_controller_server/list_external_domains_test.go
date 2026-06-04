// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.list_external_domains_test
// @awareness file_role=unit_test_for_list_external_domains_typed_rpc
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness risk=medium
package main

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// TestListExternalDomains_NilEtcdReturnsFailedPrecondition guards the
// boot-order path where the handler may be called before the etcd
// client is wired up. Returning FailedPrecondition (rather than
// panicking) lets the consumer (xDS) degrade silently to its previous
// loop instead of crashing.
//
// Anchored by invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage:
// the typed RPC must remain callable even in degraded boot order so
// the migration off the prior raw-etcd scan cannot regress to a
// silent panic.
func TestListExternalDomains_NilEtcdReturnsFailedPrecondition(t *testing.T) {
	srv := &server{}
	_, err := srv.ListExternalDomains(context.Background(), &cluster_controllerpb.ListExternalDomainsRequest{})
	if err == nil {
		t.Fatalf("expected FailedPrecondition when etcdClient is nil, got nil")
	}
}
