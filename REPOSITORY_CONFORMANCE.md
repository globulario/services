# Repository Conformance Report

Status: **Current as of 2026-03-11**

## Implemented

### Proto Contracts

#### repository.proto
- **ArtifactKind enum**: SERVICE (1), APPLICATION (2), AGENT (3), SUBSYSTEM (4), INFRASTRUCTURE (5)
- **ArtifactRef**: publisher_id, name, version, platform, kind
- **ArtifactManifest**: ref, checksum, size_bytes, modified_unix, published_unix, description, keywords, icon, alias, license, min_globular_version; oneof typed overlays (ServiceDetail, ApplicationDetail, InfrastructureDetail)
- **ServiceDetail**: proto_file, grpc_service_name, port, systemd_unit, dependencies
- **ApplicationDetail**: route, index_file, actions, roles, groups, config
- **InfrastructureDetail**: component, config_template, data_dirs, health_endpoint, upgrade_strategy
- **RPCs**: ListArtifacts, GetArtifactManifest, UploadArtifact, DownloadArtifact, SearchArtifacts, GetArtifactVersions, DeleteArtifact
- **Legacy RPCs (deprecated)**: UploadBundle, DownloadBundle, ListBundles

#### node_agent.proto
- **InstalledPackage**: node_id, name, version, publisher_id, platform, kind, checksum, installed_unix, updated_unix, status, operation_id, metadata
- **RPCs**: ListInstalledPackages, GetInstalledPackage

#### cluster_controller.proto
- **UpgradePlanItem**: service, from_version, to_version, package_name, sha256, restart_required, impacts
- **PlanServiceUpgrades / ApplyServiceUpgrades RPCs**: query repository, build plans, dispatch to nodes
- **NodeUpgradeStatus**: per-node upgrade tracking in cluster-wide rollouts

### Cluster Controller Release Types (Go structs, etcd-serialized)
- **ServiceRelease** (Spec + Status): publisher, service name, version, channel, rollout strategy, node assignments, config, replicas
- **ApplicationRelease** (Spec + Status): publisher, app name, version, route, index_file, node assignments
- **InfrastructureRelease** (Spec + Status): publisher, component, version, data dirs, unit, upgrade strategy, health endpoint
- These are internal controller types stored as JSON in etcd, not part of the gRPC contract. Field shapes are compatible with future proto generation.

### Node Agent Actions
| Action | Kind | Status |
|--------|------|--------|
| artifact.fetch | All | Implemented — downloads artifact binary |
| artifact.verify | All | Implemented — SHA256 verification |
| service.install_payload | SERVICE | Implemented — extracts bin/systemd/config, daemon-reload |
| service.write_version_marker | SERVICE | Implemented — writes canonical version marker |
| service.start | SERVICE | Implemented — systemctl start |
| service.stop | SERVICE | Implemented — systemctl stop |
| service.restart | SERVICE | Implemented — systemctl restart |
| application.install | APPLICATION | Implemented — extracts to webroot, writes app metadata to etcd |
| application.uninstall | APPLICATION | Implemented — removes files and etcd metadata |
| infrastructure.install | INFRASTRUCTURE | Implemented — extracts bin/systemd/config, daemon-reload, data dirs |
| infrastructure.uninstall | INFRASTRUCTURE | Implemented — stop/disable unit, remove files, daemon-reload |
| package.install | All | Implemented — dispatcher to kind-specific handlers |
| package.uninstall | All | Implemented — dispatcher with full teardown for all kinds |
| package.verify | All | Implemented — existence + SHA256 check |
| package.report_state | All | Implemented — writes InstalledPackage to etcd |

### Plan Compilers (Cluster Controller)
- **CompileReleasePlan**: SERVICE — fetch → verify → install_payload → write_version_marker → report_state → restart
- **CompileApplicationPlan**: APPLICATION — fetch → verify → application.install → report_state
- **CompileInfrastructurePlan**: INFRASTRUCTURE — stop → fetch → verify → infrastructure.install → report_state → restart
- **BuildServiceUpgradePlan**: Simplified service plan for upgrade dispatch
- **buildUpgradePlanForKind**: Dispatcher — routes infra (etcd, minio, envoy) to infrastructure plans, everything else to service plans

