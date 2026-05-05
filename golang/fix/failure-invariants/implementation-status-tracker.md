# Failure Invariants Implementation Tracker

## Current Status (updated 2026-05-04)

| Case | Status | Summary |
|------|--------|---------|
| 01 | DONE | Under-replication detection wired |
| 02 | PARTIAL | `Authoritative` field stamped; promotion reconciler not implemented |
| 03 | DONE | Absence-as-destructive-intent handled via LKG + IsExplicitDisable |
| 04 | PARTIAL | Shared LKG library + ingress migrated; Envoy/MinIO/DNS not yet migrated |
| 05 | DONE | `config.CriticalEtcdKeys/Prefixes` shared; doctor wired; `ValidateCriticalKeyWrite` enforced at `publishIngressSpec`; registry completeness tests pass |
| 06 | PARTIAL | Delete-approval guard implemented + 3 unit tests pass; integration + multi-domain tombstone missing |
| 07 | DONE | Completion invariant enforced |
| 08 | PARTIAL | `withBounded()` applied to 13 hot paths (reconcile, repair, pipeline, startup wiring); DNS reconciler paths not yet audited |
| 09 | DONE | Authority-first startup ordering enforced in `main.go` |
| 10 | PARTIAL | `dialNodeAgentForNode/Endpoint` fallback in workflow_trigger + main.go; integration test + runbook missing |
| 11 | PARTIAL | `IsExplicitDisable()` + DEGRADED_SPEC_INVALID etcd write on ambiguous disable; rate-limited audit missing |
| 12 | PARTIAL | `topologyPreflightForRemove` + `driftTopologyPreflight` implemented; objectstore checks + doctor lane finding missing |

## P0 Finish Order
1. Case 08: audit DNS reconciler + watcher callback context usage (final audit pass).
2. Case 12: add doctor lane-level finding when drift action denied by topology gate.
3. Case 06: add integration test for unauthorized delete auto-restore.

## P1 Finish Order
1. Case 11: rate-limited event stream record for operator audit of rejected disables.
2. Case 04: migrate LKG helper to Envoy/MinIO/DNS consumers.
3. Case 10: integration test harness mode with DNS disabled; runbook section.
4. Case 02: bootstrap→authoritative promotion reconciler.

## Exit Criteria
- All partial cases have passing unit + integration coverage for listed remaining gaps.
- Doctor emits explicit findings for missing/unsafe state before operator log-diving is required.
- No critical runtime action can be triggered by absent config.
