# cluster-doctor Degraded-Mode Diagnosis (PR-15)

> Status: PR-15 first increment delivered. Successor to the runtime vigilance
> probes track (PR-14). Where PR-14 made `ai_watcher` detect condition classes,
> PR-15 makes `cluster-doctor` stay useful when the cluster is partially broken.

## The problem PR-15 targets

A cluster-doctor that can only reach services through the same gateway that is
failing is useless exactly when it is needed. During PR-14 the gateway route to
`ai_memory` was found answering `text/html` while the `ai_memory` **backend was
healthy on its direct port** (reflection + RPCs succeeded directly). The wrong
diagnosis is "ai_memory is down." The right diagnosis is "ai_memory backend is
healthy; the gateway route is broken." That distinction is the whole point of
degraded-mode diagnosis, and it is the first concrete use case this PR encodes.

## First increment: gateway/backend divergence

cluster-doctor rules are pure functions over a pre-collected `collector.Snapshot`.
This increment adds one degraded-mode comparison and one rule:

- **Collector** (`collector/gateway_backend_divergence.go`): for selected
  services (PR-15: `ai_memory.AiMemoryService`), probe **both** paths — the Envoy
  gateway and the direct backend port — via gRPC server reflection, and record a
  `GatewayBackendProbe` on the snapshot. The direct-backend probe is the
  degraded-mode part: it does **not** depend on the failing gateway.
- **Rule** (`rules/gateway_backend_divergence.go`, `gateway.backend_divergence`):
  - gateway answers `text/html` + backend healthy → **FAIL (error): route/filter-chain suspected, NOT service-down.**
  - gateway `text/html` + backend also unreachable → FAIL: route broken AND backend not serving gRPC.
  - gateway `text/html` + backend not cross-checked → **UNKNOWN/CHECK_ERROR** (indeterminate; never a confident claim).
  - gateway merely *unavailable* (not an HTML content-type) → **no finding** (reflection does not normally route through the gateway, so this is inconclusive — a healthy gateway is never falsely accused).

### Why the content-type signal specifically

Reflection does not route through the Envoy gateway under normal operation, so a
plain "unavailable" on the gateway path is not evidence of a broken route. The
distinctive signal is the gRPC client receiving an **HTML response** instead of a
gRPC one (`unexpected content-type "text/html"`) — a route that has fallen
through to a web handler. Keying only on that keeps the false-positive rate at
zero for healthy gateways. Same class as the awareness-graph IP-as-SNI fix.

### Honesty properties

- Indeterminate cases emit `INVARIANT_UNKNOWN` with a `CheckError`, never a
  confident FAIL — consistent with the doctor masking-bug ratchet
  (`TestNoRuleEmitsConfidentFailureOnErroredSnapshot` still passes; the rule
  emits nothing on an empty/errored snapshot).
- Findings are operator-language (`intent:doctor.findings_are_operator_language`).
- Remediation steers operators to inspect the Envoy route/filter-chain and
  **warns against permanently pointing clients at the direct backend port** —
  that would hide the broken route instead of fixing it. No mutation is executed
  (`intent:remediation.must_go_through_workflow`).

### Tests / fixture

`rules/gateway_backend_divergence_test.go` is the fixture for this class:
route-broken-backend-healthy → "route suspected, not down"; route+backend-down;
backend-unchecked → indeterminate; plain-unavailable → no false positive;
healthy/empty → no finding. `collector/gateway_backend_divergence_test.go` covers
the content-type classifier and the pure folding function.

## Fix classification

`doctor_resilience_repair` — the work exists because an incident exposed
cluster-doctor's dependency on the very path that can fail.

## Known limitations / next

- **Live-cluster verification pending.** The pure rule + classifier are
  fully unit-tested; the live collector probe (endpoint resolution + reflection
  dial of gateway vs backend) needs a cluster run to confirm content-type
  detection end-to-end. Deferred with PR-16's release/runtime-proof work.
- **One service, one fallback.** This increment covers the gateway/backend
  reachability fallback for `ai_memory`. The broader degraded-mode matrix
  (service-manager→process-table, log-service→local files, AWG→embedded snapshot,
  Scylla→cqlsh, etcd→etcdctl, Envoy→admin endpoint, MinIO→mc/admin) follows the
  same collector-probe + pure-rule shape and is future work.
- **Governance entries (AWG invariant/failure-mode/test/forbidden-fix) and the
  behavioral-memory candidate** for this class are a follow-up pass, mirroring
  PR-14.
