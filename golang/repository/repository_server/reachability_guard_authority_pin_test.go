// @awareness namespace=globular.platform
// @awareness component=platform_repository.reachability_guard_authority_pin
// @awareness file_role=architectural_pin_test_for_reachability_guard_owner_rpc_routing
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness enforces=globular.platform:invariant.repository.desired_build_id_is_hard_reachability_root
// @awareness enforces=globular.platform:invariant.repository.purge_must_not_delete_active_desired_builds
// @awareness risk=critical
package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Architectural pin for the v1.2.170 refactor of reachability_guard.go.
//
// Before v1.2.170 collectDesiredBuildIDs scanned four /globular/resources/*
// etcd prefixes directly via clientv3 — bypassing the cluster_controller's
// typed ListDesiredBuildIDs RPC and the canonicalization, version, and
// audit contracts the owner applies. The fix routes the call through
// the new typed RPC.
//
// This test fails loudly if a future contributor reintroduces a direct
// etcd Get / Put / Delete against /globular/resources/* in
// reachability_guard.go, or reintroduces the clientv3 import.
//
// Anchored by:
//   invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage
//   invariant:repository.desired_build_id_is_hard_reachability_root
//   invariant:repository.purge_must_not_delete_active_desired_builds
//   forbidden_fix:read_owned_etcd_prefix_directly_instead_of_calling_owner_rpc
//
// Rebuilding the "which build_ids are desired?" semantic in a consumer
// (even via a "narrow" etcd read) is the recurrent failure pattern this
// test exists to prevent. The single canonical answer lives in
// cluster_controller.ListDesiredBuildIDs.
func TestReachabilityGuard_NoDirectEtcdAgainstResourcesPrefix(t *testing.T) {
	body, err := os.ReadFile("reachability_guard.go")
	if err != nil {
		t.Fatalf("read reachability_guard.go: %v", err)
	}

	if strings.Contains(string(body), `clientv3 "go.etcd.io/etcd/client/v3"`) ||
		strings.Contains(string(body), `"go.etcd.io/etcd/client/v3"`) {
		t.Errorf("CRITICAL reachability_guard.go imports go.etcd.io/etcd/client/v3 — " +
			"violates invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage. " +
			"The reachability guard MUST read desired build_ids through " +
			"cluster_controller.ListDesiredBuildIDs, never via direct etcd scan. " +
			"Reintroducing the etcd client re-opens the bypass vector closed in v1.2.170.")
	}

	re := regexp.MustCompile(`\.(Get|Put|Delete)\(\s*[^,)]+,\s*"/globular/resources/`)
	if loc := re.FindIndex(body); loc != nil {
		match := re.FindSubmatch(body)
		t.Errorf("CRITICAL reachability_guard.go contains a direct etcd %s against /globular/resources/* "+
			"(near byte offset %d) — violates invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage "+
			"and forbidden_fix:read_owned_etcd_prefix_directly_instead_of_calling_owner_rpc. "+
			"L2 desired state is owned by cluster_controller; call ListDesiredBuildIDs (added in v1.2.170) "+
			"or another typed RPC on the controller. Scanning etcd here is the exact regression class "+
			"that produced 'build_id not found for name=…' cascades when the controller's "+
			"in-memory contracts disagreed with raw etcd reads.",
			string(match[1]), loc[0])
	}
}
