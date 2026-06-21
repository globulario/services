# AWG Operator Guide — using the Awareness Graph as an AI operator

This is the canonical, human- and AI-readable reference for **using AWG**
(the Awareness Graph) on Globular code. It complements the seed entries in
[`service-roles/awareness-graph.yaml`](./service-roles/awareness-graph.yaml) and
[`stages/awareness-graph-operations.yaml`](./stages/awareness-graph-operations.yaml).

The authoritative, always-current docs live in the **awareness-graph repo**:
- [`docs/agent-usage.md`](https://github.com/globulario/awareness-graph/blob/master/docs/agent-usage.md) — the workflow
- [`docs/cli-reference.md`](https://github.com/globulario/awareness-graph/blob/master/docs/cli-reference.md) — every command + flag
- [`docs/api-reference.md`](https://github.com/globulario/awareness-graph/blob/master/docs/api-reference.md) — gRPC + MCP

> **Read this first.** As of 2026-06-12 AWG was **extracted from Globular** and
> is now a **standalone sidecar tool** (its own repo, binary, and MCP bridge).
> What changed for operators:
> - There is **no** `globular-awareness-graph` package, etcd registration,
>   Envoy route, or reconciler-managed version. Don't look for them.
> - The old `globular awareness …` CLI subcommands were **deleted**. Use the
>   standalone **`awg`** CLI.
> - The platform MCP awareness tools — including the `awareness_diagnose`
>   composite — were **removed**. Agents reach awareness only through the
>   **`awg` MCP bridge** (`mcp__awg__*`), which exposes **seven** tools.

---

## 1. What AWG is

AWG answers, in ~2 ms: *"what must I know before editing this file?"* — the
invariants it must uphold, the failure modes it's involved in, the fixes that
look right but are known-broken, the tests that pin the contract, and the
architectural intent that governs it.

Three moving parts, all in the `awareness-graph` repo:

| Part | What it is | How you run it |
|---|---|---|
| `awg` CLI | ~30 subcommands (query, build, authoring, validation, gating, eval) | `awg <command>` |
| `awg serve` | the gRPC server (`:10120`) + a managed Oxigraph store (`:7878`) as one unit | `awg serve [--no-seed]` |
| `awareness-mcp` | stdio MCP bridge → the seven `mcp__awg__*` tools | configured in the agent's MCP config |

The graph is compiled from `@awareness` annotations + `docs/awareness/*.yaml` +
`docs/intent/*.yaml`. The server binary embeds a seed and loads it on an empty
store.

---

## 2. The seven tools (and when to use each)

| About to… | Tool | CLI |
|---|---|---|
| Decide whether/how to touch an area | `awareness_preflight` | `awg preflight --task … --file …` |
| Edit a known file | `awareness_briefing` | `awg briefing --file …` (or `--task`) |
| Chain on file anchors, no prose | `awareness_impact` | `awg impact --file …` |
| Sanity-check a concrete edit's content | `awareness_edit_check` | `awg edit-check --file … --content-file -` |
| Expand a `referenced_id` | `awareness_resolve` | `awg resolve <class> <id>` |
| "No rule here" vs "graph thin here" | `awareness_metadata` | `awg metadata` |
| Operator/debug browse (typed) | `awareness_query` | `awg query --mode …` |

There is **no** `awareness_diagnose` tool anymore. `awareness_query` is
typed/whitelisted (`by_file`/`by_id`/`by_class`/`related`) — **never raw SPARQL**.

---

## 3. The pre-edit workflow (what an AI operator must do)

1. **Preflight** the task: `preflight(task=…, files=[…])`. Branch on
   `risk_class` **and** `status`:
   - `SECURITY_RISK` / `DATA_LOSS_RISK` / `UNKNOWN_IMPACT` → get explicit user
     approval before applying any edit.
   - `ARCHITECTURE_SENSITIVE` / `CONVERGENCE_RISK` → read everything in
     `files_to_read`; brief each file.
   - **`EMPTY` is never `LOW_RISK`** — the server only collapses to EMPTY when
     coverage is sufficient; otherwise you get `OK` + `UNKNOWN_IMPACT`.
2. **Brief** each target file: `briefing(file=…)`. Read `status`, `prose`,
   `referenced_ids`, `implementation_patterns`.
3. **Resolve** any `referenced_id` marked `high`/`critical`.
4. **Edit**, then optionally `edit_check(file=, proposed_content=)` to catch a
   bad-shape change (advisory, never blocks).
5. **Run** the `required_tests` / `tests_to_run` the graph named.
6. **Record** durable knowledge — see §5.

### Reading status

- **OK** → active constraints. Follow them.
- **EMPTY** → no direct anchors. Not "safe." Call `metadata`: healthy graph →
  trust the empty; thin graph → the empty means nothing. Say so explicitly.
- **DEGRADED** / transport error → sidecar unavailable. No high-risk changes
  without user approval; fall back to code/tests/docs and say so.

---

## 4. Operating the sidecar

```bash
# Start the sidecar (server + managed Oxigraph). Omit --no-seed to load the embedded seed.
awg serve &

# Health / coverage / freshness — interpret the verdicts, not raw counts:
awg metadata            # coverage_state EMPTY|THIN|SUFFICIENT, seed_state CURRENT|STALE
awg seed-status --require-current   # does the live store contain the current embedded seed?

# Rebuild the graph after YAML / annotation changes:
awg rebuild             # YAML -> N-Triples -> validate -> update embeddata -> reload Oxigraph
awg rebuild --check     # CI: exit 1 if the committed seed is stale
```

The MCP bridge connects to the server over plaintext gRPC; point
`--awareness-addr` at the server (`localhost:10120` by default; the dev examples
use `localhost:9090`).

### Troubleshooting (sidecar model — no Envoy/etcd)

| Symptom | Cause / fix |
|---|---|
| MCP tools return DEGRADED / refused | bridge can't reach the server. Check `awg serve` is up and `--awareness-addr` matches `--addr`. |
| Briefings all EMPTY | empty store (`awg metadata` → `coverage_state=EMPTY`) → `awg rebuild`; or YAML parse error → `awg build --strict`. |
| `seed_state=STALE` | live store predates embedded seed → `awg rebuild`, confirm with `awg seed-status --require-current`. |
| Triple count unchanged after manual reload | a raw POST appends; `DELETE` the store first, or use `awg rebuild`. |

---

## 5. The write path — record a scar (contract-first)

When a fix taught you something durable, record it with **one** typed call.
This is **CLI-only by design** (not an MCP tool) so a human stays in the loop.

```bash
awg propose --kind failure_mode --title "Stale seed served after reload" \
  --contract "reload must serve fresh triples" \
  --related-invariant awareness.seed_reload_must_be_fresh \
  --source-file golang/server/reload.go \
  --required-test golang/server/reload_test.go:TestReloadFresh \
  --evidence "observed stale node after PUT"
```

`--kind` ∈ `failure_mode | invariant | required_test | forbidden_fix |
contract_unknown`. `propose` appends the entry to the right YAML, rebuilds the
seed, reloads the local store, and `git add`s it — then **stops. It never
commits; you review and commit.**

It is **contract-first**: every entry must answer what contract was
violated/clarified, what failure was observed, what test proves it, what fix is
forbidden, and which invariant/failure_mode it connects to. Vague notes are
rejected. If the contract is genuinely unknown, use `--kind contract_unknown`
with `--proposed-contract` / `--revision-request`; the entry parks under
`docs/awareness/candidates/` until resolved.

The Claude Code `Stop` hook `awg feedback-check` is advisory — it reminds you
when a session changed risky code but wrote no graph feedback. It never blocks.

---

## 6. Gating in CI

```bash
awg validate --repo-root .           # YAML structure, dangling refs, dup ids
awg audit --check                     # freshness, coverage, stale refs (exit 1 on FAIL)
awg gate --diff origin/main...HEAD    # dry-run: report which findings WOULD block (advisory)
awg gate --contracts contracts/ --enforce   # frozen-contract gate (the one mode that can exit non-zero)
```

`awg gate` (default) and `awg edit-check` are **always advisory**. Only the
frozen-contract `--enforce` mode blocks.

---

## 7. End-of-task summary line

Append to every non-trivial code task so the human can audit what you consulted:

```
AWG: preflight(<task>) -> <risk_class> | briefing(<files>) |
     invariants: <ids> | forbidden-fixes avoided: <ids> |
     tests run: <ids> | proposed: <kind/id or none> | uncertainty: <what you couldn't verify>
```
