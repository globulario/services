# MCP → awareness-graph Transport Reliability

This page is the operator + agent contract for what happens when MCP
cannot reach the awareness-graph gRPC service. It documents the
request paths, the failure-class taxonomy, the per-call deadlines,
and the structured logs an operator should see when something goes
wrong.

Phase 6 of the awareness-graph improvement plan added the taxonomy
and deadline wrapping. Before Phase 6, every failure surfaced as
`{status: "degraded", error: "<gRPC error string>"}` — an agent had
to grep the error string to decide whether the service was down, the
graph store was broken, or the call timed out. After Phase 6, a
single `failure_class` field encodes the failure precisely.

## TL;DR for agents

- A successful awareness response has `status: ok` (or `empty` if
  the graph genuinely has no anchors for the target).
- A failed call returns `status: degraded` and a `failure_class`
  field — **never** an empty success.
- `failure_class: ""` is reserved for server-constructed degraded
  responses (e.g. `Preflight` returning `PREFLIGHT_STATUS_DEGRADED`
  because the high-risk path has zero anchors).
- Treat any `failure_class` other than `""` as transport-level
  degradation and fall back to reading the code directly. Do not
  treat it as "no awareness applies."

## TL;DR for operators

- A degraded awareness call emits one log line:
  `mcp: awareness degraded tool=<name> target="<file/id/task>" class=<CLASS> err=<error>`
- Look for the `class=` value. If you see a flood of `UNAVAILABLE`,
  the awareness-graph service is down or its endpoint isn't in etcd.
  If you see `STORE_ERROR`, Oxigraph is the suspect. If you see
  `TIMEOUT`, the store is slow or hung.

---

## Tool inventory

Every awareness tool is a thin forwarder to the awareness-graph
gRPC service. The full request path is:

```
MCP client (stdio | HTTP)
  → MCP server (golang/mcp)
    → awarenessStub  ── etcd lookup of awareness-graph endpoint
                     ── pooled mTLS gRPC client
    → AwarenessGraph.<Method>
      → Oxigraph (in-process RDF store inside awareness-graph)
```

| MCP tool name           | gRPC method | Bridge file                       | Target field used in logs                    |
|-------------------------|-------------|-----------------------------------|----------------------------------------------|
| `awareness.briefing`    | `Briefing`  | `tools_awareness.go`              | `file` (or `task:<verbatim task>`)            |
| `awareness.impact`      | `Impact`    | `tools_awareness.go`              | `file`                                       |
| `awareness.resolve`     | `Resolve`   | `tools_awareness.go`              | `class:id`                                   |
| `awareness.query`       | `Query`     | `tools_awareness.go`              | `mode=<m> [file=…] [id=…] [class=…]`         |
| `awareness.preflight`   | `Preflight` | `tools_awareness.go`              | `task=<truncated>` and/or `files=[…]`        |
| `awareness_diagnose`    | `Briefing` + cluster_doctor + cluster_controller | `tools_awareness_diagnose.go` | per-source `tool_failures` block in response |

The 5 single-source tools share `tools_awareness.go`. The composite
`awareness_diagnose` tool is structured differently (parallel
multi-source collection, per-source degradation labels inside the
response) — its reliability story lives in
`tools_awareness_diagnose.go` and is unchanged by Phase 6.

## Endpoint resolution

`awarenessEndpoint()` (in `clients.go`) reads the awareness-graph
service config from etcd at `/globular/services/awareness-graph/config`
and returns `Address:Port`. It does NOT route through Envoy — Envoy
hosts the public HTTP/gRPC-Web surface for the cluster, but the
internal MCP→awareness-graph path goes direct to the service.

If etcd has no record (e.g. early bootstrap, or the service has
never been registered), `awarenessEndpoint()` returns an error and
the tool returns `failure_class: ENDPOINT_RESOLUTION`. There is no
"localhost fallback" by design — the hard rule "etcd is the sole
source of truth" forbids it. An operator must fix etcd, not work
around it.

## Failure-class taxonomy

The complete enumeration (from `tools_awareness.go`):

