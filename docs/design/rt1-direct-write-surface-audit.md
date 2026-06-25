# RT-1 — Direct-Write Surface Audit (Tier D spike)

> **Status: DONE — this is the scoping deliverable for RT-2 / RT-3 / RT-4.**
> Read-only audit of the full owner-owned-state direct-write surface (Go services,
> static enforcement, runtime enforcement, and the external MCP/CLI/script surface).
> Snapshot: **2026-06-25**, against `master`. Method: four parallel sub-audits over
> the etcd write primitives (`golang/config/`), the `principle-check` scanner +
> allowlist, and the MCP/CLI/script surfaces. Every claim below carries file:line.

---

## TL;DR — the spine

The owners are **disciplined**; the governance gap is **not** in the services that
own the state. Three findings drive all of Tier D:

1. **Go service owners write only their own state, mostly through single
   chokepoints.** cluster_controller funnels all desired/release writes through
   `resourcestore.etcdStore.Apply/Delete`; node_agent funnels installed-state
   through `installed_state.WriteInstalledPackage`; node status flows via the
   `ReportNodeStatus` RPC, not a node-side write. **Zero cross-owner raw writes in
   the Go services.**

2. **Static governance is strong; runtime governance is inert.** The
   `principle-check` scanner is fail-closed (DRIFT-by-default) over 11 service dirs,
   and its allowlist is the complete inventory of *permitted* direct writes. But the
   *runtime* owner-ownership table (`config.CriticalKeyPolicies` +
   `ValidateCriticalKeyOwner`) **has zero non-test callers** — it is built and
   unused. Only `ValidateCriticalKeyWrite` is wired, to **2 ingress keys**. The
   config write primitives enforce no ownership at all.

3. **The actual raw-owner-write exposure lives in the surfaces the scanner does not
   sweep: the CLI and shell scripts.** `globularcli` and `scripts/` are **not** in
   the scanner's `actor_writer_dirs` — which is precisely why **9 CLI command paths**
   and **~8 scripts** still write owner-owned etcd keys directly, bypassing the
   owner RPC. The MCP surface, by contrast, is verified clean (raw etcd tools
   removed, pin-tested).

**The BH-1 connection:** these CLI/script raw writes are exactly the behavioral
forbidden-move aliases (`services_desired_set_force_cross_kind`,
`set_infra_version_raw`, `nodeagent_installed_set_raw`, …) that BH-1 now refuses at
the govops gateway — **but only if a write routes through the gate.** These paths
never reach it. **govops has zero callers in any service** (`GOVOPS_ROUTED` count =
0 everywhere). So RT-2/RT-3 are what make BH-1's carved refusal actually bite.

---

## Owner-owned-state taxonomy

| State class | Owner | etcd key prefix | Write chokepoint (today) |
|---|---|---|---|
| Desired (Service/App/Infra) | cluster_controller | `/globular/resources/{Type}/{name}` | `resourcestore.etcdStore.Apply/Delete` (`resourcestore/etcd_store.go:91/104`) |
| Release + status | cluster_controller | `/globular/resources/*Release/*` | same `resourcestore` chokepoint + `release_reconciler.go` patch helpers |
| Installed | node_agent | `/globular/nodes/{id}/packages/{kind}/{name}` | `installed_state.WriteInstalledPackage` (`installed_state/installed_state.go:133`) |
| Node runtime status | node_agent → controller | `/globular/nodes/{id}/status` | written by controller via the `ReportNodeStatus` RPC (node does not write it) |
| Storage candidates / objectstore rendered | node_agent | `/globular/nodes/{id}/storage/candidates/*`, `…/objectstore/rendered_*` | **bare `cli.Put`** — see Surface A RAW_DIRECT |
| Ingress spec / status | controller / ingress-keepalived | `/globular/ingress/v1/spec`, `…/status/{node}` | controller spec (guarded); node writes own status |
| Domains / providers | controller (via `domain` lib) | `/globular/domains/v1/{fqdn}`, `/globular/providers/v1/{name}` | `domain.EtcdDomainStore` (controller is the caller) |
| Repository artifact lifecycle ledger | repository | **ScyllaDB, not etcd** | `scylla.UpdateArtifactState` — no etcd surface |

> **Note:** `config/write_class.go` (`BestEffort/Normal/Critical/StateCommit`) is a
> *write-reliability* axis (timeout/retry/backoff), **orthogonal** to ownership. RT-2
> guarding composes with it; it is not an ownership control.

