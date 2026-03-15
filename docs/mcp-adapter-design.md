# Globular MCP Adapter — Phase 1 Design

## One-line essence

Turn Globular from a system that must be interpreted through logs and CLI text into a system that AI assistants can inspect and reason about through a safe, structured, operator-grade MCP interface.

---

## Section 1: Service Inventory

| Service | Proto Package | Operator Value | RPCs | Phase 1 Priority |
|---------|--------------|----------------|------|-----------------|
| **ClusterControllerService** | `cluster_controller` | Core cluster state, node management, health, plans, desired state, reconciliation | 34 | **Critical** |
| **ClusterDoctorService** | `cluster_doctor` | Invariant checking, drift detection, finding explanations, remediation guidance | 4 | **Critical** |
| **NodeAgentService** | `node_agent` | Node inventory, installed packages, plan execution status, backup/restore tasks | 15 | **High** |
| **BackupManagerService** | `backup_manager` | Backup jobs, artifacts, validation, retention, restore planning, recovery posture | 25 | **High** |
| **PackageRepository** | `repository` | Artifact catalog, versions, namespaces, publish states, provenance | 13 | **Medium** |
| **AuthenticationService** | `authentication` | Token validation, session diagnostics | 8 | **Low** (diagnostic only) |
| **DnsService** | `dns` | DNS record inspection for network diagnostics | 39 | **Low** (optional) |

---

## Section 2: RPC Classification Table

### ClusterControllerService (34 RPCs)

| RPC | Classification | Rationale |
|-----|---------------|-----------|
| `GetClusterInfo` | **Diagnostic** | Read-only cluster metadata |
| `GetClusterHealth` | **Diagnostic** | Legacy health summary |
| `GetClusterHealthV1` | **Diagnostic** | Detailed health with per-node status, drift, plan state |
| `GetNodeHealthDetailV1` | **Diagnostic** | Deep per-node health breakdown |
| `ListNodes` | **Diagnostic** | Node inventory with status |
| `GetNodePlan` | **Diagnostic** | Current pending plan for a node (legacy) |
| `GetNodePlanV1` | **Diagnostic** | Current pending plan (v1 format) |
| `GetDesiredState` | **Diagnostic** | Full desired-state manifest |
| `GetJoinRequestStatus` | **Diagnostic** | Join request status lookup |
| `ListJoinRequests` | **Diagnostic** | Pending join requests |
| `PreviewNodeProfiles` | **Planning** | Dry-run profile change impact |
| `PreviewDesiredServices` | **Planning** | Preview desired-state delta |
| `PlanServiceUpgrades` | **Planning** | Preview upgrade plan without applying |
| `ValidateArtifact` | **Planning** | Validate artifact before deployment |
| `ReconcileNodeV1` | **Mutating** | Trigger reconciliation for a single node |
| `SetNodeProfiles` | **Mutating** | Change node deployment profiles |
| `ApproveJoin` | **Mutating** | Approve node join request |
| `RejectJoin` | **Mutating** | Reject node join request |
| `RemoveNode` | **Mutating** | Remove node from cluster |
| `CreateJoinToken` | **Mutating** | Create new join token |
| `UpdateClusterNetwork` | **Mutating** | Change cluster network spec |
| `ApplyNodePlan` | **Mutating** | Force-apply plan to node (legacy) |
| `ApplyNodePlanV1` | **Mutating** | Force-apply plan to node (v1) |
| `UpgradeGlobular` | **Mutating** | Initiate cluster-wide upgrade |
| `ApplyServiceUpgrades` | **Mutating** | Apply upgrade plan |
| `UpsertDesiredService` | **Mutating** | Add/update desired service |
| `RemoveDesiredService` | **Mutating** | Remove desired service |
| `SeedDesiredState` | **Mutating** | Bulk-seed desired state |
| `ReportNodeStatus` | **Internal** | Node-agent heartbeat — not for AI |
| `CompleteOperation` | **Internal** | Operation lifecycle — not for AI |
| `ReportPlanRejection` | **Internal** | Plan verification — not for AI |
| `WatchNodePlanStatusV1` | **Internal** | Server-stream, use snapshot instead |
| `WatchOperations` | **Internal** | Server-stream, use snapshot instead |

### ClusterDoctorService (4 RPCs)

