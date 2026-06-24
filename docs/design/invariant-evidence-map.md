# Invariant Evidence Map (CG-1)

> CG-1 deliverable from the coherence-loop roadmap ([roadmap-to-9.md](roadmap-to-9.md)).
> Audits every invariant in `docs/awareness/invariants.yaml` for real evidence
> (guard + tests) and flags gaps. Snapshot: **2026-06-24**, 260 invariants.

## Method

For each invariant, classify the evidence signals: `status`, `severity`,
presence of `required_tests` (or `test_not_applicable_reason`), and presence of
a code anchor (`implemented_by` / `protects`). The audit is reproducible:

```
awg validate -repo-root <services> -ag-repo <awareness-graph>   # refs/ids/sources
awg audit -check -warn-stale -services-repo <services> -ag-repo . # coverage + freshness(advisory)
```

## Distribution (260 invariants)

| Signal | Count | Read |
|--------|-------|------|
| `status: active` | 243 | 93% active — the corpus is NOT mostly-proposed |
| `status: planned` | 16 | The future `k8s.*` substrate domain (legitimately pending) |
| `status: candidate` | 1 | `cluster_event_must_carry_node_or_cluster_scope` |
| has `required_tests` | 225 | — |
| has `test_not_applicable_reason` | 26 | explicit non-applicability |
| has impl anchor (`implemented_by`/`protects`) | 235 | 25 without |
| **critical/high WITHOUT test-or-NA-reason** | **0** | coverage is complete for the gating tiers |

Severity (post-CG-1 canonicalization): 142 critical, 101 high, 17 medium.

## Findings & actions

1. **6 malformed severities — FIXED** (commit canonicalizing `ERROR`/`warn`/
   `warning`/`degraded` → `{high, medium}`). All were active + tested; only the
   `severity` value was off-vocabulary.
2. **Critical/high test coverage is complete** — 0 gating-tier invariants lack a
   test or an explicit `test_not_applicable_reason`. The `awg audit` test-coverage
   check confirms (239 critical/high covered).
3. **7 active critical/high without an impl anchor** — all have `required_tests`,
   so they are *verified*; they only lack `implemented_by`/`protects` **metadata**
   (which file the invariant guards). Metadata-completeness, not a verification
   gap. Low priority backfill:
   - `canskip_predicates_must_check_multiple_fields`
   - `cross_node_staleness_must_use_server_clock`
   - `expected_sha256_param_must_carry_subject_name`
   - `hardcoded_set_must_derive_from_source`
   - `heartbeat_must_not_take_non_critical_dependencies`
   - `minio_commodity_no_hard_dependency`
   - `isbootstrap_consumer_must_check_window`
4. **17 non-active** (16 planned `k8s.*` + 1 candidate) — pending by design; no
   evidence expected yet. Tracked, not gaps.

## Open follow-ups (not done; deferred by choice)

- **Mechanize the severity vocabulary** (CG-4-adjacent): make `awg validate`
  reject any `severity` outside `{critical,high,medium,low}` as a hard finding so
  finding #1's class cannot recur silently. Deferred — fixed the instances first.
- **Impl-anchor backfill** for the 7 in finding #3 (metadata only).

## Conclusion

The corpus is materially healthier than the headline assessment implied: 93%
active, complete gating-tier test coverage, and the residual gaps are either
metadata-completeness (7 anchors) or legitimately-pending planned work (17). The
one real data-quality defect (6 malformed severities) is fixed.
