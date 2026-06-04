// @awareness namespace=globular.platform
// @awareness component=platform_mcp.tools_etcd
// @awareness file_role=mcp_etcd_direct_get_put_delete_tool_REMOVED_v1_2_167
// @awareness implements=globular.platform:intent.awareness.mcp_bridge_exposes_safe_tools_only
// @awareness enforces=globular.platform:invariant.mcp.etcd.tools_must_not_join_remote_allowlist
// @awareness enforces=globular.platform:invariant.etcd.path_has_single_owner
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness risk=critical
package main

// tools_etcd.go — registration of direct etcd_get / etcd_put / etcd_delete
// MCP tools was REMOVED in v1.2.167.
//
// History and reasoning:
//
//   Until v1.2.166 this file registered three tools that talked directly
//   to etcd:
//     - etcd_get      (read any prefix)
//     - etcd_put      (write any /globular/* key — guarded by read_only flag)
//     - etcd_delete   (delete any /globular/* key — guarded by read_only flag)
//
//   Each was a structural bypass of the layer-owner contract anchored in
//   the awareness graph:
//     - etcd_get bypassed invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage
//       (reads of L1/L2/L3 owned prefixes must go through the owner's
//        typed RPC; the owner applies canonicalization, version contracts,
//        and audit attribution that a direct etcd read skips).
//     - etcd_put + etcd_delete bypassed invariant:etcd.path_has_single_owner
//       and invariant:four_layer.layer_has_single_writing_actor (every
//       prefix has exactly one writing actor; a generic Put authorises
//       around all of them).
//     - All three bypassed invariant:mcp.etcd.tools_must_not_join_remote_allowlist
//       structurally: even though etcd_put and etcd_delete were in
//       forbiddenRemoteTools and etcd_get was simply not in
//       allowedRemoteTools, the safe state per
//       failure_mode:mcp.etcd_remote_allowlist_expansion is "the tool
//       does not exist", not "the tool exists but is blocked".
//
// What replaced them (already shipping):
//
//   For READING owned state: call the owner's typed RPC via the existing
//   MCP tools that already exist in this package and ARE in the safe
//   remote-call allowlist (aggregator_policy.go):
//     - L1 Artifact   → repository_* tools (list_artifacts, get_artifact_manifest,
//                       list_artifact_versions, search_artifacts, list_repository_findings, ...)
//     - L2 Desired    → cluster_* tools (cluster_get_desired_state,
//                       cluster_get_service_release, cluster_list_nodes,
//                       cluster_get_node_full_status, cluster_get_doctor_report, ...)
//     - L3 Installed  → nodeagent_* tools (nodeagent_get_inventory,
//                       nodeagent_list_installed_packages, nodeagent_get_installed_package,
//                       nodeagent_get_certificate_status, ...)
//     - L4 Runtime    → forwarded through L3 typed RPCs (the doctor's
//                       cluster_get_doctor_report aggregates runtime
//                       observations).
//
//   For MUTATING owned state: dispatch through the owner's typed RPC, or
//   through a workflow. There is no MCP-exposed primitive for writing
//   arbitrary etcd state — that is by design.
//
// What to do if a future task seems to need a "raw" etcd read or write
// from MCP:
//
//   1. Ask: which owner owns the prefix? Add or use that owner's typed RPC.
//   2. If a brand-new operator workflow truly needs a bootstrap-only
//      generic primitive (e.g. Day-0 day0_seed paths), scope it to a
//      fixed prefix allowlist HARD-CODED in the tool, narrowly named
//      (day0_seed_set, never etcd_put), and gate it on a Day-0 boot
//      phase that auto-disables after first apply.
//   3. Re-introducing etcd_get / etcd_put / etcd_delete here will fail
//      the architectural pin test mcp_etcd_authority_pin_test.go.
//
// See:
//   forbidden_fix:generic_etcd_write_tool_exposed_to_callers
//   forbidden_fix:generic_etcd_or_storage_read_tool_exposed_to_callers
//   forbidden_fix:cross_layer_write_by_non_owner
//   forbidden_fix:read_owned_etcd_prefix_directly_instead_of_calling_owner_rpc

// registerEtcdTools is intentionally a no-op. The function is kept so the
// call site in the server bootstrap does not need to know about the
// removal. If you delete this stub, also delete the call site — and
// confirm via the architectural pin test that no new etcd_* tool sneaks
// in to replace it.
func registerEtcdTools(_ *server) {
	// No tools registered. See file header for the rationale and the
	// architectural pin test that enforces it.
}