| RPC | Classification | Rationale |
|-----|---------------|-----------|
| `GetClusterReport` | **Diagnostic** | Full cluster invariant check |
| `GetNodeReport` | **Diagnostic** | Per-node invariant check |
| `GetDriftReport` | **Diagnostic** | Drift detection across nodes |
| `ExplainFinding` | **Diagnostic** | Deep-dive on a specific finding |

### NodeAgentService (15 RPCs)

| RPC | Classification | Rationale |
|-----|---------------|-----------|
| `GetInventory` | **Diagnostic** | Node hardware, identity, units |
| `GetPlanStatusV1` | **Diagnostic** | Current plan execution status |
| `ListInstalledPackages` | **Diagnostic** | Installed packages on this node |
| `GetInstalledPackage` | **Diagnostic** | Single package detail |
| `GetBackupTaskResult` | **Diagnostic** | Backup task outcome |
| `GetRestoreTaskResult` | **Diagnostic** | Restore task outcome |
| `ApplyPlan` | **Mutating** | Apply plan (legacy) |
| `ApplyPlanV1` | **Mutating** | Apply plan (v1) |
| `JoinCluster` | **Mutating** | Join cluster flow |
| `BootstrapFirstNode` | **Mutating** | Day-0 bootstrap |
| `RunBackupProvider` | **Mutating** | Execute backup provider |
| `RunRestoreProvider` | **Mutating** | Execute restore provider |
| `RotateNodeToken` | **Mutating** | Rotate auth token |
| `WatchPlanStatusV1` | **Internal** | Server-stream |
| `WatchOperation` | **Internal** | Server-stream |

### BackupManagerService (25 RPCs)

| RPC | Classification | Rationale |
|-----|---------------|-----------|
| `ListBackupJobs` | **Diagnostic** | Job history |
| `GetBackupJob` | **Diagnostic** | Single job detail |
| `ListBackups` | **Diagnostic** | Backup artifact inventory |
| `GetBackup` | **Diagnostic** | Single backup detail |
| `ValidateBackup` | **Diagnostic** | Validate backup integrity |
| `GetRetentionStatus` | **Diagnostic** | Retention policy status |
| `GetScheduleStatus` | **Diagnostic** | Schedule status |
| `GetRecoveryStatus` | **Diagnostic** | Recovery posture assessment |
| `ListMinioBuckets` | **Diagnostic** | Object storage buckets |
| `TestScyllaConnection` | **Diagnostic** | ScyllaDB connectivity check |
| `PreflightCheck` | **Diagnostic** | Pre-backup validation |
| `RestorePlan` | **Planning** | Preview restore steps without executing |
| `RunRestoreTest` | **Planning** | Test restore in isolation |
| `RunBackup` | **Mutating** | Execute backup |
| `RestoreBackup` | **Mutating** | Execute restore |
| `CancelBackupJob` | **Mutating** | Cancel running job |
| `DeleteBackupJob` | **Mutating** | Delete job record |
| `DeleteBackup` | **Mutating** | Delete backup artifact |
| `RunRetention` | **Mutating** | Execute retention policy |
| `PromoteBackup` | **Mutating** | Promote backup quality |
| `DemoteBackup` | **Mutating** | Demote backup quality |
| `CreateMinioBucket` | **Mutating** | Create storage bucket |
| `DeleteMinioBucket` | **Mutating** | Delete storage bucket |
| `ApplyRecoverySeed` | **Mutating** | Apply recovery seed |
| `Stop` | **Internal** | Service lifecycle |

### PackageRepository (13 RPCs)

| RPC | Classification | Rationale |
|-----|---------------|-----------|
| `ListArtifacts` | **Diagnostic** | Full artifact catalog |
| `SearchArtifacts` | **Diagnostic** | Filtered artifact search |
| `GetArtifactManifest` | **Diagnostic** | Single artifact manifest |
| `GetArtifactVersions` | **Diagnostic** | Version history for an artifact |
| `ListBundles` | **Diagnostic** | Legacy bundle listing |
| `GetNamespace` | **Diagnostic** | Namespace ownership info |
| `DeleteArtifact` | **Mutating** | Remove artifact |
| `PromoteArtifact` | **Mutating** | Change publish state |
| `SetArtifactState` | **Mutating** | Set publish state |
| `UploadBundle` | **Internal** | Client-stream upload |
| `UploadArtifact` | **Internal** | Client-stream upload |
| `DownloadBundle` | **Internal** | Server-stream download |
| `DownloadArtifact` | **Internal** | Server-stream download |

