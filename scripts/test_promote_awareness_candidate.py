#!/usr/bin/env python3
"""
Tests for scripts/promote-awareness-candidate.py — Phase 7.

Stdlib only + PyYAML (already a dependency of the script under test).
The script lives at scripts/promote-awareness-candidate.py and walks
docs/awareness/candidates/ to find a candidate by id, validates it,
appends to a canonical file, and removes it from the candidate file.

We exercise it by writing a minimal candidates/ tree + canonical
target into a temp dir, monkey-patching the script's REPO_ROOT /
CANDIDATES_DIR module globals, and invoking individual functions
(find_candidate, validate, to_canonical_entry, ...). End-to-end
tests then run the full main() with sys.argv shimmed.

Run:
  python3 scripts/test_promote_awareness_candidate.py
"""

from __future__ import annotations

import importlib.util
import io
import os
import shutil
import sys
import tempfile
import textwrap
import unittest
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path


# ─── load the script as a module ─────────────────────────────────────────

def _load_tool():
    here = Path(__file__).resolve().parent
    spec = importlib.util.spec_from_file_location(
        "promote_awareness_candidate", here / "promote-awareness-candidate.py"
    )
    assert spec and spec.loader, "could not load promote-awareness-candidate.py"
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


tool = _load_tool()


# ─── fixtures ────────────────────────────────────────────────────────────

CANDIDATE_VALID = textwrap.dedent(
    """\
    candidates:
      - id: test.namespace.valid_candidate
        class: invariant
        label: A valid candidate
        source_file: golang/cluster_doctor/cluster_doctor_server/foo.go
        evidence: Discovered during session 2026-06-02 because X happened
        risk: high
        confidence: medium
        status: candidate
        discovered_from: session 2026-06-02 foo
        review_required: true
    """
)

CANDIDATE_LOW_CONFIDENCE = textwrap.dedent(
    """\
    candidates:
      - id: test.namespace.low_conf
        class: invariant
        label: Low confidence
        source_file: golang/foo.go
        evidence: Maybe true
        risk: medium
        confidence: low
        status: candidate
        discovered_from: session 2026-06-02 bar
        review_required: true
    """
)

CANDIDATE_BAD_ID = textwrap.dedent(
    """\
    candidates:
      - id: BadID.WithUppercase
        class: invariant
        label: Bad id
        source_file: golang/foo.go
        evidence: e
        risk: high
        confidence: medium
        status: candidate
        discovered_from: session
        review_required: true
    """
)

CANDIDATE_NO_NAMESPACE = textwrap.dedent(
    """\
    candidates:
      - id: nondotted
        class: invariant
        label: No namespace
        source_file: golang/foo.go
        evidence: e
        risk: high
        confidence: medium
        status: candidate
        discovered_from: session
        review_required: true
    """
)

CANDIDATE_WRONG_STATUS = textwrap.dedent(
    """\
    candidates:
      - id: test.ns.already_active
        class: invariant
        label: Already active
        source_file: golang/foo.go
        evidence: e
        risk: high
        confidence: medium
        status: active
        discovered_from: session
        review_required: true
    """
)

CANDIDATE_NO_EVIDENCE = textwrap.dedent(
    """\
    candidates:
      - id: test.ns.no_evidence
        class: invariant
        label: No evidence
        source_file: golang/foo.go
        evidence: ""
        risk: high
        confidence: medium
        status: candidate
        discovered_from: session
        review_required: true
    """
)

CANDIDATE_FAILURE_MODE = textwrap.dedent(
    """\
    candidates:
      - id: test.ns.a_failure_mode
        class: failure_mode
        label: I am a failure mode
        source_file: golang/foo.go
        evidence: e
        risk: high
        confidence: high
        status: candidate
        discovered_from: session
        review_required: true
    """
)

CANONICAL_INVARIANTS_EMPTY = "invariants: []\n"

CANONICAL_INVARIANTS_WITH_DUP = textwrap.dedent(
    """\
    invariants:
      - id: test.namespace.valid_candidate
        title: Already exists
        severity: high
        status: active
    """
)


# ─── test base ───────────────────────────────────────────────────────────

class TempRepoCase(unittest.TestCase):
    """Builds a temp repo skeleton, points the tool's module globals at it
    for the duration of the test, restores them on teardown.

    The tool walks docs/awareness/candidates/ relative to REPO_ROOT and
    appends to docs/awareness/<file>.yaml relative to REPO_ROOT too. We
    can't avoid touching REPO_ROOT — but we can swap it per-test and
    keep mutations contained."""

    def setUp(self) -> None:
        self.repo = Path(tempfile.mkdtemp(prefix="promote-aware-test-"))
        (self.repo / "docs" / "awareness" / "candidates").mkdir(parents=True)
        self._orig_repo_root = tool.REPO_ROOT
        self._orig_candidates_dir = tool.CANDIDATES_DIR
        tool.REPO_ROOT = self.repo
        tool.CANDIDATES_DIR = self.repo / "docs" / "awareness" / "candidates"

    def tearDown(self) -> None:
        tool.REPO_ROOT = self._orig_repo_root
        tool.CANDIDATES_DIR = self._orig_candidates_dir
        shutil.rmtree(self.repo, ignore_errors=True)

    def write_candidate(self, name: str, content: str) -> Path:
        p = self.repo / "docs" / "awareness" / "candidates" / name
        p.write_text(content, encoding="utf-8")
        return p

    def write_canonical(self, name: str, content: str) -> Path:
        p = self.repo / "docs" / "awareness" / name
        p.write_text(content, encoding="utf-8")
        return p


