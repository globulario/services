# Graph Go-File Coverage

## Why this matters

The awareness graph is only as useful as the files it covers. If the graph indexes 60% of eligible Go files, then 40% of `NO_MATCH` results are structural false negatives — the graph didn't look, not that nothing applies. Low coverage must lower confidence.

## What is measured

Every `awareness.preflight` and `awareness.coverage_report` run reports:

| Field | Meaning |
|---|---|
| `eligible_go_files_total` | All `.go` files in the repo excluding vendor, .git, generated `.pb.go` |
| `indexed_go_files_total` | `source_file` nodes currently in the graph |
| `coverage_percent_go_files` | `indexed / eligible * 100` |
| `missing_files` | Eligible files not represented in the graph |
| `confidence_impact` | `none` / `medium` / `high` |

## Thresholds

| Coverage | Impact | Action |
|---|---|---|
| ≥ 85% | `none` | Proceed normally |
| 70–84% | `medium` | Blind spot added; rebuild recommended |
| < 70% | `high` | Confidence demoted; `UNKNOWN_IMPACT_LOW_COVERAGE` warning emitted |

When `confidence_impact = high`, preflight demotes confidence from `high` → `medium` and includes a `UNKNOWN_IMPACT_LOW_COVERAGE` warning. Agents must not treat `NO_MATCH` as safe when coverage is critically low.

## How to improve coverage

Rebuild the graph to pick up new files:

```bash
globular awareness build --clean
```

The graph builder walks the repo and creates `source_file` nodes for every Go file it processes. Coverage drops when:
- New packages are added but the graph hasn't been rebuilt
- A stale graph was built from a different commit

## Reading coverage in preflight output

```json
{
  "go_file_coverage": {
    "eligible_go_files_total": 412,
    "indexed_go_files_total": 389,
    "coverage_percent_go_files": 94.4,
    "confidence_impact": "none"
  }
}
```

If `confidence_impact` is `medium` or `high`, the `blind_spots` array will include a message listing the count and coverage percent.

## Tests

- `TestPreflight_NoMatchIncludesGraphCoverage` — verifies GoFileCoverage is populated
- `TestPreflight_ChangedFilesSinceGraphBuildLowersConfidence` — low coverage produces blind spots and demotes confidence
- `TestGraphCoverageReport_CountsEligibleAndIndexedGoFiles` — unit test for `enforce.GoFileCoverage`
- `TestGraphCoverageReport_ReportsMissingFiles` — missing file list is accurate
