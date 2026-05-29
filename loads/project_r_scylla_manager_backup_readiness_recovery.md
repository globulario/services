# Project R — scylla-manager backup readiness: investigation and recovery plan

Generated: 2026-05-29
Status: **INVESTIGATION COMPLETE. RECOVERY PLAN DRAFTED. NO DESTRUCTIVE ACTION TAKEN.**

This document supersedes the holding action in
`loads/scylla_manager_disable_decision.md` (override remains active until
the recovery is executed and verified).

## 1. Exact root cause

scylla-manager 3.10.1 was installed and started but **no cluster was
ever registered with it via `sctool cluster add` or the API**. The
`scylla_manager.cluster` table is empty, yet the
`scylla_manager.scheduler_task` table contains 3 orphan healthcheck
task rows that reference a synthetic `cluster_id = 15098bd9-3b0c-49f7-b036-2eede2528361`
which does not appear in the cluster table.

scylla-manager's startup `startServices` step iterates registered
healthcheck tasks and reads their `properties` BLOB to decode the
mode JSON `{"mode":"cql"|"rest"|"alternator"}`. For these 3 orphan
rows, `properties=NULL` from the beginning (proven by the pre-DELETE
snapshot — see §3), `name=NULL`, `sched=NULL`. The JSON parse fails:

```
Failed to update healthcheck tasks
error: task <id>: get healthcheck task mode: unexpected end of JSON input
```

`startServices` returns this error; scylla-manager exits;
systemd's `Restart=on-failure` re-enters the loop. Deleting the rows
does not help because scylla-manager re-creates them at startup with
the same deterministic UUIDs and the same null fields — it has no
cluster context to derive proper `properties` content.

The root cause is therefore not corrupted runtime state — it is a
**latent never-bootstrapped configuration**. The Globular package
installs and starts scylla-manager but does not register the
ScyllaDB cluster with it. The auto-created bootstrap healthcheck
rows that fire before any cluster is registered are unusable from
that point onward.

## 2. Evidence

| Evidence | Source |
|---|---|
| scylla-manager binary v3.10.1 (sha256 `5ee875cea2…`) | `/usr/lib/globular/bin/scylla_manager --version` and tarball internal `package.json` (build 308) |
| ScyllaDB 2025.3.8 (`2025.3.8-0.20260223.d657044d70fb`) | `scylla --version` |
| Config (`/var/lib/globular/scylla-manager/scylla-manager.yaml`) contains only `http:` and `database:` blocks — no healthcheck overrides; built-in defaults (`CQLPingCron`, `RESTPingCron`, `AlternatorPingCron` with `* * * * *`) are loaded at startup | Startup `"Using config"` log line |
| `scylla_manager.cluster` table: **0 rows** | `SELECT * FROM scylla_manager.cluster` |
| `scylla_manager.scheduler_task` table: 3 rows, all `type='healthcheck'`, all referencing `cluster_id=15098bd9-3b0c-49f7-b036-2eede2528361` (not in cluster table) | `SELECT … FROM scylla_manager.scheduler_task` |
| Pre-DELETE snapshot already shows `name=null, properties=null, sched=null` for all 3 rows | `loads/scylla_manager_scheduler_task_snapshot_20260529_095310.txt` (in `golang/loads/`) |
| `success_count=1485-1486` on those broken rows confirms the rows existed long-term | same snapshot |
| `sctool cluster list` returns "connection refused" because the override placeholder isn't serving the API | command output |
| Other tables empty: `backup_run`, `repair_run`, `restore_run`, `secrets`, `scheduler_task_run` | per-table count check |
| Globular package source declares scylla-manager 3.8.1; tarball-internal manifest declares 1.2.72 (Globular wrapper) with the 3.10.1 binary; `entrypoint_checksum` in source repo (`c7bdac0d…`) is stale relative to actual ship | `packages/metadata/scylla-manager/package.json` vs tarball-internal `package.json` |
| ScyllaDB user keyspaces healthy and untouched: `ai_conversations, ai_memory, log_registry, rbac_permissions` listed | `DESCRIBE KEYSPACES` |

## 3. Why the simpler repairs would fail

| Path | Why it fails standalone |
|---|---|
| DELETE the 3 rows | Already tried (loads/scylla_manager_disable_decision.md) — rows are recreated on next startup with the same null fields. The bootstrap path inserts before any cluster is registered. |
| Manually `UPDATE … SET properties=textAsBlob('{"mode":"cql"}')` | The cluster_id those rows point at is not in the cluster table — even if the JSON parse succeeded, the healthcheck would fail at execution because the cluster name lookup returns nothing. Also brittle: mode-to-id mapping is unknown without source. |
| Upgrade scylla-manager binary | Possible long-term fix if upstream patched the never-bootstrapped path, but does not address the immediate "no cluster registered" gap. Even with a fixed binary, you must `sctool cluster add` to make scylla-manager useful. |
| Downgrade to 3.8.x | Same — the cluster still won't be registered. |

