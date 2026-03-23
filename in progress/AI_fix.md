# Claude Task — Complete Globular AI Ops Implementation to Fully Meet Requirements

We now have the architectural loop in place:

- ai_watcher = eyes / incident trigger  
- ai_memory = memory / historical context  
- ai_router = reflexes / traffic shaping  
- ai_executor = hands / diagnosis + remediation orchestration  

The architecture is correct. The objective now is to fully meet the operational requirement, not just the structural one.

---

## Target Requirement

The system must support:

1. Watcher runs continuously (24/7)
2. Incidents automatically trigger AI-driven diagnosis/remediation
3. Tier 1 / Tier 2 execute automatically (within policy)
4. Tier 3 notifies human and waits
5. Approval triggers real execution (no TODO)
6. Denial prevents execution
7. Expiry cancels stale approvals
8. Outcomes are verified and recorded
9. State survives restart
10. System remains safe if AI fails

---

## 1. Tier 3 Approval Completion

Implement a full state machine:

detected → diagnosed → proposed → awaiting_approval → approved → executing → succeeded | failed | denied | expired

Requirements:
- Approved actions MUST execute (remove TODO)
- Preserve proposed action exactly (no recompute)
- Add idempotency (no double execution)
- Add expiry timeout handling
- Support API + CLI approval/deny

---

## 2. Notification System

Create Notifier interface:

type Notifier interface {
    NotifyApprovalRequired(...)
    NotifyResolved(...)
    NotifyFailed(...)
}

Implement:
- Admin UI event notifier (REQUIRED)
- Email notifier (optional)
- Slack notifier (optional)

Each notification must include:
- incident id
- service / endpoint
- severity
- proposed action
- rationale
- confidence
- approval command/link
- expiry

---

## 3. Real Action Execution

Create ActionBackend:

type ActionBackend interface {
    Execute(...)
    Validate(...)
    Supports(...)
}

Implement real actions:
- drain_endpoint
- reduce_route_weight
- tighten_circuit_breakers
- disable_retries
- restart_service

Each action must:
- validate input
- execute real change
- verify result

---

## 4. Durable Job Model

Persist remediation jobs:

Fields:
- incident_id
- action_id
- state
- approval metadata
- attempts
- result
- timestamps

Requirements:
- survive restart
- resume execution
- idempotency key

---

## 5. Safety Layer

Implement:
- allow/deny action policy
- confidence thresholds
- risk gating
- manual override always wins
- neutral fallback if AI unavailable

---

## 6. Observability

Add:
- full lifecycle logs
- structured logs (incident_id, action_id, state)
- metrics:
  - incidents_total
  - approvals_pending
  - executions_total
  - failures_total
- store meaningful summaries in ai_memory

---

## 7. Operator Flow

CLI:
- list incidents
- show details
- approve / deny
- cancel
- retry

Admin UI:
- pending approvals
- action details
- execution progress

---

## 8. Tests

Must include:

- approval → execution
- denial → no execution
- expiry → no execution
- idempotency
- restart recovery
- real action verification
- failure handling

---

## Definition of Done

System is complete when:

- watcher triggers without human interaction
- executor performs real actions
- approvals trigger execution
- notifications reach operator
- state persists across restart
- system remains safe if AI is disabled

---

## Priority Order

1. Tier 3 execution + durable job model  
2. Real action execution + verification  
3. Notification system  
4. CLI/UI flows  
5. Observability + tests  

---

## Constraints

- AI must remain optional
- system must stay deterministic
- no unsafe actions without approval
- prefer few real actions over many fake ones
- maintain auditability

9. Definition of Done

The implementation only meets the requirement when all of the following are true:

ai_watcher can create incidents continuously without human chat presence

incidents trigger diagnosis/remediation flow automatically

Tier 1 / Tier 2 supported actions are really executed, not just published as events

Tier 3 approval-required incidents notify a human through at least one real channel

approval leads to real execution of the previously proposed action

denial prevents execution

expiry prevents stale approvals from executing

outcomes are verified and recorded

action/job state survives restarts

cluster remains safe if AI services fail or are disabled

10. Implementation Priority

Please implement in this order:

Priority 1

complete Tier 3 post-approval execution

add durable remediation job model

add admin/UI event notification path

add idempotent execution

Priority 2

implement real action backends for safe high-value actions

add verification after execution

add CLI/admin flows

Priority 3

add email/slack notifiers

refine memory summaries

improve failure recovery and retries

11. Important Constraints

Keep the system deterministic and bounded

Do not introduce a dependency on an interactive chat session

Do not make cluster correctness depend on AI availability

Do not allow duplicate or unsafe remediation

Prefer a small number of real, well-verified actions over many fake actions

Preserve auditability and operator control at every step

12. Final Deliverable Request

Please produce:

A revised implementation plan for these missing pieces

The exact package/file changes you will make

The state machine for incident/action lifecycle

The persistence model for remediation jobs

The notifier design and first real notifier implementation

The concrete list of remediation actions that will be truly executable in this pass

The tests that prove the requirement is fully met
