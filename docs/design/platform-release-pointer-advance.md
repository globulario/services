# Design: advance the active platform release pointer on upgrade

Status: **decided** — direction approved 2026-06-27 (see Decision below). Implementation is a separate feature PR; this PR carries the design + decision only, no code.
Author: 2026-06-27
Related scars: ai-memory `880db878` (stale active-BOM pointer), `81f0a329` (ffmpeg
dispatch — fixed separately in services#178)
Related awareness: `release.failed_dispatch_leaves_active_bom_pointer_stale`

## Problem

`/var/lib/globular/release-index.json` is the **active platform BOM** — the
authoritative answer to "what platform release is this cluster on." It is read by:

- `repository_active_release` (operator/MCP view of platform truth),
- `node_agent/.../workflow_runner.go` (`resolveVersionFromReleaseIndex`, the Day-1
  degraded-path version fallback),
- Day-1 join (a joining node provisions from the **active BOM**, not `latest`).

**It is written exactly once — at install/day-0 time** (the platform tarball places
it). Grep confirms **no Go service writes it in production** (only tests do). So
after install, *nothing* advances it: per-service `deploy`, `services desired set`,
and `platform-upgrade` all move desired/installed state forward while the active
pointer stays frozen at the install-time tag.

Live evidence (2026-06-27, single-node cluster): services run 1.2.249–1.2.250,
but `repository_active_release` reports `platform_release: 1.2.233` /
`release_tag: v1.2.233` — the tag the node was installed with.

### Why this is a real hazard, not cosmetic

CLAUDE.md: *"Day-1 joins from active BOM, not latest"* and *"release-index.json is
the platform truth."* A node joining this cluster reads the active BOM (v1.2.233)
and would provision the **stale** platform, diverging from the running 1.2.249/250
fleet. The platform-truth marker disagreeing with running state is a latent Day-1
correctness bug; it is currently masked only because no join is happening.

### Why it is NOT "a skipped post-dispatch step"

Earlier notes assumed `platform.upgrade` had a pointer-advance step that the ffmpeg
dispatch failure skipped. **It does not.** The workflow is
`evaluate → dispatch_upgrades → audit` (see
`workflow/engine/actors_platform_upgrade.go`). None of these writes
`release-index.json`. The pointer advance is a **missing feature**. Fixing the
ffmpeg dispatch bug (services#178) makes the workflow SUCCEED but still does not
advance the pointer.

## Constraints / invariants to honor

- **etcd is the source of truth for desired state**; `release-index.json` is a
  node-local materialization of platform truth, not a second authority. The active
  tag should be derivable/anchored from an authoritative record, not invented.
- **node-agent is the system executor.** Writing a node-local file under
  `/var/lib/globular/` is node-agent's job. The controller decides; node-agent
  executes (`controller.decides_but_does_not_execute_leaf_work`).
- **cluster_controller must not write node-local files** (no `os`/file mutation of
  `/var/lib/globular` from the controller).
- **All cluster mutations flow through workflows** and must be idempotent + reach a
  terminal state.
- Must not regress the platform pointer (never point at an older tag than the
  install-time/active one) without an explicit, audited override — mirror the
  desired-state no-regression floor.

## Proposed design

> Refined by the **Decision** section below: `activate_release` is **convergence-gated**
> and lives in the per-node convergence path (after `verify_runtime`), not as a
> post-dispatch step in `platform.upgrade`. `platform.upgrade` (evaluate →
> dispatch_upgrades → audit) only sets *desired* state; the pointer advances per node
> after that node actually converges, and the controller computes the cluster-level
> active tag from per-node reports.

Add a convergence-gated **`activate_release` step** that asks **node-agent** to
materialize its local `release-index.json` projection from the upgraded tag's
verified BOM — only after the node has converged to that tag.

```
sync_bom → download_artifacts → install_or_update → verify_runtime → activate_release → publish_status
```

### Source of the new BOM

The repository already holds the synced tag's `release-index.json` (the
`repo sync --tag vX` step imports it into the local repository's PUBLISHED BOM).
node-agent fetches that BOM for the tag from the repository (typed RPC,
mesh-routable) and writes it atomically to `/var/lib/globular/release-index.json`.
No new copy of the BOM is invented; the repository's synced document is the source.

### Ownership split

- **controller** (decide): the `activate_release` action resolves the target tag,
  verifies the synced BOM exists in the repository, and dispatches the node-agent
  write action for each node (or a cluster-scoped action). It does **not** touch the
  file.
- **node-agent** (execute): receives the tag (or the BOM bytes), validates it
  (`ValidateReleaseIndexForInstall`), and writes `/var/lib/globular/release-index.json`
  **atomically** (temp + rename). Idempotent: writing the same tag is a no-op.
- **repository** (provide): serves the synced tag's BOM (likely an existing path —
  `release-show` / the sync already parsed it; confirm whether a typed
  `GetReleaseIndex(tag)` RPC exists or must be added).

### Idempotency + partial-failure

- Re-running `platform.upgrade vX` when the pointer is already `vX` → no-op.
- The activate step runs **even if dispatch had per-package failures** for native
  packages that legitimately did not converge (e.g. a package the operator removed),
  *provided* the dispatch's SemVer targets succeeded — i.e. advance the pointer to
  reflect the platform the fleet actually converged to. Open question below on the
  exact predicate.
- No-regression: refuse to point at a tag older than the current active tag unless
  `--allow-regression` (audited), mirroring `enforceServiceDesiredFloor`.

## Decision (approved 2026-06-27)

**etcd-anchored, convergence-gated, explicit activate step.** This gives the
pointer a spine: no more frozen install-era fossil pretending to be today's release.

1. **Authority — etcd owns the active release pointer.** The cluster/source-of-truth
   pointer lives in etcd (e.g. `/globular/platform/active_release`).
   `/var/lib/globular/release-index.json` becomes a **node-local materialized
   projection/cache**, never the authority. Rationale: a per-node-file authority
   recreates the split-brain shape (node A says v1.2.244, node B says v1.2.233, and
   every doctor/verifier reads tea leaves). etcd gives one declared active-release
   truth; node-agent writes the local file for fast local readers and boot
   continuity. So:
   - **etcd `active_release` pointer = authority**
   - **`release-index.json` = node-local projection/cache**
   - **repository synced BOM = source payload** used to materialize the projection

2. **Activation predicate — advance only after convergence, not dispatch.** The
   active pointer means "this node is now operating against this release," not "we
   hope it gets there." Three-state model per node:
   - `desired_release` = target tag selected by the controller
   - `prepared_release` = BOM/artifacts available + hash-verified locally
   - `active_release` = node converged and activated

   `activate_release` may write the local `release-index.json` only after **all** of:
   the BOM for the tag is present and hash-verified; the required artifacts/specs for
   this node are present; node-agent (system executor) completed activation; and
   controller-visible state reports converged/accepted-ready for that tag. The
   controller computes cluster-level active status from per-node reports.

3. **Workflow — `activate_release` is a first-class, explicit step**, never hidden
   in download/install/join/system-executor side effects:

   ```
   sync_bom → download_artifacts → install_or_update → verify_runtime → activate_release → publish_status
   ```

   `activate_release` is the **only** step permitted to materialize/update
   `/var/lib/globular/release-index.json`. This yields a clean AWG invariant:
   **release-index.json is written only by `activate_release`, from the etcd active
   release and a verified BOM.**

4. **Day-1 join — derive the target tag from etcd, not local disk.** Join reads the
   cluster desired/active release from etcd → node-agent syncs the matching BOM →
   node converges → `activate_release` writes the local projection → node reports its
   active tag. Local disk may be stale, empty, or inherited from an old install — it
   is luggage, not law.

5. **Compatibility / fallback — degraded, read-only, non-advancing.** When etcd is
   unavailable (bootstrap/outage): the local `release-index.json` may be read as a
   **last-known-local projection** only; it must NOT be treated as authoritative
   cluster truth, must NOT advance itself, and doctor must report
   *"active release authority unavailable"* rather than pretending the file is
   current.

### Resolved secondary points

- **Repository RPC.** Confirm during implementation whether a typed
  `GetReleaseIndex(tag)` exists or must be added; the synced BOM is the materialization
  source regardless.
- **Day-0/Day-1 writer unification.** The installer's day-0 write and the
  `activate_release` write must share one validated format/owner; long-term the day-0
  seed should also flow through (or be reconciled against) the etcd anchor so the two
  writers cannot drift.

## Scope of the eventual implementation PR(s)

1. (maybe) proto: `GetReleaseIndex(tag)` on repository if absent; node-agent
   activate action.
2. controller: `activate_release` workflow action + step wiring in
   `actors_platform_upgrade.go`; etcd active-tag write.
3. node-agent: atomic local `release-index.json` writer (validated, idempotent),
   invoked via the supervisor-free file path (read/write of a state file, not a
   systemd action).
4. Day-1 join: read the etcd active-tag anchor.
5. tests: idempotent re-activate, no-regression refusal, partial-dispatch behavior,
   Day-1 reads the advanced pointer.

No code is included here pending review of the open questions — especially
(1) the activation predicate and (3) the etcd-anchor decision.
