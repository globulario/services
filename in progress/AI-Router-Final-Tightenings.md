# AI Router — Final Tightenings

Pre-implementation clarifications before Phase 0/0.5.

---

## 1. Policy Staleness at xDS Watcher Merge Point

The xDS watcher receives a `RoutingPolicy` from ai_router with a `ComputedAt`
timestamp. Three states:

```go
const (
    policyFreshThreshold = 15 * time.Second   // normal: accept
    policyWarnThreshold  = 45 * time.Second   // aging: accept + warn log
    policyStaleThreshold = 90 * time.Second   // stale: reject → neutral
)

func mergeRoutingPolicy(policy *RoutingPolicy, current *xdsSnapshot) *xdsSnapshot {
    if policy == nil {
        return current  // neutral — no changes
    }

    age := time.Since(policy.ComputedAt)

    switch {
    case age < policyFreshThreshold:
        // Fresh — apply normally.
        return applyPolicy(current, policy)

    case age < policyStaleThreshold:
        // Aging — accept but log warning.
        // This happens when ai_router's scoring loop is slow or
        // Prometheus queries are delayed.
        slog.Warn("xds: routing policy aging",
            "age", age.Round(time.Second),
            "generation", policy.Generation)
        return applyPolicy(current, policy)

    default:
        // Stale — reject, fall back to neutral.
        // This means ai_router hasn't produced a fresh policy in >90s.
        // Something is wrong. Don't apply stale routing decisions.
        slog.Error("xds: routing policy stale, falling back to neutral",
            "age", age.Round(time.Second),
            "generation", policy.Generation)
        return current  // neutral — no changes
    }
}
```

---

## 2. xDS Snapshot Idempotence

Do not push a new xDS snapshot when the effective routing is unchanged.
Unnecessary pushes cause Envoy to re-process config and can briefly
disrupt active connections.

```go
func (w *Watcher) maybePublishSnapshot(newSnapshot *xdsSnapshot) {
    // Compare effective routing fields only (weights, circuit breakers,
    // outlier detection). Ignore metadata (timestamps, generation).
    if w.lastPublished != nil && effectivelyEqual(w.lastPublished, newSnapshot) {
        // No material change — skip publication.
        return
    }

    w.snapshotCache.SetSnapshot(w.nodeID, newSnapshot)
    w.lastPublished = newSnapshot
    w.lastPublishedAt = time.Now()
}

func effectivelyEqual(a, b *xdsSnapshot) bool {
    // Compare EDS endpoint weights.
    // Compare CDS circuit breaker values.
    // Compare CDS outlier detection config.
    // Compare RDS retry policy values.
    // Ignore: snapshot version, timestamps, non-routing metadata.
    //
    // Use proto.Equal on the relevant resource slices, or a stable
    // hash of the routing-relevant fields.
    return stableHash(a) == stableHash(b)
}
```

The hash covers only routing-affecting fields:
- Endpoint addresses + weights (EDS)
- Circuit breaker thresholds (CDS)
- Outlier detection config (CDS)
- Retry policy (RDS)

Certificate rotations, route additions, and other non-AI-Router changes
still trigger publication normally (different code path).

---

## 3. Dry-Run Validation Criteria (Phase 0.5)

Phase 0.5 runs the full scoring pipeline but returns nil policy (no effect).
It logs what it WOULD do and measures these criteria over a 24-hour window:

### Score Sanity

```
PASS if:
  - All scores between 0.0 and 1.0
  - No NaN or Inf values
  - Score variance across endpoints of same service < 0.5
    (if all endpoints are equally healthy, scores should be similar)
  - Scores respond to injected load (manual test: stress one endpoint,
    verify its score increases within 2 cycles)

FAIL if:
  - Any score outside 0.0-1.0
  - All scores identical for >10 minutes (scoring isn't differentiating)
  - Score doesn't change when metrics change (broken collection)
```

### Mapping Correctness

```
PASS if:
  - Every endpoint known to xDS watcher has a score
  - No "unknown endpoint" warnings in logs
  - Prometheus instance labels map 1:1 to etcd service configs
  - Service classification matches expected (log all classifications
    at startup, verify manually)

FAIL if:
  - Any endpoint missing from scoring output
  - Instance→endpoint mapping produces duplicates
  - Classification assigns stream_heavy to a unary-only service
```

### Change Frequency

```
PASS if:
  - Under stable load: <5% of cycles would produce weight changes
    (system is mostly at rest, scoring should be stable)
  - Under varying load: changes correlate with actual metric changes
    (not random noise)
  - No cycle produces >3 simultaneous weight changes across
    different services (suspicious — likely a metric collection issue)

FAIL if:
  - >30% of cycles would produce weight changes under stable load
    (flapping — smoothing/hysteresis too aggressive)
  - Changes don't correlate with metric movements (broken scoring)
```

### Fallback Frequency

```
PASS if:
  - <2% of cycles hit last-known-good fallback
  - <0.1% of cycles hit nil (neutral) fallback
  - Prometheus query success rate >98%

FAIL if:
  - >10% of cycles use fallback (collection infrastructure unreliable)
  - Any crash/panic in scoring pipeline (must be zero)
```

### Flapping Indicators

```
PASS if:
  - No endpoint's proposed weight oscillates more than twice
    in a 5-minute window (A→B→A pattern)
  - Smoothing produces monotonic weight changes during
    sustained load increase (always decreasing, not bouncing)

FAIL if:
  - Any endpoint oscillates >3 times in 5 minutes
  - Weight reversal within 2 consecutive cycles (A→B→A in 10s)
```

All criteria are logged as a summary every hour during Phase 0.5.
Phase 1 activation requires all PASS for 24 continuous hours.

---

## 4. Conservative Per-Service Classification (Explicit Statement)

Per-service classification is an **intentional simplification** for
Phases 0-4. It trades precision for safety and simplicity.

### What This Means

A service like `event.EventService` has both:
- `Publish` (unary) — could be rebalanced per-request
- `OnEvent` (server-streaming) — pinned for lifetime

Classified as `stream_heavy` because:
- The streaming behavior is the **riskier** dimension
- Applying `stateless_unary` drain strategy to a stream-heavy service
  could break active streams
- Applying `stream_heavy` drain strategy to unary RPCs is safe
  (just slower — unary requests don't need 5-minute drain grace)
- Conservative = classify by the most sensitive behavior

### What This Does NOT Cover

- A unary-only service incorrectly classified as `stream_heavy` will
  drain slower than necessary. This is suboptimal but safe.
- A stream-heavy service with latency-sensitive unary RPCs (e.g., event.Publish
  during incident response) may not get optimal unary routing.
  Acceptable for Phase 1-4.

### Future Evolution (Phase 5+)

When finer-grained control is needed:
- Split mixed services into two Envoy clusters:
  `event_unary_cluster` (Publish, Subscribe, UnSubscribe)
  `event_stream_cluster` (OnEvent)
- Route by gRPC method path prefix in RDS
- Apply different service class configs to each cluster

This requires xDS watcher changes (route splitting) and is deferred
until the basic routing model is proven stable.

### Classification is Overridable

```yaml
# In etcd: /globular/config/ai_router/service_classes
overrides:
  "event.EventService": "stream_heavy"      # explicit
  "custom.MyService": "stateless_unary"      # user override
```

Default classification is a starting point. Users can override per service
via config. The AI Router logs its classification at startup for auditability.
