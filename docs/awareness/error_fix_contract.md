# Error Fix Task Contract

When asked to "fix this error", use this sequence:

1. Diagnostic Contract
2. Fix Contract
3. Closure Ledger

## Diagnostic Contract

```
Reported error:
Initial goal:
Suspected layer:
Allowed actions:
Forbidden actions:
Diagnostic stop condition:
```

Rules:
- Do not edit code before diagnosis is complete.
- Do not treat `NO_MATCH` as safe.
- Report stale/missing runtime evidence as a blind spot.
- Rank multiple hypotheses by evidence.

## Fix Contract

```
Root cause:
Affected layer:
Affected invariants:
Affected failure modes:
Forbidden fixes checked:
Allowed files:
Allowed change types:
Forbidden scope:
Required proof:
Fix stop condition:
```

Rules:
- Patch only inside declared scope.
- Use the smallest invariant-preserving fix.
- Add/adjust tests proving failure cannot return.
- Declare scope drift before any expansion.

## Closure Ledger

```
Closure ledger:
  reported error:
  root cause:
  affected layer:
  files changed:
  invariants touched:
  forbidden fixes checked:
  tests run:
  tests passed:
  tests skipped:
  graph integrity:
  scan violations:
  live/runtime evidence freshness:
  remaining blind spots:
  learned knowledge proposal needed:
  final status:
```

Allowed final status values:
- `fixed`
- `likely_fixed_proof_incomplete`
- `patch_prepared_not_verified`
- `blocked`

Do not claim `fixed` unless targeted proof and required awareness checks exist.
