#!/usr/bin/env python3
"""Generate the `globular-operator` Claude skill from docs/operational-knowledge/.

The skill is a LOOP + ROUTER over the operational-knowledge corpus. The 58 corpus
files stay in place as on-demand references — this generator never copies their
content, only their paths + extracted titles. Re-run whenever the corpus changes.

    python3 scripts/gen-operator-skill.py           # (re)write the skill
    python3 scripts/gen-operator-skill.py --check    # CI: exit 1 if the committed skill is stale

Design (why this can't drift):
  Tier 1 — a CURATED topical quick-router (semantic sections + paired MCP tools).
           Curated file paths are validated to exist; a missing one is a hard error.
  Tier 2 — a FULL INDEX auto-enumerated from disk, grouped by the corpus's own
           taxonomy (guides / stages / runbooks / service-roles / incidents).
           Every corpus file appears here by construction, so a newly added file
           is never silently dropped and a removed file can never dangle.
"""

import argparse
import os
import re
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CORPUS_REL = "docs/operational-knowledge"
SKILL_REL = ".claude/skills/globular-operator/SKILL.md"
# Corpus meta files that are not routable knowledge entries.
EXCLUDE = {"README.md", "SCHEMA.md"}

FRONTMATTER_DESCRIPTION = (
    "Operate and diagnose a running Globular cluster — health/doctor findings, node "
    "join/remove, package deploy via repository, DNS/ACME, PKI rotation, MinIO/object-store "
    "topology, etcd/keepalived recovery, day-0/1/2 runbooks, and incident triage. Use when the "
    "task is RUNNING the cluster (observe → diagnose → act → verify), not editing service "
    "source (that's sensei-architect). Routes to the operational-knowledge corpus and the matching "
    "typed MCP tools; pulls only the relevant slice on demand."
)

PREAMBLE = """# Globular Operator

Act as the cluster's operator: observe first, act through typed APIs, verify, and leave an audit trail.

This skill does **not** duplicate the operational-knowledge corpus — it is a **loop + a router**. The facts
live in `docs/operational-knowledge/` (the signed day-0 seed that also feeds ai-memory). This skill tells you
*which slice to Read for the task in front of you* and *which typed tool pairs with it*, so you load depth
only when it's relevant.

Scope boundary: **operating** the cluster (this skill) vs **changing service code** (`sensei-architect`). If the
task is a code edit in a high-risk dir, stop and use sensei-architect instead.

## The operate loop (always)

`OBSERVE → DIAGNOSE → RECOMMEND → [APPROVE] → EXECUTE → VERIFY`

1. **Observe before acting.** Never prescribe before you diagnose. Pull live state with the MCP tools, not from memory.
2. **Walk the 4 layers, in order.** Repository → Desired → Installed → Runtime. Never assume Desired == Installed == Running. Most "mystery" findings are a layer disagreement.
3. **Never invent state.** Reason only from observable, verifiable evidence. A recalled fact (or a seed entry) is a hypothesis until a live tool confirms it. Absence of evidence is not evidence of failure — check the snapshot's completeness markers.
4. **Typed actions only.** Mutate through typed gRPC/MCP calls and workflows, never ad-hoc shell against production. If there is no typed API for an action, it should not be done by an agent.
5. **Three-tier permissions.** Tier 0 (observe) is always safe. Tier 1 (restart, clear cache) is pre-approved. Tier 2 (destructive: wipe, remove-node, topology change, PKI rollback) **requires explicit human approval** — surface the exact command and wait.
6. **Audit + fail-safe.** Every action leaves a durable record. If AI services are down, the cluster converges deterministically without you — AI is supplementary, never required.

Enforcement note: the non-negotiable HARD RULES (etcd is sole authority, no localhost for remote, mTLS everywhere,
no secrets in etcd/source, security boundaries) live in **CLAUDE.md** and are hook-enforced. This skill assumes
them; it does not restate them.
"""