---

## Surface A — Go service owners: **disciplined**

**cluster_controller + repository** (`resourcestore` chokepoint):
- All Desired/Release writes go through `srv.resources.Apply/Delete`. **No raw
  `clientv3.Put` to `/globular/resources/*`.** RAW_DIRECT = 0.
- Operator path (`desired_state_handlers.go:upsertOne`) is guarded:
  `routeInfrastructureDesired` (kind, `desired.keyed_by_kind_and_name`),
  no-regression floor (`desired.no_regression_all_paths`), artifact-existence
  (`validateArtifactInRepo`), and an audit record per write.
- Repository's lifecycle ledger is **ScyllaDB** — no etcd write surface.
- Guard-parity gap (minor): the RPC-handler Apply sites (`handlers_status.go:667`
  `ApplyServiceDesiredVersion`, and `ApplyServiceRelease/App/Infra`) use the typed
  store but bypass the `upsertOne` floor/kind guards. Not raw — but worth parity.

**node_agent + edge owners** (`installed_state` funnel):
- Installed-state: ~30 call sites, **all** through `installed_state.WriteInstalledPackage`
  / `CommitInstalledPackage`. RAW_DIRECT = 0.
- Node status via `ReportNodeStatus` RPC (correct). DNS/domain/providers/ingress-status
  are `OWNER_RPC_INTERNAL` or CAS-guarded (`domain/store.go:198 PutStatusCAS`).
- **4 RAW_DIRECT bare-`cli.Put` sites** — all node-writing-its-own objectstore keys,
  already *allowlisted* by the static scanner as observer-self-state, but unrouted
  and runtime-unguarded:
  1. `config/objectstore_admission.go:299` `SaveDiskCandidate`
  2. `config/objectstore_admission.go:320` `DeleteStaleNodeCandidates`
  3. `node_agent/.../minio_systemd_reconcile.go:295` `writeRenderedGeneration` (rendered_generation)
  4. `node_agent/.../minio_systemd_reconcile.go:301` `writeRenderedGeneration` (rendered_state_fingerprint)
  - Ambiguous: `node_agent/.../installed_services.go:680` `StampInfraConvergenceHash`
    (the `set_infra_version_raw` shape; routes through the funnel but mutates the
    controller-comparison hash from the node side — confirm it stays owner-internal
    and non-externally-invocable).
- **Doc-drift (not code):** `heartbeat.go:252` carries a stale
  `//globular:writes /globular/nodes/{node_id}/status` annotation; that function
  writes `/packages/…`, not `/status`. Fix the annotation.

---

## Surface B — Static enforcement: **strong** (RT-4 foundation already exists)

- **Scanner:** `awareness-graph/cmd/principle-check` — a regex engine
  (`main.go:161 scan`, `:232 classify`, default bucket **DRIFT** at `:273`, exit 1 on
  DRIFT/HIDDEN_WORKFLOW). Pattern (from `invariants.yaml:9922`) matches both direct
  `cli.Put/.Delete` and transaction `clientv3.OpPut/OpDelete` (the txn-form gap was
  closed 2026-06-09). Run via `make principle-check-all`.
- **Enforced invariants:** `workflow.every_state_mutation_belongs_to_a_workflow_instance`
  and `workflow.workflow_service_writes_only_own_runtime_state`.
- **Allowlist = the complete permitted-direct-write inventory** (in
  `docs/awareness/invariants.yaml`, not Go): ~50 `exception_files` +
  12 `workflow_step_handler_files`, categorized observer-only-self-state /
  bounded-auto-heal / pre-workflow-primitive / service-self-config /
  event-bus-ephemera. Everything else in the swept dirs is DRIFT → fail.
- **`actor_writer_dirs` (the swept set):** cluster_controller_server,
  node_agent_server, repository_server, cluster_doctor, mcp, ai_executor, ai_memory,
  dns, domain, backup_manager, audittrail.
- **THE COVERAGE GAP:** `globularcli` and `scripts/` are **not** in
  `actor_writer_dirs`. The CLI and scripts are unscanned — the structural reason the
  raw-write exposure (Surface D) concentrated there.
- **3 HIDDEN_WORKFLOW** known-but-disallowed multi-step writes awaiting a workflow
  lift (not allowlisted; they fail the scan): `handlers_node.go::RemoveNode`,
  `node_removal_requests.go::processNodeRemovalRequests`,
  `ingress_spec_guard.go::restoreIngressSpecFromBackup`.

