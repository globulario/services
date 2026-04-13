# Architecture Constraints

Strict boundaries that define what each component can and cannot do.

## Component Responsibilities

| Component | Role | MUST NOT |
|-----------|------|----------|
| Cluster Controller | Convergence authority, desired state management | Execute packages, run processes, use os/exec |
| Node Agent | Local execution, package install, systemd management | Make scheduling decisions, modify desired state |
| Workflow Service | Orchestration, step dispatch, run tracking | Store state directly, bypass actors |
| Repository | Artifact storage, manifest management | Execute packages, decide what should run |
| Gateway/Envoy | TLS termination, service mesh routing | Store state, make orchestration decisions |

## Execution Model

- Control plane DECIDES
- Workflow engine COORDINATES
- Node agents EXECUTE
- Repository PROVIDES artifacts
- Observability REPORTS reality

## Network Rules

- All inter-service gRPC uses mTLS via cluster PKI
- Envoy mesh routes on port 443 based on gRPC service name
- Direct service ports used for internal service-to-service calls not routed through mesh
- DNS resolution for `*.globular.internal` goes through Globular DNS service
- `config.ClusterDialContext` MUST be used for MinIO and internal DNS resolution

## State Ownership

| State | Written By | Read By | Storage |
|-------|-----------|---------|---------|
| Service config | Service on startup | All (via etcd) | etcd `/globular/services/` |
| Node state | Node agent heartbeat | Controller | Controller memory |
| Desired releases | Controller / CLI | Controller, Node agent | etcd `/globular/resources/DesiredRelease/` |
| Installed packages | Node agent | Controller, CLI | etcd `/globular/nodes/{id}/packages/` |
| Workflow runs | Workflow service | All (via gRPC) | ScyllaDB |
| Artifacts | Repository | Node agent | MinIO |
| Cluster config | Controller | All | MinIO `globular-config` bucket |

## Workflow Constraints

- Workflow definitions are YAML stored in MinIO `globular-config` bucket
- Actor endpoints MUST be passed in ExecuteWorkflowRequest
- Each step dispatches to exactly one actor
- Steps declare dependencies (DAG) — no implicit ordering
- `strategy.mode` MUST be one of: `single`, `foreach`, `dag`
- `foreach` requires `collection` and `itemName`
- onFailure hook runs when any step fails

## Package Pipeline

- Build: `globular pkg build --spec <yaml> --root <payload>`
- Publish: `globular pkg publish --file <tgz>`
- State transition: `VERIFIED → PUBLISHED` (via PromoteArtifact or auto-publish)
- Desired set: `globular services desired set <name> <version>`
- Install: node agent reconciliation loop (drift detection every 30s)

## What Is NOT Globular

- Not a container orchestrator (no pods, no sidecars)
- Not a continuous reconciliation system (changes are workflow-driven)
- Not Kubernetes (no kubelet, no API server, no etcd operator pattern)
- Not config-management (no Ansible/Puppet agent model)
