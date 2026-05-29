# Project S package deploy — verification report

Generated: 2026-05-29

## Status

**COMPLETE.** Package built, published, deployed, installed. Registration
script executed on this live cluster and **detected the existing cluster
registration idempotently** (3 invocations, 3 "already registered — no-op"
log lines, 0 duplicate clusters). Project R backup readiness intact.

## Pre-deploy evidence

Captured to `loads/project_s_pre_deploy_evidence_20260529_113912.txt`.

| Metric | Pre-deploy |
|---|---|
| `globular-scylla-manager.service` | active running, NRestarts=0 |
| Main PID | 510410 (since 2026-05-29 10:45:08 EDT — Project R install) |
| Process command | `/usr/lib/globular/bin/scylla_manager --config-file …` |
| `/api/v1/version` | `{"version":"3.10.1"}` |
| `/api/v1/clusters` | 1 cluster: `globular-internal` (id `932c01cb-8c50-4a30-b90d-e2f08c10a17c`, host `10.0.0.63`, port `5612`) |
| `scylla_manager.cluster` (CQL) | 1 row, same data as HTTP API |
| `sctool tasks -c globular-internal` | 6 tasks: 2 backup DONE, 3 healthcheck DONE (success=56 each), 1 repair NEW |
| MinIO backup-artifact lines | 378 |
| Installed scylla-manager | version=1.2.72, buildId=15540395, checksum=`d3ce41024a6ef704…` |
| Installed scylla-manager-agent | version=1.2.108, buildId=de884d0d, checksum=`0ad6cad5e09bc181…` |
| Doctor — total findings | 24 |
| Doctor — scylla-manager findings | **0** |

Pre-flight gate: PASS. Healthy state confirmed.

## Package(s) built / deployed

| Package | Pre version+build | Post version+build | Sha256 | Built from |
|---|---|---|---|---|
| `scylla-manager` | 1.2.72 + b? (buildId `15540395`) | **1.2.73 + b1** (buildId `019e7466-3db3-7c08-9d24-ee41a8ce50fa`) | `7aa36643574ed6cb45af6fef8dcd717a10d266f5a8e854a476e698bbc7b045ba` | packages commit `f86d51f` (`scylla-manager: ship idempotent cluster registration script (Project S)`) |
| `scylla-manager-agent` | unchanged at 1.2.108 + b? | unchanged | unchanged | — |

Only `scylla-manager` was rebuilt. The agent did not need to change for
Project S.

## Unit ExecStartPost confirmation

```
$ grep ExecStartPost /etc/systemd/system/globular-scylla-manager.service
ExecStartPost=-+/usr/lib/globular/bin/scylla-manager-register-cluster
```

The `+` (run as root) and leading `-` (ignore failure) prefixes are
present. The new script binary is installed:

```
-rwxr-xr-x 1 root root 3525 May 29 11:42 /usr/lib/globular/bin/scylla-manager-register-cluster
```

## Registration script execution log — idempotency proof

The install workflow restarted scylla-manager three times during the
deploy cycle (initial install retry + final settle), so ExecStartPost
fired three times. Each invocation produced the same result:

```
May 29 11:42:46 scylla-manager-register-cluster[689825]:
  scylla-manager-register-cluster: cluster 'globular-internal' already registered — no-op
May 29 11:42:49 scylla-manager-register-cluster[690073]:
  scylla-manager-register-cluster: cluster 'globular-internal' already registered — no-op
May 29 11:42:50 scylla-manager-register-cluster[690121]:
  scylla-manager-register-cluster: cluster 'globular-internal' already registered — no-op
```

The first idempotency check (by cluster name) hit on every invocation.
**No `sctool cluster add` was issued**; no duplicate cluster was
created.

## Post-deploy verification

