# Design: unify Day-1 join into the Day-0 installer (etcd-first, local packages, ordered rollback)

Status: **proposed** — direction explored with author 2026-07-04 during a live Day-1 join incident; this doc is the plan on paper before any code.
Author: 2026-07-04
Related evidence: live single→two-node join attempt on `globule-ryzen` (10.0.0.63) + `globule-nuc` (10.0.0.8), 2026-07-04. Every failure below was observed directly in that session.
Related rules: CLAUDE.md HARD RULE #2 (4-layer state), #4 (state changes through workflows), #5 (founding quorum), #9 (Day-1 joins from active BOM); `ops.day-1.join`.

---

## Problem

Day-0 (bootstrap first node) and Day-1 (join an existing cluster) are **two divergent code paths that install the same things differently**, and Day-1 is the fragile one. Day-0 is a single ordered `globular-installer` run from **local packages**; Day-1 is a shell join-script **plus** node-agent convergence **plus** gateway bundle assembly **plus** repository/desired-state resolution. Every failure in the 2026-07-04 session lived in the Day-1 path.

### Observed failures (all Day-1-path-specific)

1. **ScyllaDB install intent is never set on Day-1.** `packages/scylladb/scripts/post-install.sh` has correct, defensive join logic (seed discovery from `etcd_endpoints`, self-only-seed guard, cluster-fingerprint ownership check, fail-closed on unknown stale Raft state). But it is gated on env vars:
   ```
   SCYLLA_INSTALL_INTENT="${SCYLLA_INSTALL_INTENT:-preserve}"   # default preserve
   ALLOW_STALE_SCYLLA_REINIT_ON_JOIN="${...:-false}"
   SCYLLA_CLUSTER_FINGERPRINT="${...:-}"                          # never set on Day-1
   ```
   Only `scripts/release/install-day0.sh` sets these (`initial-node`/`first-node`). The **node-agent never sets `fresh-join`** — grep across `services`, `globular-installer`, `packages` confirms it. So Day-1 always runs in `preserve` mode: it works **only** on a perfectly clean node, and **fails closed (`exit 1`, scylla never starts)** if any stale Raft state exists (common on a re-join). The script's join-safety logic is effectively dead code on Day-1.

2. **Envoy (and gateway) restart on join although they hold no distributed data.** `node_agent/.../certificate.go` (`runCertWatcherOnce` / `restartServicesAfterCertChange`) restarts `globular-xds + globular-envoy + globular-gateway` **together** whenever the cluster cert-bundle *generation* bumps (10s debounce). A join bumps that generation (new node cert / CA redistribution), so the founder's ingress gets a **process restart + ~60s mesh blackout** — for a component with nothing to sync. Under sustained churn it becomes a restart-storm (observed: xds/envoy/gateway cycling every ~11-12s).

3. **No rollback on join failure.** The scylla join state machine (`cluster_controller/.../scylla_members.go` `reconcileScyllaJoinPhases`) has retry (replace_address, restart, wipe-and-retry) but on permanent failure only sets `ScyllaJoinPhase = ScyllaJoinFailed` — it does **not** decommission the half-joined node. By then the joiner is already a **group0 voter** (there is no non-voting/learner staging, despite `GROUP0_LIMITED_VOTERS` + zero-token support in the build). The `node.remove` workflow *has* `remove_etcd_membership` + `remove_scylla_ring`, but they are **best-effort** (`remove_scylla_ring` logs `WARN` and proceeds on failure — `actors_node_remove.go:281-286`), only run on **explicit** node-remove, and `nodetool removenode` **needs group0 quorum** that may already be gone. Live result: removing the failed node left both a stale etcd voter **and** a stale scylla group0 voter → founder etcd + scylla quorum both dropped to 1-of-2 → required manual `etcdctl member remove` + a full ScyllaDB `recovery_leader` procedure to recover.

4. **The scylla failure cascades into "unrelated" subsystems.** `repository` (and `dns`, `rbac`, …) keyspaces are **RF=1** on ScyllaDB. When the joiner took vnode token ranges and then died without decommission, group0 lost quorum and the ranges it owned became unreadable → the controller's (unchanged, identical) desired build_ids could no longer be resolved → a storm of `DesiredBuildIdOrphaned` CRITICALs + workflow install-storm. The build_ids never diverged between nodes; **the repository's ability to resolve them broke because the repository lives in the now-degraded scylla.**

