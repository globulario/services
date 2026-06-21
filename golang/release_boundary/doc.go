// Package release_boundary implements Phase 1 of PR-16: the pure
// binary-artifact release-boundary verdict engine.
//
// The contract it proves, for a single service binary:
//
//	The artifact published by the repository, identified by build_id, is
//	byte-identical to what is installed on a node and to what is actually
//	running in /proc, and the running process started after that artifact
//	was installed.
//
// The package is intentionally PURE and DETERMINISTIC. It performs no I/O:
// no RPC calls, no filesystem reads, no /proc reads, no etcd/Scylla reads,
// no process inspection. It receives already-fetched truth structs (the
// Inputs) and returns a structured Report. All collection of those truth
// structs — repository VerifyArtifact, controller GetDesiredState,
// node-agent GetInstalledPackage / GetServiceRuntimeProof — belongs to
// later PR-16 phases (MCP tool, CLI, doctor invariant), each of which can
// reuse this same verdict logic.
//
// Governance constraints honored here:
//   - No repair behavior, no writes, no cluster mutation, no direct storage.
//   - No silent proof: a missing or ambiguous truth source becomes
//     INDETERMINATE evidence, never a quiet PROVEN.
//   - Wrapper / unhashable packages are NOT_APPLICABLE, never FAILED.
//
// This phase does NOT prove the source -> published link, CI provenance,
// config / policy / RBAC / generated artifacts, or wrapper packages.
package release_boundary