CLOSING = """## After acting — close the loop

- Verify against live tools that the change took effect at the layer you targeted (don't declare success from a green summary alone).
- If the session produced a durable operational lesson (a new failure mode, a runbook gap, a corrected assumption), record it — write to **ai-memory** (`memory_store`, project `globular-services`), not flat files. The `docs/operational-knowledge/` corpus is the *seed* and is release-managed; runtime lessons go to ai-memory and are promoted to the corpus only through the build pipeline.
- Never use multi-word `text_search` on ai-memory (known bug — returns 0); query by `tags=`/`type=` or `memory_get` by id.

## Non-negotiables

- Observe before you prescribe; never fabricate state from memory.
- Tier-2 destructive actions need explicit human approval — present the exact command, wait.
- Typed actions only; no ad-hoc shell mutations against production.
- This skill is know-how, not enforcement — the HARD RULES in CLAUDE.md still bind, and this skill never overrides them.
"""

# Tier-1 curated topical quick-router. `files` are corpus-relative paths (validated
# to exist). Files may appear in more than one topic. A new corpus file that fits no
# topic is not an error — it still lands in the Tier-2 full index and is reported.
TOPICS = [
    {
        "heading": "Diagnosis & health",
        "tools": ["cluster_get_doctor_report", "cluster_explain_finding", "cluster_get_health", "infra_probe_all", "infra_explain_stall"],
        "files": [
            "service-roles/cluster-doctor.yaml",
            "runbooks/fix-doctor-error-findings.yaml",
            "runbooks/doctor-minio-data-incomplete-false-positives.yaml",
            "stages/day-2-maintenance.yaml",
        ],
    },
    {
        "heading": "Node lifecycle (join / remove / rejoin)",
        "tools": ["cluster_list_nodes", "cluster_remove_node", "cluster_get_node_full_status"],
        "files": [
            "stages/day-1-join.yaml",
            "node-removal.md",
            "incidents/node-rejoin-scylla-group0-repository-2026-07-09.md",
        ],
    },
    {
        "heading": "Packages & releases",
        "tools": ["package_build", "package_publish", "repository_active_release", "repository_verify_artifact", "cluster_get_convergence_detail"],
        "files": [
            "deploy-package-via-mcp.md",
            "packages.md",
            "stages/package-system.yaml",
            "stages/installed-artifact-system.yaml",
            "stages/day-1-deploy-pipeline.yaml",
            "runbooks/deploy-controller-fix-via-repository.yaml",
            "stages/service-version-management.yaml",
            "stages/profile-system.yaml",
        ],
    },
    {
        "heading": "Storage / MinIO / disk",
        "tools": ["backup_list_minio_buckets"],
        "files": [
            "runbooks/add-node-to-minio-pool.yaml",
            "runbooks/safe-minio-topology-restart.yaml",
            "runbooks/recover-stuck-topology-apply.yaml",
            "runbooks/minio-stale-topology-proposal.yaml",
            "runbooks/republish-artifact-data-after-pool-change.yaml",
            "runbooks/repartition-shared-disk.yaml",
            "runbooks/clean-ghost-loop-devices.yaml",
            "stages/day-1-objectstore.yaml",
        ],
    },
    {
        "heading": "etcd / ingress / DNS / networking",
        "tools": [],
        "files": [
            "runbooks/recover-etcd-member-eviction.yaml",
            "runbooks/recover-keepalived-vip-loss.yaml",
            "stages/day-1-keepalived.yaml",
            "service-roles/ingress.yaml",
            "dns-records.md",
            "service-roles/dns.yaml",
            "runbooks/setup-public-domain-acme.yaml",
            "stages/day-1-public-domain.yaml",
            "runbooks/configure-google-workspace-mail.yaml",
        ],
    },
    {
        "heading": "Security / PKI",
        "tools": [],
        "files": [
            "runbooks/rotate-pki-certificates.yaml",
            "stages/security-system.yaml",
            "incidents/scylla-manager-tls-trust-2026-06-03.md",
        ],
    },
    {
        "heading": "Bootstrap / upgrade / backup",
        "tools": ["backup_restore_plan", "backup_preflight_check", "backup_get_recovery_posture"],
        "files": [
            "runbooks/recover-day-0-bootstrap-failure.yaml",
            "stages/day-0-bootstrap.yaml",
            "runbooks/recover-failed-platform-upgrade.yaml",
            "runbooks/restore-from-backup.yaml",
        ],
    },
    {
        "heading": "Awareness graph (AWG / sensei) operations",
        "tools": [],
        "files": [
            "awg-operator-guide.md",
            "service-roles/awareness-graph.yaml",
            "stages/awareness-graph-operations.yaml",
            "runbooks/initialize-awareness-graph.yaml",
        ],
    },
    {
        "heading": "Prior scars & learned patterns (read before repeating diagnosis)",
        "tools": [],
        "files": [
            "incidents/cluster-zero-findings-2026-06-03.md",
            "incidents/sidecar-receipt-retirement-2026-06-03.md",
            "incidents/scylla-manager-tls-trust-2026-06-03.md",
            "incidents/node-rejoin-scylla-group0-repository-2026-07-09.md",
            "stages/learned-2026-06.yaml",
        ],
    },
]

