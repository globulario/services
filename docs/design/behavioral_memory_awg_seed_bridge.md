# Behavioral-memory AWG seed bridge

Status: **A-prime landed; B is the target.** Authored 2026-06-21, corrected the
same day after a verification pass (§1) found the original premise was wrong.

## 1. Correction (read this first)

The first draft of this doc claimed *"behavioral-memory has no Day-0 seed
channel."* **That was wrong.** A verification grep (the §7 gate of the original
draft) found a mature channel already exists:

- `golang/ai_memory/domains/cluster_operator/seed/` — a hand-authored pack:
  `principles.seed.yaml`, `authorities.yaml`, `conditions.yaml`,
  `forbidden_moves.yaml`, `required_evidence.yaml`.
- `golang/ai_memory/behavioral/domain/loader.go` — loads them at
  **PROPOSED_PRINCIPLE**, idempotent, **never auto-promotes**, never demotes
  governed state.
- `golang/ai_memory/domains/cluster_operator/opsknowledge/compiler.go` — also
  compiles `docs/operational-knowledge` into a behavioral seed bundle.

So behavioral-memory **is** Day-0-durable already. The verification gate did its
job: it prevented building a second bridge beside an existing one.

## 2. The real gap

The existing channel is **hand-authored / ops-knowledge-sourced**. It does **not**
project AWG authority nodes (failure_modes / invariants) into behavioral
principles. AWG is the authority for code-invariant rules (with tests and
forbidden moves); ops-knowledge is the authority for operational guidance. There
was no path that turns an AWG failure_mode into a seeded behavioral principle.

## 3. Approved model

**AWG remains the authority. The behavioral seed pack may carry projections of
AWG nodes — it must never re-author the rule.** Two phases:

- **A-prime (now, landed):** author the principle in the existing seed pack, but
  mark it AWG-derived. Every AWG-derived behavioral seed MUST carry both a
  `source_ref` (the AWG node id) and a `source_hash` (content hash of the
  canonical AWG node). Encoded in the existing `source_refs` field as
  `awg:failure_mode.<id>@sha256:<hash>` so no kernel schema change is needed.
  `generated_from: [awg:failure_modes]` marks the source family. Status stays
  PROPOSED; the loader never promotes.

- **B (target):** replace the hand-maintained projection with an AWG→behavioral
  generator:

  ```
  AWG failure_mode / invariant
    → behavioral projection generator
    → behavioral seed bundle (PROPOSED principle + catalog entries)
  ```

  removing the hand-written projection entry entirely. B does not block A-prime's
  durable safety.

## 4. Rejected alternatives

- **Plain A** (independently author the rule in the behavioral pack): rejected —
  a second clock. It repeats the rule content and invites the same
  predicate-drift demon through a different window.
- **C** (make ops-knowledge the source for this rule): rejected for this rule.
  Ops-knowledge is for operational guidance / procedures / "how agents query &
  diagnose." The sa rule is a code-authority invariant with tests and forbidden
  moves; its bell tower is AWG.

## 5. Projection hygiene: source_ref AND source_hash

- **`source_ref` → identity / idempotency.** Same AWG id already seeded ⇒ no-op.
- **`source_hash` → staleness.** Compare stored vs. freshly-computed hash of the
  AWG node:
  - equal ⇒ current, no-op.
  - differ ⇒ AWG authority CHANGED since this projection was seeded:
    - PROPOSED projection ⇒ upsert to the new content.
    - PROMOTED projection ⇒ mark the runtime projection **stale** (never silently
      serve an out-of-date active brake); re-derive and re-promote under seeder
      authority, or flag for review.

Without `source_hash` the bridge itself becomes a drift machine — serving a
projection of an AWG node that no longer says what the projection claims. Same
`id + seed_sha256` discipline the ops-knowledge seeder already uses for ai-memory.

### Drift-check (immediate follow-up, not yet built)

A validation check (pack test or cluster-doctor advisory) that, for each principle
whose `source_refs` contains an `awg:...@sha256:...` entry, recomputes the canonical
hash of the named AWG node and warns/fails on mismatch. Canonicalization MUST match
how the embedded hash was computed: `sha256(json.dumps(failure_mode_entry,
sort_keys=True, separators=(',',':')))` over the parsed `docs/awareness/failure_modes.yaml`
entry. Until this check exists, the embedded hash is a baseline/contract, not an
enforced gate.

## 6. Promotion policy

Seeded as **PROPOSED** by default (the loader enforces this). Promotion to an
active `check_action` brake is explicit and never implicit — promoting every AWG
node would turn the gate into a haunted vending machine of denials.

## 7. The sa-recognition rule (the first A-prime instance — landed)

`principle.cluster.rbac_superadmin_recognition_canonical_predicate` in
`seed/principles.seed.yaml`, with:
- `source_refs: [awg:failure_mode.rbac.authority_recognition_predicate_drift_across_sites@sha256:52f32013…]`
- `generated_from: [awg:failure_modes]`
- catalog entries: `condition.cluster.rbac.authority_recognition_change`,
  `authority.cluster.rbac.identity_recognition`,
  `evidence.cluster.rbac.canonical_predicate_tests`,
  `forbidden.cluster.rbac_sa_recognition_bypass`.

Promotion to PROMOTED (and the enforce hook to strict) is a **separate, later
step**, gated on:
- enforce-rbac-authority-gate.sh + record-rbac-gate.sh registered (done, warn)
- a real `behavioral_check_action` "allowed" verdict captured; recorder verdict
  parse confirmed against the actual field
- warn mode verified end-to-end

A promoted principle with no verified caller is an inert charm.

## 8. Validation (acceptance)

- A-prime: `go test ./golang/ai_memory/domains/cluster_operator/...` passes
  (referential integrity over the combined catalog + principle set). ✅ done.
- Day-0: fresh rebuild → Day-1 loader proposes the principle →
  `behavioral_explain_principle(principle.cluster.rbac_superadmin_recognition_canonical_predicate)`
  returns it at PROPOSED with the AWG `source_refs`.
- Drift: change the AWG failure_mode without updating the seed hash → drift-check
  warns (once built, §5).

## 9. Law preserved

```
AWG                      = authority (the bell)
behavioral seed pack     = may carry PROJECTIONS of AWG (never re-author)
promoted behavioral rows = never silently drift (source_hash guards this)
```
