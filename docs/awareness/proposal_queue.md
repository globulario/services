# Proposal Queue — The Learning Loop

When awareness detects a new failure pattern or a novel fix, it creates a **proposal** — a structured knowledge update that has not yet been approved.

Proposals live in `docs/awareness/proposals/` as YAML files with a lifecycle:
`DRAFT` → `PENDING_REVIEW` → `APPROVED` or `REJECTED`.

---

## Why Proposals Matter

Awareness knowledge can only be auto-promoted if a human (or authorized agent) has reviewed and approved it. The proposal queue is the gate between raw AI observation and permanent graph knowledge.

A stale proposal queue means:
- Learned fixes are not being captured as invariants
- The same fix may be re-learned on the next incident
- Agents will keep hitting the same blind spots

---

## Proposal Lifecycle

```
awareness.learn_from_fix  →  DRAFT proposal created in docs/awareness/proposals/
awareness.validate_proposal →  Checks consistency, test coverage, no forbidden fixes
awareness.approve_proposal  →  Moves to APPROVED, marks for graph ingest on next build
awareness.build             →  Ingests approved proposals into the graph
```

---

## Queue Health Signals

`awareness.proposal_queue_health` returns:

```json
{
  "status": "needs_review",
  "pending_proposals": 4,
  "stale_proposals": 2,
  "oldest_stale_age_hours": 72
}
```

| Status | Meaning |
|--------|---------|
| `healthy` | No pending or stale proposals |
| `needs_review` | Proposals exist but are recent |
| `stale` | One or more proposals have been DRAFT for > 24h |
| `blocked` | A proposal failed validation and is blocking ingest |

---

## Reviewing a Proposal

```bash
# List pending proposals
globular awareness pending-proposals

# Validate a proposal (checks consistency without approving)
globular awareness validate-proposal --id <proposal-id>

# Approve a proposal (adds to graph on next build)
globular awareness approve-proposal --id <proposal-id>

# Via MCP
awareness.pending_proposals {}
awareness.validate_proposal { "proposal_id": "..." }
awareness.approve_proposal  { "proposal_id": "..." }
```

---

## When to Drain vs When to Skip

**Drain the queue when:**
- A real incident was resolved and the fix should be remembered
- A new forbidden fix pattern was discovered
- A required test was written that matches an existing invariant

**Skip (let proposals expire) when:**
- The proposal was generated from a false positive
- The incident was transient and unrepresentative
- A better invariant already covers the same failure mode

**Never auto-approve all proposals in bulk.** Each proposal changes the ground truth that agents use to make decisions. Review one at a time.

---

## Hard Rules

- **Never auto-approve proposals.** Human or authorized agent review only.
- **Never promote partial knowledge.** If a proposal lacks `required_tests`, it must not be approved.
- **Never delete proposals to clear the queue.** Mark them `REJECTED` with a reason instead.
- **A REJECTED proposal is still knowledge.** Keep rejected proposals as a record of what we decided not to encode.