# Tier-2 full-index groups, in emit order: (heading, predicate on corpus-relative path).
INDEX_GROUPS = [
    ("Canonical guides", lambda p: "/" not in p),
    ("Lifecycle stages (`stages/`)", lambda p: p.startswith("stages/")),
    ("Runbooks (`runbooks/`)", lambda p: p.startswith("runbooks/")),
    ("Service roles (`service-roles/`)", lambda p: p.startswith("service-roles/")),
    ("Incidents & learned patterns (`incidents/`)", lambda p: p.startswith("incidents/")),
]


def list_corpus():
    """Return sorted corpus-relative paths of routable files."""
    root = os.path.join(REPO_ROOT, CORPUS_REL)
    out = []
    for dirpath, _dirs, files in os.walk(root):
        for name in files:
            if not (name.endswith(".yaml") or name.endswith(".md")):
                continue
            if name in EXCLUDE:
                continue
            rel = os.path.relpath(os.path.join(dirpath, name), root)
            out.append(rel)
    return sorted(out)


def _read_lines(rel):
    with open(os.path.join(REPO_ROOT, CORPUS_REL, rel), encoding="utf-8") as fh:
        return fh.read().splitlines()


def extract_title(rel):
    """A human title: metadata.title for YAML, the first `# ` heading for MD."""
    lines = _read_lines(rel)
    if rel.endswith(".md"):
        for ln in lines:
            m = re.match(r"^#\s+(.*)", ln)
            if m:
                return m.group(1).strip()
        return os.path.basename(rel)
    # YAML: first `  title:` (metadata.title is the first title in these files).
    for ln in lines:
        m = re.match(r'^\s+title:\s*"?(.*?)"?\s*$', ln)
        if m and m.group(1):
            return m.group(1).strip()
    return os.path.basename(rel)


def extract_summary(rel, limit=140):
    """One short line: first content line of the description / first prose line."""
    lines = _read_lines(rel)
    if rel.endswith(".md"):
        seen_heading = False
        for ln in lines:
            if re.match(r"^#\s+", ln):
                seen_heading = True
                continue
            if seen_heading and ln.strip() and not ln.startswith("#"):
                return _clip(re.sub(r"[*_`]", "", ln).strip(), limit)
        return ""
    # YAML: inline `  description: "..."` or block `  description: |` + first indented line.
    for i, ln in enumerate(lines):
        m = re.match(r'^\s+description:\s*(.*)$', ln)
        if not m:
            continue
        inline = m.group(1).strip().strip('"').strip("|").strip(">").strip()
        if inline:
            return _clip(inline, limit)
        for nxt in lines[i + 1:]:
            if nxt.strip():
                return _clip(nxt.strip(), limit)
            break
        break
    return ""