| Class                 | When                                                                                        | Agent action                                                                                 |
|-----------------------|---------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------|
| `""` (none)           | Successful call. `status` will be `ok`, `empty`, or `degraded` per the server's semantics.  | Treat the response per `status`.                                                              |
| `UNAVAILABLE`         | gRPC Unavailable, dial-time "connection refused", or "Unavailable" in error string.         | Service is down or not deployed. Fall back to reading code.                                  |
| `TIMEOUT`             | gRPC DeadlineExceeded, `context.DeadlineExceeded`, or "context deadline exceeded" string.   | Store may be slow or hung. Retry once with a fresh request; otherwise fall back.             |
| `STORE_ERROR`         | gRPC Internal or DataLoss codes (Oxigraph load/query failure).                              | Backend store is broken. Do NOT retry — alert operator.                                       |
| `TRANSPORT_ERROR`     | TLS handshake failure, connection reset, transport-is-closing, mid-call drop, ctx canceled. | Transient. The client pool invalidates the connection on this path; retrying may succeed.    |
| `INVALID_ARGUMENT`    | gRPC InvalidArgument code.                                                                  | Caller bug — agent passed a malformed file path / class / id. Fix and retry.                  |
| `ENDPOINT_RESOLUTION` | `awareness-graph not found in etcd` or `Address missing from etcd`.                         | The service isn't registered. Operator must check etcd.                                       |

The taxonomy is **complete** in the sense that every error path
through `tools_awareness.go` is classified — `classifyFailure` ends
in `return FailureTransportError` as the default rather than
returning `""` for an unknown failure. An empty `failure_class` on
a degraded response means the response came from the server side
(e.g. `PreflightResponse.Status = PREFLIGHT_STATUS_DEGRADED`),
not from the MCP transport layer.

### Why EMPTY and DEGRADED are distinct

The Phase 5 honest-DEGRADED gate makes this distinction load-bearing.
A briefing that returns `status: empty` means "the call succeeded
and the graph has zero direct anchors for this target." A briefing
that returns `status: degraded` with `failure_class: UNAVAILABLE`
means "the call failed at the transport layer; we don't actually
know whether the graph has anchors."

An agent that conflates the two will silently lose awareness
coverage every time the service is briefly down. The taxonomy
forbids this: a transport failure can never look like an empty
success.

## Per-call deadlines

Every call from `tools_awareness.go` to the awareness-graph client
is wrapped with `context.WithTimeout(ctx, awarenessCallTimeout)`
where `awarenessCallTimeout = 10 * time.Second`. Without this,
the MCP request would inherit whatever deadline the transport
provides — and the HTTP transport (`transport_http.go`) does NOT
set per-request deadlines. A slow Oxigraph query could otherwise
hang an MCP request indefinitely.

The 10 s budget matches the existing pattern in `tools_composed.go`
for other gateway calls. It is generous: 99th-percentile awareness
calls complete in under 500 ms. Calls that approach 10 s are
themselves a signal that something is wrong, and the timeout will
surface as `failure_class: TIMEOUT` rather than as a silent hang.

`awareness_diagnose` uses its own `diagnoseSourceTimeout =
10*time.Second` (per gRPC source) and an outer 30 s budget — same
discipline, structured differently because the tool fans out to
three services in parallel.

## Structured operator log

Every degraded result emits exactly one log line at INFO:

```
mcp: awareness degraded tool=<name> target="<target>" class=<CLASS> err=<error>
```

Concrete examples from the test suite:

```
mcp: awareness degraded tool=awareness.briefing target="golang/foo.go" class=TIMEOUT err=context deadline exceeded
mcp: awareness degraded tool=awareness.impact target="golang/x.go" class=STORE_ERROR err=rpc error: code = Internal desc = oxigraph 500
mcp: awareness degraded tool=awareness.preflight target="files=[golang/cluster_controller/x.go]" class=UNAVAILABLE err=rpc error: code = Unavailable desc = no backend
mcp: awareness degraded tool=awareness.resolve target="BogusClass:x" class=INVALID_ARGUMENT err=rpc error: code = InvalidArgument desc = unknown class
```

The `target` field is bounded — long task strings are truncated to
60 characters, file lists to 3 entries — so a single noisy caller
can't blow out the log volume.

Sensitive content is not logged. The error string we emit is the
gRPC error returned by the awareness-graph server, which never
includes credentials. The `target` is the file path, class:id, or
task string the caller supplied; if a caller passes a token-like
string in a task, that's a caller bug, not a transport bug.

## Connection pooling and retry behavior

