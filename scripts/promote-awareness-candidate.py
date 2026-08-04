#!/usr/bin/env python3
"""
Promote a session-discovered awareness candidate into canonical YAML.

Workflow
--------
A candidate sits in docs/awareness/candidates/<file>.yaml with
status:candidate. This script COPIES a single candidate (by id) into one
of the canonical knowledge files (invariants.yaml / failure_modes.yaml /
intents.yaml), strips the candidate-only fields, and records provenance.

Single authority
----------------
The canonical corpus is the only mutable authority. The candidate file is
an APPEND-ONLY discovery ledger and is never written by this script — not
to delete a promoted entry, not to set a flag. Promotion state is DERIVED:

    canonical id absent                          → PENDING
    canonical id present, provenance points here → PROMOTED
    canonical id present, belongs to something else → CONFLICT

Deriving the state rather than storing it removes an entire class of
problem. Before 2026-08-03 promotion wrote two files in sequence, so it
needed cross-file atomicity, rollback on partial failure, and — to close
the crash window between the two swaps — a write-ahead journal with
recovery-on-startup semantics. None of that exists now because the second
write does not exist. Re-running a promotion is a no-op that reports
"already promoted" rather than a duplicate-id error, so retry is
idempotent by construction rather than by protocol.

The ledger grows without bound, which is correct: what was discovered, and
when, is history worth keeping.

The script does NOT regenerate RDF triples. That flows through the
normal build pipeline: after promotion, run
  scripts/build-awareness-graph.sh
from the awareness-graph repo so the new canonical entry lands in the
seed.

Candidate file shapes
---------------------
Two shapes are accepted, matching the sensei protection deriver
(golang/architecture/protection/candidate_document.go):

    candidates: [...]                        # direct
    <generator_name>: {candidates: [...]}    # wrapped / envelope

docs/awareness/candidates/session_discovered_invariants.yaml uses the
WRAPPED shape (session_discovered_candidates:). Until 2026-08-03 this
script read only the top-level `candidates` key, so every entry in that
file was invisible: `--list` showed nothing and promotion died with
"candidate id not found". The candidates were write-only. A document
that mixes both shapes is rejected, same as the Go parser.

Validation
----------
- ID must match the canonical naming convention:
    <namespace>.<bare_id>   where each segment is [a-z0-9._]+
  A leading `candidate.<class>.` prefix is stripped first: the candidate
  id `candidate.invariant.meta.foo` promotes to canonical id `meta.foo`.
- The canonical id must be free, or already owned by THIS candidate (in
  which case promotion is a reported no-op). An id owned by a different
  candidate, or by an entry carrying no candidate provenance, is a
  CONFLICT and refuses.
- Candidate must have status:candidate and confidence != "low".
- Candidate must carry evidence in at least one evidence-bearing field.
- Candidate must carry provenance (`discovered_from` or `discovered`).
- Class must match the target file's class, case-insensitively and in
  either CamelCase or snake_case (Invariant / invariant, FailureMode /
  failure_mode):
    invariants.yaml      → class: invariant
    failure_modes.yaml   → class: failure_mode
    intents.yaml         → class: intent
    incident_patterns.yaml → class: incident_pattern
- Every relationship reference (related_invariants, related_failures,
  related_failure_modes, related_intents) must resolve to a canonical id
  or to a sibling candidate. A reference to a sibling candidate is
  reported as PENDING and allowed — that is the normal state while a
  cluster of candidates is promoted one at a time. A reference that
  resolves to neither is DANGLING and refuses the promotion unless
  --allow-dangling is passed.

Usage
-----
  # See every candidate and whether it is promotable:
  scripts/promote-awareness-candidate.py --list

  scripts/promote-awareness-candidate.py \\
    --id remediation.test_audit_writes_must_be_isolated_from_production_etcd \\
    --target docs/awareness/invariants.yaml

  # Dry-run (validate but don't write):
  scripts/promote-awareness-candidate.py --id <id> --target <file> --dry-run

Exit codes
----------
  0 — promotion succeeded (or dry-run / --list succeeded)
  1 — validation failed (id not found, duplicate, class mismatch, etc.)
  2 — usage error (missing args, bad target file)
"""

from __future__ import annotations

import argparse
import os
import re
import sys
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:
    sys.stderr.write(
        "promote-awareness-candidate: PyYAML is required. Install via 'pip install pyyaml' or your distro's package manager.\n"
    )
    sys.exit(2)


