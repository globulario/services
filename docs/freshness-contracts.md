# Freshness Contracts — Design Note

**Status:** Implemented. All AI-facing doctor read surfaces expose freshness.

---

## Contract

Every doctor read response includes a `ReportHeader` with these fields:

| Field | Type | Meaning |
|-------|------|---------|
| `source` | string | Service that produced the data (`"cluster-doctor"`) |
| `observed_at` | timestamp | When the underlying snapshot was taken |
| `snapshot_age_seconds` | float | Server-computed age (avoids clock skew) |
| `cache_hit` | bool | Whether the response came from cache |
| `cache_ttl_seconds` | float | Max staleness window for cached data |
| `freshness_mode` | enum | Echoed mode: CACHED or FRESH (never downgraded) |
| `snapshot_id` | string | Unique snapshot identifier |
| `data_incomplete` | bool | True if upstream data was partially unavailable |

## Freshness Modes

| Mode | Behavior |
|------|----------|
| `FRESHNESS_UNSPECIFIED` | Defaults to CACHED (server's choice) |
| `FRESHNESS_CACHED` | Returns cached snapshot if within TTL |
| `FRESHNESS_FRESH` | Invalidates cache, forces new upstream fetch |

## Where It Applies

### Direct RPCs (proto-level)
- `GetClusterReport` — request has `freshness` field
- `GetNodeReport` — request has `freshness` field
- `GetDriftReport` — request has `freshness` field

### CLI
- `globular doctor cluster --fresh` — forces fresh scan
- `globular doctor node --fresh` — forces fresh scan
- `globular doctor drift --fresh` — forces fresh scan
- All commands display freshness block in output header

### MCP Tools
- `cluster_get_doctor_report` — `freshness` input param + output block
- `cluster_get_drift_report` — `freshness` input param + output block
- `cluster_get_operational_snapshot` — `freshness` input param + doctor output includes freshness block
- `cluster_get_node_full_status` — `freshness` input param + doctor output includes freshness block

### ExplainFinding
- Reads from server's cached findings (no freshness param)
- By design: explanations are follow-ups to a report call
- The caller controls freshness via the preceding report call

## Cache Configuration

- Default TTL: 5 seconds (`snapshot_ttl` in doctor config)
- Singleflight: concurrent callers share a single in-flight fetch
- FRESH mode: invalidates cache before fetch

## Rule

> Every AI-facing read must disclose freshness. No silent caching.

If a new projection or read surface is added, it must include the same
freshness fields. The `render.Freshness` struct and `freshnessPayload()`
helper make this straightforward.
