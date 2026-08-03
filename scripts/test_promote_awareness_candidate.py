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


# ─── 7. wrapped candidate documents ─────────────────────────────────────

CANDIDATE_WRAPPED = textwrap.dedent(
    """\
    # a leading comment that must survive
    session_discovered_candidates:
      candidates:
        - id: candidate.invariant.meta.wrapped_one
          class: Invariant
          label: A wrapped candidate
          status: candidate
          confidence: candidate
          severity: high
          discovered: 2026-08-03
          description: something
          contract: something must hold
          source_files:
            - golang/foo.go
          forbidden_fixes:
            - do not do the bad thing
          evidence: observed on 2026-08-03
        - id: candidate.failure_mode.meta.wrapped_two
          class: FailureMode
          label: A wrapped failure mode
          status: candidate
          confidence: candidate
          severity: high
          discovered: 2026-08-03
          verified_negative: checked the siblings, they do not share the bug
    """
)

CANDIDATE_MIXED_SHAPE = textwrap.dedent(
    """\
    candidates:
      - id: test.ns.direct_one
        class: invariant
        label: direct
        status: candidate
        confidence: medium
        evidence: e
        discovered_from: session
    wrapper_key:
      candidates:
        - id: test.ns.wrapped_one
          class: invariant
          label: wrapped
          status: candidate
          confidence: medium
          evidence: e
          discovered_from: session
    """
)


class TestWrappedCandidates(TempRepoCase):
    """The session file uses `session_discovered_candidates: {candidates: []}`.
    Reading only the top-level `candidates` key made every entry in it
    invisible — discoverable by nothing, promotable by nothing."""

    def test_wrapped_candidates_are_discoverable(self):
        self.write_candidate("session.yaml", CANDIDATE_WRAPPED)
        rows = tool.iter_all_candidates()
        ids = sorted(e["id"] for _p, _w, e in rows)
        self.assertEqual(
            ids,
            [
                "candidate.failure_mode.meta.wrapped_two",
                "candidate.invariant.meta.wrapped_one",
            ],
        )
        self.assertEqual(rows[0][1], "session_discovered_candidates")

    def test_wrapped_candidate_is_selectable_by_id(self):
        self.write_candidate("session.yaml", CANDIDATE_WRAPPED)
        path, entry = tool.find_candidate("candidate.invariant.meta.wrapped_one")
        self.assertEqual(entry["label"], "A wrapped candidate")
        self.assertEqual(path.name, "session.yaml")

    def test_direct_shape_still_works(self):
        self.write_candidate("flat.yaml", CANDIDATE_VALID)
        _p, entry = tool.find_candidate("test.namespace.valid_candidate")
        self.assertEqual(entry["id"], "test.namespace.valid_candidate")

    def test_mixed_shape_rejected(self):
        # Matches the Go parser: one shape per document.
        self.write_candidate("mixed.yaml", CANDIDATE_MIXED_SHAPE)
        with self.assertRaises(SystemExit) as ctx:
            with redirect_stderr(io.StringIO()):
                tool.iter_all_candidates()
        self.assertEqual(ctx.exception.code, 1)


class TestNormalization(TempRepoCase):
    def test_class_camelcase_accepted(self):
        self.assertEqual(tool.normalize_class("Invariant"), "invariant")
        self.assertEqual(tool.normalize_class("FailureMode"), "failure_mode")
        self.assertEqual(tool.normalize_class("failure_mode"), "failure_mode")
        self.assertEqual(tool.normalize_class("IncidentPattern"), "incident_pattern")

    def test_candidate_prefix_stripped_from_id(self):
        self.assertEqual(
            tool.canonical_id_for({"id": "candidate.invariant.meta.foo"}), "meta.foo"
        )
        self.assertEqual(
            tool.canonical_id_for({"id": "candidate.failure_mode.install.bar"}),
            "install.bar",
        )
        # A plain id is untouched.
        self.assertEqual(tool.canonical_id_for({"id": "meta.foo"}), "meta.foo")

    def test_evidence_from_alternate_fields(self):
        self.assertTrue(tool.evidence_of({"evidence": "x"}))
        self.assertTrue(tool.evidence_of({"verified_negative": "checked siblings"}))
        self.assertTrue(tool.evidence_of({"prevented_instance": "would have forked"}))
        self.assertTrue(
            tool.evidence_of({"failure_sub_shapes": [{"observed_2026_08_03": ["a case"]}]})
        )
        self.assertFalse(tool.evidence_of({"description": "no evidence here"}))

    def test_provenance_from_discovered_date(self):
        self.assertEqual(
            tool.provenance_of({"discovered_from": "session foo"}), "session foo"
        )
        self.assertEqual(tool.provenance_of({"discovered": "2026-08-03"}), "session 2026-08-03")
        self.assertEqual(tool.provenance_of({}), "")

    def test_wrapped_candidate_validates_and_promotes(self):
        self.write_candidate("session.yaml", CANDIDATE_WRAPPED)
        self.write_canonical("invariants.yaml", CANONICAL_INVARIANTS_EMPTY)
        _p, entry = tool.find_candidate("candidate.invariant.meta.wrapped_one")
        tool.validate(entry, "invariants.yaml")  # must not die
        canonical = tool.to_canonical_entry(entry)
        self.assertEqual(canonical["id"], "meta.wrapped_one")
        self.assertEqual(canonical["severity"], "high")  # from severity, not risk
        self.assertEqual(canonical["status"], "active")
        # Substance carried, not dropped.
        self.assertEqual(canonical["contract"], "something must hold")
        self.assertEqual(canonical["forbidden_fixes"], ["do not do the bad thing"])
        self.assertEqual(canonical["source_files"], ["golang/foo.go"])
        self.assertEqual(canonical["provenance"]["discovered_from"], "session 2026-08-03")
        self.assertEqual(
            canonical["provenance"]["candidate_id"], "candidate.invariant.meta.wrapped_one"
        )


