#!/usr/bin/env python3
"""
Tests for scripts/awareness-coverage-report.py — Phase 4.

Uses only the stdlib (unittest + tempfile) plus PyYAML which is already
required by the tool itself. No pytest dependency.

Run:
  python3 -m unittest scripts.test_awareness_coverage_report -v

Or directly:
  python3 scripts/test_awareness_coverage_report.py
"""

from __future__ import annotations

import importlib.util
import io
import os
import shutil
import tempfile
import textwrap
import unittest
from pathlib import Path


# Import the tool by file path (it uses a dash in the filename so a
# plain `import` doesn't work). Python's importlib makes this trivial.
def _load_tool():
    here = Path(__file__).resolve().parent
    spec = importlib.util.spec_from_file_location(
        "awareness_coverage_report", here / "awareness-coverage-report.py"
    )
    assert spec and spec.loader, "could not load awareness-coverage-report.py"
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


tool = _load_tool()


def make_repo(files: dict[str, str]) -> Path:
    """Create a temp directory tree populated with the given paths
    (POSIX-rel → content). Returns the temp root. Caller is
    responsible for cleanup; ScratchRepoCase below does this via
    tearDown."""
    root = Path(tempfile.mkdtemp(prefix="aware-cov-test-"))
    for rel, content in files.items():
        full = root / rel
        full.parent.mkdir(parents=True, exist_ok=True)
        full.write_text(content, encoding="utf-8")
    return root


class ScratchRepoCase(unittest.TestCase):
    """Base case that tracks all temp repos created during a test and
    deletes them on teardown."""

    def setUp(self) -> None:
        self._dirs: list[Path] = []

    def tearDown(self) -> None:
        for d in self._dirs:
            shutil.rmtree(d, ignore_errors=True)

    def make(self, files: dict[str, str]) -> Path:
        d = make_repo(files)
        self._dirs.append(d)
        return d


# ────────────────────────────────────────────────────────────────────────
# Phase 4 Coverage Tests
# ────────────────────────────────────────────────────────────────────────

INVARIANT_YAML = textwrap.dedent(
    """\
    invariants:
      - id: test.canonical_one
        title: A canonical invariant
        severity: high
        status: active
        protects:
          files:
            - golang/cluster_doctor/cluster_doctor_server/foo.go
            - golang/repository/repository_server/bar.go
    """
)

INTENT_YAML = textwrap.dedent(
    """\
    intents:
      - id: test.intent_one
        title: A canonical intent
        status: active
        expressed_by:
          - golang/mcp/server.go
    """
)

CANDIDATE_YAML = textwrap.dedent(
    """\
    candidates:
      - id: test.candidate_one
        class: invariant
        label: A pending candidate
        source_file: golang/cluster_doctor/cluster_doctor_server/baz.go
        evidence: discovered during session foo
        risk: high
        confidence: medium
        status: candidate
        discovered_from: session 2026-06-02 foo
        review_required: true
    """
)


class TestCandidateExclusion(ScratchRepoCase):
    """Verify candidate entries are never counted as canonical coverage."""

    def test_candidate_not_counted_in_canonical_anchors(self):
        root = self.make(
            {
                "docs/awareness/invariants.yaml": INVARIANT_YAML,
                "docs/awareness/candidates/session_discovered.yaml": CANDIDATE_YAML,
                "golang/cluster_doctor/cluster_doctor_server/foo.go": "package main\n",
                "golang/cluster_doctor/cluster_doctor_server/baz.go": "package main\n",
            }
        )
        anchors, _ = tool.collect_canonical_anchors(root)
        # foo.go IS named in invariants.yaml → must be anchored
        self.assertIn("golang/cluster_doctor/cluster_doctor_server/foo.go", anchors)
        # baz.go is ONLY named in a candidate → must NOT be anchored
        self.assertNotIn("golang/cluster_doctor/cluster_doctor_server/baz.go", anchors)

    def test_candidates_collected_into_separate_list(self):
        root = self.make(
            {"docs/awareness/candidates/session_discovered.yaml": CANDIDATE_YAML}
        )
        cands = tool.collect_candidates(root)
        self.assertEqual(len(cands), 1)
        self.assertEqual(cands[0]["id"], "test.candidate_one")
        self.assertEqual(cands[0]["status"], "candidate")
        self.assertEqual(cands[0]["confidence"], "medium")

    def test_nested_candidates_directory_is_also_skipped(self):
        # A candidates/ inside a subsystem dir must still be excluded
        # — the rule is by directory NAME, not just top-level position.
        root = self.make(
            {
                "docs/awareness/some_area/candidates/inner.yaml": CANDIDATE_YAML,
                "golang/cluster_doctor/cluster_doctor_server/baz.go": "package main\n",
            }
        )
        anchors, _ = tool.collect_canonical_anchors(root)
        self.assertNotIn("golang/cluster_doctor/cluster_doctor_server/baz.go", anchors)
        # But the candidate collector picks it up under
        # docs/awareness/candidates/ specifically; nested candidates/
        # under arbitrary subsystem dirs are NOT scanned (only the
        # canonical docs/awareness/candidates/ top-level dir is).
        # This mirrors the awareness-graph extractor's skip rule which
        # excludes any directory named "candidates" from the build —
        # but the operator-facing review queue is the canonical
        # docs/awareness/candidates/ location.
        cands = tool.collect_candidates(root)
        self.assertEqual(len(cands), 0)


