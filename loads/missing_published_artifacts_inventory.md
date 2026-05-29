# Missing Published Artifacts — Inventory (Project C)

## Headline

15 InfrastructureRelease entries cannot resolve their published artifact via
the controller's resolver. The resolver queries `repository.manifests`
(Scylla) and rejects rows with `publish_state != PUBLISHED` or that simply
do not exist.

In every single case the artifact bytes ARE present:

- The repository CAS at `/var/lib/globular/repository/artifacts/` carries
  a `<key>.bin` + `<key>.manifest.json` with `publishState=PUBLISHED`.
- The local install dir at `/var/lib/globular/packages/` carries the
  matching `<name>_<version>_<platform>.tgz` (the version matches desired
  in 12/15 cases, lags in 3/15 cases).
- The BOM at `/var/lib/globular/release-index.json` carries an entry
  (the version matches desired in 12/15 cases, lags in 3/15 cases).

The break is exclusively in the **Scylla `repository.manifests` table**:
either the row is entirely absent or it exists in a skeleton state with
`publish_state=null` AND `manifest_json=null`. This is the same failure
class as INC-2026-0012 (the v1.2.113 sync that produced 24 skeleton
rows). The fix from commit d2ef80ee — make the skip predicate require
`manifestJSONPresent` — closes the gap going forward but does not
backfill the skeleton rows that were created before the fix landed.

Project C scope: **inventory only**, no repair.

## Group A — BOM lag (3 artifacts)

Desired version moved past the BOM. Same underlying Scylla skeleton-row
issue, plus the additional gap that the BOM (release-index.json) doesn't
carry the desired version.

| Artifact | Desired | BOM | Local archive | Scylla row | publish_state |
|---|---|---|---|---|---|
| gateway | 1.2.113 | 1.2.109 (b352) | 1.2.109 | YES (1.2.113 b364) | null |
| globular-cli | 1.2.113 | 1.2.110 (b360) | 1.2.110 | YES (1.2.111 + 1.2.113) | null on both |
| xds | 1.2.113 | 1.2.109 (b352) | 1.2.109 | NO ROW | (n/a) |

Classification: `missing_from_bom` (primary) +
`missing_from_repository` (compound, because Scylla index is also broken).

Risk:
- `gateway` and `xds` are **high** — control-plane critical components
  running at the older 1.2.109 bytes; desired wants 1.2.113.
- `globular-cli` is **low** — CLI tool not auto-deployed.

## Group B — Scylla index broken, BOM and CAS healthy (12 artifacts)

Desired version, BOM, local archive, and CAS manifest all agree. Only the
Scylla `repository.manifests` row is broken (NULL publish_state for
minio; entirely absent for the other 11).

| Artifact | Desired | Scylla row | Runtime | Risk |
|---|---|---|---|---|
| minio | 1.2.70 | YES (null publish_state) | inactive (CRITICAL doctor finding) | high |
| scylladb | 2025.3.8 | NO ROW | (uses scylla-server.service externally) | high |
| sidekick | 7.0.0 | NO ROW | active | medium |
| scylla-manager | 1.2.72 | NO ROW | active | medium |
| scylla-manager-agent | 1.2.108 | NO ROW | active | medium |
| node-exporter | 1.10.2 | NO ROW | active | medium |
| prometheus | 3.5.1 | NO ROW | active | medium |
| rclone | 1.73.1 | NO ROW | on-demand | low |
| restic | 0.18.1 | NO ROW | on-demand | low |
| sctool | 1.2.70 | NO ROW | on-demand | low |
| yt-dlp | 2026.2.21 | NO ROW | on-demand | low |
| sha256sum | 9.4.0 | NO ROW | utility | low |

Classification: `missing_from_repository` for all 12.

The dominant remediation is the same one INC-2026-0012's d2ef80ee fix
already enables: a `repository sync` (or import-resume) run that
re-imports each of these 12 artifacts. Since the CAS files are intact,
the import path should re-create the Scylla row with
`publish_state=PUBLISHED` and `manifest_json` populated. No republish is
needed.

## What this report explicitly does NOT propose

- No `etcd put` against any desired-state record.
- No `cqlsh INSERT/UPDATE` against `repository.manifests` to set
  `publish_state=PUBLISHED` manually. The fix path is the import-resume
  code; manually flipping a NULL `publish_state` to PUBLISHED without a
  `manifest_json` would still be skeleton.
- No deletion of any desired-state entry.
- No fabrication of new manifest rows from scratch.
- No removal of the resolver's `publish_state=PUBLISHED` filter.
- No global alias broadening or platform-fallback broadening.

## Open observations (NOT part of Project C scope)

1. **All 15 failing artifacts share the same root cause**: skeleton
   `repository.manifests` rows from a pre-d2ef80ee sync, OR entirely
   absent rows from a partial publish/sync failure. The d2ef80ee fix
   (manifestJSONPresent skip predicate) is in v1.2.116+. Re-running the
   sync for these specific artifacts should backfill them. That would
   be a separate project (call it Project D — Repository Index
   Backfill), gated on the inventory closure.

2. **For Group A (BOM-lag)**: a BOM bump is also needed for gateway,
   globular-cli, xds to 1.2.113. The publish to BOM is part of the
   normal deploy pipeline. The gap is that these three were deployed
   (CAS published) but BOM was not advanced.