REPO_ROOT = Path(__file__).resolve().parent.parent
CANDIDATES_DIR = REPO_ROOT / "docs" / "awareness" / "candidates"

# Canonical naming rule: <namespace>.<bare_id> where each segment is
# lowercase ASCII letters, digits, dots, underscores. No spaces, no
# uppercase, no slashes.
ID_PATTERN = re.compile(r"^[a-z0-9_]+(\.[a-z0-9_]+)+$")

# Candidate ids carry a `candidate.<class>.` prefix that must not survive
# promotion — the canonical corpus refers to `meta.foo`, not
# `candidate.invariant.meta.foo`. This is also the form sibling entries
# use in related_invariants / related_failures.
CANDIDATE_ID_PREFIX = re.compile(
    r"^candidate\.(invariant|failure_mode|intent|incident_pattern)\."
)

# Which canonical file holds which class.
TARGET_CLASS = {
    "invariants.yaml": "invariant",
    "failure_modes.yaml": "failure_mode",
    "intents.yaml": "intent",
    "incident_patterns.yaml": "incident_pattern",
}

# Top-level key under which entries live in each canonical file.
TARGET_LIST_KEY = {
    "invariants.yaml": "invariants",
    "failure_modes.yaml": "failure_modes",
    "intents.yaml": "intents",
    "incident_patterns.yaml": "incident_patterns",
}

CANONICAL_LIST_KEYS = ("invariants", "failure_modes", "intents", "incident_patterns")

# Fields that can carry the evidence a reviewer needs. The corpus does not
# use a single `evidence:` key everywhere — entries also record evidence as
# a verified negative, a prevented instance, or per-sub-shape observations.
EVIDENCE_FIELDS = (
    "evidence",
    "verified_negative",
    "prevented_instance",
    "proposed_doctor_evidence_shape",
    "note",
)

# Relationship fields and the id-prefix each value may carry.
RELATION_FIELDS = (
    "related_invariants",
    "related_failures",
    "related_failure_modes",
    "related_intents",
    "related_incident_patterns",
)
RELATION_PREFIXES = ("invariant:", "failure_mode:", "intent:", "incident_pattern:")

# Candidate files write a dialect the canonical corpus does not use:
# `related_failures` with `failure_mode:`-prefixed values. Across the 763
# relationship lists in docs/awareness/*.yaml there is not one prefixed value
# and not one `related_failures` key — canonical is `related_failure_modes`
# with bare ids. Copying candidate relationships through verbatim would
# validate one dialect and write another, leaving entries whose references
# resolve for the validator and dangle for every consumer.
RELATION_FIELD_ALIASES = {
    "related_failures": "related_failure_modes",
}

# Substantive fields carried across into the canonical entry. Without these
# a promoted entry is a husk: the contract, the forbidden fixes and the
# required tests are the whole reason the candidate was worth recording.
CARRIED_FIELDS = (
    "description",
    "contract",
    "summary",
    "protects",
    "enforcement",
    "source_files",
    "required_tests",
    "forbidden_fixes",
    "proposed_invariants",
    "deferred_invariant",
    "failure_sub_shapes",
    "enforcement_obligations",
    "prevented_instance",
    "prevented_instance_detectors",
    "verified_negative",
    "proposed_doctor_evidence_shape",
)
# NB: relationship fields are deliberately NOT in CARRIED_FIELDS — they go
# through canonicalize_relations() instead of being copied verbatim.


def die(msg: str, code: int = 1) -> None:
    sys.stderr.write(f"promote-awareness-candidate: {msg}\n")
    sys.exit(code)


def warn(msg: str) -> None:
    sys.stderr.write(f"promote-awareness-candidate: warning: {msg}\n")


def load_yaml(path: Path) -> Any:
    with path.open("r", encoding="utf-8") as f:
        return yaml.safe_load(f) or {}


# ─── candidate document shapes ───────────────────────────────────────────

def extract_candidate_lists(data: Any, path: Path) -> list[tuple[str | None, list]]:
    """Return [(wrapper_key_or_None, candidate_list), ...] for one document.

    Accepts the direct shape (top-level `candidates:`) and the wrapped
    shape (`<generator>: {candidates: [...]}`). Mixing both in one
    document is an error, matching the Go parser in
    golang/architecture/protection/candidate_document.go."""
    if not isinstance(data, dict):
        return []

    found: list[tuple[str | None, list]] = []

    direct = data.get("candidates")
    if isinstance(direct, list):
        found.append((None, direct))

    wrapped: list[tuple[str | None, list]] = []
    for key, value in data.items():
        if key == "candidates" or not isinstance(value, dict):
            continue
        inner = value.get("candidates")
        if isinstance(inner, list):
            wrapped.append((key, inner))

    if direct is not None and wrapped:
        die(
            f"{path}: mixes direct and wrapped candidates "
            f"(top-level 'candidates' plus {[k for k, _ in wrapped]}). "
            f"Use one shape per document."
        )
    return found + wrapped


