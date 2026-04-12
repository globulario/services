# Compute Phase 3 Baseline

## Deployed: compute@0.0.2

## Features implemented

| Feature | Phase | Status |
|---------|-------|--------|
| Workflow-driven orchestration | 1 | Single workflow path, no shortcuts |
| Remote cross-node dispatch | 1 | Via gRPC to ComputeRunnerService |
| MinIO input/output staging | 1 | SHA-256 checksums, ObjectRef pointers |
| Verification (CHECKSUM, structural) | 1 | Trust levels: UNVERIFIED → CONTENT_VERIFIED |
| Real cancellation | 1.5 | Process kill via context cancellation |
| etcd TTL leases (30s) | 1.5 | Grant, renew, revoke, expiry detection |
| Heartbeats (5s) | 1.5 | etcd keys with 15s TTL |
| Partition planner (per_input, count) | 2A | Deterministic multi-unit creation |
| Bounded retries (max 3) | 2A | Failure classification, no infinite loops |
| Aggregate manifest | 2A | Per-unit output refs in MinIO JSON |
| Profile-based placement | 3 | Strict filtering, no silent fallback |
| Resource-aware scoring | 3B | CPU + RAM + disk weighted |
| Load-aware placement | 3A | Active unit count per node |
| Progress reporting | 3D | progress.json contract, 5s polling |
| Job deadline enforcement | 3E | Pre-dispatch + await-loop checks |
| Priority scheduling | 3F | low/normal/high/critical with score boost |
| Package pipeline deployment | 3C | Deployed via desired-state reconciliation |

## Accepted guarantees

See `compute_guarantees_v1.md` for full list. Key additions since Phase 2:

- Profile filtering is strict — no fallback to ineligible nodes
- Resource minimums enforced — jobs fail if no node meets requirements
- Priority affects scoring — higher priority tolerates more node load
- Deadlines enforced — past deadlines prevent dispatch, active jobs cancelled
- Progress observable — entrypoints write progress.json, visible in unit state

## Known limitations

1. No preemption — running units never interrupted for higher priority
2. No quota/fair-share — no per-tenant resource limits
3. No speculative execution
4. Service instance ID shared across nodes (uses Address for disambiguation)
5. Aggregate result uses last unit's output ref for single-unit; manifest for multi
6. Heartbeat progress always 0.0 unless entrypoint writes progress.json
7. Workflow definitions deployed via startup publish, not package payload extraction

## Explicit Phase 4 deferred items

- Per-tenant quotas
- Fair-share scheduling
- Unit preemption
- Adaptive partition sizing
- GPU scheduling
- Multi-stage pipelines
- Streaming/windowed compute
- VM/OCI/WASM runtimes

## Build history

| Version | Build | Key changes |
|---------|-------|-------------|
| 0.0.1+1 | Phase 1 | Initial deployment |
| 0.0.1+2 | Phase 1.5 | Lease, heartbeat, cancel |
| 0.0.2+1 | Phase 3 | Full feature set with workflow defs |