3. **The skeleton-row failure mode is a partial-write atomicity gap**:
   `UpdateArtifactState` and `PutManifest` aren't atomic. The publish
   pipeline writes the row first (skeleton: NULLs), then attempts to
   set publish_state, then attempts to write manifest_json. Any
   interruption between steps leaves a partial row. INC-2026-0012's
   fix made the SKIP predicate strict; making the WRITE atomic is a
   harder problem (Scylla LWT or a different schema).

4. **node-agent, cluster-controller, cluster-doctor** (Project B
   territory) all have Scylla rows in PUBLISHED state — they are NOT
   in this inventory. The Project C gap is specifically the 15 IRs
   listed above.

## Recommended next step (not in Project C)

A Project D handoff for the user could authorize:

1. `globular repository sync` (or the matching MCP tool) against the
   15 artifacts to backfill the skeleton/missing rows.
2. A BOM bump for gateway, globular-cli, xds to v1.2.113.
3. After backfill, the controller's resolver should find all 15
   artifacts as PUBLISHED and the 15 IRs should transition through
   RESOLVED → AVAILABLE on the next reconcile cycle.

Until then, the cluster runs in a degraded state where these 15
InfrastructureReleases are stuck at PENDING/FAILED phase but the
underlying systemd units (where applicable) are mostly active. The
mismatch is between the controller's installed_state authority and the
runtime reality — the same family of issue Project B solved for
self-hosted components.

## Validation commands (read-only)

```bash
# Confirm CAS manifest presence for an artifact:
sudo ls /var/lib/globular/repository/artifacts/ | \
  grep "^core@globular.io%<name>%"

# Confirm Scylla row presence + publish_state:
cqlsh 10.0.0.63 -e "SELECT name, version, platform, publish_state \
  FROM repository.manifests WHERE name='<name>' ALLOW FILTERING"

# Confirm BOM version:
sudo cat /var/lib/globular/release-index.json | \
  python3 -c "import sys,json; d=json.load(sys.stdin); \
  [print(p) for p in d['packages'] if p['name']=='<name>']"

# Confirm InfrastructureRelease status:
sudo etcdctl --endpoints=https://10.0.0.63:2379 \
  --cacert=/var/lib/globular/pki/ca.crt \
  --cert=/var/lib/globular/pki/issued/services/service.crt \
  --key=/var/lib/globular/pki/issued/services/service.key \
  get /globular/resources/InfrastructureRelease/core@globular.io/<name>
```

## Awareness records (to add, NOT yet committed)

See the handoff document — the failure_mode and invariant blocks should
be added to `docs/awareness/failure_modes.yaml` and
`docs/awareness/invariants.yaml`. They are NOT added in this commit
because the result/repair phase is gated on user authorization for a
follow-up Project D.

Proposed text:

```yaml
# failure_modes.yaml
- id: desired_state.references_missing_published_artifacts
  summary: |
    Desired-state IRs reference artifacts whose Scylla repository.manifests
    row is either entirely absent or present in a skeleton state
    (publish_state=null AND manifest_json=null), even though the
    repository CAS carries a fully-formed manifest with
    publishState=PUBLISHED and the local install dir carries the
    matching .tgz. The controller's resolver queries Scylla and rejects
    skeleton/absent rows; the BOM lookup queries release-index.json and
    may also lag the desired version (Group A).
```

```yaml
# invariants.yaml
- id: desired_state_must_reference_resolvable_artifacts
  severity: critical
  statement: |
    Every active desired-state InfrastructureRelease (and ServiceRelease)
    must reference an artifact that is simultaneously:
      (a) present in the repository CAS as a fully-formed manifest with
          publishState=PUBLISHED,
      (b) present in the Scylla repository.manifests table with
          publish_state=PUBLISHED AND manifest_json non-null,
      (c) present in the active BOM (release-index.json) at the desired
          version, and
      (d) materialised as a local .tgz in /var/lib/globular/packages/
          (or its pinned/ subdirectory) at the matching desired version.
    Skeleton rows in repository.manifests (publish_state=null OR
    manifest_json=null) MUST cause the resolver to report a structured
    reason, never silently degrade to "no published artifact found".
```

## What classifications I did NOT use, and why

- `desired_state_stale` — desired state for these 15 is correct
  intent; the problem is the artifact side, not the desire.
- `external_dependency_not_packaged` — all 15 ARE packaged (CAS +
  local + BOM).
- `profile_excluded` — these are needed in the active profile of
  globule-ryzen (control-plane + core + storage).
- `intentionally_unmanaged` — these are managed; the management chain
  just has a broken index step.
- `release_index_delta_gap` — partially fits Group A (BOM lag) but the
  primary problem is the Scylla skeleton row, so I labelled Group A
  as `missing_from_bom` (the more specific issue) and noted the
  compound nature in evidence.
- `name_alias_mismatch` — all 15 use canonical names.
- `platform_resolution_gap` — all 15 have linux_amd64 manifests on a
  linux_amd64 node; no platform mismatch.
- `local_materialization_missing` — local archives are present in
  every case.
- `unknown_impact` — every row has bounded impact assessment.

## Summary table

| Group | Artifacts | Classification | Risk distribution |
|---|---|---|---|
| A: BOM lag | gateway, globular-cli, xds | missing_from_bom | high(2), low(1) |
| B: Scylla index broken | minio, scylladb, sidekick, scylla-manager, scylla-manager-agent, node-exporter, prometheus, rclone, restic, sctool, yt-dlp, sha256sum | missing_from_repository | high(2), medium(5), low(5) |

Total: 15 IRs, 4 high-risk, 5 medium-risk, 6 low-risk. None unknown.