def iter_all_candidates() -> list[tuple[Path, str | None, dict]]:
    """Every candidate in the tree, as (file, wrapper_key, entry)."""
    out: list[tuple[Path, str | None, dict]] = []
    if not CANDIDATES_DIR.is_dir():
        die(f"candidates dir missing: {CANDIDATES_DIR}")
    for yaml_path in sorted(CANDIDATES_DIR.rglob("*.yaml")):
        data = load_yaml(yaml_path)
        for wrapper, entries in extract_candidate_lists(data, yaml_path):
            for entry in entries:
                if isinstance(entry, dict):
                    out.append((yaml_path, wrapper, entry))
    return out


def find_candidate(candidate_id: str) -> tuple[Path, dict]:
    """Search every YAML in candidates/ for the given id. Returns
    (file_path, entry_dict). Dies if not found or found multiple times."""
    matches = [
        (path, entry)
        for path, _wrapper, entry in iter_all_candidates()
        if entry.get("id") == candidate_id
    ]
    if not matches:
        known = [e.get("id") for _p, _w, e in iter_all_candidates()]
        hint = ""
        if known:
            hint = "\n  known candidate ids:\n    " + "\n    ".join(
                str(k) for k in known if k
            )
        die(
            f"candidate id not found in any docs/awareness/candidates/*.yaml: "
            f"{candidate_id!r}{hint}"
        )
    if len(matches) > 1:
        die(
            f"candidate id {candidate_id!r} found in multiple files; ambiguous: "
            + ", ".join(str(p.relative_to(REPO_ROOT)) for p, _ in matches)
        )
    return matches[0]


# ─── normalisation ───────────────────────────────────────────────────────

def normalize_class(raw: Any) -> str:
    """Accept Invariant / invariant / FailureMode / failure_mode / ..."""
    if not isinstance(raw, str):
        return ""
    s = raw.strip()
    if not s:
        return ""
    # CamelCase → snake_case, then lowercase.
    snake = re.sub(r"(?<!^)(?=[A-Z])", "_", s).lower()
    return snake.replace("__", "_")


def canonical_id_for(candidate: dict) -> str:
    """The id the entry will carry once promoted: the candidate id minus
    its `candidate.<class>.` prefix."""
    cid = candidate.get("id")
    if not isinstance(cid, str):
        return ""
    return CANDIDATE_ID_PREFIX.sub("", cid.strip())


def evidence_of(candidate: dict) -> str:
    """First non-empty evidence-bearing field, or "" when none carries
    anything. Also scans failure_sub_shapes for observed_* entries, which
    is where sub-shaped candidates put their concrete instances."""
    for field in EVIDENCE_FIELDS:
        value = candidate.get(field)
        if isinstance(value, str) and value.strip():
            return value.strip()
        if isinstance(value, (list, dict)) and value:
            return yaml.safe_dump(value, sort_keys=False, allow_unicode=True).strip()

    for shape in candidate.get("failure_sub_shapes") or []:
        if not isinstance(shape, dict):
            continue
        for key, value in shape.items():
            if key.startswith("observed") and value:
                return yaml.safe_dump(
                    {key: value}, sort_keys=False, allow_unicode=True
                ).strip()
    return ""


def provenance_of(candidate: dict) -> str:
    """Provenance string from discovered_from, else discovered (a date)."""
    for field in ("discovered_from", "discovered"):
        value = candidate.get(field)
        if value is None:
            continue
        text = str(value).strip()
        if text:
            return text if field == "discovered_from" else f"session {text}"
    return ""


# ─── canonical corpus ────────────────────────────────────────────────────

def all_canonical_entries() -> dict[str, dict]:
    """Every canonical entry by id, from docs/awareness/*.yaml (NOT
    candidates/, NOT subdirs)."""
    entries_by_id: dict[str, dict] = {}
    canonical_dir = REPO_ROOT / "docs" / "awareness"
    for yaml_path in canonical_dir.glob("*.yaml"):  # NB: glob, not rglob — top level only
        data = load_yaml(yaml_path)
        if not isinstance(data, dict):
            continue
        for list_key in CANONICAL_LIST_KEYS:
            entries = data.get(list_key) or []
            if not isinstance(entries, list):
                continue
            for entry in entries:
                if isinstance(entry, dict) and "id" in entry:
                    entries_by_id.setdefault(entry["id"], entry)
    return entries_by_id