### AuthenticationService (8 RPCs)

| RPC | Classification | Rationale |
|-----|---------------|-----------|
| `ValidateToken` | **Diagnostic** | Check token validity |
| `Authenticate` | **Mutating** | Login — needed for MCP auth bootstrap |
| `RefreshToken` | **Internal** | Token refresh — handled by auth layer |
| `GeneratePeerToken` | **Internal** | Peer-to-peer auth |
| `SetPassword` | **Mutating** | Password management — defer |
| `SetRootPassword` | **Mutating** | Root password — defer |
| `SetRootEmail` | **Mutating** | Root email — defer |
| `IssueClientCertificate` | **Mutating** | Cert issuance — defer |

### DnsService (39 RPCs)

| RPC | Classification | Rationale |
|-----|---------------|-----------|
| `GetDomains` | **Diagnostic** | List configured domains |
| `GetA/AAAA/SRV/...` | **Diagnostic** | Read DNS records |
| `SetDomains` | **Mutating** | Modify domains |
| `Set*/Remove*` | **Mutating** | Modify DNS records |
| `Stop` | **Internal** | Service lifecycle |

---

## Section 3: Phase 1 MCP Tool Catalog

### Cluster Tools

#### `cluster_get_health`
- **Backed by:** `ClusterControllerService.GetClusterHealthV1`
- **Purpose:** Get overall cluster health: node count, healthy/unhealthy nodes, desired vs applied hash, plan states, drift summary
- **When to use:** First call to understand cluster state. Use before any diagnosis.
- **Classification:** read-only
- **Input:** `{}` (no parameters)
- **Output:** `{ overall_status, node_count, healthy_count, unhealthy_count, nodes: [{ node_id, hostname, status, last_seen, desired_hash, applied_hash, current_plan_phase, last_error, can_apply_privileged, health_checks: [{ subsystem, ok, reason }] }] }`
- **RBAC:** requires `sa` or `cluster-admin` role

#### `cluster_list_nodes`
- **Backed by:** `ClusterControllerService.ListNodes`
- **Purpose:** List all registered cluster nodes with identity, profiles, endpoint, capabilities
- **When to use:** To enumerate nodes, check registration, verify endpoints
- **Classification:** read-only
- **Input:** `{}` (no parameters)
- **Output:** `{ nodes: [{ node_id, hostname, ips, status, profiles, agent_endpoint, last_seen, capabilities }] }`
- **RBAC:** requires `sa` or `cluster-admin` role

#### `cluster_get_node_health_detail`
- **Backed by:** `ClusterControllerService.GetNodeHealthDetailV1`
- **Purpose:** Deep health breakdown for a specific node
- **When to use:** When a node appears unhealthy and you need diagnosis details
- **Classification:** read-only
- **Input:** `{ node_id: string (required) }`
- **Output:** `{ node_id, status, health_checks, installed_versions, desired_versions, drift_items, last_plan, last_error }`
- **RBAC:** requires `sa` or `cluster-admin` role

#### `cluster_get_node_plan`
- **Backed by:** `ClusterControllerService.GetNodePlanV1`
- **Purpose:** Get the current pending plan for a node (what the controller wants the node to do)
- **When to use:** When a node is "converging" or stuck, to see what plan is pending
- **Classification:** read-only
- **Input:** `{ node_id: string (required) }`
- **Output:** `{ plan_id, generation, steps, desired_hash, status, created_at, issued_by }`
- **RBAC:** requires `sa` or `cluster-admin` role

#### `cluster_preview_node_profiles`
- **Backed by:** `ClusterControllerService.PreviewNodeProfiles`
- **Purpose:** Preview impact of changing a node's profiles without applying
- **When to use:** Before SetNodeProfiles, to understand what units/configs will change
- **Classification:** preview
- **Input:** `{ node_id: string (required), profiles: string[] (required) }`
- **Output:** `{ normalized_profiles, unit_diff, config_diff, restart_units, affected_nodes }`
- **RBAC:** requires `sa` or `cluster-admin` role

#### `cluster_get_desired_state`
- **Backed by:** `ClusterControllerService.GetDesiredState`
- **Purpose:** Get the full desired-state manifest (all services, versions, platforms)
- **When to use:** To understand what the cluster *should* look like
- **Classification:** read-only
- **Input:** `{}` (no parameters)
- **Output:** `{ services: [{ service_id, version, platform, build_number }], revision }`
- **RBAC:** requires `sa` or `cluster-admin` role

