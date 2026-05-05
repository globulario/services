# Case 10: Recovery Without DNS

## Status In Code
- PARTIAL: CLI supports `--controller` IP:port flows; controller->node critical paths are not fully validated under DNS-failure simulation.

## Target Invariant
- Operators and controllers must recover cluster-critical state using direct control-plane endpoints when DNS is degraded.

## Required Implementation
- Ensure all resilience CLI commands support direct controller IP/port.
- Confirm controller-to-node critical operations can use node IDs/IPs without DNS.
- Add doctor remediation text referencing direct-IP workflows.

## Remaining To Reach DoD
- Add integration test harness mode that disables DNS resolution during recovery workflow.
- Verify and patch controller->node calls to prefer node IP from membership/state when DNS lookup fails.
- Add runbook section with direct-IP commands for ingress/scylla/schema recovery.

## Tests
- Integration: DNS unavailable, run ingress/scylla CLI checks and republish/enforce successfully via direct endpoint.

## DoD
- DNS outage does not block recovery of ingress/objectstore/schema invariants.