5. **The gateway `/join/bundle` assembly is a fragile dependency.** Day-1 downloads a tarball the gateway assembles on the fly from `/var/lib/globular/{packages,release-index.json,workflows,globular-installer}`. It takes the **version** from etcd `active_release` but tars the node-local `release-index.json` **projection**, which `platform-upgrade`/`activate` do **not** refresh — so after an upgrade the bundle is version-inconsistent. During ingress churn the download simply fails (observed: `could not download globular-1.2.268-...tar.gz from GitHub or controller fallback`).

6. **Optional profiles do not propagate to joining nodes.** `approveJoinRecordLocked` (`handlers_join.go:263-279`) takes `SuggestedProfiles` (`deduceProfiles(caps)`) or `DefaultProfiles` (`["core"]`), then `enforceFoundingProfiles` adds only `core/control-plane/storage`. `ai` (and `gpu`, `dns`, …) are never founding and are dropped unless the node's capabilities happen to deduce them, and the **join script has no `--profiles` flag**. Live result: nuc joined without the `ai` profile the founder has.

### Why this keeps "breaking from anywhere"

A join is a multi-step mutation of **three independent stateful authorities** — etcd membership, ScyllaDB group0/topology, controller desired-state — and **none of the steps are transactional or auto-compensated.** Each step immediately changes the authority node's quorum math / data ownership / desired records, optimistically, before the newcomer has proven anything. A failure at any step scatters partial, un-rolled-back state across all three. The newcomer doesn't corrupt the authority; the **join protocol makes the authority wound itself** by admitting a full voting member too early.

---

## Goals

- **One install path.** Day-1 = Day-0 with a `--join` flag. Delete the parallel join-script/convergence-bootstrap/bundle-download machinery.
- **etcd is the anchor, established first.** Everything downstream (CA/certs, node list → scylla seeds, desired config) is read from etcd, not from bundles or ad-hoc seeds.
- **Install from local packages.** The dist tarball already carries every `.tgz`; `globular-installer` installs them locally. No gateway `/join/bundle`, no repository resolution during bootstrap.
- **Ordered steps with rollback.** `etcd_join → verify → scylla_join → verify → rest`; on any step failure, run the compensating action (remove etcd member / decommission scylla) **while the authority still has quorum**, then exit non-zero.
- **Profiles are an explicit parameter** (like Day-0 `FOUNDING_PROFILES`), defaulting to *inherit the founder's set* so nodes are identical.
- **No package version / build_id changes required.** This is orchestration, not repackaging.

## Non-goals

- Rewriting ScyllaDB's own bootstrap. We use its existing zero-token / limited-voter features; we don't fork scylla.
- Changing steady-state convergence. After the installer bootstraps the node, node-agent + controller own steady-state exactly as on Day-0.
- Multi-node-at-once join / concurrent joins (future; this design is one-node-at-a-time, which is also the safe default for quorum changes).

---

## Design

### One entry point

Generalize `scripts/release/install-day0.sh` → `install.sh`:

```
sudo bash install.sh                                   # bootstrap (today's Day-0), unchanged
sudo bash install.sh --join <controller> --token <t> \ # Day-1
                     [--profiles core,control-plane,storage,ai]
```

No `--join` ⇒ bootstrap branch (existing behaviour). `--join` ⇒ the new join branch below. Same script, same `globular-installer`, same local `packages/` — only the parameters differ.

### Day-0 vs Day-1 parameters (the entire difference)