The persistent issue is **the cluster has never been registered**.
Any repair that doesn't include `sctool cluster add` will leave
scylla-manager useless even if the loop stops.

## 4. Selected repair path

**Option C-bounded: reset only scylla-manager's private metadata,
then perform the missing cluster registration**.

Rationale:
- The `scylla_manager` keyspace contains zero user data (confirmed §1-§2).
- It is scylla-manager's own internal scheduling state — it lives in
  ScyllaDB but is **owned by scylla-manager**, not by application services.
- ScyllaDB application keyspaces (`ai_conversations`, `ai_memory`,
  `rbac_permissions`, `log_registry`, …) are entirely separate and
  not touched by this operation.
- A fresh keyspace forces scylla-manager to run all migrations from
  v2.0.0 → v3.6.0 again and emit clean default healthcheck tasks
  AFTER we have registered the cluster (sequencing matters).

Option C is preferred over A and B because:
- A (config repair) does not exist for this failure shape — config is
  already correct; the missing piece is cluster registration.
- B (reinstall package) is heavier and doesn't reset the keyspace
  state, which is where the orphan rows live.

We will NOT do D (direct CQL UPDATE of the 3 rows) because:
- Mode→id mapping is not documented for 3.10.1.
- Patching the rows leaves the no-cluster-registered problem.

### Sequence (NOT YET EXECUTED — operator authorization required)

```
PRE-FLIGHT:
  1. Verify scylla-manager unit is masked / stopped or running under the
     sleep-infinity override (it currently is).
  2. Snapshot the scylla_manager keyspace schema and data to file
     (loads/scylla_manager_keyspace_full_snapshot_<ts>.cql) so a
     fully-reversible rollback is possible.
  3. Confirm /var/lib/globular/scylla-manager/scylla-manager.yaml exists
     and points at the real ScyllaDB host (10.0.0.63).

RECOVERY:
  4. cqlsh:    DROP KEYSPACE scylla_manager;
  5. Remove the /bin/sleep-infinity override drop-in:
       sudo rm /etc/systemd/system/globular-scylla-manager.service.d/disable.conf
       sudo systemctl daemon-reload
  6. systemctl stop globular-scylla-manager.service
     systemctl reset-failed globular-scylla-manager.service
  7. systemctl start globular-scylla-manager.service
     (scylla-manager re-runs all migrations against a fresh keyspace;
      its bootstrap path creates default healthcheck tasks with no
      cluster — same null-rows scenario will recur transiently)
  8. AS SOON AS the unit is active (≤30s after start), register the
     cluster via the supported API before the next healthcheck loop fires:
       sctool cluster add \
         --name globular-internal \
         --host 10.0.0.63 \
         --auth-token <generated by scylla-manager-agent if needed>
     OR via the HTTP API:
       curl -X POST http://10.0.0.63:5080/api/v1/clusters \
         -d '{"name":"globular-internal","host":"10.0.0.63"}'
  9. Verify cluster registration appears in scylla_manager.cluster
     and scheduler_task healthcheck rows for the NEW cluster_id now
     have properties != null and name != null.

POST-VERIFY (see §6 for details).
```

Note step 8: there is a small race window between `Service started`
and the first healthcheck registration attempt. If scylla-manager
exits before we register the cluster, repeat steps 6-8. If it loops
again after cluster registration, the root cause is in upstream
scylla-manager 3.10.1 and we fall back to Option B (upgrade to a
newer upstream).

## 5. Rollback plan

If Recovery step 4 succeeds but step 8 fails or the cluster fails to
register:

```
1. systemctl stop globular-scylla-manager.service
2. cqlsh: DROP KEYSPACE scylla_manager;   # again, to clear any partial state
3. Restore the snapshot from step 2 pre-flight:
     cqlsh -f loads/scylla_manager_keyspace_full_snapshot_<ts>.cql
4. Re-install the sleep-infinity override:
     sudo install -m 0644 \
       /home/dave/Documents/github.com/globulario/services/loads/scylla_manager_disable_override.conf \
       /etc/systemd/system/globular-scylla-manager.service.d/disable.conf
5. systemctl daemon-reload
6. systemctl start globular-scylla-manager.service
   (back to the holding state — same failure mode as before, but cluster
    operational)
```

The override file should be saved as
`loads/scylla_manager_disable_override.conf` before recovery starts so
this rollback is hermetic.

## 6. Pre-flight and post-repair checks

### Pre-flight (run before §4)

| Check | Expected |
|---|---|
| `systemctl is-active scylla-server.service` | `active` |
| `cqlsh -e "SELECT now() FROM system.local" 10.0.0.63` | returns timeuuid |
| `cqlsh -e "DESCRIBE TABLES" 10.0.0.63 -k scylla_manager` | listing including `cluster, scheduler_task` |
| `cqlsh -e "SELECT * FROM scylla_manager.cluster" 10.0.0.63` | 0 rows (confirms no registered cluster to lose) |
| `cqlsh -e "SELECT COUNT(*) FROM scylla_manager.backup_run" 10.0.0.63` | 0 (confirms no backup history to lose) |
| `cqlsh -e "SELECT COUNT(*) FROM scylla_manager.secrets" 10.0.0.63` | 0 (confirms no stored secrets to lose) |
| `globular-ai-memory`, `globular-rbac`, `globular-event`, `globular-workflow` | all `active` (Scylla-dependent services stay healthy) |

