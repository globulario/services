#!/usr/bin/env python3
"""
Promote a session-discovered awareness candidate into canonical YAML.

Workflow
--------
A candidate sits in docs/awareness/candidates/<file>.yaml with
status:candidate. This script moves a single candidate (by id) into one
of the canonical knowledge files (invariants.yaml / failure_modes.yaml /
intents.yaml), strips the candidate-only fields, records provenance,
and removes the entry from the candidate file.

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
- ID must NOT already exist in any canonical YAML file (no duplicates).
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

def all_canonical_ids() -> set[str]:
    """Walk docs/awareness/*.yaml (NOT candidates/, NOT subdirs) and
    collect every existing id across invariants/failure_modes/intents/
    incident_patterns lists."""
    ids: set[str] = set()
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
                    ids.add(entry["id"])
    return ids


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

    existing = all_canonical_ids()
    if cid in existing:
        die(f"duplicate id: {cid!r} already exists in canonical YAML")

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


def candidate_item_span(text: str, candidate_id: str, path: Path) -> tuple[int, int, str]:
    """Character span of the candidate's list item, plus the bullet indent.

    Located STRUCTURALLY from the parsed node tree, not by regex. The
    previous line-based matcher assumed `id` was the item's first key and
    that its line carried no inline comment; a candidate can legally
    violate either, and the failure mode was a half-done promotion —
    canonical entry appended, candidate still present, retry blocked by
    duplicate detection."""
    root = _compose(text, path)
    seq = _find_sequence_node(root, "candidates") if root is not None else None
    if seq is None:
        die(f"{path}: no candidates sequence found for structural removal")

    for i, item in enumerate(seq.value):
        if _mapping_id(item) != candidate_id:
            continue
        start = _line_start(text, item.start_mark.index)
        indent = " " * max(item.start_mark.column - 2, 0)
        if i + 1 < len(seq.value):
            end = _line_start(text, seq.value[i + 1].start_mark.index)
        else:
            end = _item_end(text, item)
        return start, end, indent
    die(f"could not locate entry {candidate_id!r} in {path} for removal")
    return 0, 0, ""  # unreachable; die() exits


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


def candidate_text_without(text: str, candidate_id: str, path: Path) -> str:
    """The candidate file's full new content, computed in memory."""
    start, end, _indent = candidate_item_span(text, candidate_id, path)
    lines = (text[:start] + text[end:]).splitlines(True)
    _collapse_emptied_list(lines, len(text[:start].splitlines()))
    return "".join(lines)


def _collapse_emptied_list(lines: list[str], removed_at: int) -> None:
    """When the removed entry was the last one, turn the now-childless
    `candidates:` line into `candidates: []`.

    A bare `candidates:` key parses as null, not as an empty list, which
    reads as "this document has no candidates list" rather than "this
    list is empty"."""
    for i in range(min(removed_at, len(lines)) - 1, -1, -1):
        line = lines[i]
        if line.strip() != "candidates:":
            continue
        indent = line[: len(line) - len(line.lstrip())]
        for follow in lines[i + 1:]:
            if not follow.strip():
                continue
            follow_indent = follow[: len(follow) - len(follow.lstrip())]
            if follow.lstrip().startswith("- ") and len(follow_indent) > len(indent):
                return  # still has items
            break
        lines[i] = f"{indent}candidates: []\n"
        return


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


def verify_candidate_text(text: str, candidate_id: str, expected: int, path: Path) -> int:
    """Parse the proposed content and assert the candidate is gone."""
    try:
        data = yaml.safe_load(text) or {}
    except yaml.YAMLError as exc:
        die(f"proposed content for {path} is not valid YAML: {exc}")
    lists = extract_candidate_lists(data, path)
    remaining = [e for _w, entries in lists for e in entries if isinstance(e, dict)]
    if any(e.get("id") == candidate_id for e in remaining):
        die(f"proposed content for {path} still contains {candidate_id!r}")
    if len(remaining) != expected:
        die(
            f"proposed content for {path} has {len(remaining)} candidates, "
            f"expected {expected}"
        )
    return len(remaining)


def atomic_replace(updates: list[tuple[Path, str]]) -> None:
    """Replace several files as close to atomically as a filesystem allows.

    Promotion mutates TWO files. Writing one and then discovering the other
    cannot be written leaves a half-done promotion: canonical entry
    appended, candidate still present, and the retry blocked by duplicate
    detection. Post-write verification cannot help — the damage is already
    on disk. So every proposed content is validated first, staged to a temp
    file beside its target, and only then swapped in; if any swap fails the
    ones already done are rolled back."""
    backups = {path: path.read_text(encoding="utf-8") for path, _ in updates}
    staged: list[tuple[Path, Path]] = []
    try:
        for path, text in updates:
            tmp = path.with_name(path.name + ".promote-tmp")
            tmp.write_text(text, encoding="utf-8")
            staged.append((path, tmp))
    except OSError as exc:
        for _p, tmp in staged:
            tmp.unlink(missing_ok=True)
        die(f"could not stage promotion writes: {exc}")

    replaced: list[Path] = []
    try:
        for path, tmp in staged:
            os.replace(tmp, path)
            replaced.append(path)
    except OSError as exc:
        for path in replaced:
            try:
                path.write_text(backups[path], encoding="utf-8")
            except OSError as rollback_exc:  # pragma: no cover - disk failure
                sys.stderr.write(
                    f"promote-awareness-candidate: CRITICAL: rollback of {path} failed: "
                    f"{rollback_exc}. Restore from git.\n"
                )
        for _p, tmp in staged:
            tmp.unlink(missing_ok=True)
        die(f"could not complete promotion writes (rolled back): {exc}")


# ─── single-file helpers (kept for direct callers and tests) ─────────────

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
    atomic_replace([(target_path, proposed)])
    sys.stdout.write(
        f"appended to {target_path.relative_to(REPO_ROOT)} ({len(entries) + 1} entries)\n"
    )


def remove_from_candidate_file(candidate_path: Path, candidate_id: str, dry_run: bool) -> None:
    text = candidate_path.read_text(encoding="utf-8")
    before = sum(
        len(entries)
        for _w, entries in extract_candidate_lists(yaml.safe_load(text) or {}, candidate_path)
    )
    proposed = candidate_text_without(text, candidate_id, candidate_path)
    remaining = verify_candidate_text(proposed, candidate_id, before - 1, candidate_path)

    if dry_run:
        sys.stdout.write(
            f"[dry-run] would remove {candidate_id} from "
            f"{candidate_path.relative_to(REPO_ROOT)} (remaining: {remaining})\n"
        )
        return
    atomic_replace([(candidate_path, proposed)])
    sys.stdout.write(
        f"removed {candidate_id} from {candidate_path.relative_to(REPO_ROOT)} "
        f"(remaining candidates: {remaining})\n"
    )


# ─── listing ─────────────────────────────────────────────────────────────

def list_candidates() -> int:
    """Print every candidate and whether it is promotable."""
    rows = iter_all_candidates()
    if not rows:
        sys.stdout.write("no candidates found under docs/awareness/candidates/\n")
        return 0

    canonical = all_canonical_ids()
    sibling_ids = {canonical_id_for(e) for _p, _w, e in rows if canonical_id_for(e)}

    sys.stdout.write(f"{len(rows)} candidate(s):\n\n")
    for path, wrapper, entry in rows:
        cid = entry.get("id", "<no id>")
        canon = canonical_id_for(entry)
        cls = normalize_class(entry.get("class"))
        target = next((f for f, c in TARGET_CLASS.items() if c == cls), "?")
        blockers: list[str] = []
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
        if canon in canonical:
            blockers.append("duplicate")
        _pending, dangling = check_relationships(entry, canonical, sibling_ids - {canon})
        if dangling:
            blockers.append(f"dangling-refs={len(dangling)}")

        state = "PROMOTABLE" if not blockers else "BLOCKED: " + ", ".join(blockers)
        sys.stdout.write(f"  {cid}\n")
        sys.stdout.write(f"      file:      {path.relative_to(REPO_ROOT)}")
        sys.stdout.write(f" (wrapper: {wrapper})\n" if wrapper else " (direct)\n")
        sys.stdout.write(f"      class:     {entry.get('class')!r} → {cls or '?'}\n")
        sys.stdout.write(f"      promotes:  {canon}  →  docs/awareness/{target}\n")
        sys.stdout.write(f"      severity:  {entry.get('severity') or entry.get('risk') or '?'}"
                         f"   confidence: {entry.get('confidence') or '?'}\n")
        sys.stdout.write(f"      state:     {state}\n\n")
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

    validate(candidate, target_filename, allow_dangling=args.allow_dangling)
    sys.stdout.write("validation: OK\n")

    new_entry = to_canonical_entry(candidate)
    sys.stdout.write(f"canonical id: {new_entry['id']}\n")
    relations = canonicalize_relations(candidate)
    if relations:
        rendered = "; ".join(f"{k}={v}" for k, v in relations.items())
        sys.stdout.write(f"relations canonicalized: {rendered}\n")

    # Promotion mutates TWO files. Build BOTH results in memory and validate
    # BOTH before anything reaches disk, then swap them in together — a
    # half-done promotion (entry appended, candidate still present) blocks its
    # own retry on duplicate detection, and no post-write check can undo it.
    list_key = TARGET_LIST_KEY[target_filename]
    canonical_before = target_path.read_text(encoding="utf-8")
    candidate_before = candidate_path.read_text(encoding="utf-8")

    existing_entries = (load_yaml(target_path) or {}).get(list_key) or []
    candidate_count = sum(
        len(entries)
        for _w, entries in extract_candidate_lists(
            yaml.safe_load(candidate_before) or {}, candidate_path
        )
    )

    canonical_after = canonical_text_with_entry(
        canonical_before, list_key, new_entry, target_path
    )
    verify_canonical_text(
        canonical_after, list_key, new_entry["id"], len(existing_entries) + 1, target_path
    )
    candidate_after = candidate_text_without(candidate_before, args.id, candidate_path)
    remaining = verify_candidate_text(
        candidate_after, args.id, candidate_count - 1, candidate_path
    )

    if args.dry_run:
        sys.stdout.write(
            f"[dry-run] would append to {target_path.relative_to(REPO_ROOT)} "
            f"({len(existing_entries)} → {len(existing_entries) + 1} entries):\n"
        )
        sys.stdout.write(_render_entry(new_entry, ""))
        sys.stdout.write(
            f"[dry-run] would remove {args.id} from "
            f"{candidate_path.relative_to(REPO_ROOT)} (remaining: {remaining})\n"
        )
        return 0

    atomic_replace([(target_path, canonical_after), (candidate_path, candidate_after)])
    sys.stdout.write(
        f"appended to {target_path.relative_to(REPO_ROOT)} "
        f"({len(existing_entries) + 1} entries)\n"
        f"removed {args.id} from {candidate_path.relative_to(REPO_ROOT)} "
        f"(remaining candidates: {remaining})\n"
    )

    if not args.dry_run:
        sys.stdout.write(
            "\nnext step: cd ../awareness-graph && "
            "SERVICES_REPO=../services scripts/build-awareness-graph.sh\n"
            "(regenerates awareness.nt with the new canonical entry)\n"
        )
    return 0


if __name__ == "__main__":
    sys.exit(main())
