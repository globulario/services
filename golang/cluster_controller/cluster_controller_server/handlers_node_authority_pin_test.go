// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.handlers_node_authority_pin
// @awareness file_role=architectural_pin_test_for_handlers_node_resourcestore_routing
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness enforces=globular.platform:invariant.etcd.path_has_single_owner
// @awareness risk=high
package main

import (
	"os"
	"regexp"
	"testing"
)

// Architectural pin for the v1.2.168 refactor of cleanNodeFromReleases.
//
// Before v1.2.168 the controller bypassed its own srv.resources
// abstraction for ServiceRelease / ApplicationRelease /
// InfrastructureRelease and used raw etcd Get + Put against
// /globular/resources/. The fix routed all 4-layer L2 reads and writes
// through srv.resources.List / srv.resources.Apply.
//
// This test fails loudly if a future contributor reintroduces a direct
// etcd Get / Put / Delete against /globular/resources/<Kind>/ from
// handlers_node.go. The principle is anchored in
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage and
// forbidden_fix:read_owned_etcd_prefix_directly_instead_of_calling_owner_rpc.
//
// "I am the owner of this prefix" is NOT a justification for raw etcd
// access — the owner's typed store applies type, version, and audit
// contracts that the raw client skips. The safe state is "the function
// goes through srv.resources", not "the function uses srv.etcdClient
// against its own prefix".
func TestHandlersNode_NoDirectEtcdAgainstResourcesPrefix(t *testing.T) {
	body, err := os.ReadFile("handlers_node.go")
	if err != nil {
		t.Fatalf("read handlers_node.go: %v", err)
	}

	// Match any `.Get(`, `.Put(`, or `.Delete(` whose second arg is a
	// quoted /globular/resources/... path. Catches both srv.etcdClient
	// and any future renamed wrapper around the raw client.
	re := regexp.MustCompile(`\.(Get|Put|Delete)\(\s*[^,)]+,\s*"/globular/resources/`)
	if loc := re.FindIndex(body); loc != nil {
		match := re.FindSubmatch(body)
		t.Errorf("CRITICAL handlers_node.go contains a direct etcd %s against /globular/resources/* "+
			"(near byte offset %d) — violates invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage "+
			"and forbidden_fix:read_owned_etcd_prefix_directly_instead_of_calling_owner_rpc. "+
			"The controller must route reads and writes of its OWN owned release prefix through "+
			"srv.resources.List / srv.resources.Apply so the resource store applies type, version, "+
			"and audit contracts. The v1.2.168 cleanNodeFromReleases refactor removed the last "+
			"instance — see that function for the pattern.",
			string(match[1]), loc[0])
	}
}