def all_canonical_ids() -> set[str]:
    """Ids of every canonical entry."""
    return set(all_canonical_entries())


# ─── promotion state (derived, never stored) ─────────────────────────────
#
# The canonical corpus is the ONLY mutable authority. A candidate's
# promotion state is DERIVED from whether its canonical id is present and
# whose it is — never recorded as a flag in the ledger and never signalled
# by deleting the ledger entry. That removes the two-file synchronization
# class entirely: no second write, no cross-file transaction, no rollback,
# no crash journal, and retry is idempotent by construction.

STATE_PENDING = "PENDING"
STATE_PROMOTED = "PROMOTED"
STATE_CONFLICT = "CONFLICT"


def promotion_state(
    candidate: dict, canonical: dict[str, dict] | None = None
) -> tuple[str, str]:
    """(state, human detail) for one candidate.

    PENDING   — canonical id absent.
    PROMOTED  — canonical id present and provenance says it came from THIS
                candidate. Robust to later hand-polishing of the canonical
                entry, which is expected and encouraged.
    CONFLICT  — canonical id present but belongs to something else: a
                different candidate, or an entry with no candidate
                provenance at all. The id is taken; promoting would either
                collide or silently overwrite meaning.
    """
    if canonical is None:
        canonical = all_canonical_entries()
    cid = canonical_id_for(candidate)
    existing = canonical.get(cid)
    if existing is None:
        return STATE_PENDING, ""

    provenance = existing.get("provenance")
    origin = provenance.get("candidate_id") if isinstance(provenance, dict) else None
    if origin == candidate.get("id"):
        return STATE_PROMOTED, f"canonical {cid!r} was promoted from this candidate"
    if origin:
        return (
            STATE_CONFLICT,
            f"canonical {cid!r} was promoted from a different candidate ({origin!r})",
        )
    return (
        STATE_CONFLICT,
        f"canonical {cid!r} already exists and carries no candidate provenance — "
        f"either it predates provenance tracking or the id is genuinely taken; "
        f"resolve by hand",
    )


def strip_relation_prefix(ref: str) -> str:
    for prefix in RELATION_PREFIXES:
        if ref.startswith(prefix):
            return ref[len(prefix):]
    return ref


def canonicalize_relations(candidate: dict) -> dict:
    """Candidate relationship fields → the canonical dialect.

    Maps alias field names (related_failures → related_failure_modes),
    strips the `invariant:` / `failure_mode:` / `intent:` /
    `incident_pattern:` value prefixes, de-duplicates, and preserves
    first-occurrence order so the output is deterministic without
    re-sorting an author's curated list."""
    out: dict[str, list[str]] = {}
    for field in RELATION_FIELDS:
        values = candidate.get(field)
        if not values:
            continue
        target = RELATION_FIELD_ALIASES.get(field, field)
        bucket = out.setdefault(target, [])
        for ref in values:
            if not isinstance(ref, str) or not ref.strip():
                continue
            cleaned = strip_relation_prefix(ref.strip())
            if cleaned and cleaned not in bucket:
                bucket.append(cleaned)
    return {k: v for k, v in out.items() if v}


def check_relationships(
    candidate: dict, canonical_ids: set[str], sibling_ids: set[str]
) -> tuple[list[str], list[str]]:
    """Resolve every relationship reference. Returns (pending, dangling).

    pending  — resolves to a sibling candidate not yet promoted. Normal
               while a cluster of candidates is promoted one at a time.
    dangling — resolves to nothing at all. That is a broken corpus
               reference and refuses promotion by default."""
    pending: list[str] = []
    dangling: list[str] = []
    for field in RELATION_FIELDS:
        for ref in candidate.get(field) or []:
            if not isinstance(ref, str) or not ref.strip():
                continue
            target = strip_relation_prefix(ref.strip())
            if target in canonical_ids:
                continue
            if target in sibling_ids:
                pending.append(f"{field}: {ref}")
            else:
                dangling.append(f"{field}: {ref}")
    return pending, dangling


# ─── validation ──────────────────────────────────────────────────────────

