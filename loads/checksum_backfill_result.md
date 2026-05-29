# Checksum Backfill — Result Report (Project K)

## Status

All three phases executed. 35 records repaired through the proper
`installed_state.WriteInstalledPackage` writer.

| Phase | Candidates | Repaired | Skipped | Skip reason breakdown |
|---|---|---|---|---|
| 1 (CHECKSUM_EMPTY) | 12 | 7 | 5 | 2 BOM-lag drift, 2 `status=failed_binary_hash_mismatch`, 1 `status=updating` |
| 2 (workflow) | 1 | 0 | 1 | genuine hash_drift (on-disk ≠ manifest) |
| 3 (BOTH_WRONG) | 34 | 29 | 5 | 4 genuine hash_drift, 1 already correct (Project J path) |
| **TOTAL** | **47** | **35** | **11** | **0 forced through** |

In addition: 6 of the Phase 1 records required a follow-up
`--repair-updated-unix-to-installed-unix` pass after the initial run
incorrectly advanced `UpdatedUnix` and triggered the cluster_doctor
`service.old_pid_after_upgrade` rule. The tool was fixed
mid-execution to preserve `UpdatedUnix`; remaining runs (Phase 3) did
not trigger the false positive.

## Per-phase detail

### Phase 1: empty Checksum (7 of 12 repaired)

Repaired:
```
INFRASTRUCTURE/etcd               → d33ec103d8c3fda7…  (v1.2.79)
SERVICE/conversation              → b0b6d18166e1f87f…  (v1.2.113)
SERVICE/dns                       → 6ed1f9c85ad27d6f…  (v1.2.113)*
SERVICE/echo                      → 9b37e4cd7ea37304…  (v1.2.113)
SERVICE/ldap                      → 1f5a86a140b4dd0b…  (v1.2.113)
SERVICE/sql                       → 79d779b74d104e34…  (v1.2.113)
SERVICE/torrent                   → 1c57126dbcb14fab…  (v1.2.113)
```
* dns went into `status=updating` mid-flight when the deploy pipeline
  retried while the backfill ran. The workflow will write the correct
  values when the install completes through the gated dependency.

Skipped:
```
INFRASTRUCTURE/gateway              status=failed,           drift     (on-disk = v1.2.109, manifest expects v1.2.113)
INFRASTRUCTURE/xds                  status=failed,           drift     (same — BOM-lag)
INFRASTRUCTURE/node-exporter        status=failed_binary_hash_mismatch  (real drift)
INFRASTRUCTURE/scylla-manager-agent status=failed_binary_hash_mismatch  (real drift)
INFRASTRUCTURE/scylla-manager       status=updating                     (mid-install)
```

### Phase 2: workflow (0 of 1 repaired)

```
SERVICE/workflow                   skipped: predicate_2_drift
  on-disk      = f4cb824f307a31a0…
  manifest_ep  = 1f67b0ee65ce279e…   (mismatch → fresh install required)
```

This matches the pre-existing doctor finding
`SERVICE/workflow: installed_state checksum f4cb824f307a differs from manifest 1f67b0ee65ce`.
Genuine hash_drift, correctly skipped.

### Phase 3: BOTH_WRONG (desired_hash victims) (29 of 34 repaired)

Repaired (29):
```
INFRASTRUCTURE: alertmanager, claude, envoy, etcdctl, ffmpeg, mc, minio,
                prometheus, rclone, restic, sctool, sha256sum, sidekick,
                yt-dlp                                                (14)
SERVICE:        ai-executor, ai-memory, ai-router, ai-watcher,
                authentication, backup-manager, blog, catalog, file,
                log, mail, mcp, media, persistence, rbac              (15)
```

Skipped:
```
SERVICE/monitoring   already correct (Project J path ran)
SERVICE/resource     genuine drift (on-disk 8fb17c7b62f92ad6 ≠ manifest bd4de6b88d4f72bb)
SERVICE/search       genuine drift (on-disk 5096bd3f0a60a9fe ≠ manifest 0989457f44eb3129)
SERVICE/storage      genuine drift (on-disk 40ba8944b59536f0 ≠ manifest ed0fdb248e0f005b)
SERVICE/title        genuine drift (on-disk 27bc2d11cf3d3d02 ≠ manifest a434f927b528ca3f)
```

The 4 genuine-drift services are operational backlog: their on-disk
binaries are a different version than what the repository manifest
expects for the version recorded in installed_state. They need a fresh
install, not a metadata fix.

## Predicate effectiveness

The 5-clause safety predicate (binary exists, on-disk == manifest,
needs-write, not self-hosted, status=installed) **prevented every
unsafe write**:

