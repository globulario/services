# Awareness Candidates

This directory holds **session-discovered facts** that have NOT yet been
promoted into the canonical awareness graph. Files here are deliberately
**ignored by the build pipeline** — they exist as a review queue, not
as authoritative knowledge.

## Why a candidate workflow

When an agent (or operator) discovers a new invariant, failure mode,
incident pattern, or intent during a code session, they should NOT
write it directly into `docs/awareness/{invariants,failure_modes,intents}.yaml`.
The canonical YAML drives the awareness-graph build and feeds every
agent's Briefing/Preflight call — a wrong entry there silently shapes
future decisions.

The candidate workflow gives discovered facts a place to land where:

- they are reviewable in a normal PR diff
- they do NOT enter the awareness graph until explicitly promoted
- their provenance (which session, which file, which evidence) is preserved
- two operators can validate the fact before it becomes load-bearing

This closes the loop named by `awareness.preflight`'s honest-DEGRADED
gate (Phase 5): when Preflight told an agent "read source directly and
add candidate annotations after edit", **here** is where those
annotations land.

## File layout

```
docs/awareness/candidates/
├── README.md                              ← this file
├── session_discovered_invariants.yaml     ← example; the default landing file
└── <topic>_candidates.yaml                ← optional per-area candidate files
```

Operators may split candidates across multiple files for organization
(e.g. `healer_candidates.yaml`, `repository_candidates.yaml`), but the
build pipeline treats every `.yaml` under this directory identically:
**ignored unless promoted**.

## Candidate schema

Every entry MUST carry:

```yaml
candidates:
  - id: <namespace>.<bare_id>            # required; same shape as canonical
    class: invariant | failure_mode | incident_pattern | intent
    label: |
      Short prose label, present-tense. One sentence.
    source_file: golang/<path>/<file>.go  # the file that surfaced the fact
    evidence: |
      What did you observe that surfaced this? Cite a commit, a finding,
      or a log line. Reviewers need this to validate.
    risk: low | medium | high | critical
    confidence: low | medium | high
    status: candidate                      # NEVER promote by editing this in place
    discovered_from: |
      Session/commit/incident handle (e.g. "Patch C M1 audit",
      "INC-2026-0017", "session 2026-06-02 17:00 EDT").
    review_required: true
```

Optional but encouraged:

```yaml
    summary: |
      Multi-paragraph context for reviewers — what the rule protects,
      how to apply it, what code would otherwise drift.
    protects:
      files:
        - golang/<path>/<file>.go
      symbols:
        - SomeFunction
```

## Promotion path

When a candidate has been reviewed and validated, promote it:

```bash
scripts/promote-awareness-candidate.py \
  --id <namespace>.<bare_id> \
  --target docs/awareness/invariants.yaml     # or failure_modes.yaml / intents.yaml
```

The promotion script:

1. Loads the candidate by ID
2. Validates namespace and ID shape against the canonical naming rules
3. Rejects duplicates against existing canonical IDs
4. Strips the candidate-only fields (`status`, `review_required`)
5. Records provenance in a `provenance:` block on the new canonical entry
6. Appends to the target file (sorted by ID)
7. Removes the entry from the candidate file
8. Prints the next step (re-run `scripts/build-awareness-graph.sh` from
   the awareness-graph repo to regenerate triples)

The script does NOT write triples directly — that flows through the
normal build pipeline so the canonical RDF stays the single source of
truth.

## What the build pipeline does

- `scripts/build-awareness-graph.sh` (in awareness-graph repo) walks
  `services/docs/awareness/` recursively and converts every `.yaml`
  file to N-Triples via `yaml2nt`.
- The walk is configured to **SKIP** any directory named `candidates/`
  — so the YAML files here never reach `yaml2nt`.
- This means an in-progress candidate cannot accidentally land in the
  cluster's awareness graph via a missed PR review.

## Don'ts

- Don't edit `status: candidate` to `status: active` by hand and hope
  the build picks it up — it won't (the dir is skipped). Run the
  promotion script.
- Don't reuse a canonical ID as a candidate ID. The promotion script
  will reject duplicates, but it's clearer to pick a unique candidate ID
  upfront.
- Don't delete a candidate without recording why. If the fact turned
  out to be wrong, leave a note in the PR removing it so future agents
  don't rediscover and re-file it.
- Don't promote candidates with `confidence: low`. Either gather more
  evidence or close as "rejected — insufficient evidence".

## Related

- `awareness/preflight_audit.md` — what `awareness.preflight` checks
  and why the honest-DEGRADED gate points here
- `awareness/coverage_priority.md` — top-20 high-risk files identified
  in the Phase 1 audit; many will need candidates filed during
  upcoming edit cycles
- `docs/design/auto-healing-path-unification-patch-c.md` — Phase 2
  annotations driven by this kind of session-discovered audit