def validate(candidate: dict, target_filename: str, allow_dangling: bool = False) -> None:
    """All the checks. Dies on failure."""
    cid = canonical_id_for(candidate)
    if not cid or not ID_PATTERN.match(cid):
        die(
            f"id {candidate.get('id')!r} (canonical form {cid!r}) does not match "
            f"canonical naming: <namespace>.<bare_id> "
            f"(segments: [a-z0-9_]+, joined by dots)"
        )

    expected_class = TARGET_CLASS.get(target_filename)
    if expected_class is None:
        die(
            f"target {target_filename!r} is not a recognized canonical file. "
            f"Supported: {sorted(TARGET_CLASS.keys())}",
            code=2,
        )
    actual_class = normalize_class(candidate.get("class"))
    if actual_class != expected_class:
        die(
            f"class mismatch: candidate.class={candidate.get('class')!r} "
            f"(normalized {actual_class!r}) but target {target_filename!r} "
            f"expects class={expected_class!r}"
        )

    if candidate.get("status") != "candidate":
        die(
            f"refusing to promote: status={candidate.get('status')!r}, expected 'candidate'. "
            f"Promotion is the ONLY way to change status."
        )

    if candidate.get("confidence") == "low":
        die(
            f"refusing to promote candidate with confidence=low. Gather more evidence "
            f"or close the candidate as rejected before promoting."
        )

    if not evidence_of(candidate):
        die(
            "refusing to promote: candidate carries no evidence. Reviewers need it. "
            f"Checked fields: {', '.join(EVIDENCE_FIELDS)}, and failure_sub_shapes[].observed_*"
        )

    if not provenance_of(candidate):
        die(
            "refusing to promote: no provenance. Set discovered_from, or discovered "
            "(a date). Provenance is required."
        )

    canonical_entries = all_canonical_entries()
    existing = set(canonical_entries)
    state, detail = promotion_state(candidate, canonical_entries)
    if state == STATE_CONFLICT:
        die(f"refusing to promote: {detail}")

    sibling_ids = {
        canonical_id_for(entry)
        for _p, _w, entry in iter_all_candidates()
        if canonical_id_for(entry) and canonical_id_for(entry) != cid
    }
    pending, dangling = check_relationships(candidate, existing, sibling_ids)
    for ref in pending:
        warn(f"relationship PENDING (sibling candidate, not yet promoted) — {ref}")
    if dangling:
        detail = "\n    ".join(dangling)
        if allow_dangling:
            warn(f"relationship DANGLING (allowed by --allow-dangling):\n    {detail}")
        else:
            die(
                f"refusing to promote: {len(dangling)} relationship reference(s) resolve to "
                f"neither a canonical entry nor a sibling candidate:\n    {detail}\n"
                f"  Fix the reference, promote the referent first, or pass --allow-dangling."
            )


# ─── canonical entry ─────────────────────────────────────────────────────

def to_canonical_entry(candidate: dict) -> dict:
    """Strip candidate-only fields, carry the substance, record provenance."""
    entry: dict = {
        "id": canonical_id_for(candidate),
        "title": str(candidate.get("label", "")).strip(),
        # Candidates written before 2026-08 use `risk`; the session files use
        # `severity`. Reading only `risk` silently downgraded every promoted
        # entry to "medium".
        "severity": candidate.get("severity") or candidate.get("risk") or "medium",
        "status": "active",
    }

    # Carry the substance across. A promoted entry without its contract,
    # forbidden fixes and required tests is a husk that satisfies the
    # promotion step while losing the knowledge it existed to hold.
    for field in CARRIED_FIELDS:
        if field in candidate and candidate[field] not in (None, "", [], {}):
            entry[field] = candidate[field]

    # Relationships are translated into the canonical dialect, never copied.
    entry.update(canonicalize_relations(candidate))

    # Provenance — operators and future agents need to know where this
    # entry came from. Evidence lives here rather than at top level: it is
    # provenance for the claim, not part of the claim.
    entry["provenance"] = {
        "promoted_from": "candidate",
        "candidate_id": candidate.get("id", ""),
        "discovered_from": provenance_of(candidate),
        "confidence_at_promotion": candidate.get("confidence", "medium"),
        "evidence": evidence_of(candidate),
    }
    return entry


# ─── non-destructive writes ──────────────────────────────────────────────

def _line_start(text: str, index: int) -> int:
    nl = text.rfind("\n", 0, index)
    return 0 if nl < 0 else nl + 1


def _line_end(text: str, index: int) -> int:
    nl = text.find("\n", index)
    return len(text) if nl < 0 else nl + 1


