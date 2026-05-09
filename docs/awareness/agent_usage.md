# Agent Usage Telemetry

## Purpose

Agent preflight skip rate measures how often an AI agent bypasses the awareness preflight check before making code changes. A high skip rate means agents are working without architectural context — the awareness system's primary protection.

## What is tracked

The `agent_usage_events` table in the awareness graph DB records each preflight and agent-context call:

| Field | Meaning |
|---|---|
| `tool` | `awareness.preflight` or `awareness.agent_context` |
| `operation` | `called` (preflight ran) or `skipped` (agent bypassed it) |
| `session_id_hash` | Anonymized session identifier (no user data stored) |
| `event_time` | UTC timestamp |

No prompts, tasks, or file paths are stored — only the event type and session fingerprint.

## Reading usage in health_pulse

`awareness.health_pulse` includes an `agent_usage` section:

```json
{
  "agent_usage": {
    "window_days": 7,
    "sessions_total": 42,
    "preflight_calls": 35,
    "preflight_skip_rate_pct": 16.7,
    "status": "ok"
  }
}
```

### Alert threshold

If `preflight_skip_rate_pct > 50%` over the 7-day window, the section status becomes `warning` and an `agent_usage.high_skip_rate` alert is emitted:

```json
{
  "severity": "warning",
  "id": "agent_usage.high_skip_rate",
  "message": "preflight skip rate 62% over last 7 days — agents may be bypassing awareness",
  "recommended_action": "..."
}
```

## Recording usage

MCP tools that run preflight call `graph.RecordAgentUsage()` after each invocation:

```go
_ = g.RecordAgentUsage(ctx, graph.AgentUsageEvent{
    ID:            uuid.New().String(),
    Tool:          "awareness.preflight",
    Operation:     "called",
    SessionIDHash: sessionHash,
})
```

Skipped calls are recorded with `operation: "skipped"` to detect bypass patterns.

## Tests

- `TestAgentUsage_RecordPreflightCall` — inserts an event and reads it back
- `TestAgentUsage_ComputesSkipRate` — correct skip rate calculation
- `TestHealthPulse_AgentSkipRateWarning` — alert fires when skip rate > 50%
- `TestGraphBuildMetadata_RecordsDurationAndCounts` — build duration is persisted

## Privacy

- Session IDs are hashed (SHA-256 of a volatile session identifier) before storage
- No prompt text, file paths, or user identifiers are stored
- The table is local to the awareness graph DB on the operator's machine