### Post-repair (run after §4 steps 1-9)

| Check | Expected |
|---|---|
| `systemctl is-active globular-scylla-manager.service` | `active` |
| `systemctl show globular-scylla-manager.service -p MainPID --value` → `ps -p $PID -o cmd=` | `/usr/lib/globular/bin/scylla_manager …` (NOT `/bin/sleep`) |
| `journalctl -u globular-scylla-manager.service --since='1 min ago' \| grep 'unexpected end of JSON input'` | empty (the error is gone) |
| `systemctl show globular-scylla-manager.service -p NRestarts` | `NRestarts=0` after warm-up |
| `cqlsh -e "SELECT COUNT(*) FROM scylla_manager.cluster" 10.0.0.63` | 1 (the registered cluster) |
| `cqlsh -e "SELECT id, type, name FROM scylla_manager.scheduler_task ALLOW FILTERING" 10.0.0.63` | 3 healthcheck rows, name set to mode (`cql_ping`/`rest_ping`/`alternator_ping`), properties != null |
| `sctool task list -c globular-internal` | shows the 3 healthcheck tasks plus any auto-created backup tasks |

## 7. Backup creation test

```
sctool backup -c globular-internal \
  --keyspace ai_memory \
  --location s3:scylla-backup-test/ai_memory   # or local: file:/var/lib/globular/backups
```

Expected:
- task accepted, `backup/<uuid>` returned
- `sctool task progress -c globular-internal backup/<uuid>` reports STAGE through `SnapshotCreate → InfoCollect → IndexUpload → DataUpload → Done`
- `cqlsh -e "SELECT COUNT(*) FROM scylla_manager.backup_run" 10.0.0.63` increments

If S3/object-store isn't available, use a local filesystem location
under `/var/lib/globular/backups/scylla-manager-test/` for the
first smoke test.

## 8. Backup execution test

After §7, force-run the task:

```
sctool task start -c globular-internal backup/<uuid>
sctool task progress -c globular-internal backup/<uuid>
sctool task history -c globular-internal backup/<uuid>
```

Expected:
- task reaches `DONE` status
- target location holds the snapshot files
- `sctool backup list -c globular-internal --location <loc>` shows the
  backup

## 9. Restore validation plan

Without performing an actual restore (which would risk app data),
verify the restore CONTRACT is intact:

```
sctool restore -c globular-internal \
  --location <same-location-as-§7> \
  --keyspace ai_memory \
  --dry-run
```

Expected:
- Dry-run reports `Plan looks good. No execution.`
- Lists which sstables would be downloaded and which token ranges
  would be loaded
- No CQL writes to `ai_memory` actually happen

A full restore validation against a temporary keyspace
(`ai_memory_restore_test`) is the longer-form validation; out of
scope for this initial readiness ticket but should be scheduled.

## 10. Is Project Q (Spec.Paused on InfrastructureRelease) still needed?

**Yes, still desirable but no longer blocking.** Project Q would
have provided a non-destructive "disable without uninstalling" path
for INFRASTRUCTURE releases — useful for any future "I need to
stop this temporarily without removing the binary" operator
scenario. Project P (commit fa44aa57) provides the proper
**remove** path. Spec.Paused fills the gap between "remove
entirely" and "leave running with ad-hoc systemd override."

Project Q is recommended for the next maintenance window, not as
a prerequisite for Project R.

## 11. Can the sleep-infinity override be removed?

**Yes — at exactly step §4.5 of the recovery sequence.** The override
file is removed atomically before the unit is restarted. Once the
real scylla-manager process is running and the post-repair checks
pass (§6), the override never needs to come back.

If the recovery fails and we fall back to the rollback plan (§5),
the override is restored to its current state — no permanent
removal until success.

## 12. Stop conditions honored

Per the operator directive:
- ✅ Did not drop the `scylla_manager` keyspace.
- ✅ Did not reinstall the scylla-manager package.
- ✅ Did not patch the 3 scheduler_task rows.
- ✅ Did not remove the systemd override.
- ✅ Documented the recovery plan as a discrete sequence with
   pre/post checks and a rollback for operator review.

## 13. Recommendation

Authorize Option C-bounded sequence (§4 steps 1-9). The destructive
step (DROP KEYSPACE) is bounded by:

- Pre-flight evidence that the keyspace holds zero user data.
- A full schema-and-data snapshot taken at step 2 for rollback.
- The application keyspaces (`ai_memory`, `ai_conversations`,
  `rbac_permissions`, `log_registry`) are not in scope and cannot be
  touched by `DROP KEYSPACE scylla_manager`.

After authorization, the recovery is expected to complete in under
five minutes and replace the holding override with a real running
scylla-manager that can accept backup/repair tasks.
