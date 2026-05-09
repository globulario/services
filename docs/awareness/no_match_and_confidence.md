# NO_MATCH and Confidence — What Awareness Cannot See

`NO_MATCH` and low confidence are the two most dangerous outputs awareness can produce. Both are easy to misread as "the change is safe."

---

## What NO_MATCH Means

`NO_MATCH` from `awareness.preflight` or `awareness.agent_context` means:

> No node in the awareness graph matched the task description or the changed files.

It does **not** mean:
- The change is safe
- No invariants are affected
- The coverage is complete

It means: **the awareness system has no opinion**.

### When NO_MATCH Is Expected

- The task involves a file that was never indexed (run `globular awareness build`)
- The task description uses terms not in the graph's vocabulary
- The graph is stale and misses recent additions

### When NO_MATCH Is Dangerous

- The file was recently changed and the graph is stale
- The file implements an invariant that was never declared in YAML
- The file is in a high-risk area (`golang/awareness/`, `golang/mcp/`, `cluster_controller/`)

**Rule:** If NO_MATCH appears for a high-risk file, grep `docs/awareness/invariants.yaml`, `failure_modes.yaml`, and `services.yaml` directly before proceeding.

---

## Confidence Levels

| Level | Meaning | How to proceed |
|-------|---------|----------------|
| `high` | Graph matched, no blind spots | Safe to rely on the output |
| `medium` | Graph matched with minor blind spots | Proceed with awareness of gaps |
| `low` | Partial match or significant blind spots | Investigate blind spots before relying on output |
| `unknown` | Graph unavailable or no match at all | Do not rely on awareness output |

### confidence = "unknown" is NOT a green light

`confidence: "unknown"` means the system could not assess the risk. It is equivalent to: "I don't know." In a high-risk area, this requires manual review.

---

## Blind Spots

Every preflight response includes a `blind_spots` array. Each item describes a check that could not be completed.

### Common Blind Spots and What to Do

| Blind Spot | Cause | Action |
|-----------|-------|--------|
| `live overlay absent` | No live-snapshot run | Run `globular awareness live-snapshot` |
| `runtime is noop` | No cluster address configured | Accept: local development mode |
| `graph stale` | Graph rebuild overdue | Run `globular awareness build` |
| `file not indexed` | New file not in graph | Run `globular awareness build` |
| `RBAC evidence missing` | RBAC extractor not run | Run `globular awareness build --include-rbac` |

**Never treat a response as complete if `blind_spots` is non-empty and you're making a risk-gated decision.**

---

## The Safe Chain

A safe edit satisfies all of the following:

1. `preflight.confidence` is `high` or `medium`
2. `preflight.blind_spots` is empty or contains only `runtime is noop`
3. `scan_violations` returned no findings
4. All `required_tests` from `pre_edit_context` pass
5. At least one `coverage` field shows `strict_verified` or `verified`

If any of these conditions is unmet, the change may be safe — but awareness cannot prove it.

---

## Example: Safe vs Unsafe NO_MATCH

### Unsafe NO_MATCH

```json
{
  "status": "NO_MATCH",
  "confidence": "unknown",
  "blind_spots": [
    "graph stale — rebuilt 72 hours ago",
    "live overlay absent — no live-snapshot recorded"
  ],
  "coverage": { "graph": "not_checked" }
}
```

→ Do NOT proceed. Rebuild the graph, run live-snapshot, retry preflight.

### Acceptable NO_MATCH

```json
{
  "status": "NO_MATCH",
  "confidence": "medium",
  "blind_spots": ["runtime is noop"],
  "coverage": { "graph": "checked", "raw_yaml": "checked" }
}
```

→ The task does not match any known invariant or service. This may be a genuinely new area. Proceed carefully, document the change, consider opening a proposal if a new pattern is established.
