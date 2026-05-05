# Warning Invariants Implementation Tracker

## Current Status (updated 2026-05-04)

| Case | Warning | Status | Summary |
|------|---------|--------|---------|
| W01 | cluster.services.drift | DONE | Drift aging implemented: WARN (<5min) → ERROR (>5min). `NodeDriftAge` in Snapshot, `driftSince` in Collector. 4 new tests. |
| W02 | pki.ca_not_published | DONE | `/globular/pki/ca` added to `config.CriticalEtcdKeys` and `critical_state_registry`. 6 pki_health tests added. |
| W03 | installed_state_runtime_mismatch | DONE | Tests added (9 unit tests). Hardcoded `commandPackage()` list validated. Catalog-driven check noted as tech debt. |
| W04 | objectstore.no_desired_state | DONE | `/globular/objectstore/config` added to `config.CriticalEtcdKeys` and `critical_state_registry`. |

## Remaining To Reach Full DoD

### W01 (drift aging)
- Integration: desired hash changes → convergence loop dispatches apply → warning clears
- CRITICAL escalation for drift affecting ingress/dns/objectstore nodes (enhancement, not blocker)

### W02 (PKI CA)
- Integration: delete `/globular/pki/ca` → controller republishes on next persist cycle

### W03 (runtime proof)
- Replace hardcoded `commandPackage()` with catalog-driven `kind` field check (tech debt)
- Integration: install keepalived, delete unit → doctor detects mismatch, repair dispatched

### W04 (objectstore guardian)
- Integration: delete `/globular/objectstore/config` → node-agents hold LKG, controller republishes
- Verify node-agent does not infer topology or restart MinIO when key absent

## Cross-cutting Notes
- `/globular/pki/ca` and `/globular/objectstore/config` now appear in BOTH the generic
  `criticalKeyRegistryPresence` rule (generating `pki.ca_missing` / `objectstore.config_missing`)
  AND their dedicated domain-specific rules. This is intentional defense-in-depth.
- The `driftSince` tracker in Collector is in-memory only. It resets on doctor restart.
  This is acceptable — the aging logic is informational guidance, not a safety gate.

## Exit Criteria
- All 4 warnings have at least unit test coverage and passing tests.
- Critical key governance covers PKI and objectstore (Case 05 requirement met).
- Drift severity reflects urgency, not background noise.
- COMMAND packages never produce false-positive runtime mismatch findings.