| Concern | Bootstrap (Day-0) | Join (Day-1) |
|---|---|---|
| etcd | `initial-cluster-state=new`, self as initial-cluster | fetch CA → `member add` → `initial-cluster-state=existing` |
| CA / certs | generate cluster CA | **fetch CA from controller**, issue node cert under it |
| scylla intent | `SCYLLA_INSTALL_INTENT=initial-node`, `SCYLLA_BOOTSTRAP_INTENT=first-node` | `SCYLLA_INSTALL_INTENT=fresh-join`, `SCYLLA_CLUSTER_FINGERPRINT=<from etcd>`, seeds = **etcd node list** |
| packages | local dist `.tgz` | **local dist `.tgz` (identical)** |
| profiles | `FOUNDING_PROFILES` env | `--profiles` (default: inherit founder's set) |
| desired/BOM | writes active `release-index.json` | reads active BOM from etcd; installs matching local packages |

### Ordered install plan (join branch)

```
0. Preflight        : reachability, token valid, dist packages present locally, clock sane
1. PKI              : fetch cluster CA from controller gateway; issue node service cert under it
                      (this is the CA-fetch the tls.go fix already routes to the controller)
2. etcd JOIN        : write etcd.yaml (state=existing, no loopback peers);
                      `etcdctl member add`; start etcd; WAIT healthy + named-member visible
   └─ verify or ROLLBACK: `etcdctl member remove <self>`; stop etcd; wipe etcd data; exit 1
3. Read from etcd   : cluster CA (confirm), node list, cluster fingerprint, active BOM tag
4. scylla JOIN      : render scylla.yaml (cluster_name, seeds=node-list, listen=self);
                      run post-install with intent=fresh-join + fingerprint + ALLOW_STALE;
                      start scylla; WAIT in gossip ring / group0 verified
   └─ verify or ROLLBACK: `nodetool removenode`/decommission this node from group0
                          (authoritative, while founder still has quorum);
                          then step-2 rollback; exit 1
5. Rest of packages : install remaining local `.tgz` via globular-installer (minio, envoy, xds,
                      gateway, node-agent, services…) using etcd-sourced config
6. Hand off         : start node-agent; controller drives steady-state convergence (as Day-0)
```

Rollback is a **stack of compensations** unwound in reverse order — the installer already models ordered steps (`etcd_join_step.go` exists), so the compensation hook is a natural addition.

### Quorum-safe membership: stage as non-voter, promote on success (recommended)

To make rollback truly free, admit the newcomer **non-voting** first and promote only after `workload_ready`:

- **etcd**: `member add --learner` (learner = non-voting). Quorum math never changes while the node converges. Promote to voter at the end. Fail-before-promote ⇒ drop the learner, zero impact.
- **ScyllaDB**: bootstrap as **zero-token / limited-voter** (features present in this build) so it does not enter the group0 voter set or take RF=1 ranges until verified; promote after. Fail-before-promote ⇒ the founder's group0 was never weakened.

This is the structural fix for HARD RULE #5's spirit: a 1-node founder with RF=1 is the worst case (losing the 2nd node loses both quorum *and* the only replica of migrated ranges); staged promotion means a failed join can't take the founder down.

### Profiles as a parameter

`--profiles` flows into the JoinPlan → `approveJoinRecordLocked`, taking precedence over `deduceProfiles`. Default when omitted: **inherit the founder's profile set** (so `ai` and friends propagate and nodes are identical). `enforceFoundingProfiles` still guarantees the founding trio on top.

---

## What this reuses vs. what is new

**Reused (the bulk):** `install-day0.sh` structure, `globular-installer` engine + `run_script`/`etcd_join`/`install_local_debs` steps, `packages/scylladb/scripts/post-install.sh` (already correct), local dist `packages/`, node-agent + controller steady-state convergence.

**New (small, orchestration-only):**
- a `--join` branch in `install.sh` (fetch CA + `member add` instead of `new`; pass `fresh-join` + fingerprint + seeds + `--profiles`);
- per-step **verify + compensation** hooks (rollback stack);
- learner/zero-token **staged promotion** (etcd learner API + scylla zero-token flag);
- `--profiles` plumbing through the JoinPlan.

---

## Bugs this subsumes (traceability to the session)

| # | Failure | How the design removes it |
|---|---|---|
| 1 | scylla `fresh-join` intent never set | installer sets it explicitly, like Day-0 sets `initial-node` |
| 2 | envoy/gateway restart on join | CA is fetched once during install (step 1), not re-pushed to a running founder mid-join; **also** flagged: move steady-state cert rotation to Envoy **SDS** (hot reload) instead of process restart — separate patch |
| 3 | no rollback on join failure | ordered compensation stack + staged (learner/zero-token) promotion |
| 4 | orphaned build_ids from scylla RF=1 quorum loss | (4) is downstream of (3); preventing quorum loss prevents the cascade |
| 5 | gateway bundle download/version-skew | eliminated — install from local dist packages, no `/join/bundle` |
| 6 | `ai` profile dropped | `--profiles` / inherit-founder default |

---

## Rollout / migration

1. Land `--profiles` + inherit-founder default first (smallest, immediately useful, low risk).
2. Add the `install.sh --join` branch behind the existing join entry (gateway `/join` can invoke `install.sh --join` instead of the current script), keeping the old path until parity is proven.
3. Add staged learner/zero-token promotion + compensation stack.
4. Deprecate the standalone join-script + bundle assembler once `--join` reaches parity.
5. Fold the Envoy-SDS cert-rotation change in parallel (independent of the join unification).

Each step is independently shippable; no step requires a package version bump.

---

## Open questions

- **CA fetch bootstrap trust.** Step 1 fetches the CA over `-k`/insecure before trust is established (same as today). Acceptable for the bootstrap window, but should be pinned (token-bound fingerprint?) — worth deciding here.
- **Zero-token → token promotion mechanics** for ScyllaDB in this version (2025.3): confirm the exact flag/procedure to bootstrap non-voting and later take tokens, and whether RF should be raised to 3 before promotion.
- **etcd learner promotion timing**: promote at `etcd_ready`, or hold until `workload_ready`? Later is safer for rollback; confirm the controller's phase gates.
- **Concurrent joins**: out of scope now, but the ordered/staged model should be documented as one-at-a-time to keep quorum changes serialized.
- **Founder-profile inheritance vs. heterogeneous clusters**: default to inherit; keep `--profiles` as the explicit override for intentionally-different nodes.

---

## Implementation progress (2026-07-04)

Shipped to the working tree (not yet deployed — single node-agent/installer rebuild when ready), each with tests + AWG briefings:

- **Rollout step 1a — inherit-founder assignable profiles.** `cluster_controller/.../profiles_normalize.go` (`inheritableClusterProfiles`) + `handlers_join.go` (`approveJoinRecordLocked`). Joining nodes inherit the cluster's real catalog profiles except hardware-gated (`control-plane/storage/gateway`), opt-in `media-server`, and non-catalog labels (`ai`). Explicit operator profiles still win. Test: `inheritable_cluster_profiles_test.go`. (Also confirmed `ai` is derived-from-installed-services, not an assignable profile — it reaches a node via `core`/`control-plane`.)

- **Step 1b — ScyllaDB `fresh-join` intent wiring.** `globular-installer` `Options.ScriptEnv → Context.ScriptEnv → run_script_step` (append to `cmd.Env`); node-agent `infrastructure_actions.go` (`scyllaJoinScriptEnv`) sets `SCYLLA_INSTALL_INTENT=fresh-join` + `ALLOW_STALE_SCYLLA_REINIT_ON_JOIN=true` only for `scylladb` + `IsJoinActive()`. Fixes Day-1 scylla failing closed in `preserve` mode. Fenced by the CA-derived ownership fingerprint (never wipes an owning member). Tests: `scylla_join_script_env_test.go`, `run_script_env_test.go`. No scylla-package/build_id change.

## Appendix: staged etcd-learner join — detailed plan (NOT yet implemented)

This is the quorum-critical piece. Do it as a focused change with a real multi-node
etcd test harness — **do not** validate promotion against a single live founder.

Current path (verified): the Day-1 **join script / installer** `etcd_join_step.go`
runs `etcdctl member add` (full voter). The controller `etcd_members.go` *observes*
phases (`prepared → member_added → started → verified`) and only rolls back on
timeout. `etcd_members.go:409` documents the hazard verbatim: *"quorum requires 2/2
members once MemberAdd is called."* `EtcdJoinVerified` clears `EtcdMemberID`
(line 534).

Change:
1. **installer `etcd_join_step.go`**: `etcdctl member add <name> --learner --peer-urls=…`
   (etcd 3.5 supports learners). A learner is non-voting, so quorum stays 1 while the
   newcomer catches up — a failed join can never drop the founder below quorum.
2. **controller `etcd_members.go`**: add an `EtcdJoinPromoting` phase between
   `Started` and `Verified`. On entry, call `client.MemberPromote(memberID)`; if etcd
   returns "not in sync with leader," stay and retry next cycle (bounded by
   `etcdJoinTimeout` → rollback). Preserve `EtcdMemberID` until after promotion.
3. **testability prerequisite**: `etcdMemberManager.client` is a concrete
   `*clientv3.Client`. Extract an `etcdClientAPI` interface (`MemberList`, `MemberAdd`,
   `MemberAddAsLearner`, `MemberRemove`, `MemberPromote`) so the FSM's promote/retry
   logic can be unit-tested with a fake that simulates "not caught up → caught up".
4. **rollback** becomes trivially safe: fail-before-promote ⇒ remove the learner
   (quorum never changed). Wire this into the existing `rollbackJoin`.
5. **founding-quorum**: learners don't count toward `enforceFoundingProfiles` /
   RF eligibility — confirm the founding-quorum invariant only counts promoted voters.

Same staged pattern applies to ScyllaDB (zero-token / `GROUP0_LIMITED_VOTERS`), and
to auto-decommission on `ScyllaJoinFailed` — both to be scoped after the etcd learner
lands and is validated on a multi-node harness.
