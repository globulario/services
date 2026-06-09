---
id: scylla_lww_is_acceptable_etcd_owns_ordering
type: architecture_decision
status: accepted
summary: Globular satisfies physical_clocks_disagree_use_logical_ordering. Order-sensitive control-plane state lives in etcd (Raft consensus = logical clock). Scylla-backed stores use standard last-write-wins-by-key, which is the inherent CQL document/KV semantic, not a defect. No code manipulates cell timestamps or compares node-origin timestamps to pick a winner.
invariants:
  - meta.physical_clocks_disagree_use_logical_ordering
related_services:
  - persistence
  - storage
  - shared-index
  - cluster-controller
---

## Scylla LWW Is Acceptable; etcd Owns Ordering

Audit of `meta.physical_clocks_disagree_use_logical_ordering` (the bug shape:
"Scylla's default conflict resolution uses cell timestamps; two skewed-clock
nodes writing the same cell concurrently can silently lose the causally-later
write"). Conclusion: **compliant — no fix warranted.**

### Evidence

1. **Order-sensitive control-plane state is in etcd, not Scylla.** Desired
   state, service releases, installed/runtime state, node status, and leader
   epoch are written through `clientv3` (etcd / Raft). Raft's `(term, index)`
   is a consensus log — exactly the logical clock the principle prescribes for
   total/causal ordering. The cluster_controller state writers use `clientv3`,
   not `gocql`.

2. **No manual cell-timestamp manipulation.** There is no `USING TIMESTAMP`
   anywhere in the Go sources. Writes use Scylla's server-assigned coordinator
   timestamp, never an application-supplied one that could encode a skewed
   clock as authority.

3. **No timestamp-comparison "pick a winner" logic.** No Scylla store compares
   two stored timestamps (`updated_at > …`, `ORDER BY …_at DESC`, `MAX(…_at)`)
   to decide which of two writes wins. `storage_store.updated_at` is a plain
   informational value column (`toTimestamp(now())`), not a conflict-resolution
   key.

4. **Contended conditional writes use Paxos LWT.** Persistence, storage, and
   shared-index use `IF` / `IF NOT EXISTS` for compare-and-set operations —
   consensus-based and skew-safe, not timestamp LWW.

### The accepted residual

The Scylla-backed stores — persistence documents (keyed by `id`), storage KV
(keyed by `k`), shared-index queue (append-only unique ids) — use Scylla's
standard last-write-wins-by-key. If the same document/key were updated
concurrently from two coordinators, the winner is decided by the coordinators'
clocks. This is the **inherent, accepted semantic of using Scylla as a
document/KV store**, not a Globular defect, and the state that genuinely
requires ordering does not use it.

### Forbidden future patterns

- Do not move order-sensitive control-plane state (desired/installed/runtime,
  leader epoch, convergence decisions) into a Scylla LWW table.
- Do not introduce `USING TIMESTAMP` with an application-supplied (especially
  cross-node) timestamp.
- Do not add "newer wins" logic that compares two node-origin `*_at` columns to
  resolve a conflict; use etcd/LWT/a logical clock instead.