def _item_end(text: str, node) -> int:
    """Offset just past a node's last line.

    A node's end_mark often points at column 0 of the FOLLOWING line (the
    next token), in which case that index already is the end of the item —
    extending to the next newline would swallow a whole extra line, which
    for a sequence followed by another top-level key means appending inside
    the wrong block."""
    idx = node.end_mark.index
    if idx == _line_start(text, idx):
        return idx
    return _line_end(text, idx)


def _mapping_id(node) -> str | None:
    """The `id` value of a MappingNode, whatever position the key sits in."""
    if not isinstance(node, yaml.MappingNode):
        return None
    for key_node, value_node in node.value:
        if getattr(key_node, "value", None) == "id":
            return getattr(value_node, "value", None)
    return None


def _find_sequence_node(root, list_key: str):
    """The SequenceNode for `list_key`, searched at top level and one level
    inside a wrapper mapping."""
    if not isinstance(root, yaml.MappingNode):
        return None
    for key_node, value_node in root.value:
        if key_node.value == list_key and isinstance(value_node, yaml.SequenceNode):
            return value_node
    for _key_node, value_node in root.value:
        if isinstance(value_node, yaml.MappingNode):
            found = _find_sequence_node(value_node, list_key)
            if found is not None:
                return found
    return None


def _compose(text: str, path: Path):
    try:
        return yaml.compose(text)
    except yaml.YAMLError as exc:
        die(f"{path}: could not parse for structural edit: {exc}")


def canonical_insertion_point(text: str, list_key: str, path: Path) -> tuple[int, str] | None:
    """Character offset at which to insert a new item, and the bullet indent.

    Inserts at the end of the ACTUAL list rather than at end-of-file, so a
    canonical file whose list is not the final top-level key stays valid.
    Returns None when there is no block sequence to append to (empty or
    flow-style list), where a structural dump is the correct fallback."""
    root = _compose(text, path)
    seq = _find_sequence_node(root, list_key) if root is not None else None
    if seq is None or not seq.value:
        return None
    last = seq.value[-1]
    indent = " " * max(last.start_mark.column - 2, 0)
    return _item_end(text, last), indent


def _render_entry(entry: dict, indent: str) -> str:
    """Render one entry as a block-list item, indented to match the file."""
    text = yaml.safe_dump(
        [entry], sort_keys=False, allow_unicode=True, width=100, default_flow_style=False
    )
    if not indent:
        return text
    return "".join(indent + line if line.strip() else line for line in text.splitlines(True))



def canonical_text_with_entry(
    text: str, list_key: str, new_entry: dict, path: Path
) -> str:
    """The canonical file's full new content, computed in memory.

    Appends TEXTUALLY at the end of the list. A full yaml.safe_dump
    round-trip of the canonical invariants file (11.5k lines, 529KB)
    rewrites all of it, reflows every folded scalar and deletes every
    comment — a diff no reviewer can read. Only the empty/flow-style case
    (nothing to preserve) falls back to a structural dump."""
    spot = canonical_insertion_point(text, list_key, path)
    if spot is None:
        data = yaml.safe_load(text) or {}
        if not isinstance(data, dict):
            data = {}
        entries = list(data.get(list_key) or [])
        entries.append(new_entry)
        data[list_key] = entries
        return yaml.safe_dump(data, sort_keys=False, allow_unicode=True)

    offset, indent = spot
    rendered = _render_entry(new_entry, indent)
    prefix = text[:offset]
    if prefix and not prefix.endswith("\n"):
        prefix += "\n"
    return prefix + rendered + text[offset:]


def verify_canonical_text(
    text: str, list_key: str, entry_id: str, expected: int, path: Path
) -> None:
    """Parse the proposed content and assert the entry landed."""
    try:
        data = yaml.safe_load(text) or {}
    except yaml.YAMLError as exc:
        die(f"proposed content for {path} is not valid YAML: {exc}")
    entries = (data or {}).get(list_key) or []
    ids = [e.get("id") for e in entries if isinstance(e, dict)]
    if entry_id not in ids:
        die(f"proposed content for {path} does not contain {entry_id!r}")
    if len(entries) != expected:
        die(
            f"proposed content for {path} has {len(entries)} {list_key} entries, "
            f"expected {expected}"
        )


