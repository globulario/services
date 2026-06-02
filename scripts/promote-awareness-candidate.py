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

Validation
----------
- ID must match the canonical naming convention:
    <namespace>.<bare_id>   where each segment is [a-z0-9._]+
- ID must NOT already exist in any canonical YAML file (no duplicates).
- Candidate must have status:candidate and confidence != "low".
- Class must match the target file's class:
    invariants.yaml      → class: invariant
    failure_modes.yaml   → class: failure_mode
    intents.yaml         → class: intent
    incident_patterns.yaml → class: incident_pattern

Usage
-----
  scripts/promote-awareness-candidate.py \\
    --id remediation.test_audit_writes_must_be_isolated_from_production_etcd \\
    --target docs/awareness/invariants.yaml

  # Dry-run (validate but don't write):
  scripts/promote-awareness-candidate.py --id <id> --target <file> --dry-run

Exit codes
----------
  0 — promotion succeeded (or dry-run succeeded)
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


def die(msg: str, code: int = 1) -> None:
    sys.stderr.write(f"promote-awareness-candidate: {msg}\n")
    sys.exit(code)


def load_yaml(path: Path) -> Any:
    with path.open("r", encoding="utf-8") as f:
        return yaml.safe_load(f) or {}


def find_candidate(candidate_id: str) -> tuple[Path, dict]:
    """Search every YAML in candidates/ for the given id. Returns
    (file_path, entry_dict). Dies if not found or found multiple times."""
    matches: list[tuple[Path, dict]] = []
    if not CANDIDATES_DIR.is_dir():
        die(f"candidates dir missing: {CANDIDATES_DIR}")
    for yaml_path in sorted(CANDIDATES_DIR.rglob("*.yaml")):
        data = load_yaml(yaml_path)
        if not isinstance(data, dict):
            continue
        candidates = data.get("candidates") or []
        if not isinstance(candidates, list):
            continue
        for entry in candidates:
            if isinstance(entry, dict) and entry.get("id") == candidate_id:
                matches.append((yaml_path, entry))
    if not matches:
        die(f"candidate id not found in any docs/awareness/candidates/*.yaml: {candidate_id!r}")
    if len(matches) > 1:
        die(
            f"candidate id {candidate_id!r} found in multiple files; ambiguous: "
            + ", ".join(str(p.relative_to(REPO_ROOT)) for p, _ in matches)
        )
    return matches[0]


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
        for list_key in ("invariants", "failure_modes", "intents", "incident_patterns"):
            entries = data.get(list_key) or []
            if not isinstance(entries, list):
                continue
            for entry in entries:
                if isinstance(entry, dict) and "id" in entry:
                    ids.add(entry["id"])
    return ids


def validate(candidate: dict, target_filename: str) -> None:
    """All the checks. Dies on failure."""
    cid = candidate.get("id")
    if not isinstance(cid, str) or not ID_PATTERN.match(cid):
        die(
            f"id {cid!r} does not match canonical naming: <namespace>.<bare_id> "
            f"(segments: [a-z0-9_]+, joined by dots)"
        )

    expected_class = TARGET_CLASS.get(target_filename)
    if expected_class is None:
        die(
            f"target {target_filename!r} is not a recognized canonical file. "
            f"Supported: {sorted(TARGET_CLASS.keys())}",
            code=2,
        )
    if candidate.get("class") != expected_class:
        die(
            f"class mismatch: candidate.class={candidate.get('class')!r} but "
            f"target {target_filename!r} expects class={expected_class!r}"
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

    if not candidate.get("evidence"):
        die("refusing to promote: candidate.evidence is empty. Reviewers need evidence.")

    if not candidate.get("discovered_from"):
        die("refusing to promote: candidate.discovered_from is empty. Provenance is required.")

    existing = all_canonical_ids()
    if cid in existing:
        die(f"duplicate id: {cid!r} already exists in canonical YAML")


def to_canonical_entry(candidate: dict) -> dict:
    """Strip candidate-only fields and record provenance."""
    # The canonical entry uses these fields (per existing yaml shape):
    #   id, title (← label), severity (← risk), status:active, summary,
    #   protects, enforcement, ...
    # We carry across what's clearly applicable; reviewers can polish
    # the resulting YAML before commit.
    entry: dict = {
        "id": candidate["id"],
        "title": candidate.get("label", "").strip(),
        "severity": candidate.get("risk", "medium"),
        "status": "active",
    }
    if "summary" in candidate:
        entry["summary"] = candidate["summary"]
    if "protects" in candidate:
        entry["protects"] = candidate["protects"]
    # Provenance — operators and future agents need to know where this
    # entry came from.
    entry["provenance"] = {
        "promoted_from": "candidate",
        "discovered_from": candidate.get("discovered_from", "").strip(),
        "confidence_at_promotion": candidate.get("confidence", "medium"),
    }
    return entry


def write_canonical(target_path: Path, list_key: str, new_entry: dict, dry_run: bool) -> None:
    data = load_yaml(target_path)
    if not isinstance(data, dict):
        data = {}
    entries = data.get(list_key) or []
    if not isinstance(entries, list):
        die(f"target {target_path} has {list_key} but it's not a list")
    entries.append(new_entry)
    entries.sort(key=lambda e: e.get("id", "") if isinstance(e, dict) else "")
    data[list_key] = entries
    if dry_run:
        sys.stdout.write(f"[dry-run] would append to {target_path.relative_to(REPO_ROOT)}:\n")
        sys.stdout.write(yaml.safe_dump({list_key: [new_entry]}, sort_keys=False, allow_unicode=True))
        return
    with target_path.open("w", encoding="utf-8") as f:
        yaml.safe_dump(data, f, sort_keys=False, allow_unicode=True)
    sys.stdout.write(f"appended to {target_path.relative_to(REPO_ROOT)}\n")


def remove_from_candidate_file(candidate_path: Path, candidate_id: str, dry_run: bool) -> None:
    data = load_yaml(candidate_path)
    if not isinstance(data, dict):
        return
    candidates = data.get("candidates") or []
    remaining = [c for c in candidates if not (isinstance(c, dict) and c.get("id") == candidate_id)]
    data["candidates"] = remaining
    if dry_run:
        sys.stdout.write(
            f"[dry-run] would remove {candidate_id} from {candidate_path.relative_to(REPO_ROOT)} "
            f"(remaining: {len(remaining)})\n"
        )
        return
    with candidate_path.open("w", encoding="utf-8") as f:
        yaml.safe_dump(data, f, sort_keys=False, allow_unicode=True)
    sys.stdout.write(
        f"removed {candidate_id} from {candidate_path.relative_to(REPO_ROOT)} "
        f"(remaining candidates: {len(remaining)})\n"
    )


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Promote a session-discovered awareness candidate into canonical YAML."
    )
    parser.add_argument("--id", required=True, help="The candidate id to promote")
    parser.add_argument(
        "--target",
        required=True,
        help="Path to the target canonical YAML "
        "(e.g. docs/awareness/invariants.yaml)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Validate and print the resulting entry; do NOT modify files",
    )
    args = parser.parse_args()

    target_path = REPO_ROOT / args.target if not os.path.isabs(args.target) else Path(args.target)
    if not target_path.exists():
        die(f"target file not found: {target_path}", code=2)
    target_filename = target_path.name

    candidate_path, candidate = find_candidate(args.id)
    sys.stdout.write(f"candidate found: {candidate_path.relative_to(REPO_ROOT)}\n")

    validate(candidate, target_filename)
    sys.stdout.write("validation: OK\n")

    new_entry = to_canonical_entry(candidate)
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