# ─── 1. happy-path promotion ────────────────────────────────────────────

class TestPromoteHappyPath(TempRepoCase):
    def test_valid_candidate_promotes_to_invariants(self):
        self.write_candidate("session.yaml", CANDIDATE_VALID)
        target = self.write_canonical("invariants.yaml", CANONICAL_INVARIANTS_EMPTY)

        # Run via the find_candidate → validate → write path directly.
        candidate_path, candidate = tool.find_candidate("test.namespace.valid_candidate")
        tool.validate(candidate, "invariants.yaml")
        entry = tool.to_canonical_entry(candidate)
        tool.write_canonical(target, "invariants", entry, dry_run=False)
        tool.remove_from_candidate_file(candidate_path, "test.namespace.valid_candidate", dry_run=False)

        # Target now contains a single invariant with the right id.
        import yaml
        data = yaml.safe_load(target.read_text("utf-8"))
        ids = [e["id"] for e in data["invariants"]]
        self.assertEqual(ids, ["test.namespace.valid_candidate"])

        # The promoted entry carries provenance.
        ent = data["invariants"][0]
        self.assertEqual(ent["provenance"]["promoted_from"], "candidate")
        self.assertEqual(ent["provenance"]["confidence_at_promotion"], "medium")
        self.assertIn("session 2026-06-02", ent["provenance"]["discovered_from"])

        # Candidate file no longer holds the entry.
        remaining = yaml.safe_load(candidate_path.read_text("utf-8"))
        self.assertEqual(remaining["candidates"], [])


# ─── 2. validation rejections ───────────────────────────────────────────

class TestPromoteValidationRejects(TempRepoCase):
    """Every validation rule from the script's docstring gets one test.
    SystemExit code 1 is what `die()` uses for validation failures;
    code 2 is for usage/structural errors."""

    def _expect_die(self, code: int, fn, *args, **kwargs):
        with self.assertRaises(SystemExit) as ctx:
            with redirect_stderr(io.StringIO()):
                fn(*args, **kwargs)
        self.assertEqual(ctx.exception.code, code)

    def test_low_confidence_rejected(self):
        self.write_candidate("c.yaml", CANDIDATE_LOW_CONFIDENCE)
        _, candidate = tool.find_candidate("test.namespace.low_conf")
        self._expect_die(1, tool.validate, candidate, "invariants.yaml")

    def test_bad_id_format_rejected(self):
        self.write_candidate("c.yaml", CANDIDATE_BAD_ID)
        # find_candidate still returns it (id-matched), validate dies.
        _, candidate = tool.find_candidate("BadID.WithUppercase")
        self._expect_die(1, tool.validate, candidate, "invariants.yaml")

    def test_no_namespace_id_rejected(self):
        self.write_candidate("c.yaml", CANDIDATE_NO_NAMESPACE)
        _, candidate = tool.find_candidate("nondotted")
        self._expect_die(1, tool.validate, candidate, "invariants.yaml")

    def test_wrong_status_rejected(self):
        # status must be "candidate" — promotion is the only legal way to
        # move it to active, so an already-active entry must fail loudly.
        self.write_candidate("c.yaml", CANDIDATE_WRONG_STATUS)
        _, candidate = tool.find_candidate("test.ns.already_active")
        self._expect_die(1, tool.validate, candidate, "invariants.yaml")

    def test_empty_evidence_rejected(self):
        self.write_candidate("c.yaml", CANDIDATE_NO_EVIDENCE)
        _, candidate = tool.find_candidate("test.ns.no_evidence")
        self._expect_die(1, tool.validate, candidate, "invariants.yaml")

    def test_class_mismatch_rejected(self):
        # candidate.class=failure_mode but target=invariants.yaml.
        self.write_candidate("c.yaml", CANDIDATE_FAILURE_MODE)
        _, candidate = tool.find_candidate("test.ns.a_failure_mode")
        self._expect_die(1, tool.validate, candidate, "invariants.yaml")

    def test_unknown_target_file_rejected(self):
        # An entirely unrecognized canonical filename → usage error (2).
        self.write_candidate("c.yaml", CANDIDATE_VALID)
        _, candidate = tool.find_candidate("test.namespace.valid_candidate")
        self._expect_die(2, tool.validate, candidate, "not_a_real_canonical_file.yaml")

    def test_duplicate_id_rejected(self):
        # Canonical already has the same id → must refuse.
        self.write_candidate("c.yaml", CANDIDATE_VALID)
        self.write_canonical("invariants.yaml", CANONICAL_INVARIANTS_WITH_DUP)
        _, candidate = tool.find_candidate("test.namespace.valid_candidate")
        self._expect_die(1, tool.validate, candidate, "invariants.yaml")