#### `cluster_get_info`
- **Backed by:** `ClusterControllerService.GetClusterInfo`
- **Purpose:** Get cluster ID, domain, creation time
- **When to use:** To identify which cluster you're connected to
- **Classification:** read-only
- **Input:** `{}` (no parameters)
- **Output:** `{ cluster_id, cluster_domain, created_at }`
- **RBAC:** requires `sa` or `cluster-admin` role

### Doctor Tools

#### `cluster_get_doctor_report`
- **Backed by:** `ClusterDoctorService.GetClusterReport`
- **Purpose:** Run all invariant checks and get findings with severity, category, evidence, remediation
- **When to use:** Comprehensive cluster diagnosis. This is the "what's wrong?" tool.
- **Classification:** read-only
- **Input:** `{}` (no parameters)
- **Output:** `{ overall_status (HEALTHY/DEGRADED/CRITICAL), findings: [{ finding_id, severity, category, summary, evidence, remediation_steps }], counts_by_category, top_issue_ids }`
- **RBAC:** requires `sa` or `cluster-admin` role

#### `cluster_get_drift_report`
- **Backed by:** `ClusterDoctorService.GetDriftReport`
- **Purpose:** Detect version/state drift between desired and actual across nodes
- **When to use:** When investigating why a service version doesn't match expected
- **Classification:** read-only
- **Input:** `{ node_id: string (optional, empty = all nodes) }`
- **Output:** `{ items: [{ node_id, entity_ref, category, desired, actual, evidence }], total_drift_count }`
- **RBAC:** requires `sa` or `cluster-admin` role

#### `cluster_explain_finding`
- **Backed by:** `ClusterDoctorService.ExplainFinding`
- **Purpose:** Deep-dive explanation of a specific finding from the doctor report
- **When to use:** After `cluster_get_doctor_report`, to understand a specific finding
- **Classification:** read-only
- **Input:** `{ finding_id: string (required) }`
- **Output:** `{ finding_id, invariant_id, why_failed, remediation_steps, evidence, plan_risk, plan_diff }`
- **RBAC:** requires `sa` or `cluster-admin` role

### Node Agent Tools

#### `nodeagent_get_inventory`
- **Backed by:** `NodeAgentService.GetInventory`
- **Purpose:** Get node identity, hardware, systemd units, and component versions
- **When to use:** To understand what's running on a specific node
- **Classification:** read-only
- **Input:** `{}` (no parameters)
- **Output:** `{ identity: { hostname, domain, ips, os, arch, agent_version }, unix_time, components, units }`
- **RBAC:** requires `sa` or node token

#### `nodeagent_get_plan_status`
- **Backed by:** `NodeAgentService.GetPlanStatusV1`
- **Purpose:** Get current plan execution status (running, succeeded, failed, steps completed)
- **When to use:** To check if a plan is executing or stuck on the node
- **Classification:** read-only
- **Input:** `{ plan_id: string (optional) }`
- **Output:** `{ plan_id, state, progress_percent, current_step, events, error }`
- **RBAC:** requires `sa` or node token

#### `nodeagent_list_installed_packages`
- **Backed by:** `NodeAgentService.ListInstalledPackages`
- **Purpose:** List all packages installed on the node with versions, kind, status
- **When to use:** To verify what's actually installed vs what's desired
- **Classification:** read-only
- **Input:** `{ kind: string (optional: SERVICE/INFRASTRUCTURE/APPLICATION) }`
- **Output:** `{ packages: [{ name, version, publisher_id, platform, kind, checksum, status, installed_unix, updated_unix }] }`
- **RBAC:** requires `sa` or node token

#### `nodeagent_get_installed_package`
- **Backed by:** `NodeAgentService.GetInstalledPackage`
- **Purpose:** Get detail for a single installed package
- **When to use:** To inspect a specific package's version, checksum, install time
- **Classification:** read-only
- **Input:** `{ name: string (required), kind: string (optional) }`
- **Output:** `{ package: { name, version, publisher_id, platform, kind, checksum, status, metadata } }`
- **RBAC:** requires `sa` or node token

### Repository Tools

