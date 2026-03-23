# Globular V1 — Completion Checklist

## Status: In Progress

What remains to close V1. Everything else (full RBAC externalization, execution governor phases 2-7E, dependency graphs, C# parity, email/Slack notifiers) is V2.

---

## 1. End-to-End AI Loop (Critical)

The 4 AI services (memory, watcher, executor, router) are deployed but the full incident lifecycle has never been triggered live on a real incident.

### What must work

- A real service crash triggers `service.exited` event
- ai_watcher receives it, matches rule, creates incident
- ai_executor picks up incident, gathers evidence, calls Claude CLI for diagnosis
- Claude returns structured diagnosis with proposed action
- Tier 0 (observe): findings recorded to ai_memory — **skeleton exists**
- Tier 1 (auto-remediate): action executes automatically (e.g., restart) — **skeleton only, needs real wiring**
- Tier 2 (require approval): notification sent via resource service, operator approves/denies — **skeleton only**
- Outcome verified and recorded

### Known blockers

- `processIncident()` in ai_watcher is a TODO placeholder
- Claude CLI stream-json subprocess not tested end-to-end
- Watcher-to-executor incident handoff not triggered live

### Definition of done

- [ ] Kill a service on the cluster, incident auto-created
- [ ] Diagnosis runs without human interaction
- [ ] Tier 1 restart action executes and service recovers
- [ ] Tier 2 action sends in-app notification, waits for approval
- [ ] Approval triggers execution, denial prevents it
- [ ] Full lifecycle recorded in ai_memory

---

## 2. Service Remove = Full Teardown

Currently removing a service from desired state leaves it running as "unmanaged." V1 requires complete lifecycle.

### Expected behavior

1. Controller removes service from desired state
2. Controller tells all nodes to uninstall
3. Node agent stops the running process
4. Node agent removes the systemd unit
5. Node agent updates installed state (removes from etcd)
6. Service no longer appears in any view

### Definition of done

- [ ] `globular services desired remove <name>` triggers full uninstall on all nodes
- [ ] Service does not appear as "unmanaged" after removal
- [ ] Works for multi-node clusters (all nodes clean up)

---

## 3. Operator CLI for AI

No way for an operator to interact with AI incidents except raw gRPC. Need CLI commands.

### Required commands

- `globular ai list` — list active incidents/jobs
- `globular ai show <id>` — show incident details, diagnosis, proposed action
- `globular ai approve <id>` — approve a Tier 2 pending action
- `globular ai deny <id>` — deny a pending action
- `globular ai retry <id>` — retry a failed action

### Definition of done

- [ ] All 5 commands implemented in globularcli
- [ ] Commands call ai_executor gRPC RPCs (already defined in proto)
- [ ] Output is human-readable with incident state, action, rationale

---

## 4. Core Tests for Approval Lifecycle

The AI_fix.md Definition of Done requires these tests. Currently missing.

### Required test cases

- [ ] Approval triggers execution (state: AWAITING_APPROVAL -> APPROVED -> EXECUTING -> SUCCEEDED)
- [ ] Denial prevents execution (state: AWAITING_APPROVAL -> DENIED)
- [ ] Expiry prevents execution (30min timeout -> EXPIRED)
- [ ] Idempotency (same incident doesn't create duplicate jobs)
- [ ] Restart recovery (durable jobs in etcd survive restart, resume)
- [ ] Action verification (post-execution check confirms outcome)
- [ ] Failure handling (action fails -> state: FAILED, recorded)

---

## 5. Fix Stale State Bugs

### 5a. PLAN_ROLLED_BACK stale state

Node shows unhealthy due to stale `PLAN_ROLLED_BACK` state that never clears.

- [ ] Identify where PLAN_ROLLED_BACK is set and what should clear it
- [ ] Fix state transition so node returns to healthy after rollback completes

### 5b. Echo service crash-loop

Echo service in failed/failed state, keeps crash-looping.

- [ ] Diagnose root cause
- [ ] Fix or remove echo from default install if it's a test service

### 5c. ai_executor etcd connection warning

Warning on startup, non-blocking but should be clean.

- [ ] Diagnose and fix connection timing/retry

---

## Priority Order

1. End-to-end AI loop (this is the flagship feature)
2. Service remove lifecycle (core platform correctness)
3. Operator CLI (usability)
4. Stale state bugs (stability)
5. Tests (confidence)

---

## Out of Scope for V1

- RBAC externalization phases 3b-6
- Execution governor phases 2-7E
- Dependency graphs and impact analysis
- C# runtime parity
- Email/Slack notifiers (in-app notifications via resource service are sufficient)
- Data exfiltration detection
- Botnet/coordinated attack correlation
