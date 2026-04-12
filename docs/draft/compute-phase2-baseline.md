# Compute Phase 2 Baseline

## Status: Working

Distributed compute with partitioned multi-unit jobs, cross-node dispatch,
bounded retries, MinIO staging, verification, and aggregation.

## What works

- Workflow-driven orchestration (single path, no shortcuts)
- Partition planner: per_input, count strategies
- Cross-node dispatch via round-robin on direct service ports
- etcd TTL leases (30s) with heartbeat renewal
- MinIO input fetch + output upload with SHA-256
- Verification: CHECKSUM and SCHEMA_VALIDATE
- Bounded retries (max 3) with failure classification
- Aggregate result with per-unit checksums
- Real cancellation (kills process)
- Deployed on 3 nodes via normal package pipeline

## Known limitations

- No adaptive scheduling / load-aware placement
- No speculative execution
- No advanced merge strategies (last output ref used)
- Profile-based filtering infrastructure exists but not enforced
- Workflow definitions published on startup (not via package pipeline)