---

## Surface C — Runtime enforcement: **inert** (the RT-2 core)

- `config/critical_keys.go` defines `CriticalKeyPolicies` — an **owner table** of 8
  governed keys (`/globular/system/config`, `/globular/ingress/v1/spec[ _backup]`,
  `/globular/pki/ca`, `/globular/objectstore/config`, `/globular/resources/` prefix →
  cluster-controller; `/globular/nodes/` prefix → node-agent;
  `/globular/scylla/schema_guard/` → cluster-controller) — and a validator
  `ValidateCriticalKeyOwner(key, writerID)` + `OwnerForKey(key)`.
- **This validator has zero non-test runtime callers — it is inert.** The only live
  table consumer is `PolicyGapsForKeys` (a doctor coverage check, not a write guard).
- The only *live* runtime owner-guard is `ValidateCriticalKeyWrite`
  (`critical_state_registry.go:189`), wired at exactly **2 sites** — the ingress
  spec/backup publish in `ingress_spec_guard.go:290/293`.
- The config write primitives (`PutRuntimeWithClass`, `PutRuntime`,
  `SaveServiceConfiguration`, …) perform **no** ownership check. A caller can write
  any key.

---

## Surface D — External writers: **the exposure**

**MCP — CLEAN (verified).** Raw etcd tools (`etcd_get/put/delete`) removed in
v1.2.167; `tools_etcd.go` is a no-op stub; two pin tests
(`mcp_etcd_authority_pin_test.go`) enforce it at runtime and source-tree level. Every
mutating MCP tool routes via the owner's typed RPC or the execution governor; the only
`config.Put*` is `PutClusterConfig` → MinIO (shared file config, not L1/L2/L3).
Residual sharp edge: `grpc_call` is a generic any-RPC tool (owner-RPC-routed when
`read_only=false`, no per-service allowlist) — note for RT-2, not a raw write.

**CLI — 9 RAW_DIRECT command paths** bypass the owner RPC (the RT-2/RT-3 work-list):

| # | Command | file:line | Owner-owned state written | Owner bypassed |
|---|---|---|---|---|
| # | Command | file:line | Owner-owned state written | Owner bypassed | Status |
|---|---|---|---|---|---|
| 1 | `state canonicalize --fix-installed --metadata-only` | `state_cmds.go:916` | L3 installed buildId `nodes/{n}/packages/{kind}/{svc}` | node_agent — **highest severity** (cross-owner L3 write) | ✅ #106 |
| 2 | `release set-infra-version` | `release_cmds.go:157` → `desired_state_helpers.go:161` | InfrastructureRelease spec.version | controller — typed `ApplyInfrastructureRelease` | ✅ #105 |
| 3 | `pkg override apply` | `pkg_override_cmds.go:199,203` | ServiceDesiredVersion + LocalOverride prefix | controller | ✅ #107 |
| 4 | `pkg override remove` | `pkg_override_cmds.go:242,246` | desired + override prefix | controller | ✅ #107 |
| 5 | `objectstore topology sanitize-pool` | `objectstore_cmds.go:152,184` | controller state blob + objectstore placement | controller | ✅ #110 |
| 6 | `objectstore disk approve`/`reject` | `objectstore_disk_cmds.go` (`config.SaveAdmittedDisk`/`DeleteAdmittedDisk`) | placement (admitted disks) | objectstore/controller | ✅ #113 |
| 7 | `objectstore topology plan` | `objectstore_disk_cmds.go` (`config.SaveTopologyProposal`) | placement proposal | objectstore/controller | ✅ #113 |
| 8 | `objectstore topology apply` | `objectstore_disk_cmds.go:712` | placement apply-request handshake | controller | ✅ #109 |
| 9 | `cluster acc set`/`reset` | `acc_cmds.go:307,346` | config-put `/globular/system/acc/config` | controller | ✅ #108 |

