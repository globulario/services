# Case 11: Destructive Action Guards

## Status In Code
- PARTIAL: `IsExplicitDisable()` path is implemented; rejected ambiguous-disable attempts are not yet surfaced as doctor findings.

## Target Invariant
- Service stop/delete/reconfigure-destructive actions require explicit validated intent.

## Required Implementation
- Add shared guard API used by node-agent/controller:
  - `IsExplicitDisable(spec) bool` (includes `explicit_disabled=true`, reason, generation).
- Reject ambiguous disables (missing reason, stale generation, bad checksum).
- Emit doctor finding when destructive action attempted without explicit intent.

## Remaining To Reach DoD
- Add invariant emission path for rejected ambiguous disable attempts (with node, service, generation, reason).
- Add rate-limited event stream record for operator audit.
- Reuse explicit-disable validator in additional destructive toggles beyond ingress.

## Tests
- Unit: malformed disable spec does not stop runtime.
- Integration: explicit valid disable stops runtime and is auditable in state.

## DoD
- Destructive transitions are always explicit, validated, and attributable.
