# Case W03: Installed State Runtime Proof and Kind-Aware Doctor

## Pattern
`installed_state_runtime_mismatch` fires for COMMAND packages (yt-dlp, cli, sha256sum)
that are not supposed to have systemd units — the doctor treats them as broken daemons.

## Root Cause
The `commandPackage()` function in the rule is a hardcoded list, not driven by package metadata.
New command packages added to the catalog will be incorrectly flagged unless the list is manually updated.
Additionally, there are no tests for this rule.

## Required Invariant
Installed state is not converged unless runtime proof matches package metadata.
COMMAND packages must not be expected to have systemd units.
Package kind is the single source of truth for runtime expectation — not a hardcoded list.

## Implementation

### W03-A: Tests for the existing rule
Add `installed_state_runtime_mismatch_test.go` covering:
- COMMAND package with no unit → no finding
- SERVICE package with no unit → finding
- SERVICE package with active unit → no finding
- SERVICE package with inactive unit → finding
- Node with stale heartbeat → finding (stale age)

### W03-B: Extend commandPackage() with catalog-driven check
The catalog (`getCatalogEntry(name)`) already knows package kind. The rule should query it:
```go
if ce := getCatalogEntry(canon); ce != nil && !ce.RuntimeRequired() {
    continue
}
```
This is a future hardening step — the hardcoded list is correct for current packages.
Track as technical debt to remove the list once catalog integration is complete.

## Files / Components
- `rules/installed_state_runtime_mismatch.go`: existing implementation, add catalog hook
- `rules/installed_state_runtime_mismatch_test.go`: new test file

## Tests
- Unit: COMMAND package (rclone) with no unit → no finding
- Unit: daemon package (keepalived) with unit active → no finding
- Unit: daemon package (keepalived) with unit missing → finding
- Unit: daemon package with unit state=failed → finding (with state in summary)
- Unit: node with stale heartbeat → finding with stale reason
- Unit: package with empty version is skipped (no false positives)

## Remaining To Reach DoD
- Replace hardcoded commandPackage() with catalog-driven runtime expectation query
- Integration: install keepalived, delete unit, doctor detects mismatch, repair dispatched

## DoD
Doctor only flags missing systemd units for packages that are supposed to run as daemons.
