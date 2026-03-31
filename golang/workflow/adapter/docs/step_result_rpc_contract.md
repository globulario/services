# Workflow Service RPC Contract for Step Result Callbacks

## 1. Goal

The callback contract exists so the workflow-service remains the authoritative owner of workflow state while execution happens remotely on node-agent.

Callbacks are not control messages.
Callbacks are execution facts.

## 2. Transport options

Preferred order:
1. gRPC callback RPC from node-agent to workflow-service
2. durable event bridge backed by workflow-service ingestion
3. polling fallback only for migration, not as final design

The recommended primary design is unary gRPC callbacks for:
- accept
- progress
- heartbeat
- terminal result

## 3. Required RPC surface

### Execute request path
Workflow-service -> node-agent

- `ExecuteWorkflowStep`
- `CancelWorkflowStep`

### Callback path
Node-agent -> workflow-service

- `ReportStepAccepted`
- `ReportStepProgress`
- `ReportStepHeartbeat`
- `ReportStepResult`

## 4. Authoritative state rule

Only workflow-service may transition:
- run state
- step state

Node-agent reports observations.
Workflow-service applies transitions.

## 5. Callback semantics

### ReportStepAccepted
Meaning:
- node-agent accepted responsibility for this attempt
- execution has started or is queued locally

This should move the step from `DISPATCHED` to `RUNNING` or `ACCEPTED`, depending on the engine model.

### ReportStepProgress
Meaning:
- non-terminal progress update
- may include percent, message, structured counters, current subphase

This must not imply success.

### ReportStepHeartbeat
Meaning:
- attempt is still alive
- no terminal result yet
- useful for long installs and restarts

### ReportStepResult
Meaning:
- terminal outcome for this exact attempt

Allowed terminal statuses:
- `SUCCEEDED`
- `FAILED`
- `CANCELLED`

This is the only callback allowed to close the attempt.

## 6. Idempotency key

Every callback must be idempotent.
Recommended key:

`run_id + step_id + attempt + sequence`

For terminal callback, keep sequence stable for retries of delivery.

Workflow-service must deduplicate repeated callbacks.

## 7. Ordering rules

Ordering is best effort, not guaranteed.
Therefore:
- progress may arrive more than once
- heartbeat may arrive after a progress event
- terminal result may race with late heartbeat
- duplicate accept/progress/heartbeat is legal

Workflow-service must treat `ReportStepResult` as the authoritative terminal event and ignore later non-terminal callbacks for the same attempt.

## 8. Terminal result payload

A terminal result should contain:
- identity fields
- terminal status
- started_at
- finished_at
- duration_ms
- output values
- error classification
- human-readable summary
- machine-readable details
- optional artifact references
- optional metrics snapshot

## 9. Error classification contract

Use a stable enum, not free text only.

Required fields:
- `error_code`
- `error_class`
- `message`

Optional:
- `retryable_hint`
- `details_json`

Retryable hint is advisory only.
Workflow-service decides whether to retry.

## 10. Heartbeat contract

Heartbeat should include:
- current timestamp
- optional progress snapshot
- optional current local operation id
- optional estimated subphase

Workflow-service should use it to extend liveness for long-running attempts.

## 11. Timeout handling

Workflow-service owns official timeouts.

If workflow-service times out the step:
- it marks the attempt timed out
- it sends `CancelWorkflowStep`
- it may ignore any later success callback or treat it as late result based on policy

Recommended rule:
- once timed out and closed, late success should be recorded as late telemetry, not reopen the step

## 12. Security

Callbacks must be authenticated.
Preferred options:
- mTLS node identity
- signed node token scoped to callback audience

Required checks:
- caller is the correct node or authorized node-agent identity
- caller is allowed to report for the target `node_id`
- callback fields match the original dispatched attempt

## 13. Minimal success flow

1. workflow-service dispatches `ExecuteWorkflowStep`
2. node-agent returns accepted
3. node-agent sends `ReportStepAccepted`
4. node-agent sends zero or more `ReportStepProgress`
5. node-agent sends zero or more `ReportStepHeartbeat`
6. node-agent sends `ReportStepResult(status=SUCCEEDED)`
7. workflow-service unlocks dependent steps

## 14. Minimal failure flow

1. workflow-service dispatches `ExecuteWorkflowStep`
2. node-agent accepts
3. node-agent executes
4. node-agent sends `ReportStepResult(status=FAILED, error_class=VERIFY_FAILED)`
5. workflow-service evaluates retry policy
6. workflow-service either re-dispatches next attempt or fails the run

## 15. Cancellation flow

1. workflow-service sends `CancelWorkflowStep`
2. node-agent acknowledges cancel request
3. node-agent stops work if possible
4. node-agent sends `ReportStepResult(status=CANCELLED)` or `FAILED`
5. workflow-service closes the attempt

## 16. Golden sentence

The callback contract is a fact channel from executor to authority.
It must be narrow, authenticated, idempotent, and terminally unambiguous.
