# AWG Contract-First Resolution Protocol

> **Grounding.** This protocol is the operating manual for the two foundational
> meta-principles authored in the **awareness-graph** repo's canonical corpus
> `docs/awareness/generic/state_authority_invariants.yaml` (category
> `perception`): `meta.contract_must_be_explicit_before_resolution` and
> `meta.no_resolution_without_a_respected_contract`. The design rationale and the
> four-way benchmark stratification live in the awareness-graph repo at
> `docs/design/contract-first-resolution.md`. The short agent-facing summary is
> wired into this repo's `CLAUDE.md` (the SESSION PRELUDE + the Contract-first
> resolution section).
>
> **Enforcement level.** This protocol is enforced as **agent discipline**
> (prompt-level) — *not yet* a mechanical CI gate. The principles are classified
> `review_only` in `docs/awareness/meta_principle_coverage.yaml`. Mechanization
> is the Phase-2 `awg gate` step (run the contract's detect rule over the diff
> against a frozen contract set). Until then, the checklist below is the gate.

## Purpose

AWG exists to make AI agents discover the rules of the game before claiming they have won the game.

Before attempting to resolve any error, bug, failing test, warning, architectural issue, or user-reported problem, the agent must first identify the contract that defines what “correct” means.

A fix is not valid just because tests pass.  
A fix is valid only when the governing contract is identified, respected, and evidence-backed.

If the contract cannot be found, the agent must not claim resolution. The correct status is `contract-unknown`.

---

## Core rule

```text
No contract, no resolution.
```

The agent may investigate, reproduce, diagnose, and propose next steps.  
The agent may not claim “fixed”, “resolved”, or “done” unless the contract is explicit or safely inferred.

---

## Required workflow

For every issue, use this order:

```text
error
→ contract search
→ invariant search
→ evidence search
→ fix plan
→ implementation
→ verification
→ graph update/proposal
```

Never use this order:

```text
error
→ patch
→ test pass
→ claim resolved
```

That is oracle-chasing, not engineering.

---

## Required pre-edit checklist

```text
No checklist, no edit.
```

The graph is not just context — it is the **pre-repair search space**. Every
error is a graph-traversal problem before it is a code-editing problem. Traverse
it before touching code, in this order:

```text
briefing → resolve IDs → follow related contracts → check invariants
         → check failure modes → check intent → only then patch
```

Before modifying any code, write down the result of that traversal:

```text
1. Contract status:      found / inferred / missing / unknown
2. Contract source IDs or files:
3. Relevant invariants:
4. Relevant failure modes:
5. Forbidden fixes:
6. Verification plan:
```

Then act on the contract status — this is the line between fixing within the
rules and doing local patch surgery that damages the organism elsewhere:

- **found** → fix within the rule.
- **inferred** → propose, then verify carefully; flag for promotion to an
  explicit contract.
- **missing** → extract a candidate invariant; do not apply a behavioral fix
  without human approval unless the change is purely diagnostic or reversible.
- **unknown** → stop pretending. Emit a revision request / architecture
  question. Do not say "fixed".

The per-status output templates below expand each branch.

---

## What counts as a contract

A contract may be explicit or inferred, but it must be grounded in project evidence.

Valid contract sources include:

- gRPC/protobuf APIs
- YAML awareness corpus entries
- invariants
- failure modes
- incident patterns
- forbidden fixes
- architecture intent files
- implementation patterns
- tests that clearly encode expected behavior
- CLI help text or documented command behavior
- schema definitions
- existing stable behavior used by callers
- repeated code patterns with clear authority
- comments or annotations that define a rule

A contract is not valid if it is only a guess, preference, style opinion, or convenient interpretation.

---

## Contract statuses

Every repair analysis must report one of these statuses.

### `contract-found`

Use when the governing rule is explicit and grounded.

Required output:

```text
Contract status: contract-found
Contract ID/source:
Expected behavior:
Observed violation:
Evidence:
Fix strategy:
Verification:
```

---

### `contract-inferred`

Use when no single explicit contract exists, but expected behavior can be inferred from multiple grounded sources.

Required output:

```text
Contract status: contract-inferred
Inferred contract:
Evidence sources:
Confidence:
Risk:
Proposed fix:
Verification:
Should this become an explicit AWG contract/invariant? yes/no
```

---

### `contract-missing`

Use when the system clearly needs a rule, but no reliable existing rule defines correctness.

Required output:

```text
Contract status: contract-missing
Missing contract:
Why it matters:
Observed ambiguity:
Proposed contract:
Suggested invariant:
Do not apply behavioral fix until human approval unless the change is purely diagnostic or reversible.
```

---

### `contract-unknown`

Use when the rule cannot be found or inferred safely.

Required output:

```text
Contract status: contract-unknown
What I searched:
What I found:
Why resolution is unsafe:
Question/proposed contract needed:
Safe next step:
```

Under `contract-unknown`, do not say “fixed”, “resolved”, or “done”.

---

## AWG tool usage

Before editing files, use AWG as the first source of architectural memory.

Required:

- Call `awareness_briefing` for each target file.
- Read returned constraints, referenced IDs, invariants, failure modes, contracts, and forbidden fixes.
- If IDs are returned, call `awareness_resolve` before editing.
- If the issue may affect other files/components, call `awareness_impact`.
- Treat `EMPTY` as “no direct anchors found”, not as proof that no rules exist.
- Treat tool errors as degraded awareness and say so explicitly.

Do not bypass AWG because the local bug seems obvious.

---

## Contract extraction duty

When fixing an issue, also ask:

```text
Did this bug reveal a missing contract or invariant?
```

If yes, propose an AWG corpus addition.

A good extracted contract or invariant should include:

```yaml
id:
title:
contract:
why_it_exists:
violation_pattern:
evidence:
source_files:
required_tests:
forbidden_fixes:
status: candidate
```

The invariant must come from real evidence:

- a bug
- a failing test
- a repeated mistake
- a production incident
- a confusing architecture boundary
- an implicit rule found in code
- a decision that would be easy for an agent to miss

Do not invent invariants only because they sound nice.

---

## Resolution rule

A claim of resolution requires all of this:

```text
contract explicit or safely inferred
violation identified
fix respects the contract
tests/checks verify the contract
no known forbidden fix used
AWG-relevant new knowledge captured or proposed
```

If any part is missing, downgrade the result:

- `verified` means contract + tests + evidence.
- `locally-fixed` means the patch works but the contract is not fully proven.
- `contract-unknown` means no valid resolution claim.
- `needs-human-contract` means ambiguity requires an architectural decision.

---

## Forbidden agent behavior

Never do these:

- Patch first and search for rules afterward.
- Treat a passing test as proof of correctness without identifying the contract.
- Infer architecture intent from convenience.
- Delete, weaken, or bypass tests to make a fix pass.
- Replace an unknown contract with a broad generic rule.
- Claim “resolved” when the governing rule is missing.
- Add AWG invariants without evidence.
- Treat AWG as runtime truth. AWG gives constraints and memory, not live cluster state.

---

## Preferred agent behavior

When the contract is unclear, say:

```text
I can reproduce or understand the symptom, but I cannot safely claim a fix until the governing contract is identified.
```

When a fix reveals a rule, say:

```text
This bug exposed an implicit contract. I recommend adding it to AWG so the next agent does not rediscover it by accident.
```

When tests pass but the contract is missing, say:

```text
The tests pass, but this is only an oracle match unless we identify the contract behind the expected behavior.
```

---

## The AWG repair loop

Every repair should improve the graph when it discovers durable knowledge.

```text
incident
→ contract
→ invariant
→ test
→ implementation
→ evidence
→ graph update
```

This is the AWG discipline.

The goal is not only to fix this bug.  
The goal is to make this class of bug harder for future agents to repeat.

---

## Short version for CLAUDE.md

```text
Before resolving any issue, first identify the governing contract. Use AWG briefing/resolution/impact tools before editing. A test pass without an identified contract is only an oracle match, not a valid resolution. If the contract is missing or unknown, report contract-missing or contract-unknown and propose the needed contract/invariant instead of claiming the issue is fixed. Every repair should ask whether a new AWG invariant or contract should be extracted from the bug.
```