def atomic_write(path: Path, text: str) -> None:
    """Replace one file's content atomically.

    Promotion writes exactly ONE file, so os.replace is genuinely atomic
    here — there is no second target and therefore no cross-file window, no
    rollback, and no crash journal. That is the point of the single-authority
    model: the transaction did not get safer, it stopped existing."""
    tmp = path.with_name(path.name + ".promote-tmp")
    try:
        tmp.write_text(text, encoding="utf-8")
        os.replace(tmp, path)
    except OSError as exc:
        tmp.unlink(missing_ok=True)
        die(f"could not write {path}: {exc}")


def write_canonical(target_path: Path, list_key: str, new_entry: dict, dry_run: bool) -> None:
    text = target_path.read_text(encoding="utf-8")
    data = yaml.safe_load(text) or {}
    if not isinstance(data, dict):
        data = {}
    entries = data.get(list_key) or []
    if not isinstance(entries, list):
        die(f"target {target_path} has {list_key} but it's not a list")

    proposed = canonical_text_with_entry(text, list_key, new_entry, target_path)
    verify_canonical_text(proposed, list_key, new_entry["id"], len(entries) + 1, target_path)

    if dry_run:
        sys.stdout.write(
            f"[dry-run] would append to {target_path.relative_to(REPO_ROOT)} "
            f"({len(entries)} → {len(entries) + 1} entries):\n"
        )
        sys.stdout.write(_render_entry(new_entry, ""))
        return
    atomic_write(target_path, proposed)
    sys.stdout.write(
        f"appended to {target_path.relative_to(REPO_ROOT)} ({len(entries) + 1} entries)\n"
    )


# ─── listing ─────────────────────────────────────────────────────────────

def list_candidates() -> int:
    """Print every candidate and whether it is promotable."""
    rows = iter_all_candidates()
    if not rows:
        sys.stdout.write("no candidates found under docs/awareness/candidates/\n")
        return 0

    canonical_entries = all_canonical_entries()
    canonical = set(canonical_entries)
    sibling_ids = {canonical_id_for(e) for _p, _w, e in rows if canonical_id_for(e)}

    counts: dict[str, int] = {}
    sys.stdout.write(f"{len(rows)} candidate(s):\n\n")
    for path, wrapper, entry in rows:
        cid = entry.get("id", "<no id>")
        canon = canonical_id_for(entry)
        cls = normalize_class(entry.get("class"))
        target = next((f for f, c in TARGET_CLASS.items() if c == cls), "?")

        # Status is DERIVED from the canonical corpus, never stored in the
        # ledger. Canonical presence is a state, not a blocker: an entry that
        # is already promoted is a success, not a duplicate-id error.
        derived, detail = promotion_state(entry, canonical_entries)

        blockers: list[str] = []
        if derived == STATE_PENDING:
            if not canon or not ID_PATTERN.match(canon):
                blockers.append("id-format")
            if entry.get("status") != "candidate":
                blockers.append(f"status={entry.get('status')!r}")
            if entry.get("confidence") == "low":
                blockers.append("confidence=low")
            if not evidence_of(entry):
                blockers.append("no-evidence")
            if not provenance_of(entry):
                blockers.append("no-provenance")
            _pending, dangling = check_relationships(entry, canonical, sibling_ids - {canon})
            if dangling:
                blockers.append(f"dangling-refs={len(dangling)}")

        if derived == STATE_PROMOTED:
            state = f"{STATE_PROMOTED} — {detail}"
        elif derived == STATE_CONFLICT:
            state = f"{STATE_CONFLICT} — {detail}"
        elif blockers:
            state = f"{STATE_PENDING} (BLOCKED: " + ", ".join(blockers) + ")"
        else:
            state = f"{STATE_PENDING} (promotable)"
        counts[derived] = counts.get(derived, 0) + 1
        sys.stdout.write(f"  {cid}\n")
        sys.stdout.write(f"      file:      {path.relative_to(REPO_ROOT)}")
        sys.stdout.write(f" (wrapper: {wrapper})\n" if wrapper else " (direct)\n")
        sys.stdout.write(f"      class:     {entry.get('class')!r} → {cls or '?'}\n")
        sys.stdout.write(f"      promotes:  {canon}  →  docs/awareness/{target}\n")
        sys.stdout.write(f"      severity:  {entry.get('severity') or entry.get('risk') or '?'}"
                         f"   confidence: {entry.get('confidence') or '?'}\n")
        sys.stdout.write(f"      state:     {state}\n\n")
    summary = "  ".join(
        f"{name}={counts.get(name, 0)}"
        for name in (STATE_PENDING, STATE_PROMOTED, STATE_CONFLICT)
    )
    sys.stdout.write(f"derived from canonical corpus: {summary}\n")
    return 0


