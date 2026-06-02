#!/usr/bin/env python3
"""
Awareness Coverage Report — Phase 4.

Scans the services repo's source tree + awareness YAML corpus and
produces a deterministic Markdown report answering:

  - which source files have direct awareness anchors today?
  - which have none?
  - which directories are best/worst covered?
  - which candidate facts are pending review (NOT counted as canonical)?
  - which files should agents prioritize when filing candidates?

Three anchor sources are aggregated:

  1. Canonical YAML in docs/awareness/*.yaml — invariants/failure_modes
     reference source files via `protects.files`; intents via
     `expressed_by` (top-level or nested).
  2. Intent YAML in docs/intent/*.yaml — `expressed_by` lists.
  3. Generated scanner outputs in
     docs/awareness/generated/*_code_symbols.yaml — files that carry
     @awareness annotations in source (each entry's `file` field).

Candidates in docs/awareness/candidates/ are scanned SEPARATELY and
listed in a dedicated section. They never count toward canonical
coverage — the awareness-graph build pipeline skips that directory by
design (Phase 3).

Output is deterministic: every list sorted by stable key; no
timestamps in the body; per-run "Generated" header line can be
stripped for diff comparison by skipping the first 3 lines.

Usage
-----
  python3 scripts/awareness-coverage-report.py \\
    --repo-root . \\
    --output docs/awareness/reports/coverage_report.md

  # Print to stdout instead of a file:
  python3 scripts/awareness-coverage-report.py --repo-root .

Exit codes
----------
  0 — report generated
  2 — usage error (bad paths, missing files)
"""

from __future__ import annotations

import argparse
import os
import sys
from collections import defaultdict
from pathlib import Path
from typing import Iterable

try:
    import yaml
except ImportError:
    sys.stderr.write(
        "awareness-coverage-report: PyYAML is required. Install via 'pip install pyyaml'.\n"
    )
    sys.exit(2)


# Directories listed by CLAUDE.md as high-risk (R2). Files under these
# paths warrant priority annotation when uncovered.
HIGH_RISK_DIRS = (
    "golang/node_agent/",
    "golang/cluster_controller/",
    "golang/repository/",
    "golang/rbac/",
    "golang/security/",
    "golang/cluster_doctor/",
    "golang/mcp/",
    "golang/services_manager/",
    "golang/ai_executor/",
)

# Files under these patterns are excluded from the source universe.
EXCLUDED_SUFFIXES = (
    "_test.go",
    ".pb.go",
    "_grpc.pb.go",
    "zz_version_generated.go",
)


def is_excluded(path: str) -> bool:
    return any(path.endswith(s) for s in EXCLUDED_SUFFIXES)


def is_high_risk(path: str) -> bool:
    return any(path.startswith(prefix) for prefix in HIGH_RISK_DIRS)


def list_source_files(repo_root: Path) -> list[str]:
    """Enumerate the Go source universe (non-test, non-generated)."""
    out: list[str] = []
    golang = repo_root / "golang"
    if not golang.is_dir():
        return out
    for path in golang.rglob("*.go"):
        rel = path.relative_to(repo_root).as_posix()
        if is_excluded(rel):
            continue
        out.append(rel)
    out.sort()
    return out


def safe_load(path: Path):
    try:
        with path.open("r", encoding="utf-8") as f:
            return yaml.safe_load(f)
    except Exception:
        return None


