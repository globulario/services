# Graph Integrity and Trust Management

## Core Principle

> A graph edge is not truth forever.
> It is a claim with a source, a verification level, and a stale policy.

The awareness knowledge graph exists to surface constraints, invariants, and
failure patterns before code is written. But a graph that is not validated over
time becomes aspirational rather than descriptive. Phase 11 adds a layer that
keeps the graph honest.

---

## The Problem: Graph Decay

Graphs decay when:

- Functions get renamed but required_tests entries are not updated
- Tests are deleted but fix cases stay DONE
- Files move but YAML references old paths
- Promoted proposals never appear as graph nodes
- Causal rules contradict forbidden fixes
- AI-authored knowledge becomes plausible but hollow

---

## What `awareness.graph_integrity_check` Validates

The tool runs four categories of checks:

### 1. Shape Validation

Every knowledge node must conform to its shape.

**FixCase shape (DONE):**
- At least one `required_tests` entry — critical if missing
- At least one `target_invariant` — warning if missing
- At least one `fixed_files` entry or notes — warning if missing
- Every required test must exist on disk — critical if function missing

**FailureMode shape:**
- `id`, `title`, `symptoms`, `root_cause` required
- Every `forbidden_fixes` reference must point to a real ID — critical if missing

**ForbiddenFix shape:**
- `id` required — critical if missing
- `summary` required — warning if missing
- `safe_alternative` required — warning if missing (engineers need to know what to do instead)

**CausalRule shape:**
- `id`, `root_signal`, `sequence`, `recommended_fix_order`, `confidence` required

### 2. Contradiction Detection

The most important active detector checks that causal rules do not recommend
forbidden operations.

**etcd.disarm_before_compact detector:**

The following order is **forbidden**:
```
alarm disarm → compact → defrag → verify disk
```

The following order is **required**:
```
compact → defrag → verify disk below quota → alarm disarm
```

If any causal rule's `recommended_fix_order` contains `alarm disarm` before
`compact`, `defrag`, or `verify disk`, the check fails with `exit_code: 2`.

This detector exists because a misorder here creates a silent NOSPACE loop:
disarming the alarm before the disk is below quota causes etcd to immediately
re-trigger the alarm on the next write.

### 3. Test Reference Integrity

Every required test in a DONE fix case is verified:

| Condition | Severity |
|-----------|----------|
| Function found on disk and not failed/skipped | PASS |
| Function found but CI says it failed | critical |
| Function found but CI says it's skipped | critical |
| Function not found anywhere in repo | critical |
| Function in CI results but no source path known | warning (REQUIRED_TEST_NO_PATH) |
| CI results unavailable and no repo root | metadata_only |

### 4. Graph-Dependent Checks

When an awareness graph database is available:
- **Stale edges**: edges pointing to nodes that no longer exist
- **Edge provenance**: critical edge types must carry provenance metadata
- **Orphan nodes**: knowledge nodes with no incoming or outgoing edges

---

## Trust Levels

Every edge in the graph carries a trust level:

| Level | Meaning |
|-------|---------|
| `strict_verified` | Required test passed in CI or local `awareness-ci-check` |
| `verified` | Symbol/file exists and extractor confirmed it |
| `declared` | YAML declares the relationship, no test proof |
| `inferred` | Heuristic only — no source verification |
| `proposal` | Pending proposal, not yet promoted |
| `stale` | Source changed or CI missing after changes |
| `invalid` | Referenced file/test/function is missing |

Trust levels appear in impact path outputs so consumers know how much to rely
on each hop.

---

## Edge Provenance

New edges should carry provenance metadata:

```yaml
source_type: yaml | code_extractor | test_discovery | ci_result | proposal
source_file: docs/awareness/fix_cases.yaml
source_commit: abc123
created_by: fix-ledger-extractor
last_verified_at: 1746700000
last_verified_by: ci-check
verification_level: strict_verified
stale_policy:
  - test_missing
  - file_changed_after_verification
```

Use `graph.AddEdgeWithProvenance()` when writing edges with known provenance.
Edges without provenance for critical edge types (`verified_by`, `requires_test`,
`implements`, `promoted_to`) are reported as warnings.

---

## Impact Path Query

`awareness.impact_path` traverses the graph from changed files to impacted
invariants, tests, and failure modes.

**Input:**
```json
{
  "changed_files": ["golang/node_agent/node_agent_server/xds_config_reconcile.go"],
  "max_depth": 6
}
```

**Output path example:**
```
file:xds_config_reconcile.go
  → implements (declared)
  → fix_case:lkg_expansion
  → requires_test (declared)
  → test:TestEtcdOutageServesFromLKG
```

Paths through `inferred` edges are labelled `low-confidence`.

---

## CI Gate

Run graph integrity in CI with:

```bash
awareness-ci-check --graph-integrity \
  --docs-dir docs/awareness \
  --repo-root . \
  --test-results .awareness/test-results.json \
  --strict
```

**CI fails on (exit code 1 or 2):**
- DONE fix case missing required tests
- Invalid metadata in any shape
- Missing forbidden fix reference
- Causal rule contradiction
- Promoted proposal not indexed in graph
- Edge without provenance for strict edge types
- Skipped required test

**CI warns (does not fail unless `--strict`):**
- PARTIAL fix cases
- Declared but unverified edges
- Pathless tests (function exists but path unknown)
- Missing `safe_alternative` in forbidden fix
- Inferred edges

---

## Why Not Full RDF/SHACL

This is not an attempt at academic RDF perfection.

The target is:
```
typed edges
+ provenance
+ shape validation
+ contradiction detection
+ trust levels
+ CI gate
= graph that stays honest over time
```

Bounded, practical, and useful.

---

## Known Limitations

1. **Trust summary requires full provenance backfill** — computed as zero until
   all critical edges carry `provenance_json`. Old edges added without provenance
   count as `declared`.

2. **Promoted proposal detection requires graph** — without a graph database,
   the check cannot verify that promoted proposals have corresponding graph nodes.

3. **Orphan detection scoped to knowledge types** — source_file and symbol nodes
   legitimately have sparse connections; they are excluded from orphan detection
   to reduce noise.

4. **No semantic contradiction detection** — only the etcd alarm ordering
   detector is hardcoded. Generic causal-rule-vs-forbidden-fix contradiction
   detection requires semantic understanding that keyword matching cannot provide.
