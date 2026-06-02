# Awareness Coverage Report — Operator Guide

This page describes the **tool**, not its output. The latest generated
report lives at `docs/awareness/reports/coverage_report.md`.

## What it answers

- Which Go source files have direct awareness anchors today?
- Which have none (and would trigger Phase 5's honest-DEGRADED gate
  if an agent ran Preflight on them)?
- Which directories are best- and worst-covered?
- Which candidate facts are pending review in `docs/awareness/candidates/`?
- Which files should agents prioritize when filing new candidates?

The output is deterministic — two runs over the same repo produce
byte-identical reports. No timestamps in the body.

## When to run

- **Before a planning session** — surfaces where annotation work would
  pay off most.
- **After landing a batch of @awareness annotations** — confirms
  coverage numbers moved.
- **Periodically** (e.g. weekly) — tracks trend over time. Commit the
  generated report alongside the code change that moved the numbers
  so the diff is auditable.

## How to run

```bash
# Standard usage — write to the canonical report path:
python3 scripts/awareness-coverage-report.py \
  --repo-root . \
  --output docs/awareness/reports/coverage_report.md

# Quick look at stdout:
python3 scripts/awareness-coverage-report.py --repo-root .
```

The tool requires only Python 3 + PyYAML (the same dependency Phase 3's
promotion script uses).

## How agents should use it after DEGRADED preflight

When `awareness.preflight` returns `PREFLIGHT_STATUS_DEGRADED` with the
"high_risk_path_no_direct_anchors" blind-spot, the recommended action
sequence is:

1. **Read the source file directly.** The graph has no facts on this
   file; Preflight is honest about that.
2. **Make your edit.** Use code reading + memory + git log as your
   anchors.
3. **Note any new invariants or failure modes you discover.** A
   "discovery" is anything the next agent in this file would benefit
   from knowing — load-bearing assumptions, surprising couplings,
   fragile boundaries.
4. **Append a candidate** to `docs/awareness/candidates/session_discovered_invariants.yaml`
   (or a topic-specific file under `candidates/`). Use the schema
   documented in `docs/awareness/candidates/README.md`.
5. **Re-run this coverage report** to confirm the candidate landed in
   the "Candidates pending review" section. It will NOT yet appear in
   "Files with at least one direct anchor" — that's correct;
   candidates don't count as canonical coverage.
6. **Open a PR** for review. A separate operator runs
   `scripts/promote-awareness-candidate.py` once reviewed.
7. **After promotion + the next awareness-graph release**, re-run the
   coverage report. The previously-DEGRADED file should now show as
   anchored, and Preflight on it will return OK.

## What "direct anchor" means here

Three sources count toward the canonical-anchor side of the report:

1. **`docs/awareness/*.yaml`** (top-level only) — invariants and
   failure_modes via `protects.files`; intents via `expressed_by`
   (and the generic `files:` field).
2. **`docs/intent/*.yaml`** — per-intent files; `expressed_by` lists.
3. **`docs/awareness/generated/*_code_symbols.yaml`** — the source
   scanner's output. Each entry's `file:` field counts that file as
   carrying a source-side `@awareness` annotation.
   (The `awareness_graph_*_code_symbols.yaml` file is skipped — its
   `file:` paths are awareness-graph-repo-relative and don't match
   the services source universe.)

A file appears in the "anchored" count if **any** of the three sources
references it.

## What does NOT count

- Entries under `docs/awareness/candidates/` — explicitly excluded;
  the awareness-graph build pipeline also skips this directory by
  design (Phase 3).
- Files in `*_test.go`, `*.pb.go`, `*_grpc.pb.go`, `zz_version_generated.go`
  — excluded from the source universe entirely.
- @awareness annotations whose scan output hasn't been regenerated
  via `scripts/build-awareness-graph.sh` in the awareness-graph repo.
  The scanner output IS committed to `docs/awareness/generated/`, so
  rebuilding before running the coverage report ensures the latest
  source annotations show up.

## Sample output sections

The generated report (at `docs/awareness/reports/coverage_report.md`)
contains:

| Section | Content |
|---|---|
| Summary | Totals: source files scanned, anchored, unanchored, candidates |
| Canonical anchors by class | Counts of invariants / failure_modes / intents / incident_patterns / code_symbols |
| High-risk directories with unanchored files | The directories CLAUDE.md R2 lists as high-risk, sorted by uncovered count |
| Best-covered directories | Top 20 by coverage ratio |
| Recommended next annotation targets | Top 50 unanchored files under high-risk dirs (the most fertile ground for candidates) |
| Candidates pending review | Every entry under `docs/awareness/candidates/`, marked "not active in awareness graph" |

## Tests

`scripts/test_awareness_coverage_report.py` covers:
- Candidate entries are NOT counted as canonical
- Top-level canonical YAML IS counted (invariants/intents/failure_modes)
- Single-entry intent files under `docs/intent/` are counted
- Files with no anchors are listed in the recommended-targets section
- Recommended targets are scoped to high-risk dirs (clean-path
  unanchored files do NOT appear there)
- Two runs over identical fixtures produce byte-identical output
- The body contains no year-like timestamp tokens
- Excluded patterns (`*_test.go`, `*.pb.go`, `zz_version_generated.go`)
  are absent from the source universe

Run: `python3 scripts/test_awareness_coverage_report.py`

## Related

- `docs/awareness/candidates/README.md` — candidate workflow (Phase 3)
- `docs/awareness/preflight_audit.md` — what Preflight checks, including
  the honest-DEGRADED gate this report exists to help operators close
- `docs/awareness/coverage_priority.md` — Phase 1's manual top-20
  list; Phase 4's output should largely confirm Phase 1's ranking
- `scripts/promote-awareness-candidate.py` — the promotion path
