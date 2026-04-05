# Globular AI Projection Clauses

**Status:** Binding rules
**Applies to:** every projection, resolver, MCP tool, and AI-consumable surface

These are the physical laws of the introspection layer. They are not
guidelines. Code review rejects anything that violates them.

---

## 1. Single Source of Truth

A projection MUST NOT invent, duplicate, or persist authoritative data.

All data originates from one of three places:

- **etcd** — desired state
- **Scylla** — persistent operational state
- **live services** — runtime truth

Projections are:
- read models only
- disposable
- reconstructible at any time from their source

> If a projection becomes required to rebuild the cluster → it is wrong.

## 2. Minimal Surface

A projection MUST answer one question and nothing more.

Each projection declares:

```yaml
question: "What does this answer?"
max_size: "target < 1KB"
fields: "only what is required"
```

> If a projection answers 2 different questions → split it.

## 3. Reader-Fallback

Every projection MUST have a direct fallback to its source.

If the projection is missing, stale, or degraded, the caller must be able to:

1. query the source directly
2. reconstruct the answer

> A projection without fallback becomes a hidden dependency.

## 4. Freshness & Trust

Every response MUST declare its freshness and origin:

```json
{
  "source": "scylla | etcd | runtime",
  "observed_at": "<timestamp>",
  "generation": "<optional monotonic counter>"
}
```

Without this, AI makes confident mistakes.

## 5. Scoped Query

No tool may return unbounded data. Every MCP tool and CLI verb MUST require
a target (`node_id`, `service`, `package`) or a filter.

```
❌ get_cluster_state()
✅ resolve_node("dell")
✅ get_failed_services(node_id)
✅ get_package_status(node_id, "minio")
```

> Unbounded queries = token explosion.

## 6. Projection Size

A projection must fit in one screen.

- **Target**: ~1 KB
- **Max**: ~3 KB edge case

Beyond that, split into sub-projections or require follow-up queries.

> If Claude has to scroll, you already lost.

## 7. Reconciliation Ownership

Projections MUST NOT mutate the system. They describe, suggest, and propose.
They never install, change etcd, or execute commands.

Only workflows and controllers perform actions.

## 8. Structured Remediation

Every proposed action MUST be typed, bounded, and risk-classified:

```json
{
  "action": "SYSTEMCTL_RESTART",
  "target": "minio",
  "risk": "LOW | MEDIUM | HIGH",
  "requires_approval": true
}
```

- `LOW` → auto-executable
- `MEDIUM` → operator confirmation
- `HIGH` → explicit approval + context

> No free-form shell commands.

## 9. Snapshot Isolation

Snapshots are append-only historical records, used only for audit and
debugging. They are never inputs to normal reasoning loops.

AI must NOT:
- scan snapshots by default
- compare multiple snapshots unless explicitly asked

## 10. Deterministic Projection

Same input state MUST produce same projection output.

- no randomness
- no hidden context
- no time-dependent logic (except timestamps)

This guarantees reproducibility, debuggability, and trust.

## 11. AI Consumption

Projections are designed for AI first, humans second.

- flat structures
- explicit fields
- no implicit meaning
- no hidden joins

```
❌ { "status": "ok" }

✅ {
     "service": "minio",
     "desired_state": "running",
     "actual_state": "stopped",
     "drift": true
   }
```

## 12. No Hidden Coupling

Projections MUST NOT depend on other projections. Each projection builds
directly from its source, never from another projection.

> projection → projection → projection → circular hell.

---

## Phase 1 contract (NodeIdentity)

The first projection. It sets the pattern for every projection that follows.

**Question answered:** "Who is this node?"

**Resolvable from:** `node_id`, `hostname`, `ip`, `mac`

**Return shape (exact):**

```json
{
  "node_id": "eb9a2dac-05b0-52ac-9002-99d8ffd35902",
  "hostname": "globule-ryzen",
  "ips": ["10.0.0.63"],
  "macs": ["e0:d4:64:f0:86:f6"],
  "labels": ["control-plane", "core", "gateway"],
  "source": "cluster-controller",
  "observed_at": 1712345678
}
```

**MUST NOT include:**
- services
- packages
- metrics
- logs
- health status
- heartbeat age

Those belong to **other** projections that each answer their own single
question. Cross-referencing is done by the caller chaining tools, not by
this projection enriching itself.

---

## Enforcement

- New projection PRs require an opening comment declaring clauses 1–12
  compliance
- The projection's MCP tool definition schema is reviewed against clause 5
  (scoped) and clause 11 (flat/explicit)
- Return shape validated against clause 4 (freshness) and clause 6 (size)
- CI budget check: mocked worst-case response must fit 3 KB
