---
id: semantic_navigation
type: architecture_decision
status: accepted
summary: Semantic distance, weighted paths, and explainable six-degrees navigation over the awareness graph.
invariants:
  - awareness.graph_is_compiled_context
tags:
  - awareness
  - navigation
  - agent-tooling
---

# Semantic Navigation

The awareness graph connects source code, invariants, failure modes, design decisions, and runtime
evidence. Semantic navigation answers the question: **how is this node connected to that one, and
why does it matter?**

---

## Concepts

### Semantic Dimension

Every traversal runs in a *dimension* that weights edges differently:

| Dimension | Focus |
|-----------|-------|
| `code` | Symbols, files, and direct structural relationships |
| `module` | Packages and import chains |
| `service` | gRPC services and RPC methods |
| `package` | Globular packages and their dependencies |
| `state` | etcd keys, state reads/writes |
| `workflow` | Workflow definitions and their phases |
| `architecture` | Invariants, design decisions, failure modes |
| `runtime` | Live runtime snapshots and evidence |
| `history` | Past incidents and failure patterns |
| `test` | Tests and coverage coverage relationships |
| `all` | Balanced across all dimensions (default) |

### Edge Weights

Each edge kind has a base cost. Lower cost = stronger structural connection.

| Cost tier | Examples |
|-----------|---------|
| 0.5 (free) | `defined_by`, `implements` |
| 1.0 (direct) | `enforces`, `tested_by`, `protects` |
| 2.0 (explicit) | `explains`, `caused_by`, `forbids` |
| 3.0 (module) | `imports`, `owns`, `depends_on` |
| 4.0 (documents) | `documents`, `describes` |
| 5.0 (structural) | `defines`, `calls`, `uses` |
| 6.0 (weak) | `affects`, `related_to` |
| 8.0 (historical) | `previously_affected`, `incident_linked` |
| 10.0 (inferred) | Default for unrecognised edge kinds |

Weights are modified by:
- **Explicit metadata**: cost × 0.7 (documented edges are cheaper to traverse)
- **Confidence**: cost × (2 − confidence) (lower confidence = higher cost)
- **Required**: cost × 0.8 (required relationships are tighter)
- **Runtime-sourced**: cost × 1.3 (live evidence is heavier)
- **Critical invariants**: effective distance − 0.5 (surfaces first)

---

## Commands

### `globular awareness related`

Find nodes semantically related to a given node, ranked by distance.

```bash
globular awareness related --node cluster_controller --dimension architecture --max-results 10
globular awareness related --node DesiredHash --dimension state
globular awareness related --node TestReplicationConvergence --dimension test
```

### `globular awareness nearest`

Find the nearest nodes of a specific type to a given node.

```bash
globular awareness nearest --node cluster_controller --type invariant
globular awareness nearest --node SetNodeProfiles --type failure_mode --dimension architecture
```

### `globular awareness path`

Find the lowest-cost semantic path between two nodes.

```bash
globular awareness path --from SetNodeProfiles --to meta.quorum_is_quality_not_constraint
globular awareness path --from DesiredHash --to TestBuildNodeContext --dimension test
```

### `globular awareness why-related`

Explain why two nodes are semantically related, with edit warnings and risks.

```bash
globular awareness why-related --from cluster_controller --to meta.quorum_is_quality_not_constraint
globular awareness why-related --from SetNodeProfiles --to do_not_wipe_storage_node
```

### `globular awareness semantic-neighborhood`

Show all semantically reachable nodes ranked by distance, across all types.

```bash
globular awareness semantic-neighborhood --node cluster_controller --max-results 20
globular awareness semantic-neighborhood --node DesiredHash --dimension state --max-depth 3
```

---

## MCP Tools

| Tool | Purpose |
|------|---------|
| `awareness.related` | Ranked related nodes by semantic distance |
| `awareness.nearest` | Nearest nodes of a specific type |
| `awareness.path` | Shortest weighted path between two nodes |
| `awareness.why_related` | Enriched explanation with invariants, warnings, forbidden fixes |
| `awareness.semantic_neighborhood` | Full neighbourhood ranked by distance |

### `awareness.why_related` — Agent usage

This is the primary tool for pre-edit safety checks when the connection between two nodes is unclear:

```json
{
  "tool": "awareness.why_related",
  "arguments": {
    "from": "SetNodeProfiles",
    "to":   "meta.quorum_is_quality_not_constraint",
    "dimension": "architecture"
  }
}
```

The response includes:
- `relationship_summary` — one-paragraph description of the connection
- `why_it_matters` — the invariant or failure mode that links them
- `edit_warnings` — what to check before editing
- `required_tests` — tests that must pass
- `forbidden_fixes` — patterns that must never be applied

---

## Traversal Limits

| Parameter | Default | Maximum |
|-----------|---------|---------|
| `max_depth` (path) | 6 | 8 |
| `max_cost` (path) | 30 | 50 |
| `max_depth` (related) | 4 | — |
| `max_cost` (related) | 20 | — |
| `max_results` | 10/20 | — |
| Node visit cap | 5000 | — |

When the cap is hit, `truncated: true` is set in the path result. Reduce depth or cost, or use
`avoid_weak_edges: true` to constrain the search.

---

## Design Invariants

- Dijkstra traverses **both outgoing and incoming edges** — relationships flow both ways.
- Runtime evidence nodes are **excluded by default** (`include_runtime: false`).
- Critical invariants receive a 0.5 distance discount so they surface first in `related` results.
- `SemanticNeighborhood` = `Related` with no type filter — it shows the full semantic gravity of a node.
- `WhyRelated` = `ShortestPath` + enrichment — same graph, richer output.

---

## Example: Pre-Edit Safety Check

Before editing `SetNodeProfiles`, run:

```bash
globular awareness why-related \
  --from SetNodeProfiles \
  --to meta.quorum_is_quality_not_constraint \
  --dimension architecture \
  --format agent
```

If the response includes edit warnings and forbidden fixes, read them before proceeding.
The awareness graph was built from source — it knows what invariants your code must uphold.
