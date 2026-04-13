# Invariants

Strict rules that MUST hold at all times. Violation indicates a bug or architectural drift.

## Source of Truth

- etcd MUST be the sole source of truth for cluster configuration
- Environment variables MUST NOT be used for configuration
- No hardcoded addresses — all endpoints resolved from etcd or service discovery
- Standard protocol ports (443, 53, etc.) are acceptable — they are protocol definitions, not config

## 4-Layer State Model

- Repository → Desired → Installed → Runtime are four INDEPENDENT layers
- NEVER collapse layers (e.g., treating Desired as Installed)
- Each layer has exactly one owner:
  - Repository: `pkg publish` / repository service
  - Desired: cluster controller / `globular services desired set`
  - Installed: node agent (auto-populated from systemd)
  - Runtime: systemd + gRPC health checks

## Address Resolution

- Services MUST resolve endpoints via etcd service discovery
- No `127.0.0.1` or `localhost` for remote addresses
- For bind/listen operations, use `0.0.0.0`
- All inter-service gRPC MUST use mTLS with cluster CA

## Workflow Execution

- All meaningful state changes MUST go through workflows
- A workflow MUST reach a terminal state (SUCCEEDED or FAILED)
- No infinite workflow loops
- Workflows MUST be idempotent (safe to re-execute)
- The workflow engine is the single orchestration path — no hidden inline shortcuts

## Service Lifecycle

- A service MUST NOT be marked installed without checksum verification
- A node agent MUST NOT execute actions without a valid lease
- Services MUST register per-node instances in etcd via PutInstance
- Service ports come from etcd — NEVER hardcoded constants

## Compute Subsystem

- Every running compute unit MUST have an etcd TTL lease
- Maximum 3 retry attempts per unit (hard cap)
- DETERMINISTIC definitions MUST NOT be retried on EXECUTION_NONZERO_EXIT
- Job terminal state MUST reflect verification result, not just exit code
- etcd stores metadata only — blobs go to MinIO
- Past deadlines MUST prevent unit dispatch (pre-dispatch check)

## Security

- `clustercontroller_server` MUST NOT use `os/exec`, `syscall`, or `systemctl`
- `nodeagent_server` can only use `os/exec` within `internal/supervisor/`
- No token/credential storage in etcd values — use file references