class TestCanonicalCounting(ScratchRepoCase):
    """Verify top-level canonical YAML is counted; field shapes work."""

    def test_invariant_protects_files_counted(self):
        root = self.make({"docs/awareness/invariants.yaml": INVARIANT_YAML})
        anchors, classes = tool.collect_canonical_anchors(root)
        self.assertEqual(classes.get("invariant"), 1)
        self.assertIn("golang/cluster_doctor/cluster_doctor_server/foo.go", anchors)
        self.assertIn("golang/repository/repository_server/bar.go", anchors)

    def test_intent_expressed_by_counted(self):
        root = self.make({"docs/awareness/intents.yaml": INTENT_YAML})
        anchors, classes = tool.collect_canonical_anchors(root)
        self.assertEqual(classes.get("intent"), 1)
        self.assertIn("golang/mcp/server.go", anchors)

    def test_single_entry_intent_file_in_docs_intent_dir_counted(self):
        single_intent = textwrap.dedent(
            """\
            id: test.single_intent
            level: principle
            title: One file, one intent
            intent: x
            status: active
            expressed_by:
              - golang/security/server.go
            """
        )
        root = self.make({"docs/intent/foo.yaml": single_intent})
        anchors, classes = tool.collect_canonical_anchors(root)
        self.assertEqual(classes.get("intent"), 1)
        self.assertIn("golang/security/server.go", anchors)


class TestUnanchoredReporting(ScratchRepoCase):
    """Files with no anchors are listed; recommended-targets logic is
    scoped to high-risk dirs only."""

    def test_unanchored_file_appears_in_report(self):
        root = self.make(
            {
                "docs/awareness/invariants.yaml": INVARIANT_YAML,
                "golang/cluster_doctor/cluster_doctor_server/foo.go": "package main\n",  # anchored
                "golang/cluster_doctor/cluster_doctor_server/quux.go": "package main\n",  # NOT anchored
                "golang/echo/echo_server/server.go": "package main\n",  # NOT anchored, clean path
            }
        )
        source = tool.list_source_files(root)
        anchors, classes = tool.collect_canonical_anchors(root)
        report = tool.render_report(root, source, anchors, classes, candidates=[])

        # The unanchored high-risk file IS recommended
        self.assertIn(
            "`golang/cluster_doctor/cluster_doctor_server/quux.go`", report
        )
        # The unanchored clean-path file is NOT in the "recommended" section
        # (the section is scoped to high-risk dirs only). It can still appear
        # in the directory-rollup tables though.
        # Find the "Recommended next annotation targets" subsection and
        # check the echo file is not in its bullets.
        rec_start = report.index("## Recommended next annotation targets")
        # Bound by the next header (## Candidates).
        rec_end = report.index("## Candidates", rec_start)
        rec_section = report[rec_start:rec_end]
        self.assertNotIn("golang/echo/echo_server/server.go", rec_section)


class TestReportDeterminism(ScratchRepoCase):
    """Two consecutive runs over the same fixture produce identical
    output bodies (the only allowed variation would be timestamps; we
    intentionally don't include any)."""

    def test_two_runs_produce_identical_output(self):
        fixture = {
            "docs/awareness/invariants.yaml": INVARIANT_YAML,
            "docs/awareness/intents.yaml": INTENT_YAML,
            "docs/awareness/candidates/session_discovered.yaml": CANDIDATE_YAML,
            "golang/cluster_doctor/cluster_doctor_server/foo.go": "package main\n",
            "golang/cluster_doctor/cluster_doctor_server/baz.go": "package main\n",
            "golang/mcp/server.go": "package main\n",
            "golang/echo/echo_server/server.go": "package main\n",
        }
        # Two independent repos with identical content
        root_a = self.make(fixture)
        root_b = self.make(fixture)

        def run(root: Path) -> str:
            source = tool.list_source_files(root)
            anchors, classes = tool.collect_canonical_anchors(root)
            cands = tool.collect_candidates(root)
            return tool.render_report(root, source, anchors, classes, cands)

        out_a = run(root_a)
        out_b = run(root_b)
        self.assertEqual(out_a, out_b, "report output must be deterministic")

    def test_no_timestamp_in_body(self):
        # The body must not include any year-like 20xx token that would
        # vary across runs. (The operator-facing header is the first
        # few lines; the comment in the script says diff-clean tools
        # should skip them, but we go further and emit no timestamps at
        # all.)
        root = self.make({"docs/awareness/invariants.yaml": INVARIANT_YAML})
        source = tool.list_source_files(root)
        anchors, classes = tool.collect_canonical_anchors(root)
        report = tool.render_report(root, source, anchors, classes, candidates=[])
        # Loose check: no 4-digit year-like tokens that vary by run.
        # (Hardcoded years in comments inside the script body are fine.)
        for line in report.splitlines():
            self.assertFalse(
                any(tok.startswith("202") and len(tok) >= 4 and tok[:4].isdigit() for tok in line.split()),
                f"timestamp leaked into output: {line!r}",
            )


class TestExcludedFilesNotInUniverse(ScratchRepoCase):
    """*_test.go, *.pb.go, zz_version_generated.go are excluded."""

    def test_test_files_excluded(self):
        root = self.make(
            {
                "golang/foo/main.go": "package main\n",
                "golang/foo/main_test.go": "package main\n",
                "golang/foo/foo.pb.go": "package main\n",
                "golang/foo/foo_grpc.pb.go": "package main\n",
                "golang/foo/zz_version_generated.go": "package main\n",
            }
        )
        files = tool.list_source_files(root)
        self.assertEqual(files, ["golang/foo/main.go"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
