# AI Fix — Implementation Plan

Closes the gaps between structural implementation and operational reality.

---

## State Machine

```
DETECTED → DIAGNOSING → DIAGNOSED
                           ↓
           ┌───────────────┼───────────────┐
           ↓               ↓               ↓
     RESOLVED(T1)    EXECUTING(T2)    AWAITING_APPROVAL(T3)
                       ↓    ↓              ↓         ↓
                  SUCCEEDED FAILED    APPROVED    DENIED
                                        ↓           ↓
                                    EXECUTING    CLOSED
                                     ↓    ↓
                                SUCCEEDED FAILED
                                              ↓
                            (after timeout) EXPIRED
```

States:
- DETECTED: incident created by watcher rule match
- DIAGNOSING: executor gathering evidence
- DIAGNOSED: root cause + proposed action determined
- EXECUTING: action being performed
- SUCCEEDED: action completed successfully
- FAILED: action failed (may retry)
- AWAITING_APPROVAL: Tier 3 waiting for human
- APPROVED: human approved, about to execute
- DENIED: human rejected, no action taken
- EXPIRED: approval timeout exceeded
- CLOSED: terminal state after denial/expiry

---

## Package/File Changes

### ai_executor (primary changes)

| File | Change |
|------|--------|
| `action_backend.go` | NEW — ActionBackend interface + real implementations |
| `job_store.go` | NEW — durable job persistence via etcd |
| `notifier.go` | NEW — Notifier interface + event notifier + email notifier |
| `state_machine.go` | NEW — incident state machine with transitions |
| `handlers.go` | UPDATE — add ApproveAction, DenyAction, RetryAction RPCs |
| `remediator.go` | UPDATE — use ActionBackend, durable jobs, verification |
| `server.go` | UPDATE — initialize new components |

### ai_executor proto

| Field | Change |
|-------|--------|
| `ApproveAction` RPC | NEW |
| `DenyAction` RPC | NEW |
| `RetryAction` RPC | NEW |
| `ActionState` enum | NEW (full state machine) |
| `Job` message | NEW (durable job record) |

### ai_watcher

| File | Change |
|------|--------|
| `server.go` | UPDATE — ApproveAction/DenyAction delegate to executor |

---

## Persistence Model (etcd)

```
Key: /globular/ai/jobs/{incident_id}

Value: {
    "incident_id": "abc-123",
    "action_id": "def-456",
    "state": "awaiting_approval",
    "tier": 2,
    "proposed_action": {
        "type": "restart_service",
        "target": "globular-event.service",
        "params": {}
    },
    "diagnosis": {
        "root_cause": "service_crash",
        "confidence": 0.8,
        "evidence": [...]
    },
    "approval": {
        "approved_by": "",
        "approved_at": 0,
        "denied_reason": "",
        "expires_at": 1711123456000
    },
    "execution": {
        "attempts": 0,
        "last_attempt_at": 0,
        "result": "",
        "error": "",
        "idempotency_key": "abc-123:restart_service:globular-event"
    },
    "created_at": 1711123200000,
    "updated_at": 1711123200000
}
```

Survives restart. Loaded on executor startup. Pending approvals resumed.
Expired jobs cleaned up. Idempotency key prevents double execution.

---

## Real Action Backends

### Phase 1 — shipping in this pass

| Action | Implementation | Verification |
|--------|---------------|--------------|
| `drain_endpoint` | Call ai_router.SetMode or publish routing event | Check ai_router weights |
| `restart_service` | Call node_agent gRPC (if available) or publish event | Check service health after |
| `tighten_circuit_breakers` | Publish event for ai_router to consume | Check ai_router policy |
| `notify_admin` | Publish notification event + call notifier | Log confirmation |

### Phase 2 — deferred

| Action | Why deferred |
|--------|-------------|
| `block_ip` | Needs Envoy rate limiter integration |
| `renew_cert` | Needs node_agent cert renewal flow |
| `clear_storage` | Dangerous — needs more safety gates |

---

## Notifier Design

```go
type Notifier interface {
    Notify(ctx context.Context, notification *Notification) error
}

type Notification struct {
    Type        NotificationType // approval_required, resolved, failed
    IncidentID  string
    Service     string
    Severity    string
    Summary     string
    RootCause   string
    Confidence  float32
    ProposedAction string
    Rationale   string
    ExpiresAt   time.Time
    ApproveCmd  string // "globular ai approve <id>"
    DenyCmd     string // "globular ai deny <id>"
}
```

### Implementations

1. **EventNotifier** (REQUIRED, shipping now)
   - Publishes `alert.approval.required`, `alert.incident.resolved`, `alert.incident.failed`
   - Admin UI and any event subscriber can consume

2. **LogNotifier** (REQUIRED, shipping now)
   - Structured slog at WARN level for approvals, INFO for resolutions
   - Always enabled as fallback

3. **EmailNotifier** (optional, Phase 2)
   - SMTP via globular mail service
   - Template with approve/deny links

---

## Concrete Tests

1. **Tier 1 observe**: event → incident → diagnosis recorded in ai_memory
2. **Tier 2 auto-remediate**: event → incident → action executed → verified
3. **Tier 3 approve → execute**: event → incident → notification → approve → action → verified
4. **Tier 3 deny**: event → incident → notification → deny → no action → closed
5. **Tier 3 expire**: event → incident → notification → timeout → expired → no action
6. **Idempotency**: approve same incident twice → only one execution
7. **Restart recovery**: create pending job → restart executor → job resumes
8. **Failure handling**: action fails → job marked failed → retry available
9. **Safety**: AI services down → cluster continues normally
