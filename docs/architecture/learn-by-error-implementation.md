# AI-Assisted Operational Learning — Implementation Plan

**Status:** Design phase. Based on `learn-by-error-reinforce.md`.

---

## What Already Exists (no new services needed)

| Data source | Where | Format |
|------------|-------|--------|
| Workflow runs | ScyllaDB `workflow.workflow_runs` | Structured (status, error, timing) |
| Workflow steps | ScyllaDB `workflow.workflow_steps` | Structured (status, error, details_json) |
| Resume decisions | Run outputs (`resume_decision.*`) | JSON in outputs_json |
| Step receipts | ScyllaDB `workflow.step_receipts` | JSON (result, timestamp) |
| Doctor findings | In-memory cache + event stream | Structured (severity, category, remediation) |
| Remediation audits | etcd `/globular/cluster_doctor/audit/*` | JSON (action, risk, outcome) |
| AI memory | ScyllaDB `ai_memory` tables | Structured (type, tags, content) |

**Key insight:** We don't need a new service. The ai-memory service already
provides structured storage with tag-based and text search. Incidents are
just a new memory type.

---

## Incident Schema

```json
{
  "type": "incident",
  "title": "install_scylladb blocked after executor crash on node.join",
  "tags": "workflow,node.join,install_scylladb,blocked,scylladb,verify_effect",
  "content": "...(narrative summary)...",
  "metadata": {
    "cluster_id": "globular.internal",
    "workflow_name": "node.join",
    "run_id": "bootstrap:814fbbb9",
    "step_id": "install_scylladb",
    "node_id": "814fbbb9-607f-5144-be1a-a863a0bea1e1",
    "package_name": "scylladb",
    "resume_policy": "verify_effect",
    "idempotency": "verify_then_continue",
    "verification_outcome": "inconclusive",
    "receipt_present": "false",
    "run_status": "BLOCKED",
    "doctor_finding_id": "abc123",
    "symptoms": "scylla health probe failed,step blocked after resume",
    "remediation_action": "node.repair",
    "outcome": "recovered",
    "time_to_recovery_sec": "420",
    "confidence": "high"
  }
}
```

**Why this shape:** It uses the existing ai-memory schema directly.
`type=incident`, `tags` for exact filtering, `metadata` for structured
attributes, `content` for narrative. No new tables needed.

---

## Service Impact

| Service | Role | Changes needed |
|---------|------|----------------|
| **ai-memory** | Store + query incidents | None — already supports memory_store/query/get with metadata |
| **workflow-service** | Emit incident on run completion | ~30 lines: after FinishRun, if FAILED/BLOCKED, store incident |
| **cluster-doctor** | Augment findings with similar incidents | ~50 lines: on finding production, query ai-memory for similar |
| **mcp** | Expose incident query tool | ~20 lines: new tool wrapping memory_query(type=incident) |

**No new service.** ai-memory is the incident store. The workflow service
writes incidents. The doctor reads them.

---

## Phase AL-1: Incident Projection

**When a workflow run finishes as FAILED or BLOCKED:**

1. The executor's FinishRun callback collects:
   - workflow name, run_id, step that failed/blocked
   - resume decision (from run outputs)
   - verification outcome
   - receipt presence
   - error message

2. Stores as an ai-memory record:
   ```
   memory_store(
     project: "globular-services",
     type: "incident",
     title: "{workflow} {step} {status}",
     tags: "{workflow},{step},{package},{status}",
     content: narrative summary,
     metadata: structured fields
   )
   ```

3. On SUCCEEDED runs with interesting patterns (e.g., resume that verified
   and skipped steps), store as a "recovery" type for positive learning.

**Implementation:** ~30 lines in `executor.go` after FinishRun. Uses the
existing `memory_store` MCP tool or direct gRPC to ai-memory.

---

## Phase AL-3: Similar Incident Query

**API:** `memory_query(type=incident, tags={workflow},{step}, text_search={symptom})`

Already works via ai-memory. No new code needed for basic exact-match.

**For semantic similarity:** Use the existing ai-memory `text_search` which
does substring matching on title + content. For V1, this is sufficient.
Embeddings can be added later as an optimization.

**MCP tool:** `incident_search(workflow, step, symptom)` — thin wrapper
around memory_query that formats results as incident summaries.

---

## Phase AL-4: Doctor Finding Augmentation

**When the doctor produces a finding for a blocked/failed workflow:**

1. Query ai-memory for incidents with matching:
   - `tags` containing the workflow name and step
   - `metadata.run_status` matching FAILED or BLOCKED

2. If similar incidents found, augment the finding's remediation steps:
   ```
   Remediation step N+1:
     description: "AI advisory: this matches N prior incidents.
       Most likely cause: {cause}. Suggested: {action}.
       Confidence based on {N} similar cases."
   ```

3. The augmentation is a **remediation step** — it goes through the same
   display path as all other remediation guidance. It does NOT execute
   anything.

**Implementation:** ~50 lines in the doctor's `cacheFindings` or a post-
processing step after `EvaluateAll`.

---

## Phase AL-5: Outcome Tracking

**When a recommendation is followed (or ignored):**

1. If the operator runs the suggested remediation workflow:
   - The workflow run's correlation_id links back to the incident
   - On completion, update the incident's `metadata.outcome`

2. If the operator ignores the suggestion:
   - The incident stays as-is (no negative signal needed initially)

**Implementation:** The correlation already exists via run_id/finding_id
chains. Updating the incident is a `memory_update` call.

---

## Safety Analysis

**AI remains advisory because:**

1. Incidents are stored in ai-memory (read-only from the workflow engine's
   perspective)
2. Doctor augmentation adds remediation **steps** (suggestions), not
   remediation **actions** (mutations)
3. The workflow engine still enforces verification, approval gates, and
   resume policies regardless of AI suggestions
4. No ai-memory record can bypass a BLOCKED run or skip verification
5. The operator/AI still must call `StartRemediationWorkflow` to act — the
   doctor finding with AI augmentation is just better information

**The deterministic core is still authoritative.** AI makes the findings
smarter, not the engine looser.

---

## First Vertical Slice

**Ship this first:**

1. Workflow executor stores incidents to ai-memory on FAILED/BLOCKED
2. Doctor queries ai-memory for similar incidents when producing findings
3. Similar incidents appear as remediation guidance in doctor findings

**That's it.** Three connection points. No new services. No new tables.
No new APIs. Just the existing systems talking to each other through
ai-memory as the knowledge bus.

**The system starts learning from day one of production use.** Every
failure becomes a reference case for the next failure.

---

## Implementation Sequence

| Phase | What | Effort | Depends on |
|-------|------|--------|------------|
| AL-1 | Store incidents on FAILED/BLOCKED runs | ~30 lines | Nothing |
| AL-3 | Incident search MCP tool | ~20 lines | AL-1 |
| AL-4 | Doctor augments findings with similar incidents | ~50 lines | AL-1 |
| AL-5 | Outcome tracking (update incident on recovery) | ~20 lines | AL-1 |
| AL-6 | Preflight rollout risk advisor | Later | AL-1 + enough data |

Total for the first slice (AL-1 + AL-3 + AL-4): **~100 lines of new code.**
