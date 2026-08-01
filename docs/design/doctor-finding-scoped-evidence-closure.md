# Finding-scoped evidence closure for doctor remediation

## Decision

A remediation may execute only when every observation required by that finding, target, and action is complete and sufficiently trustworthy.

A failed collector that is unrelated to the finding must remain visible in snapshot diagnostics, but must not downgrade the entire healer cycle from `enforce` to `observe`.

## Root cause

The snapshot currently exposes a cluster-wide `DataIncomplete` boolean. Any collector error sets it. The periodic healer then treats that aggregate signal as execution authority and converts the whole cycle to observe-only.

This safely prevents action on false-positive findings produced by incomplete data, but it also lets an unrelated collector failure veto independently proven remediation. On a settling or loaded multi-node cluster, the complete-harvest window can be too narrow for reliable autonomous enforcement.

## Required behavior

1. Keep aggregate snapshot incompleteness as operator-visible diagnostic truth.
2. Record harvest failures with structured scope, including service, RPC, node, and entity where available.
3. Attach explicit evidence requirements to auto-remediable findings.
4. Resolve those requirements before privileged execution through the single remediation gate.
5. Refuse when a required observation is failed, absent, stale, or untrusted.
6. Allow when all required observations are usable, even if unrelated collectors failed.
7. Preserve cluster-wide fail-closed behavior for actions whose requirements genuinely span the cluster or a complete member set.

## Initial bounded slice

The first implementation covers `node.systemd.units_running`:

- required observation: `node_agent/GetInventory`
- scope: the finding's target node
- positive proof: the named Globular-managed unit is present and observed inactive or failed

Failures in workflow telemetry, package-integrity verification, Envoy probing, or a different node's collection do not invalidate that observation.

## Safety boundary

Removing the aggregate veto without adding scoped requirements is forbidden. The existing evidence-provenance trust gate, hard blocklist, approval policy, cooldown, failure-rate policy, workflow routing, and audit trail remain mandatory.

## Proof obligations

- unrelated harvest failure permits the target restart
- target inventory failure blocks the restart with an exact reason
- another node's missing endpoint does not block the target node
- stale or untrusted target evidence remains blocked
- genuinely cluster-scoped requirements remain fail-closed
- operator and background paths produce the same eligibility verdict
- audit records distinguish blocking requirements from unrelated harvest errors
