# Design: advance the active platform release pointer on upgrade

Status: **proposed** (design-first; no code in this PR)
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

Add an **`activate_release` step** to the `platform.upgrade` workflow, after a
successful-enough `dispatch_upgrades`, that asks **node-agent** to rewrite its local
`release-index.json` to the upgraded tag's BOM.

```
evaluate → dispatch_upgrades → activate_release → audit
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

## Open questions (resolve before implementation)

1. **Activation predicate.** Advance the pointer when *all* dispatched SemVer
   packages converged? Or unconditionally to the synced tag once dispatch is
   "successful enough"? The safe default: advance only after the per-node reconciler
   reports the node converged to the tag's BOM (installed == desired for that tag's
   changed set). This avoids pointing at a platform the node hasn't actually reached.
2. **Per-node vs cluster-scoped.** `release-index.json` is per-node. In a multi-node
   cluster, nodes may converge at different times. The pointer is read per-node by
   that node's node-agent + Day-1 join uses the controller's view — decide whether
   each node advances its own file independently (preferred: matches per-node
   convergence) and whether the controller also keeps a cluster-level active-tag
   record in etcd for Day-1 join to read instead of any single node's file.
3. **etcd anchor.** Should the authoritative active-tag live in etcd
   (`/globular/platform/active_release`) with the node-local file as a materialization
   (so Day-1 join reads etcd, not a node file)? This better honors "etcd is the
   source of truth" and sidesteps per-node file skew. Strong candidate.
4. **Repository RPC.** Does a typed `GetReleaseIndex(tag)` already exist, or must it
   be added? (Sync parses it internally; confirm the surface.)
5. **Day-0 vs Day-1 writer unification.** The installer writes the file at day-0;
   this step writes it on upgrade. Ensure one validated format/owner so the two
   writers can't drift.

## Recommendation

Lead with **option 3 (etcd-anchored active tag)**: the controller writes the active
release tag to etcd on successful convergence; node-agent materializes
`/var/lib/globular/release-index.json` from the repository's synced BOM for that tag;
Day-1 join reads the etcd anchor. This keeps etcd as the single source of truth,
makes the node-local file a derived cache, and avoids per-node file skew. The
node-agent write stays within the system-executor boundary.

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