`clientPool` (in `clients.go`) caches one gRPC connection per
endpoint. When a call returns an error matched by `isConnError`
("tls:", "connection refused", "connection reset", "transport is
closing"), the pool entry is invalidated so the next call
re-dials. This is the mechanism that recovers from a brief
awareness-graph restart: the first call after the restart sees
TRANSPORT_ERROR; the second call dials fresh and succeeds.

The bridge does NOT implement automatic retry. The MCP contract is
that one tool call equals one upstream RPC — the caller decides
whether to retry, because the caller has the semantic context
(retrying a query is cheap; retrying a preflight may not be useful
if the agent has already moved on).

## Known limitation: HTTP transport session-drop after restart

`transport_http.go` keeps the MCP session map in-process (the
package-level `sessionStore` var). When the MCP server restarts,
the map is empty, and the next request that carries an existing
`Mcp-Session-Id` header fails with:

```
HTTP 404
{"jsonrpc":"2.0","error":{"code":-32600,"message":"invalid or expired session"}}
```

This is independent of awareness-graph reachability — it's a
property of the MCP HTTP transport, not the awareness bridge. A
client that hits this error must call `initialize` again to get a
fresh session id. This is documented here because operators
investigating "MCP returns errors after restart" often think the
problem is awareness-graph; it is not.

Hardening this would require persisting the session map (Scylla or
etcd would both work; Scylla is the better fit for an
opaque-token-with-TTL surface). It's tracked as a separate
improvement; Phase 6 deliberately scoped to the bridge code only.

## Test coverage

`tools_awareness_reliability_test.go` covers:

- `TestClassifyFailure_GRPCCodes` — every taxonomy class is
  produced by at least one canonical input (12 sub-cases).
- `TestBriefing_StubFailureClassifiedAsEndpointResolution` — the
  stub-fetch path (before any RPC) carries `ENDPOINT_RESOLUTION`.
- `TestBriefing_UnavailableMapsToUnavailable` — gRPC Unavailable
  surfaces as `UNAVAILABLE` (not as a generic transport error).
- `TestImpact_StoreErrorMapsToStoreError` — gRPC Internal surfaces
  as `STORE_ERROR`; tool/target fields are populated.
- `TestResolve_InvalidArgumentMapsToInvalidArgument` — caller
  bugs surface as `INVALID_ARGUMENT` with a `class:id` target.
- `TestQuery_TimeoutFromContextDeadlineMapsToTimeout` — gRPC
  DeadlineExceeded surfaces as `TIMEOUT`.
- `TestPreflight_DegradedNeverShadowsTransportFailure` — the
  critical safety property: a transport failure never produces an
  empty `failure_class`.
- `TestBriefing_RespectsPerCallTimeout` — a hung backend is
  bounded by the per-call timeout; the test would deadlock if the
  wrapper were missing.
- `TestBriefing_SuccessReturnsTypedResponse` — the happy path is
  unchanged: `status: ok`, prose intact, no extraneous fields.
- `TestBriefing_EmptyServerResponseStaysEmpty` — a server-side
  EMPTY response stays EMPTY; the bridge does not paint
  `failure_class` onto it.

Run with:

```bash
cd golang && go test ./mcp/ -run 'TestClassifyFailure|TestBriefing|TestImpact|TestResolve|TestQuery|TestPreflight_DegradedNeverShadows' -count=1
```

## Future improvements (not in Phase 6 scope)

1. **Per-call metrics**: emit a histogram of awareness call latency
   labeled by tool + failure_class. The Prometheus integration in
   `cluster_doctor` would be the natural home. Today the only
   signal is the log line, which an operator has to grep for.
2. **HTTP session persistence**: covered above. Untying the session
   map from process lifetime would close the "every MCP restart
   drops every connected client" gap.
3. **Adaptive timeout**: a hung Oxigraph today produces a flood of
   TIMEOUT classifications, one per call, each waiting 10 s.
   A circuit breaker on `UNAVAILABLE` / `TIMEOUT` rate would let
   callers fail fast during outages. The cluster_doctor circuit
   breaker pattern (see `session_zombie_leader_alertmanager.md`)
   would transplant cleanly.

These are intentionally out of Phase 6 scope. Phase 6 promised: do
not change awareness semantics unless a real bug is found. None of
the above is required to make the current bridge correct — they
are operator-experience improvements.

## Related

- `docs/awareness/preflight_audit.md` — what Preflight checks,
  including the Phase 5 honest-DEGRADED gate that depends on EMPTY
  vs DEGRADED being distinct.
- `docs/awareness/coverage_report.md` — Phase 4 coverage tool;
  operators use it to close the gaps Preflight's DEGRADED gate
  surfaces.
- `golang/mcp/tools_awareness.go` — the bridge code.
- `golang/mcp/tools_awareness_reliability_test.go` — Phase 6 tests.
- `golang/mcp/tools_awareness_diagnose.go` — the composite tool
  with its own multi-source reliability story.
- `golang/mcp/clients.go` — `clientPool`, `isConnError`,
  `awarenessEndpoint`.
- `golang/mcp/transport_http.go` — HTTP transport, including the
  in-process session map mentioned above.
