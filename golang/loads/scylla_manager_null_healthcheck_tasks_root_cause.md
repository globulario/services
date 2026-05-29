# Investigation: scylla-manager null healthcheck task rows

Filed alongside `scylla_manager_disable_decision.md`. The disable is a
holding fix; this file tracks the root-cause investigation.

## 1. Version inventory

| Component | Version |
|---|---|
| scylla-manager (binary) | 3.10.1 |
| scylla-manager-agent (package) | 1.2.108 (Globular) |
| Scylla server | 2025.3.8 (`2025.3.8-0.20260223.d657044d70fb`) |
| Cassandra protocol reported via `system.local.release_version` | 3.0.8 (protocol level only, not the Scylla build version) |
| Globular package wrapper version (scylla-manager) | 1.2.72 |
| ScyllaDB schema migration applied (latest) | `v3.6.0.cql` (applied 2026-05-28 01:42:48Z) |

Per upstream ScyllaDB compatibility matrix, scylla-manager 3.10.x is
documented as supporting Scylla 5.x and Enterprise 2024.x. Scylla
2025.3.8 is a newer Enterprise LTS line — version pairing is not
obviously unsupported, but is not in the documented test matrix.

## 2. Config source

- File: `/var/lib/globular/scylla-manager/scylla-manager.yaml`
- Contents (minimal Globular override):
  ```yaml
  http: 10.0.0.63:5080
  database:
    hosts:
      - 10.0.0.63
    port: 9042
  ```
- No `healthcheck:` block in this file.
- scylla-manager applies built-in defaults for `CQLPingCron`,
  `RESTPingCron`, `AlternatorPingCron` when not overridden (per the
  config logged at startup: each cron has `spec: "* * * * *"`).
- The healthcheck tasks are auto-created by scylla-manager itself on
  start, keyed by the cluster's UUID and a fixed mode identifier — that
  is why the same 3 UUIDs reappear after DELETE.

## 3. Schema / source analysis

Table:
```cql
CREATE TABLE scylla_manager.scheduler_task (
    cluster_id uuid,
    type       text,        -- "healthcheck", "backup", "repair", ...
    id         uuid,        -- deterministic per (cluster_id, type, mode)
    deleted    boolean,
    enabled    boolean,
    error_count int,
    labels     map<text, text>,
    last_error timestamp,
    last_success timestamp,
    name       text,
    properties blob,        -- ← parsed as JSON by healthcheck code
    sched      frozen<schedule>,
    status     text,
    success_count int,
    PRIMARY KEY (cluster_id, type, id)
);
```

The error message comes from
`pkg/cmd/scylla-manager/server.go:364` in startServices:

```
Failed to update healthcheck tasks
error: task <id>: get healthcheck task mode: unexpected end of JSON input
```

Reading scylla-manager's source, `get healthcheck task mode` decodes
`properties` as JSON to extract `{"mode":"cql"|"rest"|"alternator"}`.
When `properties` is `null` (or zero-length), `json.Unmarshal([]byte(""), …)`
returns `unexpected end of JSON input`. That is the literal symptom.

Empirical state of the 3 affected rows after auto-recreation:

| id (prefix) | type | name | properties | sched | status |
|---|---|---|---|---|---|
| 25ed47a4 | healthcheck | null | null | null | DONE |
| 2d66ab4d | healthcheck | null | null | null | RUNNING |
| b034574c | healthcheck | null | null | null | DONE |

`name`, `properties`, and `sched` are all unset. The code path that
recreates these rows writes the row's primary key (cluster_id+type+id)
and `status`, but does not populate `properties` with the expected
mode JSON before the next start-time read.

## 4. Probable root cause

The most likely shape of the bug is in scylla-manager 3.10.1's
`startServices` / healthcheck-init code path:

1. On boot, scylla-manager iterates `(cluster, "healthcheck", *)` rows
   to register cron jobs.
2. For each row, it reads `properties` to derive the mode and the
   ping target.
3. Decoding fails because the row's `properties` was never populated
   (or was zeroed by a migration that ran from `v3.2.x` → `v3.6.0`
   without backfilling the new column shape).
4. `startServices` returns the aggregated error to systemd; process
   exits.
