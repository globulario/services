# Case W01: Services Drift Aging and Severity Escalation

## Pattern
`cluster.services.drift` warning stays at WARN indefinitely regardless of age.

## Root Cause
The doctor rule has static `SEVERITY_WARN` for any desired/applied hash mismatch.
There is no time-based escalation and no per-node drift start timestamp tracking.

## Required Invariant
Desired/applied hash drift must either converge automatically or escalate with a reason.
Drift older than 5 minutes affecting the whole cluster is not a warning — it is an error.

## Implementation

### W01-A: Drift start tracking in Collector
- Add `driftSince map[string]time.Time` and `driftMu sync.Mutex` to `Collector`
- When a snapshot is built, compare desired vs applied hash per node
- First time drift is detected: record `time.Now()` in `driftSince[nodeID]`
- When drift clears: delete entry from `driftSince`
- Propagate `NodeDriftAge map[string]time.Duration` into each `Snapshot`

### W01-B: Severity escalation in the rule
```
< 2 min  → SEVERITY_WARN
> 5 min  → SEVERITY_ERROR
> 5 min AND affects critical services → SEVERITY_CRITICAL
```

### W01-C: Critical service detection
Services that elevate drift to CRITICAL:
- ingress / keepalived
- dns
- objectstore / minio
- cluster-controller
- node-agent

## Files / Components
- `collector/collector.go`: add driftSince tracking + propagate NodeDriftAge
- `collector/snapshot.go`: add `NodeDriftAge map[string]time.Duration`
- `rules/cluster_services_drift.go`: use age from snapshot for severity

## Tests
- Unit: drift < 2min → WARN
- Unit: drift > 5min → ERROR
- Unit: drift > 5min on node with ingress/dns → CRITICAL

## Remaining To Reach DoD
- Integration: desired hash changes, convergence loop dispatches apply, warning clears

## DoD
Drift severity reflects urgency. Stale drift becomes a blocking finding, not background noise.