def collect_canonical_anchors(repo_root: Path) -> tuple[dict[str, list[dict]], dict[str, int]]:
    """
    Walk canonical YAML (docs/awareness/*.yaml — top level only — and
    docs/intent/*.yaml) and return:
      - file_anchors:   {source_file_path: [list of {class, id, source_yaml}]}
      - class_counts:   {class: total entries of that class}
    Skips docs/awareness/candidates/ entirely.
    """
    file_anchors: dict[str, list[dict]] = defaultdict(list)
    class_counts: dict[str, int] = defaultdict(int)

    def add_anchor(file_path: str, cls: str, ent_id: str, src_yaml: str) -> None:
        # Strip leading slashes for stable key shape (sometimes YAML has
        # "/golang/..." or relative — we want POSIX-rel form).
        f = file_path.lstrip("/")
        file_anchors[f].append({"class": cls, "id": ent_id, "source_yaml": src_yaml})

    # 1) docs/awareness/*.yaml (top level — NOT candidates/ which is skipped)
    awareness_dir = repo_root / "docs" / "awareness"
    if awareness_dir.is_dir():
        for yaml_path in sorted(awareness_dir.glob("*.yaml")):
            data = safe_load(yaml_path)
            if not isinstance(data, dict):
                continue
            src_yaml = yaml_path.relative_to(repo_root).as_posix()
            for list_key, cls in (
                ("invariants", "invariant"),
                ("failure_modes", "failure_mode"),
                ("intents", "intent"),
                ("incident_patterns", "incident_pattern"),
            ):
                entries = data.get(list_key) or []
                if not isinstance(entries, list):
                    continue
                for entry in entries:
                    if not isinstance(entry, dict):
                        continue
                    ent_id = entry.get("id", "<unnamed>")
                    class_counts[cls] += 1
                    # invariants/failure_modes name files in protects.files
                    protects = entry.get("protects")
                    if isinstance(protects, dict):
                        for f in protects.get("files") or []:
                            if isinstance(f, str):
                                add_anchor(f, cls, ent_id, src_yaml)
                    # intents (and sometimes others) name files in expressed_by
                    for f in entry.get("expressed_by") or []:
                        if isinstance(f, str):
                            add_anchor(f, cls, ent_id, src_yaml)
                    # also handle top-level files: list
                    for f in entry.get("files") or []:
                        if isinstance(f, str):
                            add_anchor(f, cls, ent_id, src_yaml)

    # 2) docs/intent/*.yaml (per-intent files; single entry per file)
    intent_dir = repo_root / "docs" / "intent"
    if intent_dir.is_dir():
        for yaml_path in sorted(intent_dir.rglob("*.yaml")):
            data = safe_load(yaml_path)
            if not isinstance(data, dict):
                continue
            if "id" not in data:
                continue  # not a single-entry intent file
            src_yaml = yaml_path.relative_to(repo_root).as_posix()
            ent_id = data.get("id", "<unnamed>")
            class_counts["intent"] += 1
            for f in data.get("expressed_by") or []:
                if isinstance(f, str):
                    add_anchor(f, "intent", ent_id, src_yaml)

    # 3) generated/*_code_symbols.yaml (source-side @awareness annotations).
    #
    # File paths in these scanner outputs are relative to the SCANNED
    # repo. platform_* and echo_service_* scans walked the services
    # repo, so their `file:` fields use the services-relative shape
    # (golang/<svc>/...) that matches our source universe. The
    # awareness_graph_* scan walked the awareness-graph repo, so its
    # `file:` fields are awareness-graph-relative (cmd/loadnt/main.go,
    # golang/server/...) and would falsely appear as unmatched paths.
    # We skip the awareness_graph_* prefix to keep counts honest for
    # the services repo.
    generated_dir = repo_root / "docs" / "awareness" / "generated"
    if generated_dir.is_dir():
        for yaml_path in sorted(generated_dir.glob("*_code_symbols.yaml")):
            if yaml_path.name.startswith("awareness_graph_"):
                continue
            data = safe_load(yaml_path)
            if not isinstance(data, dict):
                continue
            src_yaml = yaml_path.relative_to(repo_root).as_posix()
            for entry in data.get("code_symbols") or []:
                if not isinstance(entry, dict):
                    continue
                f = entry.get("file")
                if not isinstance(f, str):
                    continue
                class_counts["code_symbol"] += 1
                add_anchor(f, "code_symbol", entry.get("id", "<unnamed>"), src_yaml)

    return file_anchors, dict(class_counts)


def collect_candidates(repo_root: Path) -> list[dict]:
    """Walk docs/awareness/candidates/ and return every candidate entry
    with provenance fields. These are NEVER counted as canonical."""
    out: list[dict] = []
    cand_dir = repo_root / "docs" / "awareness" / "candidates"
    if not cand_dir.is_dir():
        return out
    for yaml_path in sorted(cand_dir.rglob("*.yaml")):
        data = safe_load(yaml_path)
        if not isinstance(data, dict):
            continue
        rel = yaml_path.relative_to(repo_root).as_posix()
        for entry in data.get("candidates") or []:
            if not isinstance(entry, dict):
                continue
            out.append(
                {
                    "id": entry.get("id", "<unnamed>"),
                    "class": entry.get("class", "?"),
                    "confidence": entry.get("confidence", "?"),
                    "risk": entry.get("risk", "?"),
                    "status": entry.get("status", "?"),
                    "discovered_from": (entry.get("discovered_from") or "").strip(),
                    "source_yaml": rel,
                }
            )
    out.sort(key=lambda c: c["id"])
    return out


