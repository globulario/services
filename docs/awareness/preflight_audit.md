# Preflight Audit — what `awareness.preflight` actually checks

`awareness.preflight` is the agent-facing pre-edit decision RPC on
`globular.awareness_graph.AwarenessGraph`. It composes Briefing's
direct-anchor matcher with a deterministic risk classifier and returns a
single branch-able verdict: `risk_class`, `confidence`, `status`,
`coverage`, and bounded `required_actions` / `forbidden_fixes` /
`files_to_read` / `tests_to_run`.

This page enumerates the checks Preflight actually performs, in order,
so operators and agents know what the verdict is grounded in — and
where it falls back to honest-degraded rather than inventing safety.

## Inputs

A `PreflightRequest` carries:

- `task` — free-form task description, matched against
  `aw:activationTrigger` literals on `ImplementationPattern` nodes
- `files` — optional list of repo-relative paths the agent intends to
  edit
- `mode` — `PREFLIGHT_COMPACT` (default; top-3 entries) or
  `PREFLIGHT_STANDARD` (top-7, ≤10 action items)

At least one of `task` or `files` must be set.

## Check 1 — store availability

If the awareness-graph backend store is unreachable, Preflight returns a
DEGRADED response immediately:

| Field | Value |
|---|---|
| `status` | `PREFLIGHT_STATUS_DEGRADED` |
| `risk_class` | `UNKNOWN_IMPACT` |
| `confidence` | `CONFIDENCE_LOW` |
| `coverage.sufficient` | `false` |
| `blind_spots` | `awareness_store_unavailable` |
| `required_actions` | "Retry after awareness-graph store is healthy" |

This is the canonical "do not trust as proof of safety" response. No
graph data was consulted because none was reachable.

## Check 2 — per-file impact resolution

For every file in `request.files`, Preflight runs the same `collectImpact`
query Briefing uses: it walks the file's `aw:implements` edges in the
RDF store and returns the direct knowledge nodes anchored to that file.
Returned classes:

- `Invariant` — load-bearing rules the code enforces
- `FailureMode` — known ways this code goes wrong
- `Intent` — high-level reason the code exists
- `ForbiddenFix` — tempting "fixes" that are structurally wrong
- `Test` — required regression tests

If a per-file query errors, the failure is recorded in `blind_spots` as
`impact_query_failed_for_<file>` and other files continue. Partial
results are never silently treated as complete.

## Check 3 — implementation-pattern matching

Preflight loads every `ImplementationPattern` from the graph and scores
each pattern against the request:

- **Strong** — a full `aw:activationTrigger` phrase is contained in
  `task` (case-insensitive), OR ≥4 distinct activation keywords overlap
- **Medium** — 2–3 keyword overlap with a single trigger
- **Narrow** — the first file's last two path segments match a
  reference file's shape (e.g. `*_client/*_client.go`) AND ≥1 weak
  keyword overlap exists

Pattern matching is pure keyword + path-shape work. No graph traversal,
no inferred semantics.

## Check 4 — coverage classification

`coverage.sufficient` is true if any of:

- ≥1 direct anchor was found
- ≥1 file in the request is indexed in the graph (even without anchors)
- ≥1 strong-tier pattern matched

Otherwise `sufficient` is false. The `note` field explains which branch
fired.

## Check 5 — risk classification

The pure-function classifier (`classifyRisk` in `awareness-graph`) walks
a priority-ordered rule table. The first matching rule wins.

| # | Trigger | risk_class | Notes |
|---|---|---|---|
| 1 | `coverage.sufficient = false` | `UNKNOWN_IMPACT` | reason: `coverage_insufficient:` |
| 2 | anchors fired + data-loss keyword | `DATA_LOSS_RISK` | matches `data_loss`, `etcd.wipe`, `minio.format`, `blob_missing`, … |
| 3 | anchors fired + security keyword | `SECURITY_RISK` | matches `security.`, `rbac.`, `pki.`, `jwt`, `mtls`, `cert`, `token.*`, … |
| 4 | anchors fired + convergence keyword | `CONVERGENCE_RISK` | matches `convergence.`, `reconcil`, `desired_state`, `runtime.identity`, `build_id`, `entrypoint_checksum`, … |
| 5 | anchors fired + critical severity OR high-risk path | `ARCHITECTURE_SENSITIVE` | the catch-all sensitivity bump |
| 6 | anchors fired, no category | `LOW_RISK` | anchored but benign |
| 7 | no anchors + strong pattern + high-risk path | `ARCHITECTURE_SENSITIVE` | pattern alone never certifies safety on high-risk paths |
| 8 | no anchors + strong pattern + clean path | `LOW_RISK` | recipe match on a low-risk path |
| **9** | **no anchors + no strong pattern + high-risk path** | **`UNKNOWN_IMPACT`** | **Phase 5 — reason prefix `high_risk_path_no_direct_anchors:`** |
| 10 | no anchors + no patterns + sufficient coverage + clean path | `LOW_RISK` | graph indexes the area, no rules apply |