> **RT-2 CLI progress: 9 / 9 migrated** (#105, #106, #107, #108, #109, #110, #113).
> The scanner-flagged raw-`cli.Put`/`cli.Delete` baseline
> (`exception_pending_owner_routing_migration`) is empty, and items 6 & 7 — the
> architectural owner-write migrations the scanner does not sweep
> (`config.SaveAdmittedDisk` / `DeleteAdmittedDisk` / `SaveTopologyProposal`) —
> are now routed through controller owner RPCs (`ApproveObjectStoreDisk`,
> `RejectObjectStoreDisk`, `PlanObjectStoreTopology`). The whole Surface-D CLI
> work-list is migrated. Remaining RT-2: the 4 node_agent objectstore bare-`cli.Put`
> sites (Surface A) and the 6+2 shell scripts.

Correctly routed already (for reference): `services desired set/remove` (no `--force`
by design — only audited `--allow-regression`), `release apply/scale/rollback`,
`cluster profiles set`, `cluster remove-node`, `deploy`, and **`ops apply`** — which
is the one path already `GOVOPS_ROUTED` (Validate gate → typed dispatch). `ops apply`
is the migration template for the 9 above.

**Scripts — 6 steady-state + 2 state.json** writers that mutate owner-owned state
behind the live controller (the `stop controller → etcdctl del → restart`
anti-pattern, hardcoded node UUIDs): `reset-all-plans.sh`, `fix-ghost-nuc.sh`,
`nuke-and-restart.sh`, `prepare-rejoin.sh`, `reset-releases.sh`, `fix-stale-plans.sh`
(+ `fix-ghost-nodes.sh`, `fix-remote-agents.sh` rewriting controller `state.json`).
Legitimate/bootstrap (out of scope): `release/install-day0.sh` (Tier-0 seed before
the controller exists), etcd membership scripts, and all read-only `etcdctl get`.

---

## Scoping → RT-2 / RT-3 / RT-4

### RT-2 — route/guard all owner-owned writes (L)
Work-list, in priority order:
1. **CLI (9 paths above)** → migrate onto typed owner RPCs / the govops `ops apply`
   pattern. Start with #1 (`state canonicalize --fix-installed`, cross-owner L3) and
   #2 (`release set-infra-version` — the typed dispatcher already exists, so it is a
   delete-and-redirect, not new code).
2. **Scripts (6 + 2)** → replace `etcdctl del`-behind-controller with the typed CLI /
   RPC path, or reclassify as explicit, gated break-glass (post-reconciled).
3. **Activate the inert runtime guard (Surface C)** → wire `ValidateCriticalKeyOwner`
   into the config write primitives and/or the two chokepoints, so owner-ownership is
   enforced at runtime, not just CI-scanned. This is the highest-leverage single move
   — the table already exists.
4. **The 4 node_agent objectstore bare-Puts (Surface A)** → route through an owner
   helper / class, or add them explicitly to the runtime-guard's owner-internal set.

### RT-3 — govops as the enforced front door (M)
- Wire govops `Validate` into the **chokepoints**, not scattered sites:
  `resourcestore.etcdStore.Apply/Delete` (controller) and
  `installed_state.WriteInstalledPackage` (node_agent). One intercept per owner.
- `ops apply` already demonstrates the pattern end-to-end (Validate → typed dispatch).
  Generalize it so the CLI paths from RT-2 land on it.
- Once a write flows through `Validate`, **BH-1's forbidden-move refusal and the
  structural gates apply automatically** — this is where the carved gate starts to
  bite real mutation paths.

### RT-4 — principle-check scanner: no new raw writes (M)
- **Largely already exists** (DRIFT-by-default, fail-closed). The residual is
  **coverage**, not capability:
  - **Add `golang/globularcli` to `actor_writer_dirs`** — this is the single change
    that would have caught the 9 CLI paths. (Expect to allowlist the legitimate
    bootstrap/day-0 CLI writes as the existing categories already do for services.)
  - **Add a scripts check** (the scanner is Go-AST; scripts need a lightweight
    `etcdctl (put|del)` grep gate over `scripts/` with a day-0/membership allowlist).
  - **Lift the 3 HIDDEN_WORKFLOW** sites onto workflows (then drop them from the
    disallowed set).
- The "RT-4 and BH-1 are one scanner" note from the roadmap resolves cleanly: the
  existing `principle-check` *is* the raw-owner-write static scanner; RT-4 extends its
  *reach* (CLI + scripts), it does not build a second one.

---

## Confidence / residual uncertainty

- High confidence on the Go-service inventory, the scanner/allowlist, and the MCP
  verification (all pin-tested or grep-complete).
- The CLI/script work-list is the actionable core; each item carries file:line.
- `grpc_call` (generic any-RPC MCP tool) and the RPC-handler guard-parity gap are
  noted but secondary — they are owner-RPC-routed, not raw writes.
- govops integration *intent* (which chokepoint, what fail-mode) is a design choice
  for RT-3, not derivable from current code (govops is unwired).
