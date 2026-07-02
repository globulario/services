# CG-3 — convergence.identity_is_build_id

> Status: implemented via Path C and verified in repo state.

This note originally scoped the highest-value deferred CG-3 spine change the
same way as RT-1/OT-1/EX-1: establish the real current state before proposing
code. The verify-first conclusion held: the safe slice was **not** "make the
cluster hash carry build_id"; it was the targeted authority fix below. That
slice is now the landed state in `services`.

## The invariant
`convergence.identity_is_build_id` (status: active; coverage tier: planned):
> Any equality that decides "is this node converged?" must compare build_id, the
> sole convergence identity. The services-drift hash today compares the version
> string only (`hashDesiredServiceVersions`)… The drift detector must carry
> build_id and must distinguish "node ahead of desired (desired regressed)" from
> "node behind desired (needs upgrade)."

Two clauses: (a) convergence equality uses build_id; (b) ahead-vs-behind
distinction.

## Verify-first findings (code-grounded)

1. **Per-package convergence already uses build_id (NOT the violation).**
   `classifyPackageConvergence` (release_runtime_convergence.go:188–205) enforces
   build_id as an independent dimension (D3): same version + same hash + different
   build_id ⇒ RepairRequired, fails closed. `driftReconciler` uses
   `versionutil.EqualFull(version, buildNumber, …)`.

2. **The NAMED violation is the cluster *summary* hash — version-blind on BOTH
   sides.**
   - desired: `hashDesiredServiceVersions` (service_identity.go:24) → `name=version;`
   - applied: `computeAppliedServicesHash` (installed_services.go:599) → `key=version;`

3. **The cluster summary hash IS a convergence-AUTHORITY gate (the real bite).**
   `repair_node_workflow.go:685` REFUSES a reference node when
   `node.AppliedServicesHash != desiredHash`. A version-blind hash can accept a
   reference node on a *different build_id, same version* — exactly the drift the
   invariant forbids. (Other consumers: reconcile_nodes:315 stamps it;
   handlers_health:165 reports it.)

4. **build_id is NOT available on the applied side.** `InstalledServiceInfo`
   (installed_services.go:72) carries Version, Kind, Config, ConfigDigest — **no
   BuildID**. The desired side CAN resolve build_id (`lookupServiceReleaseBuildID`,
   reconcile_actions.go:518) but only by threading it into 3 call sites.

5. **Landmine: build_id is not universal.** Upstream-native services (etcd,
   scylla) and first-build/dev have no build_id; the per-package path already
   gates this (`requireBuildID`). Any cluster-hash change must handle "no build_id"
   identically on both sides or those services drift forever.

## The fork

### Path A — full cluster-hash spine change (literal reading)
Make both hashes carry build_id: add `BuildID` to `InstalledServiceInfo` +
populate it + include in `computeAppliedServicesHash`; resolve build_id at the 3
desired call sites + include in `hashDesiredServiceVersions`; bump both
`@hash_schema` annotations; migrate consumers; handle no-build_id uniformly.
- **Pro:** honors the invariant's literal text (the cluster hash itself).
- **Con:** HIGH blast radius. If the two sides' formats ever disagree (or the
  no-build_id fallback differs), every node reads as drifted **permanently** —
  the precise harm the invariant exists to prevent. Cross-layer (node-agent +
  controller + doctor), schema-versioned, hard to make reversible.

### Path B — declare the cluster hash a summary, lean on per-package
Relabel the cluster hash as a coarse version-summary and rest convergence
authority on the already-build_id-correct per-package path.
- **Con:** does NOT fix the actual authority bite — `repair_node_workflow` still
  gates reference-node selection on the version-blind hash. Dodges the named
  violation. Rejected as ceremonial.

### Path C — targeted authority fix (RECOMMENDED)
Fix the one convergence-AUTHORITY consumer to use the per-package build_id check
that already exists; relabel the cluster summary hash honestly as coarse.
- Replace the `AppliedServicesHash != desiredHash` gate in `repair_node_workflow`
  with a per-package build_id convergence check for the reference node (reuse the
  `classifyPackageConvergence` machinery against the reference node's installed
  build_ids from etcd L3 state).
- Document the cluster summary hash as a version-coarse signal, NOT convergence
  authority (it is fine for reconcile stamping / health display).
- **Pro:** fixes the real bite (authority equality now uses build_id) using
  already-correct, already-tested machinery; LOW blast radius (one workflow,
  no schema bump, no both-sides-hash rebuild). **Pro:** smallest slice that makes
  the invariant promotable without risking cluster-wide permanent drift.
- **Con:** the cluster summary hash itself stays version-coarse — but it is no
  longer load-bearing for convergence authority, which is what the invariant
  actually governs.

## Landed resolution
**Path C** is what shipped.

- `repair_node_workflow.go` now treats the cluster summary hash as a coarse
  version pre-check only.
- Reference-node acceptance uses `referenceNodeBuildIDMismatch(...)` as the
  build-identity authority check.
- Same version + different build_id is refused.
- Services with no resolved desired build_id are skipped, matching the
  per-package `requireBuildID` discipline.
- The per-package convergence path remains build_id-correct independently.

Evidence in this repo:

- [golang/cluster_controller/cluster_controller_server/repair_node_workflow.go](/home/dave/Documents/github.com/globulario/services/golang/cluster_controller/cluster_controller_server/repair_node_workflow.go:681)
- [golang/cluster_controller/cluster_controller_server/repair_node_buildid_test.go](/home/dave/Documents/github.com/globulario/services/golang/cluster_controller/cluster_controller_server/repair_node_buildid_test.go:1)
- [golang/cluster_controller/cluster_controller_server/build_id_convergence_dimension_test.go](/home/dave/Documents/github.com/globulario/services/golang/cluster_controller/cluster_controller_server/build_id_convergence_dimension_test.go:1)

## Result
The decision point is closed in favor of "cluster summary hash is coarse; build
identity is the real convergence authority." No schema bump, no node-agent
struct change, and no cross-layer hash rewrite were required.