High-risk directories (CLAUDE.md R2):
`golang/{node_agent, cluster_controller, repository, rbac, security, cluster_doctor, mcp, services_manager, ai_executor}`.

## Check 6 — honest-DEGRADED gate (Phase 5)

After classification, an additional handler-level gate runs:

> **If** the request includes at least one file under a high-risk
> directory **and** the merged direct-anchor set is empty, **then**
> `status` is escalated to `PREFLIGHT_STATUS_DEGRADED`, `confidence` is
> clamped to `CONFIDENCE_LOW`, and `required_actions` is **prepended**
> with the explicit "read source / file candidates" guidance.

This catches the false-OK case where the graph indexed the file
(satisfying `coverage.sufficient`) but has no actual facts about it.
Without this gate, an unannotated high-risk file like
`rules/heal_policy.go` would have returned `LOW_RISK` via rule 10 — the
graph's indexing was being read as a safety signal when it was actually
just "scanner saw this file."

`blind_spots` always includes the line:

```
this is NOT proof of safety — the graph has no facts about this file
```

`required_actions` is prepended with:

```
Read the source file directly before editing — Preflight has no anchored facts
After your edit, file any newly-discovered invariants/failure_modes as
  candidates in docs/awareness/candidates/ so future Preflight calls become richer
```

## Check 7 — confidence

`confidence` is a separate tiered signal:

- `CONFIDENCE_HIGH` — ≥3 direct anchors AND coverage sufficient
- `CONFIDENCE_MEDIUM` — 1–2 direct anchors OR a strong-tier pattern
- `CONFIDENCE_LOW` — anything else (including all DEGRADED responses)

The Phase 5 gate forces LOW for high-risk-no-anchor regardless of what
the tier check would otherwise return.

## Check 8 — action assembly

Bounded by mode caps (compact: 5 entries per list; standard: 10):

- `required_actions` — from matched-pattern `requiresCall` + direct
  invariants ("Verify invariant still holds: …") + per-risk-class
  generic guidance + Phase 5 prepended lines
- `files_to_read` — canonical reference files from matched patterns
- `tests_to_run` — IDs from `direct_required_tests`
- `forbidden_fixes` — labels from `direct_forbidden_fixes` + matched
  pattern's `forbidsCall` ("Do not call …")

Every entry is derived from anchored facts; nothing is invented.

## What Preflight does NOT do

- **No SPARQL surface.** Inputs are typed proto fields. The graph store
  is queried via the same fixed query shape as Briefing.
- **No graph traversal beyond direct anchors.** `aw:implements` edges
  are followed one hop. Inference is reserved for a future phase; today
  Preflight is direct-only by contract.
- **No safety inference from missing data.** EMPTY, DEGRADED, and
  UNKNOWN_IMPACT all mean "the graph cannot certify this is safe."
  None of them imply safety.
- **No mutation.** Preflight is a read RPC.

## Status meanings (summary)

| `status` | When | Treat as |
|---|---|---|
| `OK` | direct anchors found OR a strong pattern matched | grounded verdict — risk_class is meaningful |
| `EMPTY` | no anchors, no patterns, coverage sufficient, no high-risk file in request | graph knows the area, nothing applies — proceed with normal review |
| `DEGRADED` | store unavailable OR high-risk file with zero anchors | best-effort — DO NOT treat as proof of safety; read source directly |

## Verification

The handler is covered by 27 tests in `awareness-graph/golang/server/`:
- `risk_classify_test.go` — 15 classifier rule-table tests
- `preflight_test.go` — 12 handler integration tests including the
  three Phase 5 gate tests (`HighRiskNoAnchorsReturnsDegraded`,
  `HighRiskWithAnchorsDoesNotDegrade`, `CleanPathNoAnchorsRemainsOk`)

Live probe to confirm the deployed contract:

```bash
grpcurl -insecure -import-path proto -proto awareness_graph.proto \
  -d '{"files":["golang/cluster_doctor/cluster_doctor_server/rules/heal_policy.go"]}' \
  globule-ryzen.globular.internal:443 \
  globular.awareness_graph.AwarenessGraph/Preflight
```

Expected for a high-risk unannotated file: `status=DEGRADED`,
`risk_class=UNKNOWN_IMPACT`, `confidence=LOW`, blind-spots cite
`high_risk_path_no_direct_anchors:`, required_actions name
`docs/awareness/candidates/`.

## Related

- `awareness/daily_workflow.md` — when to call Preflight in a session
- `awareness/composed_path_failures.md` — what Preflight catches that
  Briefing alone does not
- `awareness/agent_decision_rules.md` — how to branch on `risk_class`
- `docs/design/auto-healing-path-unification-patch-c.md` — Patch C
  history that motivated Phase 5's honest-DEGRADED gate