# ─── 8. relationship validation ─────────────────────────────────────────

CANDIDATE_WITH_REFS = textwrap.dedent(
    """\
    session_discovered_candidates:
      candidates:
        - id: candidate.invariant.meta.refs_one
          class: Invariant
          label: Has references
          status: candidate
          confidence: candidate
          severity: high
          discovered: 2026-08-03
          evidence: observed
          related_invariants:
            - invariant:{inv_ref}
          related_failures:
            - failure_mode:{fm_ref}
        - id: candidate.failure_mode.meta.sibling_target
          class: FailureMode
          label: The sibling referenced above
          status: candidate
          confidence: candidate
          severity: high
          discovered: 2026-08-03
          evidence: observed
    """
)

CANONICAL_WITH_ONE = textwrap.dedent(
    """\
    invariants:
      - id: existing.canonical_invariant
        title: Exists
        severity: high
        status: active
    """
)


class TestRelationshipValidation(TempRepoCase):
    def _setup(self, inv_ref: str, fm_ref: str):
        self.write_candidate(
            "session.yaml", CANDIDATE_WITH_REFS.format(inv_ref=inv_ref, fm_ref=fm_ref)
        )
        self.write_canonical("invariants.yaml", CANONICAL_WITH_ONE)
        return tool.find_candidate("candidate.invariant.meta.refs_one")[1]

    def test_canonical_ref_resolves(self):
        entry = self._setup("existing.canonical_invariant", "meta.sibling_target")
        with redirect_stderr(io.StringIO()):
            tool.validate(entry, "invariants.yaml")  # must not die

    def test_sibling_candidate_ref_is_pending_not_fatal(self):
        # meta.sibling_target is another candidate in the same file. That is
        # the normal state while a cluster is promoted one at a time — warn,
        # do not refuse.
        entry = self._setup("existing.canonical_invariant", "meta.sibling_target")
        err = io.StringIO()
        with redirect_stderr(err):
            tool.validate(entry, "invariants.yaml")
        self.assertIn("PENDING", err.getvalue())
        self.assertIn("meta.sibling_target", err.getvalue())

    def test_dangling_ref_refuses_promotion(self):
        entry = self._setup("existing.canonical_invariant", "totally.made_up_reference")
        with self.assertRaises(SystemExit) as ctx:
            with redirect_stderr(io.StringIO()):
                tool.validate(entry, "invariants.yaml")
        self.assertEqual(ctx.exception.code, 1)

    def test_allow_dangling_downgrades_to_warning(self):
        entry = self._setup("existing.canonical_invariant", "totally.made_up_reference")
        err = io.StringIO()
        with redirect_stderr(err):
            tool.validate(entry, "invariants.yaml", allow_dangling=True)
        self.assertIn("DANGLING", err.getvalue())

    def test_prefix_stripping_on_refs(self):
        self.assertEqual(tool.strip_relation_prefix("invariant:meta.foo"), "meta.foo")
        self.assertEqual(tool.strip_relation_prefix("failure_mode:x.y"), "x.y")
        self.assertEqual(tool.strip_relation_prefix("meta.foo"), "meta.foo")


# ─── 9. non-destructive writes ──────────────────────────────────────────

