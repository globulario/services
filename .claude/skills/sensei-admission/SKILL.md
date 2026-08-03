---
name: sensei-admission
description: Use for architecture-sensitive implementation when the agent asks "may I change this", "safe to modify", "admit this task", "permission to edit", "change envelope", "verify admission", "scope compliance", or needs admission before mutation. This skill decides whether one exact proposed change may be attempted; it does not prove correctness.
---

# Sensei Admission

Use this skill to decide whether one exact architecture-sensitive mutation is
permitted inside a bounded convergence bundle.

Admission is permission to attempt. It is not proof of correctness, not a
substitute for tests or review, and not a promotion path for candidates.

## When To Use

- The user asks whether an exact edit is allowed.
- A task has a convergence bundle and mutation is about to start.
- A diff must be checked against an existing admission envelope.
- `sensei-architect` routes an architecture-sensitive implementation here.

Do not use this skill for broad design, repository import, blind external proof,
or closure work that is still waiting on questions or evidence.

## Core Loop

1. Identify the exact move: files, allowed operation, expected behavior, and
   known proof still pending.
2. Prefer MCP `admit_change` when available:
   `bundle_dir`, `request_path`, `graph_nt`, `repo`, optional `policy`, optional
   `detail`.
3. CLI fallback:
   `sensei admit-change --bundle <dir> --request <request.yaml> --graph-nt <graph.nt> --repo <checkout> --output <decision.yaml> --format yaml`.
4. Interpret the decision:
   - `admitted`: edit only inside the envelope.
   - `admitted_with_conditions`: edit only inside the envelope and keep the
     conditions visible as pending proof.
   - `waiting`, `refused`, `uncertifiable`: stop mutation and route to
     `sensei-closure` or the user.
5. During editing, stop if the needed file set, behavior, or authority surface
   expands beyond the envelope.
6. Verify the diff with MCP `verify_admission`:
   `decision_path`, `bundle_dir`, `repo`, optional `detail`.
7. CLI fallback:
   `sensei verify-admission --decision <decision.yaml> --bundle <dir> --repo <checkout> --output <verification.yaml> --format yaml`.
8. Inspect receipts when needed:
   `sensei admission-status --decision <decision.yaml> --verification <verification.yaml>`.

## Compact Output

```text
Admission Brief
- Move: <exact action>
- Decision: <admitted | admitted_with_conditions | waiting | refused | uncertifiable>
- Envelope: <allowed files and operations>
- Conditions: <pending proof or none>
- Waiting on: <architect | evidence | governance | none>
- Stop rule: <what would require new admission>
```

```text
Admission Verification
- Scope: <compliant | scope_violated | unavailable>
- Extra tracked paths: <paths or none>
- Extra untracked paths: <paths or none>
- Correctness: not certified
- Remaining proof: <tests, review, observations>
```

## Routing

- Architecture-sensitive mutation: stay here.
- Waiting or refused because architecture is incomplete: use `sensei-closure`.
- Foreign repository onboarding: use `sensei-import`.
- Blind historical external proof: use `sensei-benchmark`.
- General architecture audit or incident reasoning: use `sensei-architect`.

## Non-Negotiables

- Never call admission proof of correctness.
- Never call scope compliance correctness.
- Never broaden the envelope while editing.
- Never hide untracked files from verification.
- Never mutate on `waiting`, `refused`, stale, or `uncertifiable`.
- Never execute probes, record answers, or advance convergence from this skill.
- Never use Preflight, Gate, or EditCheck as admission substitutes.
- Never invent a bundle, digest, decision, or verification receipt.
- Never treat candidate knowledge as active authority.

## References

- [references/ADMISSION-MODEL.md](references/ADMISSION-MODEL.md)
- [references/AGENT-WORKFLOW.md](references/AGENT-WORKFLOW.md)
- [references/DECISION-SEMANTICS.md](references/DECISION-SEMANTICS.md)
- [references/DIFF-VERIFICATION.md](references/DIFF-VERIFICATION.md)
