# Annotation Coverage

`globular awareness annotation-coverage` reports gaps where awareness metadata is missing or drifting from code.

## What It Reports

- High-risk files with no `//globular:` annotations (watchlist-driven)
- Critical invariants with no enforcing symbols (`enforces` / `protects`)
- Symbols with `hash_schema` contracts but no `tested_by`
- Symbols with `state_transition` contracts but no `tested_by`
- Files touched by critical fix-ledger cases that still have no annotations

## Command

```bash
globular awareness annotation-coverage
```

Optional:

```bash
globular awareness annotation-coverage --watchlist docs/awareness/high_risk_files.yaml --json
```

## Watchlist

Default watchlist path:

`docs/awareness/high_risk_files.yaml`

This file should include convergence, reconciliation, desired/installed/runtime, package install, restart/supervisor, repository authority, objectstore topology, and xDS/Envoy generation paths.
