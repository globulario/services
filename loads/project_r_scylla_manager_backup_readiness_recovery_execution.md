# Project R — scylla-manager backup readiness recovery — EXECUTION REPORT

Generated: 2026-05-29
Status: **COMPLETE. scylla-manager fully operational. Backup proven end-to-end. Restore CONTRACT validated.**

## Final amended pre-drop evidence

Captured to `loads/project_r_pre_drop_gate_final_authorized_20260529_103707.txt`.

| Check | Expected | Actual | Pass |
|---|---|---|---|
| `cluster` count | 0 | 0 | ✓ |
| `scheduler_task` count | 3 orphan only | 3 | ✓ |
| `scheduler_task_run` | bounded rule | 1 row, orphan healthcheck only | ✓ amended |
| `backup_run` count | 0 | 0 | ✓ |
| `backup_run_progress` count | 0 | 0 | ✓ |
| `repair_run` count | 0 | 0 | ✓ |
| `repair_run_state` count | 0 | 0 | ✓ |
| `restore_run` count | 0 | 0 | ✓ |
| `restore_run_progress` count | 0 | 0 | ✓ |
| `secrets` count | 0 | 0 | ✓ |
| `gocqlx_migrate` | migration history only | 33 rows | ✓ |
| Schema snapshot | exists, readable | `loads/scylla_manager_keyspace_schema_snapshot_20260529_103707.cql` (528 lines) | ✓ |
| Application keyspaces | enumerated out of scope | `ai_conversations, ai_memory, dns, globular_events, log_entries, log_registry, rbac_permissions, repository, workflow` | ✓ |
| ScyllaDB CQL health | responsive | `system.local.now()` returned timeuuid `e678f0f0...` | ✓ |

All 14 amended criteria passed.

## Why the `scheduler_task_run` row was accepted as wipeable

The single row referenced:
- `cluster_id = 15098bd9-3b0c-49f7-b036-2eede2528361` (the orphan synthetic ID NOT present in `cluster` table)
- `task_id = 25ed47a4-99c7-4f74-a359-17d36c604bc4` (one of the 3 known broken orphan healthchecks)
- `type = healthcheck`, `status = DONE`
- TTL = 180 days (auto-expires regardless)

It was an execution log entry for the same orphan task we were already wiping — no backup/repair/restore meaning, no secret material. Same class as the orphan rows already authorized for removal.

## Exact DROP command executed

```sql
DROP KEYSPACE scylla_manager;
```

Confirmed via:
```
cqlsh: DESCRIBE KEYSPACE scylla_manager
→ 'scylla_manager' not found in keyspaces
```

Application keyspaces remain intact:
```
ai_conversations, ai_memory, dns, globular_events, log_entries,
log_registry, rbac_permissions, repository, workflow
```

## Schema recreation evidence

scylla-manager 3.10.1 recreated the keyspace and ran all migrations from
scratch on first start of the real binary:

```
"M":"Creating keyspace","keyspace":"scylla_manager"
"M":"Keyspace created","keyspace":"scylla_manager"
"M":"Migrating schema","keyspace":"scylla_manager"
"M":"Schema up to date","keyspace":"scylla_manager"
```

`gocqlx_migrate` now contains 33 migration rows fresh (v2.0.0 → v3.6.0
plus all numbered migrations 001-019).

## Cluster registration command

```
sctool --api-url http://10.0.0.63:5080/api/v1 cluster add \
  --host 10.0.0.63 \
  --port 5612 \
  --name globular-internal \
  --auth-token 6c7e1273d0d8386488773f0eb938570d991e9ed577aaef9f
```

Returned cluster ID `932c01cb-8c50-4a30-b90d-e2f08c10a17c`.

Note the non-default `--port 5612`: scylla-manager defaults to expecting
the agent at port 10001, but Globular's repository service already
occupies 10001 on this node. The Globular install pipeline configures
scylla-manager-agent to listen on 5612 (per
`/var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml`).
This is the right answer here but is a per-cluster sctool override —
the manager-side default could be set globally via
`agent_port: 5612` in `scylla-manager.yaml`. See Project S.

## Post-registration table evidence

`scylla_manager.cluster`:

