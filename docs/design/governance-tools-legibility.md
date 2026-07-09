# Governance Tools Legibility — Enhancement Plan & Status

> **Principle:** *Keep the correctness tax. Remove the blindfold.*
> The gate stays iron; the path to the gate gets lanterns.
>
> This is an instance of the **Controlled Co-Construction** legibility rule:
> environmental enforcement is only half of "any agent can operate this safely" —
> the other half is that the environment must be **self-describing**. A gate that
> can only say *"no"* is safe but not legible; a gate that says *"no, and here is
> the complete contract you must satisfy"* is what lets a smaller/weaker agent
> succeed without brute force, grep, or blind retries.

## Objective

Any capable agent must be able to operate the behavioral-memory / governance
tools **without** guessing schemas, grepping seed files for canonical refs,
retrying failed calls to discover required fields, or learning the promotion
contract only by being rejected.

The goal is **not** to reduce enforcement. The goal is to make enforcement
**self-describing**.

## Scope decision (why phased)

The full 12-priority spec is internally excellent but, taken all at once, **fails
the proportionality law** the project is disciplined around ("controlled, not
perfect" / "don't let governance outgrow the thing governed"). The behavioral
store currently holds a small number of promoted principles, and the evidence
that this friction matters is real but early (observed across agent sessions, not
yet across many external operators). Therefore:

- **Phase 1** ships the friction-killers justified by measured friction.
- **Phase 2** waits until real usage produces evidence the cheaper fixes did not cover.
- **Reconsider / spin-out** items are either philosophically risky or a separate,
  much larger effort.

**Evidence gate for the enhancement itself:** do not build a Phase-2 item until an
agent that is *not the author's primary assistant* stalls somewhere Phase 1 did
not cover.

## Core principle — what stays strict vs what becomes legible

Stays **strict** (unchanged enforcement):
contract-first promotion · live evidence over memory · observable conditions ·
mapped authority · contradiction checks · required tests · explicit approval.

Becomes **legible** (new):
required fields · accepted field names · valid authority refs · valid evidence
refs · why a gate blocked · the exact next operation that satisfies the block.

**Security invariant (must hold for every legibility change):** legibility of
*requirements* is safe **only because** the *satisfaction checks stay
substantive* — knowing you need a human approver does not produce one; knowing you
need observable evidence does not fabricate it. The day a check degrades from
"fact true" to "field present," publishing the recipe becomes publishing the
exploit. Keep satisfaction checks substantive so "knowing the recipe" ≠ "able to
fake the recipe."

---

## Status Matrix

Legend — Status: ✅ Done · 🟡 In progress · ⬜ Not started · ⏸ Deferred (Phase 2) · 🔬 Reconsider/spin-out

| P | Title | Phase | Requires proto? | Status | Notes |
|---|-------|-------|-----------------|--------|-------|
| 8 | Standardized error taxonomy (**governance tools only**) | 1 | no (Go-only) | ✅ | `behavioral/api/errors.go` — `ErrorCode` taxonomy + structured `GovernanceError`; `behavioral_handlers.go` `govCodeToGRPC` maps code→gRPC status. Kernel stays transport-free. |
| 5 | Validate references on write (no comma-split prose) | 1 | no | ✅ | `governance.go` `validateProposalRefs` — rejects whitespace/comma/paren refs syntactically, reports **all** offenders at once. Golden test `governance_refs_test.go`. |
| 4 | Authority & evidence discovery tools | 1 | yes (new RPCs) | ✅ | `ListAuthorities`/`ListConditions`/`ResolveRef` RPCs + MCP tools `behavioral_list_authorities`/`list_conditions`/`resolve_ref`. Single-partition scans, no `ALLOW FILTERING`. Golden test `discovery_test.go`. Kills the grep-the-disk move. |
| 1 | Complete contract on every validation failure | 1 | maybe | 🟡 | Mechanism landed: `GovernanceError` renders the full contract (code + all offenders + expected) in one response. Remaining: apply to missing-field / unknown-field surfaces beyond ref validation. |
| 3 | Gate returns the full satisfaction recipe | 1 | yes (proto: `SatisfactionStep` + 2 fields) | ✅ | Blocked/review decisions carry `satisfaction_steps` (requirement + detail + how-to + `next_operations`) and `satisfaction_summary`, threaded kernel→handler→MCP. Golden test `governance_recipe_test.go`. Next-ops reference only existing tools until P4/P6 land. |
| 6 | Amend path for proposed principles | 1 | yes (new RPC) | ✅ | `AmendProposal` RPC + MCP tool. Edits a PROPOSED principle in place (set-merge refs, scalar gate inputs); re-validates refs (P5); resets a prior contradiction check; refuses non-PROPOSED. Golden test `discovery_test.go`. |
| 10 | Golden tests — scoped to Phase 1 only | 1 | no | ✅ | P5 `governance_refs_test.go`, P3 `governance_recipe_test.go`, P4+P6 `discovery_test.go`. Covers every shipped Phase-1 behavior. |
| 11 | Surface contracts in the agent briefing | 2 | — | ⏸ | Needs AWG ↔ behavioral-memory bridge. High value later. |
| 9 | Preflight / dry-run for governance mutations | 2 | — | ⏸ | Largely redundant with a good P3; add only if P3 proves insufficient. |
| 2 | `describe_tool` contract discovery | 2 | — | ⏸ | Partly already provided by MCP JSON schemas; verify the real gap before building. |
| 7 | Safe alias handling (`service→unit`, …) | reconsider | — | 🔬 | Cuts against strict-and-explicit. If done: only the "ambiguous → explain" half; never silent-normalize. |
| 12 | Extend legibility to CLI + all gRPC handlers | spin-out | — | 🔬 | Right principle, wrong container — a separate, ~10× larger project. |

---

## Phase 1 — build order (dependencies first)

1. **P8 (minimal)** — define a stable error-code taxonomy for the governance surface.
   Codes: `missing_required_fields`, `unknown_field`, `invalid_field_type`,
   `invalid_enum_value`, `invalid_reference_format`, `reference_not_found`,
   `authority_not_mapped`, `evidence_not_observable`, `evidence_post_hoc`,
   `evidence_stale`, `contradiction_detected`, `required_tests_missing`,
   `approver_required`, `promotion_contract_unsatisfied`, `unsafe_operation_refused`.
   Each carries: `error_code`, `severity`, `blocked_operation`, `why_it_matters`,
   `complete_contract`, `next_valid_operations`.
2. **P5** — reference validation on write (structured refs, reject comma-prose).
3. **P4** — discovery tools (`list_authorities`, `list_conditions`, `resolve_ref`).
4. **P1** — complete-contract validation errors (build on P8).
5. **P3** — gate satisfaction recipe (enrich the promote block response).
6. **P6** — `amend_proposal`.
7. **P10** — golden tests covering exactly 1–6.

## Progress log

**2026-07-06 — Phase 1, increment 1 (Go-only foundation, no proto).**
Shipped **P8** (error taxonomy + structured `GovernanceError` + `code→gRPC` mapping)
and **P5** (write-time reference validation), plus the scoped golden test (**P10**).
Files: `golang/ai_memory/behavioral/api/errors.go` (new),
`golang/ai_memory/behavioral/core/governance.go` (validation wired into
`ProposePrinciple`), `golang/ai_memory/ai_memory_server/behavioral_handlers.go`
(`behavioralErr` now maps `GovernanceError`), `golang/ai_memory/behavioral/core/governance_refs_test.go` (new).
`go build` clean; new tests pass; full behavioral suite green (no regressions).
This directly closes the comma-split footgun observed in the 2026-07-06 promotion
session. No `golang/mcp/` change yet, so no awareness briefing required for this increment.

**2026-07-07 — Phase 1, increment 2 (P3, proto batch).**
Shipped **P3** (gate satisfaction recipe). Awareness briefing run first for
`golang/mcp/tools_behavioral.go` (status OK; three meta-principles applied —
admin tools call the typed API, real errors surface, no ownership bypass).
Added `SatisfactionStep` message + `satisfaction_steps`/`satisfaction_summary`
fields to `proto/behavioral_memory.proto`; regenerated `behavioral_memorypb`
(targeted `protoc`, same flags as `generateCode.sh`). Kernel builds the recipe
from the gate's block reasons (`satisfactionStep`/`satisfactionSummary` in
`governance.go`) without changing the gate itself; handler maps the new fields
(`apiSatisfactionStepsToPB`); MCP `behavioral_promote_principle` surfaces
`satisfaction_summary` + `satisfaction_steps` (and adds `unresolved_conditions`).
Golden test `governance_recipe_test.go`. `go build` + full behavioral/mcp suites green.
The recipe's `next_operations` deliberately reference **only tools that exist today**
(propose / register_condition / record_evidence / run_contradiction_check /
promote) — they will be upgraded to `list_authorities` / `amend_proposal` when
those land, so no shipped guidance points at a non-existent tool.