#### `repository_list_artifacts`
- **Backed by:** `PackageRepository.ListArtifacts`
- **Purpose:** List all artifacts in the repository catalog
- **When to use:** To see what packages are available for deployment
- **Classification:** read-only
- **Input:** `{}` (no parameters)
- **Output:** `{ artifacts: [{ ref: { publisher_id, name, version, platform, kind }, checksum, size_bytes, build_number, publish_state, description }] }`
- **RBAC:** requires `sa` or authenticated user

#### `repository_search_artifacts`
- **Backed by:** `PackageRepository.SearchArtifacts`
- **Purpose:** Search artifacts by name, publisher, kind, or keyword
- **When to use:** To find specific packages in the catalog
- **Classification:** read-only
- **Input:** `{ query: string (optional), kind: string (optional), publisher_id: string (optional) }`
- **Output:** `{ artifacts: [{ ref, checksum, size_bytes, publish_state, description }] }`
- **RBAC:** requires `sa` or authenticated user

#### `repository_get_artifact_manifest`
- **Backed by:** `PackageRepository.GetArtifactManifest`
- **Purpose:** Get full manifest for a specific artifact (dependencies, entrypoints, provenance)
- **When to use:** To inspect a package before deployment
- **Classification:** read-only
- **Input:** `{ publisher_id: string, name: string, version: string (optional), platform: string (optional) }`
- **Output:** `{ manifest: { ref, provides, requires, defaults, entrypoints, service_detail/app_detail/infra_detail, provenance } }`
- **RBAC:** requires `sa` or authenticated user

#### `repository_get_artifact_versions`
- **Backed by:** `PackageRepository.GetArtifactVersions`
- **Purpose:** List all versions of an artifact
- **When to use:** To check available versions for upgrade planning
- **Classification:** read-only
- **Input:** `{ publisher_id: string (required), name: string (required) }`
- **Output:** `{ versions: [{ version, build_number, publish_state, published_unix, size_bytes }] }`
- **RBAC:** requires `sa` or authenticated user

### Backup Tools

#### `backup_list_jobs`
- **Backed by:** `BackupManagerService.ListBackupJobs`
- **Purpose:** List backup/restore/retention job history
- **When to use:** To check recent backup activity and job outcomes
- **Classification:** read-only
- **Input:** `{ state: string (optional: QUEUED/RUNNING/SUCCEEDED/FAILED/CANCELED), limit: int (default 20) }`
- **Output:** `{ jobs: [{ job_id, job_type, state, created_ms, started_ms, finished_ms, plan_name, backup_id, message }], total }`
- **RBAC:** requires `sa` or `backup-admin` role

#### `backup_get_job`
- **Backed by:** `BackupManagerService.GetBackupJob`
- **Purpose:** Get detailed status of a specific backup job including provider results
- **When to use:** To inspect why a backup/restore job failed or what it produced
- **Classification:** read-only
- **Input:** `{ job_id: string (required) }`
- **Output:** `{ job: { job_id, state, timestamps, results: [{ provider, state, summary, bytes_written, error }], backup_id } }`
- **RBAC:** requires `sa` or `backup-admin` role

#### `backup_list_backups`
- **Backed by:** `BackupManagerService.ListBackups`
- **Purpose:** List backup artifacts with quality state, size, provider coverage
- **When to use:** To assess backup inventory and recovery posture
- **Classification:** read-only
- **Input:** `{ mode: string (optional: SERVICE/CLUSTER), limit: int (default 20) }`
- **Output:** `{ backups: [{ backup_id, plan_name, created_ms, total_bytes, quality_state, provider_results, mode }], total }`
- **RBAC:** requires `sa` or `backup-admin` role

#### `backup_get_backup`
- **Backed by:** `BackupManagerService.GetBackup`
- **Purpose:** Get full detail of a backup artifact
- **When to use:** To inspect what a backup contains before restoring
- **Classification:** read-only
- **Input:** `{ backup_id: string (required) }`
- **Output:** `{ backup: { backup_id, timestamps, provider_results, manifest_sha256, total_bytes, quality_state, validation_report, restore_test_report, node_coverage } }`
- **RBAC:** requires `sa` or `backup-admin` role

#### `backup_validate_backup`
- **Backed by:** `BackupManagerService.ValidateBackup`
- **Purpose:** Run integrity validation on a backup artifact
- **When to use:** To verify a backup is valid before restoring
- **Classification:** read-only (validation does not modify state)
- **Input:** `{ backup_id: string (required), deep: bool (default false) }`
- **Output:** `{ valid: bool, issues: [{ severity, message, provider, detail }] }`
- **RBAC:** requires `sa` or `backup-admin` role

