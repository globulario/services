# Claude Instruction: Backend Awareness Stabilization Plan Before Frontend Awareness

> **Authoritative directive received 2026-05-10.** This document supersedes
> any conflicting guidance in the awareness instruction set. It formalises
> the F-A10 recommendation in `claude_frontend_awareness_graph_instructions.md`.

## Decision

Do **not** start frontend awareness yet.

Frontend awareness is valuable, but it should wait until backend awareness
becomes stable, boring, and part of Claude's normal working routine.

The priority now is to let backend awareness mature through real use.

## Reason

Backend awareness is still discovering its own missing pieces:

- trust envelope propagation
- MCP/preflight output consistency
- failure_mode coverage gaps
- detector mappings
- freshness semantics
- prefix/id convention bugs
- coverage ratchets
- learning-loop closure
- semantic-diff authority budget
- untracked YAML policy

Starting frontend awareness now would multiply these unstable patterns
across a much larger and faster-changing surface.

Frontend awareness should inherit a mature backend awareness pattern, not
an experimental one.

The frontend awareness design itself warns about this: do not start F1
until trust envelope wiring is shipped and observed working for at least
one cycle.

---

# Current Strategy

## Phase 1: Stabilize Backend Awareness

For the next backend work cycles, use awareness as part of normal development.

Claude should:

1. Run awareness preflight before risky backend edits.
2. Inspect the trust envelope before trusting any recommendation.
3. Treat `NO_MATCH` as unknown, not safe.
4. Fix missing graph edges when discovered.
5. Promote repeated failures into:
   - failure_modes
   - invariants
   - forbidden_fixes
   - detector mappings
   - regression tests
6. Keep coverage ratchets green.
7. Consolidate duplicated trust/freshness/coverage logic.
8. Document any missing pattern that appears more than once.
9. Improve awareness only when real use reveals a gap.
10. Avoid adding new awareness subsystems unless the existing trust path
    requires it.

The goal is not expansion.

The goal is operational discipline.

---

# Immediate Backend Awareness Priorities

Complete these before any frontend-awareness implementation:

## P0-1: Trust envelope into preflight and MCP

Every agent-facing awareness surface must expose the trust envelope.

Required surfaces:

- `awareness_preflight`
- `awareness_decision_context`
- `awareness_impact_file`
- `awareness_match_incident_patterns`
- `awareness_pre_edit_context`

The open requirements list this as the top P0 item because agents must not
receive guidance without freshness and coverage gates.

## P0-2: End-to-end integration test against a real diff

Add one black-box test proving the joined system works:

```text
git diff
  -> preflight
  -> coverage lookup
  -> freshness check
  -> assurance.Compose()
  -> TrustEnvelope
```

> **Note**: the source directive cut off mid-code-block at this point.
> The full specification of the integration test (assertion list, fixture
> guidance, location) lives in
> `claude_awareness_next_pr_instructions.md` under "Scope C: End-to-End
> Integration Test Against a Real Diff". Treat that as canonical.

Key assertions the test must enforce (from Scope C):

1. Preflight returns a report.
2. Report contains a non-empty `TrustEnvelope`.
3. Freshness gates the verdict.
4. Coverage affects the verdict.
5. A high-risk matched failure mode cannot return `trusted` unless
   coverage is sufficient/strong AND freshness is fresh.
6. A stale fixture cannot produce a trusted safety verdict.
7. A `NO_MATCH` diff returns `unknown` or `unsafe`, never `trusted`.

The previous prefix bug shipped because isolated unit tests passed while
the composed path was wrong. This test is the guard against that class of
failure.

---

# Gates to Frontend Awareness

Frontend awareness work (Phase F1+) may begin only when:

1. P0-1 has shipped and at least one production cycle has passed without
   trust-envelope-related issues.
2. P0-2 is in CI and has caught at least one would-be regression (i.e.
   the test has demonstrated value, not just existence).
3. The CI ratchets (`--min-well-covered`, `--min-detected`,
   `--baseline-orphans`) have remained green across that cycle without
   manual lowering.

If any of these three conditions slip, the frontend start date slips with
them. Do not start F1 to compensate for missed deadlines on the backend
gates.

---

# Cross-References

- `claude_awareness_next_pr_instructions.md` — full P0-1 / P0-2 specification
- `claude_awareness_open_requirements.md` — wider P0/P1/P2 backlog
- `claude_frontend_awareness_graph_instructions.md` — frontend design (deferred)
- `awareness_assurance_honest_assessment.md` — context for why this matters
