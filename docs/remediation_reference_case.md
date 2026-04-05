# Remediation Reference Case — the Golden Path

**Status:** Frozen. Protect this path.
**First validated:** 2026-04-05
**Workflow:** `remediate.doctor.finding`
**Scope:** one finding, one structured action, one node

This document memorializes the first end-to-end SUCCESS of the
`remediate.doctor.finding` workflow. It is a regression reference: any
future change to the doctor's remediation layer must still produce this
exact shape on an equivalent scenario.

---

## The scenario

1. Stop a single Globular-managed service on one node:
   ```
   sudo systemctl stop globular-torrent.service
   ```
2. Wait for the doctor's snapshot TTL to expire (5s default).
3. Trigger a fresh scan (any `GetNodeReport` / `GetClusterReport` call).
4. A finding appears from the `node.systemd.units_running` invariant with
   a structured `SYSTEMCTL_RESTART` action, LOW risk, targeting that unit
   on that node.
5. Run the workflow:
   ```
   globular doctor remediate --workflow <finding-id> --endpoint localhost:12100
   ```
6. All five pipeline steps succeed. Verify scan confirms the finding
   cleared. Service is active again.

---

## The expected CLI output

```
✓ workflow: SUCCEEDED (run_id=run-<ts>)
  resolve: node=<node_id> action=SYSTEMCTL_RESTART risk=RISK_LOW
  assess:  auto_executable=true requires_approval=false
  execute: status=executed executed=true audit_id=rem-<ts>
           output=restart globular-torrent.service on <node_id>: state=active
  verify:  converged=true finding_still_present=false
```

## The expected doctor log sequence

```
workflow: step resolve_finding starting    (actor=cluster-doctor action=doctor.resolve_finding)
workflow: step resolve_finding SUCCEEDED
workflow: step assess_risk starting        (actor=cluster-doctor action=doctor.assess_risk)
workflow: step assess_risk SUCCEEDED
workflow: step require_approval starting   (actor=cluster-doctor action=doctor.require_approval)
workflow: step require_approval SUCCEEDED
workflow: step execute_remediation starting (actor=cluster-doctor action=doctor.execute_remediation)
INFO remediation executed finding_id=<id> action_type=SYSTEMCTL_RESTART dry_run=false subject=system
workflow: step execute_remediation SUCCEEDED
workflow: step verify_convergence starting  (actor=cluster-doctor action=doctor.verify_convergence)
workflow: step verify_convergence SUCCEEDED
```

---

## What this proves (the invariants)

1. **Actor routing**: `cluster-doctor` actor correctly dispatches all
   five `doctor.*` handlers.
2. **Embedded YAML**: the workflow definition is compiled into the
   doctor binary (go:embed) and loads without disk I/O.
3. **Wrap, don't bypass**: the execute step calls
   `ClusterDoctorServer.ExecuteRemediation` in-process — the same RPC
   handler the CLI calls directly — so the audit trail, blocklist, and
   approval gates are identical across both paths.
4. **Verify re-scans**: `verify_convergence` runs a fresh doctor scan
   (not a cache read) and checks whether the finding is still present.
5. **onFailure hook**: on any step failure the workflow terminates and
   `doctor.mark_failed` records a warning log with the finding_id.
6. **CLI surface**: `globular doctor remediate --workflow` prints every
   pipeline stage. No output = no execution.

---

## Rules of engagement for new structured actions

New invariants or structured actions added after this point must be
able to reproduce the same output shape on a comparable scenario. Before
merging, the author must:

- Write a scenario with a clear reproducer (stop X → finding appears).
- Confirm the action is `RISK_LOW` AND auto-executable by the executor
  (globular-* unit for SYSTEMCTL_RESTART, safe-trash path for FILE_DELETE).
- Run the workflow end-to-end against a live doctor and capture the
  5-step success log.
- Not expand the pipeline. The pipeline is frozen:
  `resolve → assess → approve → execute → verify`.

---

## Current structured-action inventory (as of 2026-04-05)

| Rule | Action | Condition | Risk |
|---|---|---|---|
| `node.reachable` | restart `globular-node-agent` on the unreachable node | heartbeat stale or status=unreachable | LOW |
| `node.systemd.units_running` | restart any failed/inactive `globular-*` unit | systemd reports failed or inactive | LOW |
| `node.inventory.complete` | restart `globular-node-agent` | inventory scan stalled (0 components or 0 units) | LOW |
| `cluster.services.drift` (no-priv variant) | restart `globular-node-agent` | node lacks sudo for systemctl | LOW |
| `node.agent_crash` (heartbeat variant) | restart `globular-node-agent` | controller has no contact | WARN (LOW) |
| `node.agent_crash` (timeout variant) | restart `globular-node-agent` | node-agent request timed out | WARN (LOW) |

All six share the same verified pattern:
stopped/hung `globular-*` unit → `SYSTEMCTL_RESTART` → verify scan clears
the finding.

---

## Extending outward

This reference case ends at "one finding, one node, one action." The
broader reconcile dispatch path — feeding doctor findings into
`cluster.reconcile.choose_workflow` so the reconciler auto-fires
`remediate.doctor.finding` per drift — is deliberately **out of scope**
for this frozen release. Do not touch it without a new reference case.