CANONICAL_WITH_COMMENTS = textwrap.dedent(
    """\
    # This header comment must survive promotion.
    invariants:
      - id: existing.entry_one
        title: First
        severity: high
        status: active
        description: >-
          A folded scalar that must not be reflowed into one long line
          by a full safe_dump round-trip.
    """
)


class TestNonDestructiveWrite(TempRepoCase):
    def test_append_preserves_comments_and_existing_text(self):
        self.write_candidate("session.yaml", CANDIDATE_WRAPPED)
        target = self.write_canonical("invariants.yaml", CANONICAL_WITH_COMMENTS)
        before = target.read_text("utf-8")

        _p, entry = tool.find_candidate("candidate.invariant.meta.wrapped_one")
        canonical = tool.to_canonical_entry(entry)
        with redirect_stdout(io.StringIO()):
            tool.write_canonical(target, "invariants", canonical, dry_run=False)

        after = target.read_text("utf-8")
        # Everything that was there before is still there, byte for byte.
        self.assertTrue(after.startswith(before), "existing text was rewritten, not appended")
        self.assertIn("# This header comment must survive promotion.", after)
        self.assertIn("A folded scalar that must not be reflowed", after)

        import yaml
        data = yaml.safe_load(after)
        ids = [e["id"] for e in data["invariants"]]
        self.assertEqual(ids, ["existing.entry_one", "meta.wrapped_one"])

    def test_removal_preserves_wrapper_and_does_not_mix_shapes(self):
        path = self.write_candidate("session.yaml", CANDIDATE_WRAPPED)
        with redirect_stdout(io.StringIO()):
            tool.remove_from_candidate_file(path, "candidate.invariant.meta.wrapped_one", dry_run=False)

        import yaml
        data = yaml.safe_load(path.read_text("utf-8"))
        self.assertEqual(list(data.keys()), ["session_discovered_candidates"])
        self.assertNotIn("candidates", data, "a stray top-level candidates key was added")
        remaining = data["session_discovered_candidates"]["candidates"]
        self.assertEqual([e["id"] for e in remaining], ["candidate.failure_mode.meta.wrapped_two"])
        self.assertIn("# a leading comment that must survive", path.read_text("utf-8"))

    def test_removing_last_entry_leaves_empty_list_not_null(self):
        path = self.write_candidate("session.yaml", CANDIDATE_WRAPPED)
        with redirect_stdout(io.StringIO()):
            for cid in ("candidate.invariant.meta.wrapped_one",
                        "candidate.failure_mode.meta.wrapped_two"):
                tool.remove_from_candidate_file(path, cid, dry_run=False)
        import yaml
        data = yaml.safe_load(path.read_text("utf-8"))
        self.assertEqual(data["session_discovered_candidates"]["candidates"], [])

    def test_dry_run_does_not_mutate_wrapped_file(self):
        path = self.write_candidate("session.yaml", CANDIDATE_WRAPPED)
        target = self.write_canonical("invariants.yaml", CANONICAL_WITH_COMMENTS)
        before_c, before_t = path.read_text("utf-8"), target.read_text("utf-8")
        _p, entry = tool.find_candidate("candidate.invariant.meta.wrapped_one")
        with redirect_stdout(io.StringIO()):
            tool.write_canonical(target, "invariants", tool.to_canonical_entry(entry), dry_run=True)
            tool.remove_from_candidate_file(path, entry["id"], dry_run=True)
        self.assertEqual(path.read_text("utf-8"), before_c)
        self.assertEqual(target.read_text("utf-8"), before_t)


# ─── 10. --list ──────────────────────────────────────────────────────────

class TestListMode(TempRepoCase):
    def test_list_reports_wrapped_candidates_and_targets(self):
        self.write_candidate("session.yaml", CANDIDATE_WRAPPED)
        self.write_canonical("invariants.yaml", CANONICAL_INVARIANTS_EMPTY)
        out = io.StringIO()
        with redirect_stdout(out):
            rc = tool.list_candidates()
        text = out.getvalue()
        self.assertEqual(rc, 0)
        self.assertIn("2 candidate(s)", text)
        self.assertIn("candidate.invariant.meta.wrapped_one", text)
        self.assertIn("meta.wrapped_one  →  docs/awareness/invariants.yaml", text)
        self.assertIn("session_discovered_candidates", text)
        self.assertIn("PROMOTABLE", text)

    def test_list_flags_blocked_candidates(self):
        self.write_candidate("c.yaml", CANDIDATE_LOW_CONFIDENCE)
        out = io.StringIO()
        with redirect_stdout(out):
            tool.list_candidates()
        self.assertIn("BLOCKED", out.getvalue())
        self.assertIn("confidence=low", out.getvalue())


if __name__ == "__main__":
    unittest.main(verbosity=2)
