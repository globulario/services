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
) + RELATION_FIELDS


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

def _render_entry(entry: dict, indent: str) -> str:
    """Render one entry as a block-list item, indented to match the file."""
    text = yaml.safe_dump(
        [entry], sort_keys=False, allow_unicode=True, width=100, default_flow_style=False
    )
    if not indent:
        return text
    return "".join(indent + line if line.strip() else line for line in text.splitlines(True))


def _block_list_indent(text: str, list_key: str) -> str | None:
    """Indent of the block-list items under list_key, or None when the list
    is absent, empty, or written in flow style ("key: []")."""
    lines = text.splitlines()
    for i, line in enumerate(lines):
        if not line.startswith(list_key + ":"):
            continue
        if line[len(list_key) + 1:].strip():
            return None  # inline value, e.g. "invariants: []"
        for follow in lines[i + 1:]:
            if not follow.strip():
                continue
            stripped = follow.lstrip()
            if stripped.startswith("- "):
                return follow[: len(follow) - len(stripped)]
            return None
        return None
    return None


def write_canonical(target_path: Path, list_key: str, new_entry: dict, dry_run: bool) -> None:
    """Append one entry to the canonical file.

    Appends TEXTUALLY when the target holds a block list. A full
    yaml.safe_dump round-trip of docs/awareness/invariants.yaml rewrites
    529KB, reflows every folded scalar and deletes every comment — a diff
    no reviewer can read. Only the empty/flow-style case (nothing to
    preserve) falls back to a structural dump."""
    data = load_yaml(target_path)
    if not isinstance(data, dict):
        data = {}
    entries = data.get(list_key) or []
    if not isinstance(entries, list):
        die(f"target {target_path} has {list_key} but it's not a list")

    text = target_path.read_text(encoding="utf-8")
    indent = _block_list_indent(text, list_key) if entries else None

    if indent is not None:
        rendered = _render_entry(new_entry, indent)
        if dry_run:
            sys.stdout.write(
                f"[dry-run] would append to {target_path.relative_to(REPO_ROOT)} "
                f"(textual append, {len(entries)} → {len(entries) + 1} entries):\n"
            )
            sys.stdout.write(rendered)
            return
        if not text.endswith("\n"):
            text += "\n"
        target_path.write_text(text + rendered, encoding="utf-8")
    else:
        entries = list(entries)
        entries.append(new_entry)
        data[list_key] = entries
        if dry_run:
            sys.stdout.write(f"[dry-run] would append to {target_path.relative_to(REPO_ROOT)}:\n")
            sys.stdout.write(yaml.safe_dump({list_key: [new_entry]}, sort_keys=False, allow_unicode=True))
            return
        with target_path.open("w", encoding="utf-8") as f:
            yaml.safe_dump(data, f, sort_keys=False, allow_unicode=True)

    # Prove the write did what it claimed rather than trusting the append.
    after = load_yaml(target_path)
    got = after.get(list_key) or []
    ids = [e.get("id") for e in got if isinstance(e, dict)]
    if len(got) != len(entries) + (1 if indent is not None else 0) or new_entry["id"] not in ids:
        die(
            f"post-write verification failed for {target_path}: expected "
            f"{new_entry['id']!r} among {len(got)} entries"
        )
    sys.stdout.write(
        f"appended to {target_path.relative_to(REPO_ROOT)} ({len(got)} entries)\n"
    )


def _entry_line_span(lines: list[str], candidate_id: str) -> tuple[int, int] | None:
    """[start, end) line span of the `- id: <candidate_id>` block."""
    pattern = re.compile(r"^(\s*)-\s+id:\s*[\"']?" + re.escape(candidate_id) + r"[\"']?\s*$")
    for i, line in enumerate(lines):
        m = pattern.match(line)
        if not m:
            continue
        indent = m.group(1)
        for j in range(i + 1, len(lines)):
            follow = lines[j]
            if not follow.strip():
                continue
            current = follow[: len(follow) - len(follow.lstrip())]
            # Next sibling item, or a dedent out of the list.
            if follow.lstrip().startswith("- ") and current == indent:
                return i, j
            if len(current) <= len(indent) and not follow.lstrip().startswith("- "):
                return i, j
        return i, len(lines)
    return None


def _collapse_emptied_list(lines: list[str], removed_at: int) -> None:
    """When the removed entry was the last one, turn the now-childless
    `candidates:` line into `candidates: []`.

    A bare `candidates:` key parses as null, not as an empty list, which
    reads as "this document has no candidates list" rather than "this
    list is empty"."""
    for i in range(min(removed_at, len(lines)) - 1, -1, -1):
        line = lines[i]
        stripped = line.strip()
        if stripped != "candidates:":
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


def remove_from_candidate_file(candidate_path: Path, candidate_id: str, dry_run: bool) -> None:
    """Remove the entry TEXTUALLY, preserving the document's shape.

    The previous implementation set data["candidates"] = remaining and
    dumped. Against a wrapped document that ADDED a second, top-level
    `candidates` key while leaving the wrapper intact — producing exactly
    the mixed-shape document the Go parser rejects, and reflowing the
    whole file besides."""
    text = candidate_path.read_text(encoding="utf-8")
    lines = text.splitlines(True)
    span = _entry_line_span(lines, candidate_id)
    if span is None:
        die(f"could not locate entry {candidate_id!r} in {candidate_path} for removal")
    start, end = span

    remaining = lines[:start] + lines[end:]
    _collapse_emptied_list(remaining, start)
    remaining_text = "".join(remaining)
    data_after = yaml.safe_load(remaining_text) or {}
    remaining_count = sum(
        len(entries) for _w, entries in extract_candidate_lists(data_after, candidate_path)
    )

    if dry_run:
        sys.stdout.write(
            f"[dry-run] would remove {candidate_id} from {candidate_path.relative_to(REPO_ROOT)} "
            f"(remaining: {remaining_count})\n"
        )
        return

    candidate_path.write_text(remaining_text, encoding="utf-8")
    still_there = any(
        e.get("id") == candidate_id
        for _w, entries in extract_candidate_lists(yaml.safe_load(remaining_text) or {}, candidate_path)
        for e in entries
        if isinstance(e, dict)
    )
    if still_there:
        die(f"post-removal verification failed: {candidate_id!r} still present in {candidate_path}")
    sys.stdout.write(
        f"removed {candidate_id} from {candidate_path.relative_to(REPO_ROOT)} "
        f"(remaining candidates: {remaining_count})\n"
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
    write_canonical(target_path, TARGET_LIST_KEY[target_filename], new_entry, args.dry_run)
    remove_from_candidate_file(candidate_path, args.id, args.dry_run)

    if not args.dry_run:
        sys.stdout.write(
            "\nnext step: cd ../awareness-graph && "
            "SERVICES_REPO=../services scripts/build-awareness-graph.sh\n"
            "(regenerates awareness.nt with the new canonical entry)\n"
        )
    return 0


if __name__ == "__main__":
    sys.exit(main())