5. On the next start, scylla-manager re-inserts the same rows from a
   hard-coded healthcheck registry (cluster.RegisterCronJobs or
   similar) but, again, only writes the row's keys + status, leaving
   `properties=null`.

Candidates for the underlying cause:

- **Schema migration gap**: a migration introduced or renamed the
  `properties` column without backfilling existing or auto-created
  healthcheck rows. Latest applied migration is `v3.6.0.cql`; the
  scylla-manager 3.10.1 binary may expect a `v3.7.x` or `v3.8.x`
  migration that was never shipped because the Globular package only
  bundles up to v3.6.
- **Initialization-order bug**: registerCronJob inserts row keys
  before writing properties, and a concurrent goroutine reads them
  back before the properties write commits.
- **Package install ordering**: the Globular scylla-manager package
  may install the binary without populating an initial seed file that
  the binary expects (e.g. a default tasks JSON).

Without source-level access to the exact 3.10.1 healthcheck init code,
the precise variant cannot be confirmed from this node alone.

## 5. Recovery options

### Option A — manual UPDATE on the 3 rows

```sql
UPDATE scylla_manager.scheduler_task
SET properties = textAsBlob('{"mode":"cql"}')
WHERE cluster_id = 15098bd9-3b0c-49f7-b036-2eede2528361
  AND type = 'healthcheck'
  AND id = <one of the 3 ids>;
```

Risks: mode-to-id mapping is unknown without source; wrong mode may
mis-classify the healthcheck. scylla-manager may overwrite back to
null on next start.

### Option B — upgrade scylla-manager package

Check for a 3.10.2 / 3.11.x / 3.12.x release fixing this. Globular
ships scylla-manager via its own package (currently 1.2.72 → upstream
3.10.1); a Globular package bump that ships a newer upstream binary
may resolve.

### Option C — downgrade scylla-manager to 3.9.x

If the bug was introduced in 3.10, the prior major may work against
Scylla 2025.3.8. Requires schema rollback compatibility check.

### Option D — keep disabled

Cluster runs without backup/repair scheduling. Backups + repairs can
be invoked manually via `sctool` until the package is fixed. This is
the **current state** and is the safe holding action.

### Option E — drop the keyspace and re-init

```sql
DROP KEYSPACE scylla_manager;
```

scylla-manager will re-run all migrations from scratch on next start.
**Untested**: same broken init may recur. Last-resort, not
recommended without first verifying source-level fix.

## 6. Recommendation

- **scylla-manager should remain disabled** (Option D) until a known
  upstream fix is identified. Disable does not affect ScyllaDB data
  plane; Globular continues to run normally.
- **Doctor classification**: the current invariant
  `node.systemd.units_running` correctly clears now (unit reports
  `active`). For future incidents where a component is intentionally
  disabled at the operator level, a small enhancement would let the
  doctor classify disabled-by-operator separately from
  active/inactive/failed. Optional follow-up.
- **Project P candidate** (controller): the removal workflow's first
  step (`controller.release.mark_applying`) cannot transition out of
  `FAILED`. Either the action should be `controller.release.mark_removing`
  (proper transition `FAILED → REMOVING`), or the controller should
  pre-transition `FAILED → REMOVING` before dispatching. This bug
  blocked the canonical Globular path for INFRASTRUCTURE removal in
  this incident.
- **Project Q candidate** (controller): `reconcileInfraRelease` does
  not honor `Spec.Paused` (it's read into the handle but never
  checked). The ServiceRelease reconciler honors it at the entry
  guard. Symmetric fix: short-circuit at the top of
  `reconcileInfraRelease` when `Spec.Paused` is true. Would have
  provided a non-destructive disable mechanism for INFRASTRUCTURE in
  this incident.

## 7. Evidence preserved

- `loads/scylla_manager_scheduler_task_snapshot_20260529_095310.txt`
  — snapshot of the 3 corrupted rows pre-DELETE.
- `loads/infra_release_scylla_manager_before_*.json` — snapshot of
  InfrastructureRelease before the Spec.Removing attempt.
- `loads/scylla_manager_disable_decision.md` — operational decision
  + holding-action implementation.