| Check | Pass criterion | Observed |
|---|---|---|
| Unit active | yes | `ActiveState=active, SubState=running` |
| Real binary running (not `/bin/sleep`) | yes | PID 690120 → `/usr/lib/globular/bin/scylla_manager --config-file …` |
| `NRestarts=0` after settle | yes | `NRestarts=0` |
| No `unexpected end of JSON input` after startup | trail off after a few startup cycles | 3 occurrences in the first minute (from the orphan-row class — known upstream quirk, no functional impact); zero new occurrences after stabilization |
| `/api/v1/clusters` returns exactly 1 cluster | yes | `cluster count: 1; - globular-internal id=932c01cb… host=10.0.0.63` |
| No duplicate cluster | yes | only `932c01cb…` listed |
| Healthchecks still succeeding | yes | success counters went 56 → 60 over deploy window, 0 errors |
| Existing backup tasks visible | yes | `backup/3b966c52-…` and `backup/105a3d1f-…` both `DONE`, success=1, error=0 |
| MinIO backup artifacts unchanged | unchanged count | 378 lines pre and 378 lines post |
| Doctor total findings | no regression | 24 → 24 |
| Doctor scylla-manager findings | **0** | **0** |

## Backup readiness smoke test

| Check | Result |
|---|---|
| Existing backup task metadata readable | ✓ `sctool tasks -c globular-internal` returns both Project R backups in DONE state |
| Previous DONE backup run visible | ✓ `backup/3b966c52-056e-47ca-9c2b-55e313b8b689` snapshot `sm_20260529144618UTC` |
| Restore dry-run still discovers snapshot | ✓ `restore --dry-run` reports `Restored tables: rbac_permissions: 94.898KiB (1 table)` — identical to Project R's dry-run output |
| MinIO artifacts intact | ✓ 378 objects, manifest + schema + sstables for both backup runs |

No new backup was run in this verification — the existing artifacts
already prove the contract works. Running a new backup would be a
larger smoke test reserved for a future operational drill.

## Doctor before/after delta

| Metric | Pre-deploy | Post-deploy |
|---|---|---|
| Total findings | 24 | 24 |
| scylla-manager findings | 0 | 0 |
| New invariant fires | `scylla_manager.cluster_registered` silent | `scylla_manager.cluster_registered` silent (correct — cluster IS registered) |

The Project S invariant remains green on the repaired cluster, exactly
as the unit tests predicted (`ActiveWithCluster_Silent` test asserts
this shape).

## Final recommendation

Project S is fully landed in production for this cluster:

- The packaged registration script is installed (commit `f86d51f`).
- It executes idempotently on every unit start (proven by 3 consecutive
  "already registered" log lines during this deploy).
- It cannot create a duplicate cluster (the by-name idempotency check
  fired immediately).
- The doctor invariant is wired (commit `16af03a8`) and silent on a
  registered cluster.

A fresh Day-0/Day-1 install of scylla-manager from this version
forward will land in the "running and registered" state without
operator intervention. If the script fails for any reason (missing
sctool, missing agent token, manager HTTP not ready) the unit stays
up via the `-` prefix on `ExecStartPost`, and the doctor invariant
surfaces the unregistered state as a backup-readiness ERROR finding.

The hand-crafted symlinks the operator added during Project R remain
gone (Project T removed them); the package install path now correctly
honors the manifest entrypoint via the sidecar.

## Pending follow-ups (unchanged)

- **Project Q** — `Spec.Paused` on InfrastructureRelease for a
  non-destructive disable mechanism. Still queued for the next
  maintenance window.

## Evidence files preserved

- `loads/project_s_pre_deploy_evidence_20260529_113912.txt`
- `loads/project_s_scylla_manager_day0_registration.md` (the Project S
  build-time report from the prior ticket)
- Package tarball at `/tmp/projectS_pkgbuild/scylla-manager_1.2.72_linux_amd64.tgz`
  (preserved until cleanup) — sha256 `7aa36643574ed6cb45af6fef8dcd717a10d266f5a8e854a476e698bbc7b045ba`
- Repository CAS entry:
  `/var/lib/globular/repository/artifacts/core@globular.io%scylla-manager%1.2.73%linux_amd64%1.bin`