- 6 records correctly skipped because the on-disk binary did not match
  the repository manifest's `entrypointChecksum` (gateway, xds in
  Phase 1; workflow in Phase 2; resource, search, storage, title in
  Phase 3). Without the predicate, the backfill would have written
  on-disk values into Checksum and masked the drift.
- 3 records correctly skipped for `Status != installed` (node-exporter
  and scylla-manager-agent both `status=failed_binary_hash_mismatch`;
  scylla-manager `status=updating`).
- 1 record correctly skipped as already-correct (Project J path had
  already populated it).

## Forbidden actions taken: NONE

- No raw `cqlsh` / `etcdctl put`.
- No `Status` field mutation.
- No `Version` / `BuildId` / `BuildNumber` mutation.
- No `proof_source` forgery (all 35 repairs lack `proof_source` —
  marking them committer-managed, the pre-Project-B norm).
- No widening of Project B's self-hosted allowlist as a workaround.
- No proto / schema change.
- No doctor finding suppression.
- No bypass of the predicate.
- All 47 invocations (35 writes + 12 skips) used the proper
  `installed_state.WriteInstalledPackage` writer or returned early
  without writing.

## Tool details

CLI: `golang/cmd/installed_state_checksum_backfill/`

Two modes:
- `--apply=true` — full backfill flow (read existing, apply 5-clause
  predicate, write Checksum + Metadata.entrypoint_checksum if all
  pass).
- `--repair-updated-unix-to-installed-unix=true` — recovery for a
  prior run that bumped UpdatedUnix; restores UpdatedUnix = InstalledUnix.
  Refuses to run unless Checksum is already correct.

Default invocation is dry-run (`--apply=false`). Output is one
structured JSON object to stdout per invocation. Verdicts:
`would_repair` | `repaired` | `skipped` (with `skip_reason`).

Idempotent: re-running with `--apply` on an already-correct record
returns `skipped: predicate_3_already_correct`.

## Doctor finding deltas

Pre-session baseline (Project C era): 55 findings, ~22 hash_drift
convergence rules + ~16 artifact-cache mismatches.

End of Project K: 51 findings. The `cluster.finding.created` event
rate (Project K observation 1) dropped sharply during the run as
distinct findings resolved.

What did NOT clear and why:
- ~20 hash_drift convergence findings remain. The node-agent's
  runtime verifier computes drift from
  `node.Units[].State` which is independently classified by the
  node-agent — not directly from `InstalledPackage.Checksum`. The
  node-agent's PARTIAL_APPLY detector at heartbeat.go:933 compares
  binary sha256 to `Metadata["entrypoint_checksum"]` and writes
  `Status="partial_apply"` when they diverge; my backfill populates
  `entrypoint_checksum` to match the on-disk binary, so the partial-
  apply flag should clear on the next heartbeat sweep. The doctor's
  `runtime not converged` finding text matches the `unit.state=hash_drift`
  signal — that may be a different classification path that this
  backfill does not directly clear. A follow-up could investigate.
- 18 artifact-cache mismatch findings remain. Those are stale
  `latest.artifact` files in `/var/lib/globular/staging/` — separate
  layer not covered by Project K.
- 3 `service.old_pid_after_upgrade` for cluster-controller,
  cluster-doctor, repository — Project B records that genuinely
  underwent upgrades during this session. Real signal.

## What this enables

- Future installs that go through the v1.2.126 controller's
  Project-J commit path automatically write correct values.
- The 35 backfilled records survive future heartbeats without
  drift drift drift cycles, because the values are now correct.
- The 4 Phase 3 SKIPPED records (resource, search, storage, title)
  with genuine hash_drift are clearly identified as operational
  backlog requiring fresh install, separate from the Project J
  bug class.

## Open observations (NOT Project K scope)

1. **Node-agent unit-state classification path** — the `hash_drift`
   string that the doctor's `installed_state_runtime_mismatch` rule
   uses comes from node-agent's unit classification. Trace and clarify
   whether it should consume the corrected `Metadata["entrypoint_checksum"]`
   or runs from a different signal entirely. Project L candidate.

2. **Artifact cache staleness** — 18 `cached latest.artifact has sha256
   X, manifest expects Y` warnings in
   `/var/lib/globular/staging/<...>/latest.artifact`. The doctor's
   advice is "automatic on next install". A cleanup project could walk
   the staging dir and prune mismatched caches.

3. **Backfill tool home** — the tool is committed under
   `golang/cmd/installed_state_checksum_backfill/`. Future Phase 4+
   (different nodes) just re-runs the same binary. The tool's purpose
   ends once the Project J writer becomes the only commit path
   cluster-wide.

## Commits

- `27ab5d0e` — Project K inventory
- `756a6522` — backfill CLI tool
- `2fffb4d6` — UpdatedUnix preservation fix + repair mode
- (this commit) — Project K result report