**2026-07-07 — Phase 1, increment 3 (P4 + P6, proto batch — Phase 1 COMPLETE).**
Shipped **P4** (discovery) and **P6** (amend). Added `ListAuthorities`,
`ListConditions`, `ResolveRef`, `AmendProposal` RPCs (+ messages, all with
`(globular.auth.authz)` annotations) to `proto/behavioral_memory.proto`;
regenerated. New kernel methods in `behavioral/core/discovery.go`; `Store` gained
`ListAuthorities`/`ListConditions` (Scylla single-partition scans — no `ALLOW
FILTERING` — plus memory + unconfigured impls); handler RPCs + MCP tools
(`behavioral_list_authorities`/`list_conditions`/`resolve_ref`/`amend_proposal`).
`AmendProposal` edits only PROPOSED principles, set-merges refs, re-validates via
P5, and **invalidates any prior contradiction check** (a content change must be
re-checked before promotion). Golden test `discovery_test.go`. The P3 recipe's
`next_operations` were **upgraded** to reference the now-real
`behavioral_list_authorities` / `behavioral_amend_proposal` tools.
`go build` + full behavioral/handler/mcp suites green; all four proto importers build.

**Phase 1 is complete** (P8, P5, P4, P3, P6, P10 ✅; P1 mechanism landed 🟡).
Remaining P1 work (complete-contract errors on missing-field/unknown-field
surfaces beyond ref validation) is a small follow-up. Phase 2 (P11, P9, P2) and
the reconsider/spin-out items (P7, P12) stay gated on real usage evidence per the
scope decision above.

## Non-Goals (load-bearing — do not cross)

- Do **not** weaken promotion requirements.
- Do **not** auto-promote principles.
- Do **not** accept malformed refs silently.
- Do **not** bypass authority mapping.
- Do **not** replace approval gates with convenience.
- Do **not** allow disk state to become the only discoverable source of truth.

## Definition of Done (Phase 1)

An agent that hits a blocked promotion can, using **only official tools**, discover:
1. why it was blocked,
2. the complete contract it failed,
3. the authorities available,
4. the evidence required,
5. the exact amendment needed,
6. the tests required,
7. whether the corrected payload would pass —

with **no guessing, no grep, no blind retries, and no safety dilution.**