#### `backup_restore_plan`
- **Backed by:** `BackupManagerService.RestorePlan`
- **Purpose:** Preview what a restore would do without executing it
- **When to use:** Before restoring, to understand the impact and get warnings
- **Classification:** preview
- **Input:** `{ backup_id: string (required), include_etcd: bool, include_config: bool, include_minio: bool, include_scylla: bool }`
- **Output:** `{ steps: [{ provider, action, description }], warnings: [{ severity, message }], confirmation_token }`
- **RBAC:** requires `sa` or `backup-admin` role

#### `backup_get_retention_status`
- **Backed by:** `BackupManagerService.GetRetentionStatus`
- **Purpose:** Get current retention policy state (how many backups, total size, policy)
- **When to use:** To assess storage usage and retention health
- **Classification:** read-only
- **Input:** `{}` (no parameters)
- **Output:** `{ current_backup_count, current_total_bytes, policy: { keep_last_n, keep_days, max_total_bytes }, last_run_ms }`
- **RBAC:** requires `sa` or `backup-admin` role

#### `backup_preflight_check`
- **Backed by:** `BackupManagerService.PreflightCheck`
- **Purpose:** Verify all backup prerequisites (tools installed, storage accessible, services healthy)
- **When to use:** Before running a backup, or when diagnosing backup failures
- **Classification:** read-only
- **Input:** `{}` (no parameters)
- **Output:** `{ tools: [{ name, installed, version, path }], all_ok: bool }`
- **RBAC:** requires `sa` or `backup-admin` role

#### `backup_get_schedule_status`
- **Backed by:** `BackupManagerService.GetScheduleStatus`
- **Purpose:** Get backup schedule configuration and next run time
- **When to use:** To verify backups are scheduled and running on time
- **Classification:** read-only
- **Input:** `{}` (no parameters)
- **Output:** `{ enabled, cron_expression, next_run_ms, last_run_ms, last_result }`
- **RBAC:** requires `sa` or `backup-admin` role

#### `backup_get_recovery_status`
- **Backed by:** `BackupManagerService.GetRecoveryStatus`
- **Purpose:** Assess overall recovery readiness (latest valid backup, coverage, gaps)
- **When to use:** To answer "can we recover from a disaster right now?"
- **Classification:** read-only
- **Input:** `{}` (no parameters)
- **Output:** `{ recoverable: bool, latest_valid_backup_id, latest_backup_age_hours, coverage_gaps, warnings }`
- **RBAC:** requires `sa` or `backup-admin` role

#### `backup_list_minio_buckets`
- **Backed by:** `BackupManagerService.ListMinioBuckets`
- **Purpose:** List MinIO/S3 buckets used by backup storage
- **When to use:** To verify object storage is configured and accessible
- **Classification:** read-only
- **Input:** `{}` (no parameters)
- **Output:** `{ buckets: [{ name, created, size_bytes, object_count }] }`
- **RBAC:** requires `sa` or `backup-admin` role

#### `backup_test_scylla_connection`
- **Backed by:** `BackupManagerService.TestScyllaConnection`
- **Purpose:** Test connectivity to ScyllaDB for backup/restore operations
- **When to use:** When ScyllaDB backup/restore fails
- **Classification:** read-only
- **Input:** `{}` (no parameters)
- **Output:** `{ reachable: bool, cluster_name, datacenter, host, error }`
- **RBAC:** requires `sa` or `backup-admin` role

### Auth Tools

#### `auth_validate_token`
- **Backed by:** `AuthenticationService.ValidateToken`
- **Purpose:** Check if a token is valid and get its identity
- **When to use:** To diagnose "invalid token" / "ed25519" errors
- **Classification:** read-only
- **Input:** `{ token: string (required) }`
- **Output:** `{ valid: bool, client_id: string, expired: bool }`
- **RBAC:** none (the token is the credential)

### DNS Tools (optional)

#### `dns_get_domains`
- **Backed by:** `DnsService.GetDomains`
- **Purpose:** List configured DNS domains
- **When to use:** To verify DNS configuration
- **Classification:** read-only
- **Input:** `{}` (no parameters)
- **Output:** `{ domains: string[] }`
- **RBAC:** requires `sa` or `dns-admin` role

---

## Section 4: Composed Tools

These aggregate multiple RPCs into a single operator-oriented view.