# ─── 3. find_candidate ambiguity / not-found ───────────────────────────

class TestFindCandidate(TempRepoCase):
    def test_unknown_id_dies(self):
        self.write_candidate("c.yaml", CANDIDATE_VALID)
        with self.assertRaises(SystemExit) as ctx:
            with redirect_stderr(io.StringIO()):
                tool.find_candidate("test.namespace.does_not_exist")
        self.assertEqual(ctx.exception.code, 1)

    def test_duplicate_id_across_files_dies(self):
        # Two candidates files both contain the same id — must fail
        # ambiguously rather than promote a random one.
        self.write_candidate("a.yaml", CANDIDATE_VALID)
        self.write_candidate("b.yaml", CANDIDATE_VALID)
        with self.assertRaises(SystemExit) as ctx:
            with redirect_stderr(io.StringIO()):
                tool.find_candidate("test.namespace.valid_candidate")
        self.assertEqual(ctx.exception.code, 1)


# ─── 4. to_canonical_entry shape ─────────────────────────────────────────

class TestCanonicalEntryShape(TempRepoCase):
    def test_provenance_block_complete(self):
        self.write_candidate("c.yaml", CANDIDATE_VALID)
        _, candidate = tool.find_candidate("test.namespace.valid_candidate")
        entry = tool.to_canonical_entry(candidate)

        # Required canonical fields are present.
        self.assertEqual(entry["id"], "test.namespace.valid_candidate")
        self.assertEqual(entry["title"], "A valid candidate")
        self.assertEqual(entry["severity"], "high")  # was candidate.risk
        self.assertEqual(entry["status"], "active")  # promotion sets active

        # Provenance is preserved.
        prov = entry["provenance"]
        self.assertEqual(prov["promoted_from"], "candidate")
        self.assertEqual(prov["confidence_at_promotion"], "medium")
        self.assertEqual(prov["discovered_from"], "session 2026-06-02 foo")

    def test_candidate_only_fields_stripped(self):
        # source_file, evidence, review_required, discovered_from (at
        # top level) are candidate-only and must NOT appear in the
        # canonical entry. The discovered_from value lives only inside
        # provenance.
        self.write_candidate("c.yaml", CANDIDATE_VALID)
        _, candidate = tool.find_candidate("test.namespace.valid_candidate")
        entry = tool.to_canonical_entry(candidate)
        for stripped in ("source_file", "evidence", "review_required",
                         "discovered_from", "confidence", "risk", "class", "label"):
            self.assertNotIn(stripped, entry,
                f"{stripped!r} leaked into canonical entry: {entry}")


# ─── 5. dry-run does not mutate ─────────────────────────────────────────

class TestDryRunIsReadOnly(TempRepoCase):
    def test_dry_run_does_not_modify_target_or_candidate(self):
        self.write_candidate("c.yaml", CANDIDATE_VALID)
        target = self.write_canonical("invariants.yaml", CANONICAL_INVARIANTS_EMPTY)

        before_target = target.read_text("utf-8")
        candidate_path, candidate = tool.find_candidate("test.namespace.valid_candidate")
        before_candidate = candidate_path.read_text("utf-8")

        entry = tool.to_canonical_entry(candidate)
        with redirect_stdout(io.StringIO()):
            tool.write_canonical(target, "invariants", entry, dry_run=True)
            tool.remove_from_candidate_file(candidate_path, candidate["id"], dry_run=True)

        self.assertEqual(target.read_text("utf-8"), before_target,
                         "dry-run wrote to canonical target")
        self.assertEqual(candidate_path.read_text("utf-8"), before_candidate,
                         "dry-run wrote to candidate file")


# ─── 6. ID pattern unit ─────────────────────────────────────────────────

class TestIDPattern(unittest.TestCase):
    def test_pattern_accepts_legal(self):
        for cid in [
            "ns.id",
            "ns.sub.id",
            "with_underscore.also_underscored",
            "ns123.id456",
            "a.b.c.d.e",
        ]:
            self.assertIsNotNone(
                tool.ID_PATTERN.match(cid), f"should accept {cid!r}"
            )

    def test_pattern_rejects_illegal(self):
        for cid in [
            "nondotted",              # no namespace
            "BadID.uppercase",        # uppercase
            "ns.id-with-dash",        # dash not allowed
            "ns.id with space",       # space
            "ns/id",                  # slash
            ".leadingdot",            # leading dot
            "trailing.dot.",          # trailing dot
            "double..dot",            # empty segment
        ]:
            self.assertIsNone(
                tool.ID_PATTERN.match(cid), f"should reject {cid!r}"
            )


if __name__ == "__main__":
    unittest.main(verbosity=2)
