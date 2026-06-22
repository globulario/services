#!/usr/bin/env bash
# SessionStart hook for globular-services.
#
# Injects, as SessionStart additionalContext, a compact index (id — title) of
# the "always"-tagged operational-knowledge entries from
# docs/operational-knowledge/, plus instructions for using ai-memory as an
# operational-awareness INPUT (not a write-sink).
#
# Why: the corpus is auto-seeded into ai-memory at Day-0/Day-1, but Claude only
# benefits if it KNOWS the entries exist and queries them. This hook makes the
# always-load set visible at session start so it actually reaches the agent.
# See ai-memory feedback entry be59f98e and the text_search-bug entry c7177713.
#
# Reads from disk (no service dependency) so it works even when ai-memory or the
# cluster is down. Degrades gracefully (still emits valid JSON) if PyYAML or the
# corpus is missing.

set -euo pipefail

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
OPS_DIR="${PROJECT_DIR}/docs/operational-knowledge"

python3 - "$OPS_DIR" <<'PY'
import sys, os, json, glob

INSTRUCTIONS = (
    "OPERATIONAL KNOWLEDGE (Day-0 seed) — ai-memory is an INPUT, not just a write-sink.\n"
    "The ops.* corpus below is auto-seeded into ai-memory (project='globular-services'). "
    "Consult it BEFORE diagnosing from raw logs.\n"
    "- Session start: mcp__globular__memory_query(project='globular-services', tags='always').\n"
    "- Before touching a subsystem: query by tag/prefix — e.g. ops.day-1.join, ops.role.rbac, "
    "ops.day-1.minio — and tags='incident' for what the AI tier already recorded.\n"
    "- NEVER use multi-word text_search (it returns 0 — known bug c7177713). "
    "Use tags=, type=, or memory_get by id.\n"
    "- source=seed entries are immutable operational truth; still verify time-sensitive facts against live tools."
)

def emit(ctx):
    print(json.dumps({"hookSpecificOutput": {
        "hookEventName": "SessionStart",
        "additionalContext": ctx,
    }}))

ops_dir = sys.argv[1]
try:
    import yaml
except Exception:
    emit(INSTRUCTIONS + "\n\n(ALWAYS-LOAD INDEX unavailable: PyYAML not installed.)")
    sys.exit(0)

items = []
for f in sorted(glob.glob(os.path.join(ops_dir, "**", "*.yaml"), recursive=True)):
    try:
        d = yaml.safe_load(open(f))
    except Exception:
        continue
    if not isinstance(d, dict):
        continue
    for e in (d.get("entries") or []):
        if isinstance(e, dict) and "always" in (e.get("tags") or []):
            items.append((e.get("id", ""), (e.get("title", "") or "").strip()))

items = sorted(set(items))
if not items:
    emit(INSTRUCTIONS + f"\n\n(ALWAYS-LOAD INDEX empty: no always-tagged entries under {ops_dir}.)")
    sys.exit(0)

index = "\n".join(f"  {i} — {t}" for i, t in items)
emit(INSTRUCTIONS + f"\n\nALWAYS-LOAD ENTRIES ({len(items)}) — query these by id/tag, don't re-derive:\n" + index)
PY
