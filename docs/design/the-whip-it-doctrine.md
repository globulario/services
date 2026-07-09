# The Whip It Doctrine

> *When a problem comes along / You must whip it*
> *Before the cream sits out too long / You must whip it*
> *When something's going wrong / You must whip it*
> *Now whip it / Into shape / Shape it up / Get straight*
> *Go forward / Move ahead / Try to detect it / It's not too late*
> — Devo, "Whip It" (1980)

Mark Mothersbaugh wrote a convergence loop in 1980 and put an energy dome on it.
Every line maps to a principle this codebase already enforces. Keep it near — it's
the whole operating ethos in one chorus.

## Verse by verse

> **When a problem comes along / You must whip it** — the incident fires; you don't observe-and-record it into a drawer, you fix it.
>
> **Before the cream sits out too long / You must whip it** — this is the whole thesis. Invariants rot one "simple fix" at a time; scars go stale; drift spoils the milk. Whip it while it's fresh or it curdles into an outage you can't trace.
>
> **When something's going wrong / You must whip it** — wrong-out-loud. `analysis_available=false` next to `backend_ready=true`. Surface it, don't mask it green.
>
> **Now whip it / Into shape / Shape it up / Get straight** — contract-first. Bring the code back into conformance with the invariant. That's convergence — desired == installed == running.
>
> **Go forward / Move ahead** — forward-only. The promotion ladder only climbs; versions advance, never regress (`--allow-regression` is the DANGER flag for a reason).
>
> **Try to detect it** — the gate, the doctor, the schema guard, the tests. Detection is half the game; tonight the bug was whipped into the light the instant it shipped.
>
> **It's not too late** — the safety net. Because all four layers agree and rollback candidates exist, it's never too late. That's the floor you reached.

## Where each line is enforced

| Lyric | Principle | Where it lives |
|-------|-----------|----------------|
| **When a problem comes along / You must whip it** | Diagnose then act — don't observe-and-record an incident into a drawer. AI rule: OBSERVE → DIAGNOSE → RECOMMEND → EXECUTE → VERIFY. | `docs/ai/ai-rules.md`, `CLAUDE.md` |
| **Before the cream sits out too long / You must whip it** | The core thesis: invariants rot one "simple fix" at a time; drift spoils the milk. Whip it while it's fresh — awareness-briefing before edits, scar→law before the lesson goes stale. | `CLAUDE.md` (HARD RULE 7), `docs/awareness/` |
| **When something's going wrong / You must whip it** | **Wrong-out-loud.** Surface the red state next to the green (`analysis_available=false` beside `backend_ready=true`); never let a green mask hide a red one. | `meta.authority_must_express_uncertainty` |
| **Now whip it / Into shape / Shape it up / Get straight** | **Contract-first + convergence.** Bring code back into conformance with the governing contract; make desired == installed == running. | `docs/design/contract-first-resolution-protocol.md` |
| **Go forward / Move ahead** | **Forward-only.** The promotion ladder only climbs; versions advance, never regress (`--allow-regression` is the DANGER flag). Migrations move ahead (`behavioralSchemaVersion` bumps forward). | `docs/operational-knowledge/deploy-package-via-mcp.md` |
| **Try to detect it** | **Detection is half the game.** The gate, the doctor, the schema guard, the tests — a bug whipped into the light the instant it ships beats one found weeks later. | `make check-services`, the promotion gate, `behavioral_scylla_integration_test.go` |
| **It's not too late** | **The floor.** Because all four layers agree and rollback candidates exist, it is never too late to repair. This is what "secure" means — not that you stop falling, but that something catches you immediately and legibly. | The 4-layer state model (`CLAUDE.md`) |

## The one-line version

**Whip it while the cream is fresh.** Fix problems fast, surface them loudly,
shape the code to the contract, only move forward, detect early — and trust the
floor, because it's not too late. That's *controlled, not perfect*, set to a
synth-punk beat.

---

*This doctrine earned its place the night of 2026-07-07: a 429 came along (whipped
it), an ALLOW-FILTERING bug shipped and was whipped into the light the instant it
ran, a version raced ahead (whipped it straight), and the floor held the whole
time. When a problem comes along, you must whip it. 🎛️*
