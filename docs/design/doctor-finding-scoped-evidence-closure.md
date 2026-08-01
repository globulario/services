# Finding-scoped evidence closure for doctor remediation

## Decision

A remediation may execute only when the evidence supporting that finding and target remains conclusive and sufficiently trustworthy.

A failed collector that is unrelated to the finding remains visible in snapshot diagnostics, but does not downgrade the entire healer cycle from `enforce` to `observe`.

## Root cause

The snapshot exposes a cluster-wide `DataIncomplete` boolean. Any collector error sets it. The periodic healer treated that aggregate diagnostic signal as execution authority and converted the whole cycle to observe-only before individual findings reached the gated dispatcher.

The invariant registry already implements the more precise rule:

- if a finding's own evidence source failed, the finding is downgraded to `INVARIANT_UNKNOWN` with `CheckError`;
- if only unrelated sources failed, the finding keeps its conclusive verdict and receives a `[reduced-harvest]` annotation.

The healer discarded that precision by applying a second cluster-wide veto.

A second gap existed in the shared execution path: a full-harvest `INVARIANT_UNKNOWN` finding could still have fresh individual evidence rows. Freshness alone must never convert an indeterminate verdict into mutation authority.

## Bounded implementation

This PR removes the redundant global veto and makes one universal remediation verdict closure authoritative across background and operator-driven execution:

1. `DataIncomplete` remains operator-visible diagnostic truth.
2. `enforce`, `dry_run`, and `observe` retain their configured meaning during reduced harvest.
3. Non-dry-run remediation requires `INVARIANT_FAIL` and an empty `CheckError`, regardless of harvest mode.
4. A finding downgraded to `INVARIANT_UNKNOWN` is refused before mutation.
5. `PASS`, `PENDING`, and full-harvest `UNKNOWN` findings are also diagnostic states, never execution authority.
6. The closure participates in the shared evidence-trust gate, so direct and background non-dry-run execution have parity.
7. Dry-runs still traverse the central gate for rehearsal and audit.

## Target scope for systemd findings

The first target-specific slice is `node.systemd.units_running`.

Collector failures are already recorded as `node_agent@<node_id>/<RPC>`. The rule now stamps its `GetInventory` evidence with the same instance-qualified writer identity:

- node 1 evidence: `node_agent@node-1 / GetInventory`
- node 2 failure: `node_agent@node-2 / GetInventory`

`Snapshot.HadError` treats instance-qualified queries exactly. Therefore a node 2 inventory failure cannot downgrade node 1's stopped-unit finding, while a node 1 inventory failure does downgrade it and blocks execution.

The existing provenance classifier recognizes instance-qualified node-agent writers as node-agent service observations, preserving the normal freshness gate.

## Safety boundary

This change does not act on arbitrary partial data. It preserves all existing protections:

- registry reduced-harvest downgrade
- universal conclusive-failure verdict closure
- evidence provenance and freshness
- single gated remediation path
- behavioral-governance decision
- action hard blocklist and unit allowlist
- approval, cooldown, and failure-rate policy
- workflow routing and audit

A future generic evidence-requirement declaration may be useful for actions depending on complete member sets or multiple heterogeneous sources, but it is outside this bounded PR. Such actions remain governed by their existing rule-level source guards and registry downgrade behavior.

## Proof obligations in this PR

- reduced harvest no longer changes configured `enforce` into `observe`
- an unrelated collector failure permits a conclusive target finding to dispatch
- another node's `GetInventory` failure does not veto the target node
- the target node's `GetInventory` failure downgrades the finding to `UNKNOWN`
- compromised reduced-harvest findings do not dispatch for execution
- full-harvest `UNKNOWN` findings do not dispatch for execution
- a nominal `FAIL` carrying `CheckError` does not dispatch for execution
- compromised findings may still traverse dry-run rehearsal
- instance-qualified node-agent evidence retains normal trust classification
- the shared trust gate enforces the same verdict closure for direct execution