### `cluster_get_operational_snapshot`
- **Composed from:** `GetClusterHealthV1` + `ListNodes` + `GetClusterReport` + `GetDesiredState` + `ListBackupJobs(limit=5)`
- **Purpose:** Single call to understand the full cluster operational posture
- **When to use:** First call in any diagnostic session. Gives the AI all context.
- **Output shape:**
```json
{
  "cluster": { "id", "domain", "created_at" },
  "health": { "overall_status", "node_count", "healthy", "unhealthy" },
  "nodes": [{ "node_id", "hostname", "status", "profiles", "last_seen" }],
  "desired_state": { "service_count", "revision" },
  "doctor": { "overall_status", "finding_count", "top_issues" },
  "recent_backups": [{ "job_id", "type", "state", "finished_ms" }]
}
```
- **Classification:** read-only
- **Rationale:** This is the highest-value tool. Instead of 5 separate calls, one snapshot gives the AI enough context to reason about what's wrong and what to investigate next.

### `cluster_get_node_full_status`
- **Composed from:** `GetNodeHealthDetailV1` + `ListInstalledPackages` + `GetPlanStatusV1` + `GetNodeReport`
- **Purpose:** Complete picture of a single node
- **When to use:** When investigating a specific node issue
- **Input:** `{ node_id: string (required) }`
- **Output shape:**
```json
{
  "health": { "status", "checks", "last_error" },
  "installed_packages": [{ "name", "version", "kind", "status" }],
  "current_plan": { "plan_id", "state", "progress" },
  "findings": [{ "finding_id", "severity", "summary" }]
}
```

### `backup_get_recovery_posture`
- **Composed from:** `GetRecoveryStatus` + `ListBackups(limit=3)` + `GetRetentionStatus` + `GetScheduleStatus`
- **Purpose:** Answer "are we protected?" in one call
- **When to use:** Backup health assessment
- **Output shape:**
```json
{
  "recoverable": true,
  "latest_backup": { "backup_id", "age_hours", "quality_state", "total_bytes" },
  "retention": { "backup_count", "total_bytes", "policy" },
  "schedule": { "enabled", "next_run_ms" },
  "warnings": []
}
```

---

## Section 5: Exclusion List

| RPC | Service | Reason |
|-----|---------|--------|
| `ReportNodeStatus` | ClusterController | Internal heartbeat; called by node-agent, not operators |
| `CompleteOperation` | ClusterController | Internal operation lifecycle |
| `ReportPlanRejection` | ClusterController | Internal plan verification |
| `WatchNodePlanStatusV1` | ClusterController | Server-stream; use `cluster_get_node_plan` snapshot instead |
| `WatchOperations` | ClusterController | Server-stream; use composed snapshot instead |
| `WatchPlanStatusV1` | NodeAgent | Server-stream |
| `WatchOperation` | NodeAgent | Server-stream |
| `UploadBundle` | Repository | Client-stream binary upload; not suitable for AI |
| `UploadArtifact` | Repository | Client-stream binary upload |
| `DownloadBundle` | Repository | Server-stream binary download |
| `DownloadArtifact` | Repository | Server-stream binary download |
| `Authenticate` | Authentication | Login flow; handled by MCP auth layer, not exposed as tool |
| `RefreshToken` | Authentication | Token lifecycle; handled by MCP auth layer |
| `GeneratePeerToken` | Authentication | Internal peer auth |
| `Stop` | BackupManager/DNS | Service lifecycle; not for AI |
| All `Set*/Remove*` DNS | DNS | Phase 1 is read-only; mutations deferred |

---

## Section 6: Adapter Architecture

### Overview

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────────────┐
│  AI Client   │────▶│  MCP Server (Go) │────▶│  gRPC Services      │
│  (Claude)    │◀────│  stdio transport  │◀────│  (existing Globular) │
└─────────────┘     └──────────────────┘     └─────────────────────┘
                           │
                    ┌──────┴──────┐
                    │ Tool Router  │
                    │ Auth Layer   │
                    │ Error Mapper │
                    │ Audit Log    │
                    │ Timeout Mgr  │
                    └─────────────┘
