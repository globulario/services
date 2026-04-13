# Proto Contracts

## Location

All `.proto` files live in `/proto/`. Generated code goes to:
- Go: `golang/<service>/<service>pb/`
- TypeScript: `typescript/<service>/`

## Regeneration

```bash
./generateCode.sh
```

This regenerates all Go and TypeScript stubs from all proto files.

## Key Proto Files

| Proto | Service | Purpose |
|-------|---------|---------|
| `compute.proto` | ComputeService | Job definitions, submissions, results |
| `compute_runner.proto` | ComputeRunnerService | Unit staging, execution, heartbeat |
| `workflow.proto` | WorkflowService + WorkflowActorService | Workflow execution + actor callbacks |
| `cluster_controller.proto` | ClusterControllerService | Node management, desired state, releases |
| `repository.proto` | PackageRepository | Artifact storage, manifests |
| `resource.proto` | ResourceService | Identity, accounts, groups |
| `rbac.proto` | RbacService | Access control |
| `authentication.proto` | AuthenticationService | Token management |
| `event.proto` | EventService | Event bus |
| `file.proto` | FileService | File operations |
| `dns.proto` | DnsService | DNS record management |

## Conventions

- Package names are lowercase, single-word (e.g., `compute`, `workflow`)
- Go package path: `github.com/globulario/services/golang/<pkg>/<pkg>pb`
- Service names use CamelCase (e.g., `ComputeService`)
- RPC names use CamelCase verbs (e.g., `SubmitComputeJob`)
- Enums use UPPER_SNAKE_CASE with a type prefix (e.g., `JOB_COMPLETED`)
- Timestamps use `google.protobuf.Timestamp`
- Flexible key-value data uses `google.protobuf.Struct`