def _clip(s, limit):
    s = s.rstrip(".")
    return s if len(s) <= limit else s[: limit - 1].rstrip() + "…"


def entry_line(rel):
    title = extract_title(rel)
    summary = extract_summary(rel)
    path = f"{CORPUS_REL}/{rel}"
    if summary:
        return f"- **{title}** — {summary} · `{path}`"
    return f"- **{title}** · `{path}`"


def render(corpus):
    covered = set()
    parts = []
    # Frontmatter
    parts.append("---")
    parts.append("name: globular-operator")
    parts.append(f"description: {FRONTMATTER_DESCRIPTION}")
    parts.append("---")
    parts.append("")
    # Generated banner (not the source of truth)
    parts.append("<!-- GENERATED by scripts/gen-operator-skill.py from docs/operational-knowledge/ — do not edit by hand; re-run the generator. -->")
    parts.append("")
    parts.append(PREAMBLE.rstrip())
    parts.append("")
    # Tier 1 — topical quick-router
    parts.append("## Router — topical quick-reference")
    parts.append("")
    parts.append("Read paths on demand; do not preload the corpus. Tools are the typed MCP calls that pair with each topic.")
    parts.append("")
    for topic in TOPICS:
        for f in topic["files"]:
            abspath = os.path.join(REPO_ROOT, CORPUS_REL, f)
            if not os.path.exists(abspath):
                raise SystemExit(f"gen-operator-skill: curated topic '{topic['heading']}' references missing file: {f}")
            covered.add(f)
        parts.append(f"### {topic['heading']}")
        if topic["tools"]:
            parts.append("tools: " + ", ".join(f"`{t}`" for t in topic["tools"]))
            parts.append("")
        for f in topic["files"]:
            parts.append(entry_line(f))
        parts.append("")
    # Tier 2 — full index (guarantees 100% coverage)
    parts.append("## Full index — every corpus entry")
    parts.append("")
    parts.append("Auto-enumerated from `docs/operational-knowledge/`; complete by construction.")
    parts.append("")
    indexed = set()
    for heading, pred in INDEX_GROUPS:
        group = [p for p in corpus if pred(p) and p not in indexed]
        if not group:
            continue
        parts.append(f"### {heading}")
        for p in sorted(group):
            parts.append(entry_line(p))
            indexed.add(p)
        parts.append("")
    # Any file matching no index group (shouldn't happen given the catch-all guides pred)
    leftover = [p for p in corpus if p not in indexed]
    if leftover:
        parts.append("### Uncategorized")
        for p in sorted(leftover):
            parts.append(entry_line(p))
        parts.append("")
    parts.append(CLOSING.rstrip())
    parts.append("")
    text = "\n".join(parts)
    untopiced = sorted(set(corpus) - covered)
    return text, untopiced


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--check", action="store_true", help="exit 1 if the committed skill differs from a fresh render")
    args = ap.parse_args()

    corpus = list_corpus()
    text, untopiced = render(corpus)
    skill_abs = os.path.join(REPO_ROOT, SKILL_REL)

    if args.check:
        existing = ""
        if os.path.exists(skill_abs):
            with open(skill_abs, encoding="utf-8") as fh:
                existing = fh.read()
        if existing != text:
            print("gen-operator-skill: STALE — re-run `python3 scripts/gen-operator-skill.py`", file=sys.stderr)
            return 1
        print(f"gen-operator-skill: up to date ({len(corpus)} corpus files)")
        return 0

    os.makedirs(os.path.dirname(skill_abs), exist_ok=True)
    with open(skill_abs, "w", encoding="utf-8") as fh:
        fh.write(text)
    print(f"gen-operator-skill: wrote {SKILL_REL} ({len(corpus)} corpus files, "
          f"{len(corpus) - len(untopiced)} in topical router)")
    if untopiced:
        print("  note: not in any topical section (still in the full index):")
        for p in untopiced:
            print(f"    - {p}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