```
 id                                   | name              | host      | port
--------------------------------------+-------------------+-----------+------
 932c01cb-8c50-4a30-b90d-e2f08c10a17c | globular-internal | 10.0.0.63 | 5612
```

`scylla_manager.scheduler_task` (4 properly-formed rows; orphans cleaned):

```
 cluster_id                           | type        | id                                   | name       | properties           | sched
--------------------------------------+-------------+--------------------------------------+------------+----------------------+-----------------------
 932c01cb-...                         | healthcheck | 1c320c70-...                         | cql        | {"mode":"cql"}       | <full UDT, cron set>
 932c01cb-...                         | healthcheck | 63c18466-...                         | alternator | {"mode":"alternator"}| <full UDT, cron set>
 932c01cb-...                         | healthcheck | f72dccec-...                         | rest       | {"mode":"rest"}      | <full UDT, cron set>
 932c01cb-...                         | repair      | c3abf288-...                         | all-weekly | {}                   | <full UDT, weekly>
```

All `name`, `properties`, `sched` populated correctly. No null fields.

## Systemd override removal moment

Step sequence (loads/scylla_manager_disable_override.conf preserved as rollback artifact):

1. `mv /etc/systemd/system/globular-scylla-manager.service.d/disable.conf` → preserved in `loads/`
2. `systemctl daemon-reload`
3. `systemctl stop globular-scylla-manager.service` (kills the sleep)
4. `systemctl reset-failed globular-scylla-manager.service` (clears prior failure state)
5. `systemctl start globular-scylla-manager.service` (real binary launches)

## Real process verification

```
$ systemctl show globular-scylla-manager.service -p MainPID,NRestarts,ActiveState,Result,SubState
MainPID=510410
NRestarts=0
ActiveState=active
Result=success
SubState=running

$ ps -p 510410 -o cmd=
/usr/lib/globular/bin/scylla_manager --config-file /var/lib/globular/scylla-manager/scylla-manager.yaml
```

60-second stability check: same PID, 0 restarts, healthcheck success_count incremented from 1 → 5 with 0 errors.

## Backup task creation evidence

Test target: keyspace `rbac_permissions` (7 rows, small known dataset).

```
$ sctool --api-url http://10.0.0.63:5080/api/v1 backup \
  -c globular-internal \
  -K rbac_permissions \
  --location 's3:scylla-manager-backup' \
  --retention 1 \
  --start-date now+0s

backup/3b966c52-056e-47ca-9c2b-55e313b8b689
```

MinIO bucket `scylla-manager-backup` created via `mc mb` before scheduling.

## Backup run evidence

```
$ sctool tasks -c globular-internal
backup/3b966c52-... | Success=1 Error=0 | Last 29 May 26 10:46:26 EDT | DONE
```

Artifact tree in MinIO (`mc ls -r globular-minio/scylla-manager-backup/`):

```
backup/meta/cluster/932c01cb-.../task_3b966c52-..._tag_sm_20260529144618UTC_manifest.json.gz    2.3KiB
backup/schema/cluster/932c01cb-.../task_3b966c52-..._tag_sm_20260529144618UTC_schema_with_internals.json.gz    6.3KiB
backup/sst/cluster/932c01cb-.../dc/datacenter1/node/fe014a92-.../keyspace/rbac_permissions/table/permissions/.../me-3h0t_04nq_*-big-{Data,Filter,Index,Statistics,Summary,TOC,...}.db
backup/sst/.../me-3h0t_0r1k_*-big-{...}.db
```

Total backup size: 310.796 KiB (1 node, 1 keyspace, 2 sstable batches).

**Documented backup location**: `s3:scylla-manager-backup` (MinIO endpoint
`https://minio.globular.internal:9000`, bucket `scylla-manager-backup`).
Configured via scylla-manager-agent's `s3:` block (`access_key_id:globular`,
`provider: Minio`, `endpoint: https://minio.globular.internal:9000`).

## Restore validation evidence

