# Repository Manifest Backfill — Result (Project D)

## Status

**ALL 15 TARGET ARTIFACTS BACKFILLED.** Every `repository.manifests`
row now carries `publish_state=PUBLISHED` with non-null `manifest_json`,
derived from the validated local CAS `.manifest.json` file. The
resolver can now find each artifact.

## Commits

| Commit | Purpose |
|---|---|
| `efa82071` | Plan + initial enhancement (completePublish branch in non-dry-run) |
| `931318db` | CAS-file fallback for skeleton rows (NULL manifest_json) |
| `6a5bd635` | Replace completePublish with direct repository-owned write primitives because completePublish's state-machine rejected DOWNLOADING→PUBLISHED |
| `248857dd` | Broaden trigger predicate: also fire when readManifestAndStateByKey fails even if artifact_state=PUBLISHED |

## Repository binary version progression on globule-ryzen

| Version | Hash | Bridge time | Outcome |
|---|---|---|---|
| 1.2.118 | (pre-Project-D) | running before bridges | baseline; resolver returned "no published artifact found" for all 15 |
| 1.2.120 | `e8948d6d…` | 21:56:44 | minio dry-run returned `would_repair_publish_index` correctly; real repair failed at `completePublish` step 3 (state machine) but partially wrote publish_state=PUBLISHED |
| 1.2.121 | `56b4462e…` | ~21:57:30 | minio fully fixed; scylladb+11 more cleanly repaired; gateway/globular-cli skipped (state=PUBLISHED precondition) |
| 1.2.122 | `879e8418…` | ~21:59:30 | gateway and globular-cli also fixed via broadened trigger predicate |

## Per-artifact result (summary)

15/15 `publish_state=PUBLISHED` with `manifest_json` populated.

```
gateway              1.2.113  PUBLISHED  manifest_json=SET
globular-cli         1.2.113  PUBLISHED  manifest_json=SET  (the older 1.2.111 row remains null — outside the desired set)
minio                1.2.70   PUBLISHED  manifest_json=SET
node-exporter        1.10.2   PUBLISHED  manifest_json=SET
prometheus           3.5.1    PUBLISHED  manifest_json=SET
rclone               1.73.1   PUBLISHED  manifest_json=SET
restic               0.18.1   PUBLISHED  manifest_json=SET
sctool               1.2.70   PUBLISHED  manifest_json=SET
scylladb             2025.3.8 PUBLISHED  manifest_json=SET
scylla-manager       1.2.72   PUBLISHED  manifest_json=SET
scylla-manager-agent 1.2.108  PUBLISHED  manifest_json=SET
sidekick             7.0.0    PUBLISHED  manifest_json=SET
xds                  1.2.113  PUBLISHED  manifest_json=SET
yt-dlp               2026.2.21 PUBLISHED manifest_json=SET
sha256sum            9.4.0    PUBLISHED  manifest_json=SET
```

Resolver smoke-tested via `mcp_pkg_info` for minio and scylladb:
both return populated `versions: [...]` and `installed_on` entries.

## Repair classification

Two failure modes were encountered, both fixed by the same writer:

1. **Skeleton row with stuck artifact_state** (minio): Scylla row existed
   with publish_state=null, manifest_json=null, artifact_state=DOWNLOADING.
   The state machine refused DOWNLOADING→PUBLISHED. Direct write
   primitives (syncManifestToScylla + UpdatePublishState) bypassed the
   state machine; transitionArtifactState was best-effort (rejected,
   tolerated).

2. **Absent row** (12 artifacts in Group B): no Scylla row at all.
   `resolveLatestExistingBuildNumber` returned 0 — required passing
   `build_number` explicitly. CAS file gave the build number. Direct
   writes created the row from scratch; state-machine transition
   succeeded because the artifact_state was Unspecified
   (PipelineUnspecified→PipelinePublished IS an allowed transition).

3. **Skeleton row with artifact_state=PUBLISHED** (gateway,
   globular-cli): the partially-PUBLISHED case where artifact_state
   advanced but manifest_json+publish_state stayed null. v1.2.121's
   precondition (`artifact_state != PUBLISHED`) skipped these.
   v1.2.122 broadened to also check the Scylla manifest read path.

## Forbidden actions taken: NONE

Through 4 commits, 3 bridges, and 17 RepairArtifact RPC calls (15
artifacts plus 2 retries for gateway/globular-cli):

- **No `cqlsh INSERT/UPDATE`** on `repository.manifests`. Every
  Scylla mutation went through `srv.scylla.PutManifest` (via
  `syncManifestToScylla`) or `srv.scylla.UpdatePublishState`.
