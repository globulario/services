# Project S — scylla-manager bootstrap registration

## Why this exists

scylla-manager 3.10.1 ships with a startup path that auto-creates 3
default healthcheck task rows under a synthetic `cluster_id` regardless
of whether any cluster is registered. When no cluster is registered:

- the rows are written with `name=null, properties=null, sched=null`
- on subsequent start, `startServices` reads `properties` to derive
  the healthcheck mode and fails with
  `unexpected end of JSON input`
- the rows recreate themselves on every restart

Project R recovered the running cluster by manually:
1. dropping the `scylla_manager` keyspace
2. removing the systemd disable override
3. running `sctool cluster add` against the local Scylla node
4. deleting the orphan rows post-registration

This is operator work that should not be required on every fresh
install. Project S brings the registration into the Globular
package install flow.

## Goal

The Globular scylla-manager package must orchestrate cluster
registration during Day-0/Day-1 install so a clean deployment lands
in a "running and registered" state without operator intervention.

## Requirements

### R1 — idempotent registration

Day-0 / Day-1 install must run or orchestrate `sctool cluster add` for
the local Scylla node. Re-running install must NOT create duplicate
cluster records, change the cluster ID, or break existing tasks.

Implementation shape:

```
if sctool cluster list | grep -q <expected-cluster-name>:
  log "already registered"
  exit 0
else:
  sctool cluster add --host <local-scylla-routable-ip> \
                     --port <agent-port-from-yaml> \
                     --name <expected-cluster-name> \
                     --auth-token <from-agent-config>
```

### R2 — existing-registration detection

The install step must distinguish three states and react correctly:

| State | Action |
|---|---|
| no cluster registered | add it |
| cluster registered with matching name + host | no-op |
| cluster registered under a DIFFERENT name or host | error, surface as doctor finding, do not silently re-register |

### R3 — healthcheck task validation

After registration, the install step must verify that the auto-created
healthcheck tasks for the newly registered cluster have non-null
`name`, `properties`, and `sched` fields:

```
cqlsh -e "SELECT name, blobastext(properties), sched
          FROM scylla_manager.scheduler_task
          WHERE cluster_id=<just-registered-id>
          AND type='healthcheck' ALLOW FILTERING"
```

If any row has null fields, install fails and surfaces a Day-0
finding. (This is the post-registration sanity check that would have
caught the upstream 3.10.1 bug at first install.)

### R4 — backup target creation or validation

The install step must ensure a backup target exists. Two acceptable
shapes:

(a) **Default to MinIO**: create bucket `scylla-manager-backup` via
    the Globular `mc` alias if absent. Backup target string:
    `s3:scylla-manager-backup` (this is what Project R used and
    proved end-to-end).

(b) **Operator-supplied**: read backup location from a Globular
    config key (e.g. `scylla-manager.backup_location` in the
    cluster_controller etcd state). Default to MinIO when unset.

Without a target configured, scylla-manager runs but cannot accept
backup tasks — the package starts a service that cannot do its job.

### R5 — doctor invariants

Add two doctor invariants:

- `scylla_manager.cluster_must_be_registered`: WARN when
  `globular-scylla-manager.service` is `active` but
  `scylla_manager.cluster` is empty. (The exact symptom that started
  Project R.)
- `scylla_manager.healthcheck_tasks_must_be_well_formed`: WARN when
  any row in `scylla_manager.scheduler_task` of type=`healthcheck`
  has `name=null` OR `properties=null` OR `sched=null`. (Catches the
  orphan-row class regardless of source.)

Both invariants are non-fatal but surface the broken state instead
of letting it sit silent.

## Related Globular bugs (out of scope for Project S itself)

- **Project T candidate**: the verifier infers the binary path from
  the package NAME (hyphenated) rather than from the package's
  `entrypoint` field. Hyphen→underscore-binary packages (currently
  only scylla-manager and scylla-manager-agent) fail the verifier's
  hash check until a manual symlink is in place.
- **Project Q candidate**: `reconcileInfraRelease` does not honor
  `Spec.Paused`. Either of these (when fixed) would have softened
  the Project R recovery — Q would have provided a non-destructive
  disable; T would have let the package install pipeline converge
  without the symlink workaround.

## Acceptance criteria

A fresh Day-0/Day-1 install of scylla-manager on a node with no
prior `scylla_manager` keyspace MUST result in:

1. `scylla_manager.cluster` containing exactly one row for the
   intended local cluster
2. 3 auto-created healthcheck tasks with valid
   `name`/`properties`/`sched`
3. 1 auto-created `repair/all-weekly` task (or operator-configured
   equivalent)
4. backup target accessible (configured bucket exists, agent can
   authenticate)
5. `sctool backup` test against any application keyspace succeeds
6. doctor reports no scylla-manager findings

No operator intervention beyond running the standard install.

## Migration for existing nodes

For nodes currently in the post-Project-R recovered state (where the
operator manually registered the cluster), the Project S install must
detect the existing registration and treat it as the target state
(R2 case "matching" → no-op). The package upgrade must NOT
re-register or drop and recreate the cluster.

## Suggested implementation surface

Most natural place: a new install step in
`/home/dave/Documents/github.com/globulario/packages/metadata/scylla-manager/specs/scylla_manager_service.yaml`
after the existing `start-scylla-manager` step:

```yaml
  - id: register-cluster
    type: run_script
    after_service_active: globular-scylla-manager.service
    timeout: 60s
    script: |
      #!/bin/sh
      # Project S — idempotent cluster registration.
      # ...
      sctool ... cluster list | grep -q globular-internal && exit 0
      AGENT_TOKEN=$(grep '^auth_token:' /var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml | awk '{print $2}')
      AGENT_PORT=$(grep -E '^https:' /var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml | awk -F: '{print $3}')
      SCYLLA_HOST=<from-etcd-or-local-config>
      sctool cluster add --host "$SCYLLA_HOST" --port "$AGENT_PORT" \
                         --name globular-internal --auth-token "$AGENT_TOKEN"
```

Plus a `validate-tasks` step that runs the R3 sanity check, plus a
`validate-backup-target` step that runs R4.

## References

- Execution report: `loads/project_r_scylla_manager_backup_readiness_recovery_execution.md`
- Original investigation: `loads/scylla_manager_null_healthcheck_tasks_root_cause.md`
- Recovery plan: `loads/project_r_scylla_manager_backup_readiness_recovery.md`
- Override artifact (rollback): `loads/scylla_manager_disable_override.conf`