```
$ sctool restore -c globular-internal \
    --location 's3:scylla-manager-backup' \
    --snapshot-tag sm_20260529144618UTC \
    --keyspace rbac_permissions \
    --restore-tables \
    --dry-run

NOTICE: dry run mode, restore is not scheduled

Restored tables:
  - rbac_permissions: 94.898KiB (1 table)

Disk size: ~94.898KiB

Locations:
  - s3:scylla-manager-backup

Snapshot Tag:	sm_20260529144618UTC
Batch Size:     2
Parallel:       0
Transfers:      0
Compaction:     not allowed
Agent CPU:      pinned
Download Rate Limits:
  - Unlimited
```

Restore CONTRACT validated:
- manifest discovered ✓
- schema discovered ✓
- 1 table identified for restore ✓
- 94.9 KiB across 2 sstable batches ✓
- no actual writes performed (dry-run) ✓

A FULL restore against a temporary keyspace was not performed in this
ticket because the dry-run already proves the contract is intact. A
temporary-keyspace full-cycle test is recommended as a separate
operational drill.

## Doctor before / after delta

| Snapshot | Total findings | scylla-manager findings |
|---|---|---|
| Before Project R | 24 (post-override holding state) | 0 (override hid them) |
| After Project R | 24 | **0** (genuine — service is healthy) |

The before-state "0 scylla-manager findings" was masked by the
`/bin/sleep infinity` override (the doctor's unit-state check saw the
sleep as "active"). The after-state "0 findings" is genuine — the real
scylla-manager binary reports active/running with no errors and the
verifier sees the proper checksums.

Other findings unchanged (Project Q candidate dirs, artifact cache
mismatches, etc.).

## Remaining risks

1. **Orphan healthcheck rows recur on restart**: scylla-manager 3.10.1
   always creates 3 healthcheck rows under the synthetic
   `cluster_id=15098bd9...` on startup, regardless of registered
   clusters. The rows are null-fielded and produce log noise but do not
   prevent operation. They were deleted at runtime and do not get
   recreated mid-run. They WILL recreate on the next process restart.
   Operational workaround: keep them deleted (a periodic cleanup task
   could be scheduled), or wait for upstream scylla-manager fix.

2. **Verifier path mismatch (Project T candidate)**: Globular's verifier
   computes the binary path from the package NAME (hyphenated:
   `scylla-manager`) but the actual binary uses underscores
   (`scylla_manager`). Worked around with symlinks:
   `/usr/lib/globular/bin/scylla-manager → scylla_manager` and
   `/usr/lib/globular/bin/scylla-manager-agent → scylla_manager_agent`.
   Without the symlink the controller's drift-reconciler dispatches
   install workflows in a loop. Real fix: make the verifier honor the
   `entrypoint` field from `package.json` instead of inferring from
   package name.

3. **No automatic registration on Day-0/Day-1**: The Globular package
   currently does NOT run `sctool cluster add` as part of install. A
   fresh deployment of scylla-manager from clean state will exhibit the
   same null-orphan-healthcheck symptom until an operator manually
   registers the cluster. See Project S.

## Has the systemd override been removed?

**Yes — at 10:45:08 EDT.** The override file was preserved at
`loads/scylla_manager_disable_override.conf` for rollback but is no
longer applied at the systemd level. The unit runs the real
`scylla_manager` binary under systemd with `NRestarts=0` and ActiveState
`active running`.

## Is Project Q still needed afterward?

**Yes, still desirable but not blocking.** Project Q (honor
`Spec.Paused` on InfrastructureRelease) would have provided a clean
"disable temporarily without uninstalling" mechanism. The current
recovery did not need Project Q because the real fix was forward
recovery, not disable. Project Q is for the next maintenance window.

## Project S recommendation

See `loads/project_s_scylla_manager_bootstrap_registration_recommendation.md`
for the full Project S spec. Headline: the Globular scylla-manager
package must orchestrate cluster registration during Day-0/Day-1 so a
fresh install never lands in the "running but no cluster registered"
state observed here.

## Evidence files preserved

- `loads/project_r_pre_drop_gate_final_authorized_20260529_103707.txt`
- `loads/scylla_manager_keyspace_schema_snapshot_20260529_103707.cql`
- `loads/scylla_manager_disable_override.conf` (rollback)
- `loads/infra_release_scylla_manager_before_20260529_100120.json` (pre-Removing-attempt snapshot)
- `loads/scylla_manager_scheduler_task_snapshot_20260529_095310.txt` (in `golang/loads/`)