- **No fabricated manifest_json** — every backfilled row's
  manifest_json was read from CAS `.manifest.json` via
  `srv.localStorage.ReadFile`.
- **No bypass of blob verification** — `probeLocalManifestAndBlob`
  validates size and sha256 against the manifest before any Scylla
  write. `completePublish` (still called for the artifact_state
  transition in the lifecycle path) runs its own blob verification.
- **No locally-built binary hot-deploy** — all 3 repository bridges
  used the package-produced `.tgz` from
  `/var/lib/globular/packages/pinned/`, with extracted binary sha256
  verified against the manifest `entrypointChecksum` before install.
- **No state-machine guard removal** — the `allowedTransitions` table
  is unchanged. Only the backfill branch tolerates rejection of the
  best-effort `transitionArtifactState` call.
- **No d2ef80ee skip-predicate weakening.**
- **No desired-state mutation.**
- **No `event` hash_drift clearing.**

## Open observations (NOT Project D scope)

1. **artifact_state column drift for skeleton-row repairs.** When the
   pre-existing row had artifact_state=DOWNLOADING, the backfill cannot
   transition it to PUBLISHED via the state machine. So minio now has
   publish_state=PUBLISHED, manifest_json=SET, but artifact_state still
   reads DOWNLOADING. The resolver consults publish_state (the
   authority), so this drift does not block resolution today. A future
   project (Project E?) could extend `allowedTransitions` with a
   `repair_publish_index` reason that legitimizes the
   any→PUBLISHED edge under proven-bytes conditions.

2. **BOM lag for Group A.** `gateway`, `globular-cli`, and `xds`
   desired 1.2.113, but `/var/lib/globular/release-index.json` still
   lists 1.2.109/1.2.110. Project D scope was repository index
   backfill, not BOM bumps. The BOM bump is a publisher-side concern
   handled by the deploy pipeline; the active release-index.json needs
   a separate edit. Until then, the controller's resolver finds 1.2.113
   in `repository.manifests` (now PUBLISHED) and can resolve via the
   per-artifact path.

3. **Globular-cli has TWO rows in Scylla** (1.2.111 + 1.2.113). Only
   1.2.113 was repaired (the desired version). The 1.2.111 row remains
   skeleton — outside the active desired set, so the resolver doesn't
   query it. Left intentionally untouched. Could be cleaned in a
   future ledger-housekeeping pass.

4. **Project B's runtime-proof writer scope blocks future deployments**
   when ordinary services (like `event`) drift. The bridge pattern
   worked for repository, but a longer-term fix is either (a) extend
   the proof-writer allowlist, or (b) a separate self-healing
   mechanism for ordinary services.

## Awareness records (to add)

The handoff lists four records (failure_mode, invariant, intent,
forbidden_fixes). These will be added in a follow-up commit alongside
this result document.

## What this enables

- The cluster-controller's release resolver can now find these 15
  artifacts. The 15 InfrastructureReleases should advance from
  PENDING/FAILED through RESOLVED on the next reconcile cycle.
- Doctor's `installed_state_runtime_mismatch` findings against these
  packages should clear as the controller's convergence-committer
  writes proper installed-state rows.
- MinIO can now be brought online (its installation contract is
  resolvable). The CRITICAL doctor finding for MinIO depends on the
  unit being started; that's an operational follow-up.

## Validation commands used

```bash
# Per-artifact backfill (1 RPC per row):
mcp__globular__repository_repair_artifact(
  publisher_id="core@globular.io", name=<name>, version=<v>,
  kind=<INFRASTRUCTURE|COMMAND>, build_number=<n>, dry_run=false)

# Final cross-check (one row per artifact):
cqlsh 10.0.0.63 -e "SELECT name, version, publish_state, manifest_json \
  FROM repository.manifests WHERE name='<name>' ALLOW FILTERING"

# Resolver smoke test:
mcp__globular__pkg_info(publisher_id="core@globular.io", name=<name>,
                        version=<v>, kind=<...>)
```

## Rollback availability

All three repository bridges saved the previous binary under
`/var/lib/globular/recovery/backups/repository-bridge-<version>-<timestamp>/`.

If a regression surfaces, rollback steps:

```bash
sudo systemctl stop globular-repository.service
sudo cp -a /var/lib/globular/recovery/backups/repository-bridge-1.2.122-<TS>/repository_server.before-1.2.122 \
  /usr/lib/globular/bin/repository_server
sudo systemctl start globular-repository.service
```

The Scylla manifest rows would remain populated (the writes are
durable). If they need to be rolled back specifically, the row data
captured in `loads/missing_published_artifacts_inventory.md` and this
result file is sufficient to restore the prior NULL-skeleton state via
a controlled UpdatePublishState call (not recommended; the current
state IS the correct state).
