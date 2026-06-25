# Governance Arc Close-Out — RT · OT · EX

> **After EX-3b, structural governance is materially complete, but behavioral
> enforcement still needs its own verified scope.**

This note freezes the map at the close of the three-surface governance arc built on
`feat/governed-operation-gateway`. It is a checkpoint, not a launch: it records what
is *structurally* done, classifies what remains as carry-forward, and hands the next
tier (E / behavioral) a clean blade rather than a junk drawer.

## The three authority surfaces

The arc fenced the three ways the platform exercises authority. Each was opened with
a verify-first surface audit, then closed by routing the ungoverned paths through a
governed seam **plus a ratchet** that keeps them closed.

| Surface | One-line | Audit | Outcome |
|---|---|---|---|
| **RT — writes** (Tier D) | Unsafe hands fenced | `rt1-direct-write-surface-audit.md` | Owner-owned writes to critical state flow through guarded, leader-fenced, identity-checked seams; 4 ratchets prevent regression |
| **OT — reads** (Tier F) | Lying eyes constrained | `ot1-observe-truth-surface-audit.md` | The doctor's evidence tells the truth about its own freshness and harvest completeness; reduced-harvest downgrades, not just labels; source-name matching reliable; RBAC cache cross-instance invalidated |
| **EX — execution** (Tier G) | Ungoverned actions routed through governed execution | `ex1-execution-surface-audit.md` | The governor, three-tier model, leader-fence, approval-token, and evidence-trust gate already existed; EX made the node_agent host-execution boundary coherent + enforced, and made two doctor safety memories survive restart/failover |

**The shape that worked, every time:** `verify-first audit → find the genuinely
ungoverned seam (correcting the audit's overstatements) → route through a governed
primitive → add a ratchet (a test/scanner/gate) so it cannot silently reopen.`

## What each tier actually shipped

- **RT (Tier D)** — owner-guarded write primitives + multi-writer model; controller
  critical-state publish paths funneled through guarded primitives (ingress, objectstore,
  pki/ca, scylla); break-glass scripts gated + CI-enforced; single ownership-truth table
  (`config.CriticalKeyPolicies`). Four ratchets: config-primitive coverage test,
  principle-check scanner, break-glass CI gate, registry-agreement test.

- **OT (Tier F)** — evidence stamped with real collection time (re-arming the freshness
  gate); reduced-harvest **downgrade** of conclusive findings whose own evidence source
  errored (not just a label); base↔instance `HadError` source-name consolidation (which
  also fixed a live dead gate); atomic desired+runtime config write; service-config
  cache freshness signal + doctor rule; RBAC `srv.cache` cross-instance invalidation via
  the generation-key watcher.

- **EX (Tier G)** — node_agent mutating systemd unit actions routed through
  `internal/supervisor` with a **non-vacuous** boundary gate, wired into CI (the
  security-boundary checks were not CI-run before); HARD RULE #6 reconciled to the
  enforced rule. Two doctor safety memories now survive restart/failover via ai-memory
  (never etcd): the **escalation gate** (EX-3, a safety refusal — no TTL, cleared only
  by success/operator) and the **failure-rate audit ring** (EX-3b, operational memory —
  30d TTL).

## Carry-forward residuals (NOT a fourth governance tier)

These are tidy-up, classified explicitly so none masquerades as load-bearing
governance. Each is independent and low-severity unless the BH scoping audit proves
otherwise.

| Residual | Surface | Severity | Note |
|---|---|---|---|
| **OT-2 #3** — absence-as-health | OT | low | A confident PASS that reads an empty map *without citing the source* in its evidence isn't reachable by the reduced-harvest downgrade net; needs a per-rule guard. A finite audit-each-PASS-rule sweep. |
| **OT-4 principle promotion** | OT / AG repo | curation | Promote `meta.binding_outlives_evidence_until_invalidated` + `health.requires_fresh_evidence` candidate→active in the awareness-graph repo (turns the OT discipline into enforced rules). Cross-repo; needs embeddata rebuild. |
| **Surface-B gap #4** — `WithSerializable()` runtime reads | OT | medium, parked | Non-quorum reads can serve a stale follower view during leader election. |
| **EX-4 / sibling** | EX | n/a | None — EX-3b was the last sibling. |
| **CG-3** | CG | deferred | Pre-existing deferral, unrelated to RT/OT/EX. |

## Next: Tier E / behavioral (BH) — verify-first, no code yet

Tier E is the **last real tier**: the behavioral-enforcement layer (meta-principles +
`principle-check` scanners + behavioral ratchets that CI hard-gates) the whole
coherence loop rests on. It is opened the same disciplined way as RT-1/OT-1/EX-1:

1. **A verify-first scoping audit first** — confirm BH-2/BH-3's definitions against the
   live roadmap, ai-memory, the awareness graph, and the code **before** proposing any
   change. The session's repeated lesson: claimed-remaining items evaporate or reframe
   under verification (RT-4 globularcli, the HIDDEN_WORKFLOW lifts, OT-3 #3, EX day-0,
   the EX-3 TTL forbidden-fix). Do not trust the roadmap's framing un-checked.
2. **Treat the residuals above as separate carry-forward** unless the audit proves one
   is load-bearing for BH.
3. **Build BH-2/BH-3 only after the scope is proven** — the same `audit → governed seam
   → ratchet` shape that closed RT/OT/EX.

The plaque, in one line: **the platform's authority skeleton is closed; its behavioral
nervous system is the next, separately-scoped tier.**
