# Incident — node remove/rejoin drill cascade: Scylla group0 quorum loss + repository index split (2026-07-09)

**Status**: open — durability restored on the live cluster; 5 scars filed for lawful repair (code + tests + contracts). No fixes applied yet.
**Cluster**: 2-node dev (`globule-ryzen` 10.0.0.63 = profiles `[ai, control-plane, core, media-server, storage]`; `globule-nuc` 10.0.0.8 = `[control-plane, core, gateway, storage]`).
**Trigger**: operator drill — remove `nuc`, verify `ryzen` alone, rejoin `nuc`, loop until day-1 clean.

---

## 1. What happened (timeline)

1. **Remove** — `clean-node.sh` ran on `nuc`. `nodetool status` was **unreachable** on `nuc` at clean time, so the script counted live Scylla peers as `0`, classified the node "single-node", **skipped `nodetool decommission`**, and hard-killed Scylla. `nuc` was actually a live voter in the 2-node ring → a **dead Raft voter** → **group0 lost quorum** (2 voters, 1 alive). `removenode` failed: `raft_operation_timeout_error … no raft quorum`. etcd member removal *did* succeed (clean-node handled it), so etcd stayed at quorum 1/1.
2. **Group0 recovery** — ScyllaDB 2025.3.8 manual Raft recovery (snapshot first): `UPDATE system.scylla_local SET value='recovery' WHERE key='group0_upgrade_state'` → restart → `nodetool removenode <dead-host-id>` (works in recovery/legacy mode) → `TRUNCATE system.topology; TRUNCATE system.discovery; TRUNCATE system.group0_history; DELETE value FROM system.scylla_local WHERE key='raft_group0_id'` → `DELETE FROM system.scylla_local WHERE key='group0_upgrade_state'` → restart → group0 auto-reinit to `use_post_raft_procedures`, single healthy voter. **All keyspace data preserved.**
3. **Single-node fix** — RF=2 keyspaces could not meet `LOCAL_QUORUM` (needs 2, alive 1) → DNS reload degraded. The schema guard only *raises* RF, never shrinks (`scylla_schema_guard.go:265`), so it would not self-heal. Reduced app keyspaces to RF=1 (matches `desiredRFForCluster(1)=1`), restarted dns/minio → 0 errors. (Note: gocql caches keyspace RF metadata — a service restart was needed for the new RF to take effect.)
4. **Rejoin** — gateway `/join` script: etcd 2 voters, Scylla ring 2 nodes (fresh host-id).
5. **Convergence stall** — controller reconcile logged `missing_package remediation … child_status=SUCCEEDED` every 30 s but never installed 8 core services. Worked around with `globular services apply-desired` **run on nuc** (CLI had to be scp'd — clean removed it). All core services then came up. `apply-desired` also over-installed `torrent` (media-server) onto nuc → new orphan; and reported `17 failed: artifact … pipeline state UNKNOWN — not installable` for platform infra whose repo **index** state is UNKNOWN though blobs exist in CAS.
6. **Durability restored** — re-raised app keyspaces to pre-drill RF (2/3), `nodetool repair -full` → nuc load 336 KB → 29.3 MB; RF=2 real again.

---

## 2. Scars (root causes, fixes, tests, contracts)

### SCAR-1 — Remove must not hard-kill Scylla without a proven decommission/quorum precondition

**Symptom**: killing a live 2-node ring member without decommission destroys group0 quorum.
**Root cause (fail-open on "unknown")**: an unreachable `nodetool status` is collapsed to peer count `0`, which the `-gt 1` guard reads as "single-node → skip decommission → hard-kill".
- `scripts/clean-node.sh:292` — `_SCYLLA_UP=$(nodetool status 2>/dev/null | grep -cE "^U[NL] " || echo "0")`; skip branch `:293,301-303`.
- `Globular/internal/gateway/handlers/cluster/clean-node.sh:68-77` `count_scylla_up_nodes()` (authoritative copy, embedded into the gateway binary at build; contains the exact incident log string), decommission branch `:384-395`.
- **Two copies must both be fixed.**
- Independent membership signals exist but are unused: controller/gateway node registry, etcd member list, a peer's `nodetool status`, and local `scylla.yaml` `seeds:` with remote IPs.

**Fix**: `count_scylla_up_nodes` must return a distinguishable `UNKNOWN` sentinel on unreachable probe (never `0`). Caller: on `UNKNOWN`, **fail closed** (`die` with operator guidance) unless an explicit `--last-node` assertion is passed. **`--force` (suppress prompts) MUST NOT imply `--last-node` (data-safety assertion)** — different authorities. The `die` fires in Phase 0.2, before `hard_stop_scylla` in Phase 1.

**Test (bats, stub `nodetool`/`systemctl` on PATH)**: core guard — `_SCYLLA_UP==UNKNOWN` without `--last-node` MUST exit non-zero and MUST NOT emit "Single-node ScyllaDB". Plus: `--last-node` override skips; confirmed 2-node → decommission; confirmed single node → skip (no false fail-closed).

**Proposed contract**
```yaml
invariants:
  - id: cluster.teardown.membership_must_be_confirmed_before_destructive_stop
    class: invariant
    category: signal
    statement: >
      A node teardown MUST NOT hard-stop a stateful ring member (ScyllaDB/etcd)
      until it has POSITIVELY confirmed the node is not a live voter in a
      multi-node ring. An unreachable/empty membership probe is UNKNOWN, not zero.
    related_invariants: [meta.absent_signal_is_not_a_zero_signal, meta.fail_closed_on_destructive_unknown]
forbidden_fixes:
  - id: cluster.teardown.do_not_treat_probe_failure_as_empty_ring
    statement: >
      Do NOT `|| echo 0` a failed membership probe into a "skip the safe teardown"
      branch. Unknown is not zero; unreachable on a destructive path is fail-closed.
      Do NOT let --force imply the single-node/last-node data-safety assertion.
```

---

### SCAR-2 — Controller reconcile success must require dispatch + observation proof

**Symptom**: `reconcile-workflow: item terminal … type=missing_package … child_status=SUCCEEDED` loops every 30 s; the package is never installed; `workflow_list_runs` shows only `reconcile:*` runs.
**Root cause (converged-by-assertion)**: `release.apply.package` finalizes `AVAILABLE` (→ workflow SUCCEEDED) whenever `len(selected_targets)==0`, and the reconcile parent clears the drift observation on `child_status==SUCCEEDED` without re-reading installed-state. The scanner and selector use **asymmetric** filters.
- `golang/workflow/definitions/release.apply.package.yaml:89-108` — `short_circuit_if_no_targets` (`when: len(selected_targets)==0` → `controller.release.finalize_noop status=AVAILABLE`).
- `golang/cluster_controller/cluster_controller_server/workflow_release.go:558-561` — `FinalizeNoop` patches phase Available.
- `workflow_release.go:808-838` — `selectReleaseTargets` drops a genuinely-missing package's node for **non-convergence** reasons (`BootstrapPhase==Admitted/""`, `!bootstrapPhaseReady/!bootstrapInfraReady`, profile mismatch, `isActiveInfraMember`).
- `reconcile_actions.go:274-296` — drift scanner emits `missing_package` with only `FilterDesiredByIntent`+`isActiveInfraMember` (narrower than the selector) → asymmetry.
- `reconcile_actions.go:684-690` — `reconcileMarkItemTerminal` clears the drift observation on `SUCCEEDED` with no installed-state re-check → infinite no-progress loop, no FAILED escalation.
- Incident trigger on nuc: its post-rejoin bootstrap phase never read "ready", so the selector dropped it for a non-convergence reason while the package was genuinely absent.

**Fix**: (1) A `missing_package`/`version_drift` remediation may be marked terminal-SUCCEEDED (and its observation cleared) **only** if, after the child returns, observed `installed_state[node][pkg].build_id == desired resolved_build_id` (fallback version/desired_hash). (2) Distinguish *converged-skip* from *ineligible-skip*: 0 targets where a candidate was dropped for a non-convergence reason with the package absent finalizes **DEFERRED/BLOCKED**, not AVAILABLE. (3) Per-`(node,pkg,drift_type)` no-progress counter → escalate to FAILED + `cluster.reconcile.item_failed(reason=remediation_no_progress)` after N passes. (4) Make scanner/selector filter sets symmetric (or annotate non-dispatchable reason).

**Test (Go, `cluster_controller_server` + selector companion in `workflow/engine`)**: seed desired `rbac` on `node-A`, installed-state absent, selector returns 0 for a non-convergence reason, stub child `SUCCEEDED`. Assert: installed-state still nil AND drift observation NOT cleared; after N passes item is FAILED + `remediation_no_progress` emitted; positive path (installed-state updated) clears normally; selector finalizes `DEFERRED`, not `AVAILABLE`.

**Proposed contract**
```yaml
invariant:
  - id: reconcile_success_requires_observed_install_convergence
    statement: >
      A missing_package/version_drift remediation may be terminal-SUCCEEDED (and
      its drift observation cleared) only when observed installed_state for
      (node,pkg) matches the desired build identity AFTER the child returns.
      Child SUCCEEDED is dispatch/finalize acknowledgement, NOT install proof.
      len(selected_targets)==0 is "converged" only if every candidate was dropped
      for a convergence reason; ineligible-drop with the package absent is
      DEFERRED/BLOCKED, never AVAILABLE.
    related_invariants: [meta.silence_is_not_valid_for_unexpected, convergence.no_infinite_retry]
forbidden_fix:
  - id: reconcile_clear_drift_on_dispatch_ack
    statement: >
      Do NOT clear a drift observation / mark terminal-SUCCEEDED from child-workflow
      status (dispatch ack or noop finalize) rather than a fresh installed_state read.
```

---

### SCAR-3 — Repository CAS/index split needs an owner reindex-from-blobs API

**Symptom**: `artifact "<name>" is in pipeline state UNKNOWN — not installable` and repeated `DesiredBuildIdOrphaned`, while the `.tgz` blob is present in `/var/lib/globular/packages/`. cluster-doctor advises `repository repair-index` — **a command that does not exist**.
**Root cause (CAS ↔ index divergence, no bulk reverse repair)**:
- State enum + gate: `golang/repository/repository_server/artifact_state.go:77-104` (`ArtifactPipelineState`, incl. `UNKNOWN` sentinel), `:436-461` `isRowInstallable` (requires `publish_state==PUBLISHED` && `manifest_json` non-empty && `artifact_state ∈ {PUBLISHED,""}`), `:591-608` `readArtifactState` returns `UNKNOWN` when Scylla errors + cache miss.
- Download gate: `artifact_handlers.go:2339-2354` — anything not `PUBLISHED`/`""` → `FailedPrecondition … not installable`.
- Emitter + phantom command: `resolver.go:316-321,363-366` ("…roll desired forward or run repair-index").
- The reconstruct primitive already exists per-artifact: `artifact_verify_rpc.go:205-381` (Project-D: read CAS `.manifest.json` sidecar, verify size+sha256, `syncManifestToScylla(PUBLISHED)` + `UpdatePublishState` + `transitionArtifactState`; supports dry-run) — but needs a `ref` and has no bulk driver.
- The reverse reconciler `repository_reconciler.go:108-176` is **startup-only** (ticker `:199-207` runs only the forward pass) and **receipt-only** (`receipt.json` is written only on the download path, `repository_source_resolver.go:392-432`), so publish-pipeline blobs at `artifacts/{key}.bin`+`artifacts/{key}.manifest.json` are invisible to it.

**Fix**: add owner `RepairIndex` RPC + `globular repository repair-index --dry-run/--commit` CLI + `repository_repair_index` MCP tool. Walk **CAS `artifacts/*.manifest.json` sidecars** (union with `receipt.json`), verify size+sha256, and for missing/skeleton/non-PUBLISHED rows reuse the Project-D reconstruct (UNKNOWN/skeleton → PUBLISHED). **Never mint identity** — build_id/publisher/checksum come only from the sidecar/blob; a blob failing checksum is reported un-repairable, never published; `REVOKED`/`QUARANTINED` are not auto-elevated. Also add the reverse pass to the hourly ticker, sourced from CAS sidecars (not just receipts).

**Test (`repair_index_test.go`)**: seed CAS blob+sidecar (matching sha256) + Scylla row UNKNOWN/empty/missing. Dry-run → `would_repair_publish_index`, commits nothing. Commit → `readArtifactState==PUBLISHED`, `isRowInstallable==true`, `row.BuildID==sidecar.BuildId`. Negative: checksum mismatch → skip (no unverified publish); `REVOKED` → not elevated.

**Proposed contract**
```yaml
invariant:
  - id: invariant.repository.cas_present_implies_index_repairable
    statement: >
      When a verified blob exists in local CAS (artifacts/{key}.bin + matching
      {key}.manifest.json), the repository MUST reconstruct an installable index
      row (publish_state=PUBLISHED) via an owner-triggered auditable op — without
      upstream re-fetch and without minting identity. UNKNOWN/skeleton over a
      verified blob is repairable, not terminal.
    related_invariants: [meta.half_done_must_not_look_done, repository.fallback_requires_manifest_and_checksum]
forbidden_fix:
  - id: forbidden_fix.repository.roll_desired_forward_instead_of_repairing_index
    statement: >
      Do NOT resolve a CAS-present/index-UNKNOWN split by rolling desired forward,
      manually UPSERTing publish_state, or minting build_id/publisher/checksum.
      Repair the index from the blob (checksum-verified) or not at all.
```

---

### SCAR-4 — Placement/drift hash must be profile-scoped, not all-profile-vs-capability

**Symptom**: persistent ERROR `Node <id> services state hash mismatch (desired ≠ applied)` on a node that legitimately cannot host part of the desired set; both nodes report the **same** `desired_hash` string.
**Root cause (asymmetric authority domains)**: the per-node **desired** hash is cluster-wide (never intersected with node profiles); the **applied** hash is node-scoped.
- `golang/cluster_controller/cluster_controller_server/server.go:70-83` — `filterVersionsForNode` copies **every** desired service; `node.Profiles` is never consulted (misnamed).
- `handlers_health.go:164-165` — `stableServiceDesiredHash(filtered)` → identical string on ryzen and nuc.
- `golang/cluster_doctor/cluster_doctor_server/rules/cluster_services_drift.go:45-57,116-121` — raises mismatch, escalates to ERROR after 5 min.
- Catalog: `media`/`title`/`ffmpeg` declare `Profiles: ["media-server"]` (`component_catalog.go:430-593`) → nuc can never contain them.
- The correct predicate already exists and is used elsewhere: `placementAllows` (`component_resolve.go:423-428`), `ServicesForProfiles` (`component_catalog.go:189-208`), `component_catalog.PackagesForProfiles` (`profilemap.go:200`). The orphan half is already handled correctly by `placement.installed_package_orphaned` (`rules/placement_orphaned_install.go`) with a lawful removal path.
- `buildPlanActions: unknown profile "ai"/"media-server"` (`unit_actions.go:51-71`) is **benign** log noise (those profiles carry no infra units) — not the drift cause.

**Fix**: intersect desired with the node's authorized set inside `filterVersionsForNode` using the **existing single law book** (`placementAllows`), before hashing. Catalog-unknown services keep prior behavior. Apply the same intersection to the service-summary/health-count path (`serviceOnlyDesired` in `handlers_health.go:126-208`) so the "external install detected → stamp applied hash" logic compares like-for-like. Unauthorized-but-installed packages continue to surface only via `placement.installed_package_orphaned`.

**Test**: `TestFilterVersionsForNode_ExcludesUnauthorizedServices` (pure fn): `media`/`title` excluded on a no-media-server node, core `echo` kept. Doctor E2E: `DesiredServicesHash==AppliedServicesHash` for the authorized subset → **zero** drift findings; an unauthorized install → `placement.installed_package_orphaned`, not `cluster.services.drift`.

**Proposed contract**
```yaml
invariant:
  - id: controller.desired_services_hash_must_be_profile_scoped
    statement: >
      The per-node desired services hash MUST be computed over the node's
      profile-AUTHORIZED desired subset (single catalog law book: placementAllows),
      not the cluster-wide desired set, since it is compared against the
      node-scoped applied hash. Comparing all-profile desired truth vs node-scoped
      applied truth makes capability-restricted nodes perpetually "drifted".
    related_invariants: [doctor.layout_drift_must_reflect_real_risk, meta.authority_scope_of_compared_values_must_match]
forbidden_fix:
  - id: forbid.silence_drift_by_adding_profile_or_suppressing_rule
    forbidden:
      - Adding media-server/ai profile to a node just to make the hash match.
      - Suppressing cluster.services.drift globally or special-casing node IDs.
      - Loosening the rule to ignore mismatches when applied is a subset.
    required_instead: Scope the desired hash to the node's authorized subset; orphans stay a distinct finding.
```

---

### SCAR-5 — No lawful node-local package uninstall path

**Symptom**: `yt-dlp`/`torrent` installed on `nuc` (no `media-server` profile) are flagged orphans "operator action required", but there is **no node-scoped uninstall**. `globular services desired remove <name>` is **cluster-wide** (would strip the service from ryzen where it is legitimate); manual `systemctl`/`rm` is (correctly) blocked by the deploy/convergence hook.
**Confirmed CLI surface**: `globular pkg` (no uninstall), `globular services` (adopt-installed, apply-desired, list-desired, repair, seed, verify-integrity, desired{diff,list,remove,set} — all cluster-scoped), `globular nodeagent` (no uninstall).

**Fix**: add a **node-scoped** uninstall path — a node-agent RPC + `globular services uninstall <name> --node <id>` (and/or have the controller's convergence lawfully retire an unauthorized install on a node whose profiles don't allow it, via the supervisor path), so orphan cleanup does not require cluster-wide desired mutation or blocked manual surgery. Must go through the supervisor (HARD RULE #6) and stamp installed-state removal.

**Proposed contract**
```yaml
invariant:
  - id: placement.orphan_removal_needs_a_lawful_node_scoped_path
    statement: >
      A package installed on a node whose profiles do not authorize it MUST have
      a lawful node-scoped removal path (node-agent RPC / node-scoped CLI, via the
      supervisor). "operator action required" without a lawful tool forces either
      wrong-blast-radius cluster desired mutation or blocked manual surgery.
    related_invariants: [meta.every_flagged_state_needs_a_lawful_repair_path]
```

---

## 3. Current live-cluster state (post durability restore)

| Item | State |
|------|-------|
| Both nodes up, all core services active | ✅ |
| etcd | 2 voters, healthy |
| Scylla ring | 2 nodes UN; group0 healthy (recovered) |
| Durability (RF=2 + repaired) | ✅ nuc 336 KB → 29.3 MB |

**Still open (frozen for lawful repair, not improvised)**: 17 repo-index `UNKNOWN` build_ids (SCAR-3), nuc media-server drift (SCAR-4), yt-dlp/torrent orphans on nuc (SCAR-5), torrent/sidecar receipts, cluster-controller local-override missing blob (pre-existing dev state), event release-boundary `A0=FAILED` on both nodes.

## 4. Meta-lesson

The remove/rejoin drill did not *create* most of these — it *exposed* them. Four of the five are fail-open / assertion-of-success patterns (kill-on-unknown, success-on-noop, installable-by-index-only, drift-vs-wrong-authority) plus one missing lawful repair path. Each should be fossilized as an AWG contract via the proper write-path, with the regression test as its enforcement.

**ai-memory**: incident `559ae23b` (project `globular-services`, tags `incident,scylla,group0,repository,convergence,scar`).
