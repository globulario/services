---
name: globular-repair-evolution
description: Use for Globular incident repair, simulation-discovered defects, feature implementation, architecture evolution, scenario creation, adversarial cluster testing, autonomous engineering, and behavioral learning from execution. Enforces the simulation-first governed repair/evolution lifecycle.
---

# Globular Repair and Evolution

Canonical strategy: `docs/design/autonomous-repair-evolution-strategy.md`.

Read the canonical strategy before substantial work. Do not duplicate or reinterpret it here.

Core law:

> Novel repair and evolution must be proven against a reproducible model of the system before they may alter the authoritative system.

Route work through:

```text
CLASSIFY
  → GROUND CONTRACT + AUTHORITY
  → CHANGE ENVELOPE
  → REPRODUCE / SPECIFY SCENARIOS
  → CHANGE IN ISOLATION
  → LOCAL PROOF
  → CLUSTER SIMULATION PROOF
  → COUNTEREXAMPLE SEARCH WHEN RISK WARRANTS
  → SENSEI ADMISSION
  → NORMAL IMMUTABLE RELEASE
  → PRODUCTION VERIFICATION
  → BEHAVIORAL LEARNING
```

Use `sensei-architect` to discover/challenge contracts, ownership, invariants, failure modes, and proof. Use `sensei-admission` for the exact mutation boundary, and `sensei-closure` when admission cannot be established.

Known governed operational remedies may run through their typed production workflows. Novel code/schema/workflow/architecture repair must happen in an isolated engineering environment. Never create an AI hot-patch path into production.

Required simulation actions that skip or are unsupported are **not PASS**. Valuable adversarial counterexamples must retain a deterministic event trace and should be minimized into durable scenarios.

Simulation may write observations, evidence, outcomes, contradictions, and candidate behavioral knowledge autonomously. It may not auto-promote new production principles, destructive remedies, authority rules, contract changes, or permission escalation. Production may rely on promoted knowledge; unpromoted knowledge is hypothesis/evidence only.

When production and simulation materially disagree, record a fidelity defect instead of hiding the discrepancy.
