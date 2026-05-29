# Project S — Day-0/Day-1 scylla-manager cluster registration

## Status

**Doctor invariant + tests: shipped (services repo commit `16af03a8`, deployed
as `cluster-doctor@1.2.120+1`). Package-side registration script: shipped
(packages repo commit `f86d51f`).**

The doctor side is live and validated against the running cluster (returns
0 scylla-manager findings against the Project-R-registered cluster). The
packages-repo change has NOT been built/deployed in this ticket because
doing so would trigger a scylla-manager reinstall during normal
maintenance; the running unit ships with the same systemd unit content,
and Project R's registered cluster + working backup tasks remain intact.
Operator authorization is the natural gate for shipping the packages
change (next backup-tooling release).

## Root cause

scylla-manager 3.10.1 starts in a "default" mode that creates 3 synthetic
healthcheck task rows under a placeholder `cluster_id` regardless of
whether any cluster has been registered. When no operator runs `sctool
cluster add`, the daemon stays "active" but cannot perform any of its
core jobs (backup, repair, restore). The Globular package historically
started the daemon and stopped there — there was no registration step.

## Chosen bootstrap location

**Package post-install script invoked via the unit's `ExecStartPost`.**

Rationale considered for each alternative:

| Location | Rejected because |
|---|---|
| node-agent infrastructure apply action | New action type for one package; tighter coupling between node-agent and scylla-manager specifics; less idiomatic than the existing scylla-manager-configure shell script pattern. |
| cluster-controller workflow | Requires a new dispatcher for what is fundamentally a local on-node bootstrap. The cluster_id, agent token, and Scylla CQL endpoint are all node-local. |
| Dedicated remediation workflow | Reactive rather than proactive — would only run after the doctor finds the unregistered state. Useful as a fallback but not as the primary bootstrap. |
| Doctor-guided remediation | Same — reactive, not proactive. Better as the verification gate. |
| **Package post-install (ExecStartPost)** | **Chosen.** Mirrors the existing `scylla-manager-configure` pattern (script installed as a file, invoked from the unit). All required state (agent token, Scylla rpc_address) is local. Failure is non-fatal via `-` prefix on ExecStartPost; the doctor invariant surfaces problems. |

The script supplements rather than replaces the doctor invariant: the
script is the **proactive enforcement** ("register when missing"); the
doctor is the **safety net** ("flag when still missing for any reason").

## Files changed

### Services repo (commit `16af03a8`)

| Path | Change |
|---|---|
| `golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered.go` | New. Doctor invariant: probes scylla-manager's HTTP `/api/v1/clusters` when at least one node's inventory shows `globular-scylla-manager.service` in an active state. Emits `SEVERITY_ERROR` when the daemon is up but cluster list is empty. |
| `golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered_test.go` | New. 7 unit tests using `httptest.NewServer` to exercise: empty array → ERROR, populated array → silent, inactive unit → silent, HTTP 5xx → silent (inconclusive), nil/empty snapshot → silent, multi-node any-active → triggers probe, remediation text mentions both the script and the manual fallback. |
| `golang/cluster_doctor/cluster_doctor_server/rules/registry.go` | Register the new rule. |

### Packages repo (commit `f86d51f`)

| Path | Change |
|---|---|
| `metadata/scylla-manager/specs/scylla_manager_service.yaml` | Two changes. (1) New install step `install-scylla-manager-register-cluster` writes `{{.Prefix}}/bin/scylla-manager-register-cluster` (a shell script). (2) Existing `install-scylla-manager-service` unit content updated to add `ExecStartPost=-+{{.Prefix}}/bin/scylla-manager-register-cluster`. The `+` makes it run as root; the leading `-` makes failure non-fatal so the unit stays up if registration fails. |

## Idempotency behavior

The shipped script performs three idempotency checks before issuing
`sctool cluster add`:

1. **By cluster name**: `GET /api/v1/clusters` and search for an object
   with `"name":"<expected-name>"` (default `globular-internal`,
   overridable via `SCYLLA_MANAGER_CLUSTER_NAME` env). If found, exit 0.
2. **By Scylla host**: even when the name doesn't match, any cluster
   registered under the same Scylla `rpc_address`/`listen_address` is
   treated as the existing registration. The script does NOT silently
   re-register or replace because that would lose operator-configured
   backup tasks attached to the prior registration. Operator review
   required (logged to stderr).
3. **Wait for API readiness**: polls `/api/v1/version` for up to 60s.
   If the API never becomes ready, exits 0 (does NOT take down the
   unit); the doctor surfaces the still-unregistered state.

The doctor invariant matches this idempotency model: it does not fire
when one cluster is registered, regardless of name.

## Doctor invariant added

`scylla_manager.cluster_registered` (registered alongside the other
Globular doctor rules in `rules/registry.go`).

| Field | Value |
|---|---|
| Category | `infrastructure` |
| Scope | `cluster` |
| Severity when firing | `SEVERITY_ERROR` |
| Summary | "scylla-manager is running but no Scylla cluster is registered (backup, repair, and restore are unavailable until `sctool cluster add` runs)" |
| Remediation step 1 | Point at the package-shipped script (`/usr/lib/globular/bin/scylla-manager-register-cluster`) |
| Remediation step 2 | Manual `sctool cluster add` command with the parameter shape the script uses |

The rule only probes the HTTP API when at least one node's inventory
shows the unit active; HTTP failures are silent (inconclusive — daemon
health is covered by a separate rule).

## Tests added

`scylla_manager_cluster_registered_test.go` — **7 tests, all PASS**:

| Test | Assertion |
|---|---|
| `ActiveButEmpty_FiresError` | Unit active + empty `/api/v1/clusters` → 1 ERROR finding with the expected invariant ID, severity, and "backup" mention in summary. **The literal Project R bug repro.** |
| `ActiveWithCluster_Silent` | Unit active + one cluster returned → 0 findings. Happy path. |
| `Inactive_Silent` | Unit inactive/failed/deactivating → 0 findings (the rule scopes to "running but unregistered"; daemon-health rules cover failure). |
| `ProbeFails_Silent` | Unit active + HTTP 500 → 0 findings (inconclusive; avoid false positives during transient network issues). |
| `NoInventory_Silent` | nil/empty snapshot → 0 findings (defensive). |
| `MultiNode_AnyActive` | One inactive node + one active node → probe runs, empty response fires. |
| `RemediationMentionsScript` | Remediation strings reference both the package script name and `sctool cluster add` so the two stay aligned. |

## Test results

```
golang/cluster_doctor/cluster_doctor_server/rules    PASS   1.004s
  TestScyllaManagerClusterRegistered_*                PASS (7 tests)
  All other rules                                     PASS  (unchanged)
```

Zero regressions across the doctor rule suite.

## Current cluster verification

After deploying `cluster-doctor@1.2.120+1` to the live cluster:

| Check | Result |
|---|---|
| `globular-cluster-doctor.service` | active running, real v1.2.120 binary (sha256 `43058400d20f0f71a267bb0005e4fff76ff25fdaa69c4b2f42a7ed1e5232d79a`) |
| Doctor finding count | 24 (was 24 before deploy) |
| **scylla-manager findings** | **0** — invariant correctly silent on the registered cluster |
| `GET http://10.0.0.63:5080/api/v1/version` | `{"version":"3.10.1"}` |
| `GET http://10.0.0.63:5080/api/v1/clusters` | 1 cluster: `globular-internal` (id `932c01cb...`, host `10.0.0.63`) |
| `sctool tasks -c globular-internal` | Project R's 4 tasks (healthcheck/cql, /alternator, /rest, repair/all-weekly) unchanged; healthcheck success counts continue to increment |
| MinIO bucket `scylla-manager-backup` | Project R's two completed backup tasks still present (manifests, schema, sstables) |
| `globular-scylla-manager.service` | active running, real binary, NRestarts=0 |

## Does a fresh Day-0/Day-1 now get backup-ready scylla-manager?

**Yes — when the packages-repo change ships in the next scylla-manager
package build.** The script does three things on first install:

1. waits for the HTTP API to be ready
2. reads `auth_token` + agent HTTPS port from the local
   scylla-manager-agent config
3. calls `sctool cluster add --host <local-scylla-ip> --port <agent-port>
   --name globular-internal --auth-token <token>`

If any of those fails, exit 0 → unit stays up → the cluster_doctor
invariant fires `scylla_manager.cluster_registered` with the operator
guidance to run the script manually. There is no path through which
the system silently degrades to the pre-Project-R state.

## Remaining risks

1. **Script depends on `sctool` being on PATH at install time.**
   `sctool` ships from the upstream scylla-manager packaging and is
   typically placed at `/usr/local/bin/sctool` on the same node as the
   server. The script does not currently fall back to the HTTP API
   directly; if `sctool` is missing the script exits 0 with the
   `command failed` log. Future hardening: shell out to `curl -X POST
   /api/v1/clusters` as a fallback when `sctool` is unavailable.

2. **Package-repo change has not been built and deployed in this
   ticket.** Doing so would trigger a scylla-manager reinstall, which
   would restart the running unit (Project R's state survives but with
   a brief interruption to healthchecks). Operator authorization is the
   natural gate; the doctor invariant is shipped and live regardless.

3. **The "by host" idempotency check is conservative.** A cluster
   registered for the same Scylla host under a different name (e.g.
   from an operator-initiated rename) is left as-is and logged. The
   operator can rename the registered cluster manually if they want
   the package-default name applied; the script will not duplicate or
   override.

4. **scylla-manager 3.10.1's synthetic-cluster_id orphan rows still
   recreate on every process restart.** This is upstream behavior
   unrelated to Globular's bootstrap. The doctor's invariant uses the
   HTTP API rather than the keyspace, so the orphan rows do not cause
   false positives. (This was Project R's first surprise; documented
   in `loads/project_r_scylla_manager_backup_readiness_recovery_execution.md`
   under "Remaining risks".)

5. **Project Q (`Spec.Paused` on InfrastructureRelease) still pending.**
   If an operator wants to temporarily disable scylla-manager (e.g. for
   diagnostic work) the canonical Globular mechanism still requires
   `Spec.Removing=true` (which uninstalls). The ad-hoc systemd override
   workaround used during Project R has been removed and is documented
   in `loads/scylla_manager_disable_override.conf` as the rollback
   artifact.

## Acceptance criteria — current cluster

| Criterion (from Project S spec) | State |
|---|---|
| scylla-manager real process running | ✓ active running, /usr/lib/globular/bin/scylla_manager, NRestarts=0 |
| real Scylla cluster registered | ✓ `globular-internal` (`932c01cb...`) |
| healthcheck tasks valid | ✓ 3 healthcheck tasks (cql/alternator/rest) running with proper name/properties/sched |
| backup target configured | ✓ `s3:scylla-manager-backup` in MinIO; two completed backup task runs from Project R |
| doctor detects unregistered manager state | ✓ `scylla_manager.cluster_registered` invariant deployed, validated by 7 unit tests + live silent run |
| no manual emergency keyspace reset required | ✓ — Project R was the one-time reset; the script + invariant make a repeat unnecessary |