```

### Implementation: Go MCP server

- **Language:** Go (same as services, shares proto imports, no extra build toolchain)
- **Transport:** stdio (Claude Code native)
- **Binary:** `globular-mcp-server` — single binary, deployed alongside services
- **Config:** Reads Globular config for service endpoints and TLS

### Auth propagation

1. MCP server authenticates to Globular using the local SA token (`security.GetLocalToken`)
2. All gRPC calls carry the SA token in metadata
3. Per-tool RBAC is enforced by the existing service interceptors
4. The MCP server does NOT expose auth mutation tools in phase 1

### gRPC client management

- One shared gRPC connection per service, lazy-initialized
- TLS config from `config.GetEtcdTLS()` / standard Globular cert paths
- Connection health: probe on first use, reconnect on failure
- Idle timeout: 5 minutes

### Timeout model

- Default: 10 seconds per tool call
- Composed tools: 30 seconds (aggregate multiple RPCs)
- Configurable per-tool override

### Error translation

| gRPC Code | MCP Response |
|-----------|-------------|
| `OK` | Tool result with data |
| `NotFound` | Tool result with `{ error: "not found", detail: ... }` |
| `Unavailable` | Tool error: "Service unavailable — is {service} running?" |
| `DeadlineExceeded` | Tool error: "Request timed out — service may be overloaded" |
| `PermissionDenied` | Tool error: "Permission denied — check RBAC configuration" |
| `Unauthenticated` | Tool error: "Authentication failed — token may be expired" |
| Other | Tool error with gRPC status message |

### Stream handling

- **No raw streams exposed.** All streaming RPCs are replaced by snapshot/polling tools.
- Server-streams (`Watch*`) are not exposed; use the corresponding `Get*` snapshot RPCs.
- Client-streams (`Upload*`) are not exposed; binary transfer is not an AI operation.

### Audit logging

- Every tool invocation logged to `slog` with: tool name, input params (redacted), duration, result status
- Audit entries include a correlation ID for tracing
- Logged at INFO level; errors at WARN

### RBAC enforcement

- RBAC is enforced at the gRPC service level (existing interceptors)
- The MCP server adds tool-level metadata annotations but does NOT duplicate RBAC checks
- Phase 2 may add MCP-level policy (e.g., "allow AI to run mutating tools only with confirmation")

---

## Section 7: Phase 2 Roadmap

After phase 1 diagnostics are stable:

### Phase 2a: Preview/planning tools (safe mutations)
- `cluster_preview_service_upgrades` — preview upgrade plan
- `cluster_preview_desired_services` — preview desired-state delta
- `backup_run_restore_test` — test restore in isolation

### Phase 2b: Guarded operational tools (require confirmation)
- `cluster_approve_join` — approve pending join request
- `cluster_set_node_profiles` — change node profiles (with preview first)
- `backup_run_backup` — trigger manual backup
- `backup_run_retention` — trigger retention cleanup

### Phase 2c: Advanced operations (require explicit opt-in)
- `cluster_remove_node` — remove node (with drain)
- `cluster_upgrade` — initiate cluster upgrade
- `backup_restore_backup` — execute restore
- `repository_set_artifact_state` — change publish state

### Phase 2d: DNS tools
- `dns_list_records` — composed view of all record types for a domain
- `dns_get_record` — get specific record

### Missing RPCs to add before MCP

| Gap | Recommendation |
|-----|---------------|
| No `GetServiceConfig` RPC | Add to config package or gateway: read service config from etcd by ID |
| No `ListOperations` RPC | Add to ClusterController: list recent operations with status |
| No `GetClusterNetwork` read RPC | Already exists but not in phase 1; add to composed snapshot |
| No per-node `GetDriftReport` filter | ClusterDoctor already supports `node_id` param |

---

## Implementation Order

1. **Scaffold:** Go MCP server with stdio transport, tool registry, auth bootstrap
2. **Core composed tool:** `cluster_get_operational_snapshot` (highest value, proves the architecture)
3. **Cluster diagnostics:** `cluster_get_health`, `cluster_list_nodes`, `cluster_get_node_health_detail`
4. **Doctor:** `cluster_get_doctor_report`, `cluster_get_drift_report`, `cluster_explain_finding`
5. **Node agent:** `nodeagent_get_inventory`, `nodeagent_list_installed_packages`, `nodeagent_get_plan_status`
6. **Backup:** `backup_list_jobs`, `backup_list_backups`, `backup_get_recovery_posture`
7. **Repository:** `repository_list_artifacts`, `repository_search_artifacts`
8. **Auth/DNS:** `auth_validate_token`, `dns_get_domains`