### Repository Server
- All 10 RPCs implemented (7 artifact + 3 legacy bundle)
- Manifest enrichment from archive manifest.json (description, keywords, icon, license)
- Dual-write bridge: UploadBundle also creates modern artifact copy
- DownloadArtifact fallback: tries artifact path → v-prefixed → legacy bundle path
- **DeleteArtifact safety**: checks installed-state before deletion; rejects unless force=true when artifact is still installed on nodes; never triggers uninstall

### Installed-State Registry
- etcd key schema: `/globular/nodes/{node_id}/packages/{kind}/{name}`
- Values: protojson-encoded InstalledPackage records
- Operations: Write, Get, Delete, ListByNode, ListAllNodes
- Node Agent is the sole writer (via package.report_state action)
- Gateway admin UI reads directly from etcd for `/admin/packages` endpoint

### Gateway Admin API
- `GET /admin/upgrades/status` — service/infra/app status with Kind field
- `POST /admin/upgrades/plan` — delegates to controller
- `POST /admin/upgrades/apply` — delegates to controller
- `GET /admin/upgrades/jobs` — operation status tracking
- `GET /admin/upgrades/history` — historical records
- `GET /admin/packages` — installed packages from etcd
- `GET /admin/repository/search` — artifact catalog search
- `GET /admin/repository/manifest` — single artifact details
- `GET /admin/repository/versions` — version list
- `DELETE /admin/repository/artifact` — artifact deletion
- `GET /admin/metrics/services` — service health with Kind field

### CLI
- `globular pkg build` — builds .tgz with manifest.json for all package types
- `globular pkg publish` — dual-write: UploadBundle + best-effort UploadArtifact
- Package verification handles SERVICE, APPLICATION, INFRASTRUCTURE types

## Still Intentionally Deferred

| Item | Reason |
|------|--------|
| Proto definitions for ServiceRelease/ApplicationRelease/InfrastructureRelease | Internal controller types; JSON-serialized to etcd, not used over gRPC. Will migrate to proto when cross-service contract is needed. |
| AGENT and SUBSYSTEM artifact kinds | Defined in proto enum but no lifecycle implementation yet. Reserved for future use. |
| Resource service metadata/RBAC registration from application.install | App metadata written to etcd directly by Node Agent. Full Resource service integration deferred until resource migration is complete. |
| Service discovery registration during install | Services register themselves on startup via existing config flow. |
| Automatic route registration for applications | Route info written to etcd; gateway integration for serving apps is a separate feature. |
| Full purge mode for uninstall | Current uninstall removes package-owned config. User-managed persistent data (e.g., database files) is preserved. |
| Cascade delete (repository → node uninstall) | By design: DeleteArtifact never uninstalls. Repository and node lifecycle are independent. |

## Compatibility Retained

### Legacy Bundle RPCs
- `UploadBundle`, `DownloadBundle`, `ListBundles` — still functional, marked as deprecated
- UploadBundle performs dual-write to artifact storage
- DownloadArtifact falls back to legacy bundle path
- ListBundles queries Resource service (existing PackageBundle storage)

### Discovery Publish Flow
- `SetPackageDescriptor` still called during `pkg publish` (Step A)
- PackageDescriptor compatibility maintained for older CLI versions

### Resource Descriptor Compatibility
- `GetPackageDescriptor` / `SetPackageDescriptor` remain functional
- Legacy `PackageType` enum used for compat (INFRASTRUCTURE maps to SERVICE in legacy)
- Descriptor ID generation unchanged

### Old CLI Compatibility
- CLI `pkg publish` uses UploadBundle (primary) + UploadArtifact (best-effort)
- Older CLI versions without artifact support continue to work via bundle path
- TODO(migration) markers in code track planned transition to artifact-only path
