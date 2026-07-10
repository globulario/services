# Design — `globular explain`

Status: design (2026-06-22). Operator-facing access to platform self-knowledge.

## Purpose

A read-only (Tier 0) command that answers **"what does the platform know about this?"**
— sourced and freshness-honest — for an operator, or an AI acting on their behalf.
It fronts the same knowledge corpus the AI uses, so humans get the platform's intent,
history, and failure-knowledge without an AI in the loop.

This is the operator rung of the coherence loop (see
`docs/awareness/coherence_pipeline.yaml`): read/restore and AI/dev access already exist;
`explain` gives end-users a first-class window into what the platform knows about itself.

## Decision: ai-memory first, AWG optional

`explain` is backed by **ai-memory** (the in-mesh store globularcli already talks to;
holds the ops-knowledge seed, runtime lessons, `ops.role.*`, failure-mode seeds, and the
coherence-loop entries). It does NOT re-introduce a CLI→awareness-graph dependency (the
`globular awareness` CLI was deliberately removed when AWG became a sidecar). AWG
enrichment (full invariant/failure-mode graph traversal) is a later, best-effort,
optional layer behind a flag/endpoint — never required, always degrades cleanly.

## Subject types → routing

| `explain <arg>` | Detected as | Query |
|---|---|---|
| `convergence`, `capacity posture` | topic (free text) | `memory_query(tags/text)` over the corpus + runtime memories |
| `cluster-doctor`, `node-agent`, `mcp` | service/component | `memory_get(ops.role.<svc>*)` + related ids + (opt) live status |
| `golang/node_agent/heartbeat.go` | file path | memory by file/component tag; (v2) AWG `briefing(file=)` |
| `ops.always.doctor.reduced-harvest-honesty` | id | `memory_get(id)` + `related_ids` expansion |
| a live doctor finding id | finding | finding → invariant → what it protects → repair_plan/runbook |

Resolution order: explicit `class:id` → looks-like-id (dotted, `ops.*`/`meta.*`) →
existing file path → known service (registry/catalog) → else free-text topic.
`--as topic|service|file|id` forces classification.

NOTE on text search: ai-memory's multi-word `text_search` returns 0 (known bug). The
explain core MUST tokenize and prefer `tags=` / `type=` / `memory_get` by id, and split
free-text into single-term tag/text probes — never pass a multi-word string verbatim.

## Output contract (grounded + honest — the whole value)

Sectioned; every claim cited with its source id + `authoredIn` path:

1. **Summary** — one paragraph from the best-matching entry.
2. **Why it exists / what it protects** — intents + invariants.
3. **How it breaks** — failure_modes + incident patterns.
4. **Forbidden** — forbidden_fixes ("do NOT …").
5. **How to operate / fix** — repair_plans + runbooks + representative `cli_commands`.
6. **Observed** (only with `--live`) — runtime status from `cluster_get_*` / `infra_probe`,
   rendered as an explicit **intent vs observed** contrast.
7. **Provenance & freshness** — sources, knowledge status, and the standing caveat:
   *this is recorded intent/knowledge, not runtime authority — verify live state.*

Honesty rules (non-negotiable):
- **No fabrication** — if nothing matches, say *"no recorded knowledge for X"*
  (absence is explicit — `degraded_is_explicit_not_hidden`).
- **Label intent vs runtime** — never present recorded knowledge as observed truth.
- **Degraded-aware** — if a source is unreachable, answer from what's available and say so.
- **Citations always** — an uncited claim is a bug.

## Flags

`--live` (runtime contrast), `--depth brief|standard|deep`, `--format table|json|yaml|md`,
`--as <type>`, `--source memory[,ops,graph]`, plus standard `--controller/--token/--insecure/--memory`.

## Architecture — one core, three faces

A shared `explain` core: `Subject → ExplainResult{sections[], citations[], freshness, status}`.
The **CLI** (`globular explain`), an **MCP `explain` tool**, and the **globular-admin
console panel** all consume the one core. CLI is a thin presenter; `--format json` makes it
scriptable and console-ready.

Placement: `golang/globularcli/explain_cmds.go` (package main) for the CLI; the core in a
small reusable package (e.g. `golang/explain/`) so MCP + console import it. ai-memory access
reuses the existing `ai_memorypb` client + `dialGRPC` (as `ops-knowledge` cmds do).

## Phasing

- **v1** — topic / service / id over ai-memory, cited, `--format json`, read-only.
- **v2** — `--live` runtime contrast; finding → repair_plan chains; optional AWG enrichment.
- **v3** — surface the same core in the admin console "what the platform knows" panel.

## Why it's safe (and more than a search box)

- **Grounded** — answers cite source ids + `authoredIn`; "here's what we know and where,"
  not a hallucination.
- **Honest about uncertainty** — recorded knowledge is labeled as intent, not runtime truth.
- **Self-current** — the write-back loop (`ops-knowledge export` + awareness rebuild) keeps
  the corpus true as the platform changes, so `explain` doesn't rot like a wiki.

Realizes `globular.vision.ai_operable_cluster`: a cluster an operator — or an AI on their
behalf — can converse with about its own design and history, and trust the answers.