# ─── main ────────────────────────────────────────────────────────────────

def main() -> int:
    parser = argparse.ArgumentParser(
        description="Promote a session-discovered awareness candidate into canonical YAML."
    )
    parser.add_argument("--id", help="The candidate id to promote")
    parser.add_argument(
        "--target",
        help="Path to the target canonical YAML "
        "(e.g. docs/awareness/invariants.yaml). Defaults to the file matching the "
        "candidate's class.",
    )
    parser.add_argument(
        "--list",
        action="store_true",
        dest="list_candidates",
        help="List every candidate with its promotion target and blockers, then exit",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Validate and print the resulting entry; do NOT modify files",
    )
    parser.add_argument(
        "--allow-dangling",
        action="store_true",
        help="Warn instead of refusing when a relationship reference resolves to nothing",
    )
    args = parser.parse_args()

    if args.list_candidates:
        return list_candidates()

    if not args.id:
        die("--id is required (or use --list)", code=2)

    candidate_path, candidate = find_candidate(args.id)
    sys.stdout.write(f"candidate found: {candidate_path.relative_to(REPO_ROOT)}\n")

    target = args.target
    if not target:
        cls = normalize_class(candidate.get("class"))
        match = next((f for f, c in TARGET_CLASS.items() if c == cls), None)
        if match is None:
            die(
                f"cannot infer target for class {candidate.get('class')!r}; pass --target",
                code=2,
            )
        target = f"docs/awareness/{match}"
        sys.stdout.write(f"target inferred from class: {target}\n")

    target_path = REPO_ROOT / target if not os.path.isabs(target) else Path(target)
    if not target_path.exists():
        die(f"target file not found: {target_path}", code=2)
    target_filename = target_path.name

    # Promotion state is derived, so a repeat run is a no-op rather than a
    # duplicate-id failure. This is what makes retry safe without a journal:
    # there is nothing to reconcile, only something to observe.
    state, detail = promotion_state(candidate)
    if state == STATE_PROMOTED:
        sys.stdout.write(
            f"already promoted: {detail}\n"
            f"no changes made — the candidate ledger keeps its entry by design.\n"
        )
        return 0

    validate(candidate, target_filename, allow_dangling=args.allow_dangling)
    sys.stdout.write("validation: OK\n")

    new_entry = to_canonical_entry(candidate)
    sys.stdout.write(f"canonical id: {new_entry['id']}\n")
    relations = canonicalize_relations(candidate)
    if relations:
        rendered = "; ".join(f"{k}={v}" for k, v in relations.items())
        sys.stdout.write(f"relations canonicalized: {rendered}\n")

    # Promotion writes exactly ONE file: the canonical target. The candidate
    # ledger is append-only and is never touched — its entries are a
    # permanent record of what was discovered, and promotion state is
    # DERIVED from canonical identity presence. Re-running is therefore a
    # no-op rather than a conflict, and there is no second write to
    # synchronize, roll back, or journal.
    list_key = TARGET_LIST_KEY[target_filename]
    canonical_before = target_path.read_text(encoding="utf-8")
    existing_entries = (load_yaml(target_path) or {}).get(list_key) or []

    canonical_after = canonical_text_with_entry(
        canonical_before, list_key, new_entry, target_path
    )
    verify_canonical_text(
        canonical_after, list_key, new_entry["id"], len(existing_entries) + 1, target_path
    )

    if args.dry_run:
        sys.stdout.write(
            f"[dry-run] would append to {target_path.relative_to(REPO_ROOT)} "
            f"({len(existing_entries)} → {len(existing_entries) + 1} entries):\n"
        )
        sys.stdout.write(_render_entry(new_entry, ""))
        sys.stdout.write(
            f"[dry-run] {candidate_path.relative_to(REPO_ROOT)} is NOT modified "
            f"(append-only ledger)\n"
        )
        return 0

    atomic_write(target_path, canonical_after)
    sys.stdout.write(
        f"appended to {target_path.relative_to(REPO_ROOT)} "
        f"({len(existing_entries) + 1} entries)\n"
        f"{candidate_path.relative_to(REPO_ROOT)} unchanged (append-only ledger); "
        f"{args.id} now derives as PROMOTED\n"
    )
    sys.stdout.write(
        "\nnext step: cd ../awareness-graph && "
        "SERVICES_REPO=../services scripts/build-awareness-graph.sh\n"
        "(regenerates awareness.nt with the new canonical entry)\n"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
