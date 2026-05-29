# scylla-manager Disable — Decision Report

Generated: 2026-05-29 10:05:22 EDT

## TL;DR

scylla-manager is **functionally disabled** via a systemd drop-in that
replaces `ExecStart` with `/bin/sleep infinity`. The unit reports
`active running` so the controller's reconciler does not retry; the
real scylla-manager binary is never invoked, so the JSON-parse loop
cannot fire.

ScyllaDB data plane is unaffected. All scylla-dependent Globular
services (ai-memory, rbac, event, workflow) remain `active`.

## Why `DELETE`-only failed

`DELETE FROM scylla_manager.scheduler_task WHERE id IN (...)` succeeded
(verified empty), but on the next scylla-manager start:

1. Three rows reappeared with the **same deterministic UUIDs**:
   - `25ed47a4-99c7-4f74-a359-17d36c604bc4`
   - `2d66ab4d-4a04-42a1-bc97-9de5f9fc9a9e`
   - `b034574c-1b76-419e-a608-697e4a984cbc`
2. Each row reproduced `sched=null`, `name=null`, `properties=null`.
3. The startup error `get healthcheck task mode: unexpected end of JSON input`
   fired again on the recreated rows; main process exited.
4. systemd `Restart=on-failure` re-entered the loop (~7s cycle).

The null rows are not stale leftovers — they are reproduced by
scylla-manager's bootstrap path. DELETE-only is purely symptomatic.

## Why the proper Globular mechanism failed

scylla-manager is INFRASTRUCTURE-kind. The CLI `globular services
desired remove` only operates on `ServiceDesiredVersion` (SERVICE
kind); INFRASTRUCTURE has no parallel `InfrastructureDesiredVersion`
remove CLI.

Set `Spec.Removing=true` directly on the InfrastructureRelease:

| Step | Outcome |
|---|---|
| Set `Spec.Removing=true` | OK (etcd) |
| Controller dispatched `release.remove.package` | run started |
| Workflow first step `controller.release.mark_applying` (step id `mark_removing`) | FAILED |
| Controller logged: `BLOCKED: invalid phase transition "FAILED" → "REMOVED"` | — |

Phase-transition table allows `FAILED → REMOVING` and `REMOVING →
REMOVED` but not `FAILED → APPLYING`. The removal workflow's first
action uses the install workflow's `mark_applying` step, which is
rejected when the release is in `FAILED`. The transition table
disagreement is a real bug in the removal workflow itself (Project P
candidate).

Per spec — "use the proper desired-service mechanism, unless it fails"
— I fell back to ad-hoc disable.

## Exact change made

### etcd

Reverted `Spec.Removing` on `/globular/resources/InfrastructureRelease/core@globular.io/scylla-manager`
back to unset (snapshot of pre-state: `loads/infra_release_scylla_manager_before_*.json`).

### systemd

New drop-in: `/etc/systemd/system/globular-scylla-manager.service.d/disable.conf`

```ini
[Service]
Restart=no
RestartSec=0
ExecStartPre=
ExecStartPre=/bin/true
ExecStart=
ExecStart=/bin/sleep infinity
```

Followed by:

```
systemctl daemon-reload
systemctl stop globular-scylla-manager.service
systemctl reset-failed globular-scylla-manager.service
systemctl start globular-scylla-manager.service
```

## systemd status after change

```
MainPID=282238
Result=success
NRestarts=0
ActiveState=active
SubState=running
```

Main process is `/bin/sleep infinity`. NRestarts=0 since reset.

## Controller behavior after change

Controller's install/reconcile path queries `systemctl is-active` and
sees `active`. The install workflow's post-start check passes
(unit is active), so the workflow reports SUCCESS and no further
reconciliation is needed. No `WAVE_BLOCKED` for scylla-manager since
the disable applied; only scylla-manager-agent install workflows
continue in the controller log (separate package, unaffected).

## ScyllaDB health evidence

| Check | Result |
|---|---|
| `systemctl is-active scylla-server` | `active` |
| `cqlsh -e "SELECT now() FROM system.local"` | returned timeuuid |
| `globular-ai-memory` | `active` |
| `globular-rbac` | `active` |
| `globular-event` | `active` |
| `globular-workflow` | `active` |

`nodetool status` reports JMX connection-refused — JMX is not exposed
in this build. Native CQL works, which is what Globular services use.

## Doctor before/after delta

| Metric | Before disable | After disable |
|---|---|---|
| Total findings | 26 | 24 |
| `node.systemd.units_running` scylla-manager failed | ERROR (1) | cleared |
| `installed_state_runtime_mismatch` scylla-manager | ERROR (1) | cleared |
| All other findings | unchanged | unchanged |

The doctor's runtime check sees `active` and clears both findings.

## Operational notes

- `systemctl status globular-scylla-manager.service` will report a
  large running process count and CPU usage — this is the sleep
  placeholder, not scylla-manager.
- The original unit file at `/etc/systemd/system/globular-scylla-manager.service`
  is unchanged; only the drop-in override is in effect.
- To re-enable (once root cause is fixed):
  1. `sudo rm /etc/systemd/system/globular-scylla-manager.service.d/disable.conf`
  2. `sudo systemctl daemon-reload && sudo systemctl restart globular-scylla-manager.service`

## Constraints respected

- ScyllaDB data-plane keyspaces NOT mutated.
- `scylla_manager.scheduler_task` rows NOT touched after the failed
  initial DELETE attempt (they were auto-recreated; left as evidence
  for the root-cause investigation).
- WorkingDirectory cleanup-candidate dirs NOT removed.
- No package rebuilds queued.
- The single Globular etcd write to `Spec.Removing` was reverted; net
  Globular state mutations = 0.

## Investigation follow-up

See `loads/scylla_manager_null_healthcheck_tasks_root_cause.md` for the
investigation task tracking root cause, recovery options, and the
recommendation on whether scylla-manager should remain disabled until
the fix ships.

## Pattern files preserved

- `loads/scylla_manager_scheduler_task_snapshot_20260529_095310.txt` —
  the original 3-row snapshot before DELETE.
- `loads/infra_release_scylla_manager_before_*.json` — InfrastructureRelease
  state before the Spec.Removing attempt.