def dir_of(path: str) -> str:
    """Return the directory portion (POSIX-rel) of a file path."""
    i = path.rfind("/")
    return path[:i] if i >= 0 else "."


def rollup_by_directory(
    source_files: list[str], file_anchors: dict[str, list[dict]]
) -> list[tuple[str, int, int]]:
    """Return [(directory, anchored_count, total_count)] sorted by directory."""
    counts: dict[str, list[int]] = defaultdict(lambda: [0, 0])  # [anchored, total]
    for f in source_files:
        d = dir_of(f)
        counts[d][1] += 1
        if file_anchors.get(f):
            counts[d][0] += 1
    return sorted((d, a, t) for d, (a, t) in counts.items())


def render_report(
    repo_root: Path,
    source_files: list[str],
    file_anchors: dict[str, list[dict]],
    class_counts: dict[str, int],
    candidates: list[dict],
) -> str:
    """Render the Markdown report. Deterministic — no timestamps in
    body, lists sorted by stable keys."""
    anchored = sorted(f for f in source_files if file_anchors.get(f))
    unanchored = sorted(f for f in source_files if not file_anchors.get(f))

    # Per-directory rollup; pick top-N worst-covered (largest unanchored
    # count among high-risk dirs) and best-covered.
    dir_rollup = rollup_by_directory(source_files, file_anchors)
    # high-risk dirs sorted by absolute unanchored count desc
    high_risk_uncovered = sorted(
        [
            (d, anch, tot, tot - anch)
            for d, anch, tot in dir_rollup
            if any(d.startswith(p.rstrip("/")) for p in HIGH_RISK_DIRS) and tot > anch
        ],
        key=lambda x: (-x[3], x[0]),  # most-uncovered first, then alpha
    )
    # all dirs sorted by coverage ratio desc (best-covered first)
    best_covered = sorted(
        [(d, anch, tot) for d, anch, tot in dir_rollup if tot > 0],
        key=lambda x: (-(x[1] / x[2]), -x[1], x[0]),
    )

    # Recommended next targets: top unanchored files in high-risk dirs,
    # sorted by directory then filename for stability.
    recommended = [f for f in unanchored if is_high_risk(f)]
    recommended.sort()

    lines: list[str] = []
    lines.append("# Awareness Coverage Report")
    lines.append("")
    lines.append(
        "Deterministic per-run output. To diff cleanly between runs, skip "
        "the first 3 lines (the operator-facing header)."
    )
    lines.append("")

    # ── Summary ───────────────────────────────────────────────────────
    total = len(source_files)
    a_count = len(anchored)
    u_count = len(unanchored)
    pct = (a_count * 100 // total) if total else 0
    lines.append("## Summary")
    lines.append("")
    lines.append(f"- **Source files scanned (Go, non-test, non-generated):** {total}")
    lines.append(f"- **Files with at least one direct anchor:** {a_count} ({pct}%)")
    lines.append(f"- **Files with zero direct anchors:** {u_count} ({100 - pct}%)")
    lines.append(f"- **Candidate entries (NOT counted in canonical coverage):** {len(candidates)}")
    lines.append("")

    # ── Anchors by class ──────────────────────────────────────────────
    lines.append("## Canonical anchors by class")
    lines.append("")
    lines.append("| Class | Entries in canonical YAML |")
    lines.append("|---|---|")
    for cls in ("invariant", "failure_mode", "intent", "incident_pattern", "code_symbol"):
        lines.append(f"| {cls} | {class_counts.get(cls, 0)} |")
    lines.append("")
    lines.append(
        "_`code_symbol` entries come from `docs/awareness/generated/*_code_symbols.yaml` "
        "(the source-side `@awareness` annotation scan)._"
    )
    lines.append("")

    # ── High-risk uncovered ──────────────────────────────────────────
    lines.append("## High-risk directories with unanchored files")
    lines.append("")
    lines.append(
        "Files under these paths are listed in CLAUDE.md R2 as high-risk. "
        "Uncovered files here trigger Phase 5's honest-DEGRADED gate in Preflight."
    )
    lines.append("")
    if high_risk_uncovered:
        lines.append("| Directory | Anchored / Total | Uncovered |")
        lines.append("|---|---|---|")
        for d, anch, tot, unc in high_risk_uncovered[:20]:
            lines.append(f"| `{d}` | {anch}/{tot} | {unc} |")
    else:
        lines.append("_(none — every high-risk file has at least one anchor)_")
    lines.append("")

    # ── Best-covered directories ──────────────────────────────────────
    lines.append("## Best-covered directories (top 20 by coverage ratio)")
    lines.append("")
    lines.append("| Directory | Anchored / Total | % |")
    lines.append("|---|---|---|")
    for d, anch, tot in best_covered[:20]:
        pct = anch * 100 // tot if tot else 0
        lines.append(f"| `{d}` | {anch}/{tot} | {pct}% |")
    lines.append("")

    # ── Recommended next annotation targets ──────────────────────────
    lines.append("## Recommended next annotation targets (high-risk + unanchored)")
    lines.append("")
    lines.append(
        "These are uncovered Go files under CLAUDE.md R2 high-risk dirs. "
        "Sorted by path. Use this list as the source for the next round of "
        "candidate filings — Preflight will return DEGRADED on each of them today."
    )
    lines.append("")
    if recommended:
        for f in recommended[:50]:
            lines.append(f"- `{f}`")
        if len(recommended) > 50:
            lines.append(f"- _… and {len(recommended) - 50} more (truncated to top 50 for readability)_")
    else:
        lines.append("_(none — every high-risk file is anchored)_")
    lines.append("")

    # ── Candidates ────────────────────────────────────────────────────
    lines.append("## Candidates pending review")
    lines.append("")
    lines.append(
        "These entries live in `docs/awareness/candidates/`. They are NOT "
        "active in the awareness graph — the build pipeline explicitly skips "
        "that directory (Phase 3). Promote with "
        "`scripts/promote-awareness-candidate.py --id <id> --target <target.yaml>`."
    )
    lines.append("")
    if candidates:
        lines.append("| Candidate ID | Class | Risk | Confidence | Discovered from |")
        lines.append("|---|---|---|---|---|")
        for c in candidates:
            df = c["discovered_from"]
            # Truncate noisy multi-line provenance for the table
            df_short = df.splitlines()[0][:80] if df else ""
            lines.append(
                f"| `{c['id']}` | {c['class']} | {c['risk']} | {c['confidence']} | {df_short} |"
            )
    else:
        lines.append("_(no candidates pending review)_")
    lines.append("")
    lines.append(
        "_All entries above are marked **not active in awareness graph** until "
        "explicitly promoted._"
    )
    lines.append("")

    return "\n".join(lines) + "\n"


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument(
        "--repo-root",
        default=".",
        help="Path to the services repo root (default: current directory)",
    )
    parser.add_argument(
        "--output",
        default=None,
        help="Output path for the Markdown report (default: stdout)",
    )
    args = parser.parse_args(argv)

    repo_root = Path(args.repo_root).resolve()
    if not (repo_root / "docs" / "awareness").is_dir():
        sys.stderr.write(
            f"awareness-coverage-report: {repo_root}/docs/awareness/ not found; "
            "is --repo-root pointing at the services repo?\n"
        )
        return 2

    source_files = list_source_files(repo_root)
    file_anchors, class_counts = collect_canonical_anchors(repo_root)
    candidates = collect_candidates(repo_root)

    report = render_report(repo_root, source_files, file_anchors, class_counts, candidates)

    if args.output:
        out_path = Path(args.output)
        if not out_path.is_absolute():
            out_path = repo_root / out_path
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(report, encoding="utf-8")
        # Print as repo-relative when possible; else absolute.
        try:
            shown = str(out_path.relative_to(repo_root))
        except ValueError:
            shown = str(out_path)
        sys.stderr.write(f"wrote {shown}\n")
    else:
        sys.stdout.write(report)
    return 0


if __name__ == "__main__":
    sys.exit(main())
