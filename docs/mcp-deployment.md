# Globular MCP Server — Deployment Architecture

## Cluster Topology

```
Claude / remote MCP client
        |
      HTTPS (port 8443)
        |
      Envoy (gateway)
        |
        v
globular-mcp-server :10250 (HTTP transport)
        |
        +-- ClusterControllerService :12000
        +-- ClusterDoctorService
        +-- NodeAgentService :11000
        +-- PackageRepository
        +-- BackupManagerService
        +-- RBAC / Resource services
        +-- FileService
        +-- PersistenceService
        +-- StorageService
```

## Key Principles

1. **One MCP endpoint per cluster** — not one per node
2. **Exposed through Envoy** over HTTPS — not directly
3. **Admin/ops component** — not day-0 core dependency
4. **Read-only phase 1** — no mutations
5. **Least-privilege identity** — dedicated `mcp-operator` service account
6. **Internal service access** — uses normal Globular gRPC service discovery

## Node-Local Policy

Per-node MCP servers are **not needed**. The cluster-facing MCP service queries node agents and services directly via gRPC. Claude connects to exactly one MCP endpoint.

If node-local diagnostic helpers exist, they are internal-only and not registered in Claude.

## Transport Modes

| Mode | Use Case | Config |
|------|----------|--------|
| **HTTP** (default for cluster) | Remote MCP clients via Envoy | `transport: "http"`, `http_listen_addr: ":10250"` |
| **stdio** (default for dev) | Local Claude Code | `transport: "stdio"` |

## Envoy Integration

### Recommended: Dedicated admin subdomain

Route `mcp.globular.internal` (or `mcp.<cluster-domain>`) to the MCP service.

Envoy virtual host:
```yaml
- name: mcp-admin
  domains: ["mcp.globular.internal", "mcp.*"]
  routes:
    - match: { prefix: "/mcp" }
      route:
        cluster: globular-mcp
        timeout: 60s
    - match: { prefix: "/health" }
      route:
        cluster: globular-mcp
        timeout: 5s
```

### HTTP route shape

The MCP server serves `POST /mcp` for JSON-RPC requests and `GET /health` for health checks. These are the only two routes. Envoy should proxy directly to these paths without rewriting.

### Phase-1 upstream topology

Phase 1 assumes a **single MCP instance** per cluster. There is no need for round-robin or horizontal scaling. Use a single stable upstream endpoint:

Envoy cluster:
```yaml
- name: globular-mcp
  type: STATIC
  load_assignment:
    cluster_name: globular-mcp
    endpoints:
      - lb_endpoints:
          - endpoint:
              address:
                socket_address:
                  address: 127.0.0.1
                  port_value: 10250
```

### Security boundary

- HTTPS termination at Envoy
- Plain HTTP between Envoy and MCP server (loopback only)
- Internal auth: MCP server uses its own SA token for gRPC calls to backend services
- Client auth: see below

### Client authentication (default: token header)

Phase 1 uses **token-header authentication** as the default:

1. The MCP client sends an authorization token in the `token` HTTP header
2. Envoy forwards the header to the MCP server
3. The MCP server validates the token using the authentication service
4. The caller identity from the token is included in audit log entries

This is simpler than mTLS for initial deployment and matches the existing Globular auth model. mTLS at Envoy is a phase-2 option for higher-security environments.

To propagate caller identity to audit logs, the MCP HTTP handler extracts the `token` header and includes the decoded principal in audit entries. If no token is present (e.g., localhost dev mode), audit logs record `caller: "anonymous"`.

## Service Identity & RBAC

### Phase-1 default: SA token

In phase 1, the MCP server authenticates to backend services using the local SA token (`security.GetLocalToken`). This provides broad read access and is acceptable for single-node and trusted-admin deployments.

### Phase-2 target: Dedicated `mcp-operator` identity

For production multi-node deployments, create a least-privilege service account:

#### Step 1: Create the `mcp-reader` role

```bash
# Create role with all read-only RPCs used by MCP tools
globular rbac role create mcp-reader \
  --action "/cluster_controller.ClusterControllerService/GetClusterInfo" \
  --action "/cluster_controller.ClusterControllerService/GetClusterHealthV1" \
  --action "/cluster_controller.ClusterControllerService/GetNodeHealthDetailV1" \
  --action "/cluster_controller.ClusterControllerService/ListNodes" \
  --action "/cluster_controller.ClusterControllerService/GetNodePlanV1" \
  --action "/cluster_controller.ClusterControllerService/GetDesiredState" \
  --action "/cluster_controller.ClusterControllerService/PreviewNodeProfiles" \
  --action "/cluster_doctor.ClusterDoctorService/GetClusterReport" \
  --action "/cluster_doctor.ClusterDoctorService/GetNodeReport" \
  --action "/cluster_doctor.ClusterDoctorService/GetDriftReport" \
  --action "/cluster_doctor.ClusterDoctorService/ExplainFinding" \
  --action "/node_agent.NodeAgentService/GetInventory" \
  --action "/node_agent.NodeAgentService/GetPlanStatusV1" \
  --action "/node_agent.NodeAgentService/ListInstalledPackages" \
  --action "/node_agent.NodeAgentService/GetInstalledPackage" \
  --action "/repository.PackageRepository/ListArtifacts" \
  --action "/repository.PackageRepository/SearchArtifacts" \
  --action "/repository.PackageRepository/GetArtifactManifest" \
  --action "/repository.PackageRepository/GetArtifactVersions" \
  --action "/repository.PackageRepository/ListBundles" \
  --action "/repository.PackageRepository/GetNamespace" \
  --action "/backup_manager.BackupManagerService/ListBackupJobs" \
  --action "/backup_manager.BackupManagerService/GetBackupJob" \
  --action "/backup_manager.BackupManagerService/ListBackups" \
  --action "/backup_manager.BackupManagerService/GetBackup" \
  --action "/backup_manager.BackupManagerService/ValidateBackup" \
  --action "/backup_manager.BackupManagerService/RestorePlan" \
  --action "/backup_manager.BackupManagerService/GetRetentionStatus" \
  --action "/backup_manager.BackupManagerService/PreflightCheck" \
  --action "/backup_manager.BackupManagerService/GetScheduleStatus" \
  --action "/backup_manager.BackupManagerService/GetRecoveryStatus" \
  --action "/backup_manager.BackupManagerService/ListMinioBuckets" \
  --action "/backup_manager.BackupManagerService/TestScyllaConnection" \
  --action "/rbac.RbacService/ValidateAccess" \
  --action "/rbac.RbacService/ValidateAction" \
  --action "/rbac.RbacService/GetActionResourceInfos" \
  --action "/rbac.RbacService/GetResourcePermissions" \
  --action "/rbac.RbacService/GetResourcePermissionsBySubject" \
  --action "/rbac.RbacService/GetResourcePermissionsByResourceType" \
  --action "/rbac.RbacService/GetRoleBinding" \
  --action "/rbac.RbacService/ListRoleBindings" \
  --action "/resource.ResourceService/GetAccount" \
  --action "/resource.ResourceService/GetRoles" \
  --action "/resource.ResourceService/GetGroups" \
  --action "/resource.ResourceService/GetOrganizations" \
  --action "/file.FileService/GetFileInfo" \
  --action "/file.FileService/GetFileMetadata" \
  --action "/file.FileService/ReadDir" \
  --action "/file.FileService/ReadFile" \
  --action "/file.FileService/GetPublicDirs" \
  --action "/persistence.PersistenceService/Ping" \
  --action "/persistence.PersistenceService/FindOne" \
  --action "/persistence.PersistenceService/Find" \
  --action "/persistence.PersistenceService/Count" \
  --action "/persistence.PersistenceService/Aggregate" \
  --action "/storage.StorageService/GetItem" \
  --action "/storage.StorageService/GetAllKeys"
```

#### Step 2: Create the `mcp-operator` service account

```bash
# Register the account
globular resource register-account mcp-operator --email mcp@globular.internal

# Bind the role
globular rbac role-binding set mcp-operator --role mcp-reader
```

#### Step 3: Generate a token for the MCP service

```bash
# Generate a long-lived service token
globular auth generate-peer-token --subject mcp-operator --ttl 8760h > /var/lib/globular/mcp/token
chmod 600 /var/lib/globular/mcp/token
chown globular:globular /var/lib/globular/mcp/token
```

#### Step 4: Configure the MCP service to use the token

Add to `/var/lib/globular/mcp/config.json`:
```json
{
  "service_token_path": "/var/lib/globular/mcp/token"
}
```

### Required permissions by group

| Group | Services Accessed | RPCs |
|-------|-------------------|------|
| cluster | ClusterController | GetClusterInfo, GetClusterHealthV1, ListNodes, GetNodeHealthDetailV1, GetNodePlanV1, GetDesiredState, PreviewNodeProfiles |
| doctor | ClusterDoctor | GetClusterReport, GetNodeReport, GetDriftReport, ExplainFinding |
| nodeagent | NodeAgent | GetInventory, GetPlanStatusV1, ListInstalledPackages, GetInstalledPackage |
| repository | PackageRepository | ListArtifacts, SearchArtifacts, GetArtifactManifest, GetArtifactVersions, ListBundles, GetNamespace |
| backup | BackupManager | ListBackupJobs, GetBackupJob, ListBackups, GetBackup, ValidateBackup, RestorePlan, GetRetentionStatus, PreflightCheck, GetScheduleStatus, GetRecoveryStatus, ListMinioBuckets, TestScyllaConnection |
| rbac | RBAC | ValidateAccess, ValidateAction, GetActionResourceInfos, GetResourcePermissions, GetResourcePermissionsBySubject, GetResourcePermissionsByResourceType, GetRoleBinding, ListRoleBindings |
| resource | Resource | GetAccount, GetRoles, GetGroups, GetOrganizations |
| file | File | GetFileInfo, GetFileMetadata, ReadDir, ReadFile, GetPublicDirs |
| persistence | Persistence | Ping, FindOne, Find, Count, Aggregate |
| storage | Storage | GetItem, GetAllKeys |

## Profile Placement

| Profile | MCP Included? |
|---------|--------------|
| core / day-0 minimal | No |
| control-plane | No |
| gateway | No |
| **admin / ops** | **Yes** |
| full | Yes |

## Operational Rollout

### Phase 1: Local development
```json
{"transport": "stdio", "tool_groups": {"cluster": true, "..."}}
```
Configure in `.claude/settings.json`:
```json
{"mcpServers": {"globular": {"command": "globular-mcp-server"}}}
```

### Phase 2: First cluster deployment
1. Install MCP package: `globular pkg install mcp_service`
2. Configure: edit `/var/lib/globular/mcp/config.json`
3. Start: `systemctl start globular-mcp.service`
4. Add Envoy route for `mcp.<domain>`
5. Configure Claude with remote MCP endpoint

### Phase 3: Admin/ops profile integration
- Add to admin/ops profile manifest
- Auto-deployed on admin nodes
- Auto-routed through Envoy

## What Must NOT Be Exposed

- Mutation tools (phase 1)
- Arbitrary file system access (without allowlist)
- Arbitrary database queries (without allowlist)
- Raw credentials, tokens, keys
- Bootstrap internals
- Binary upload/download
- Shell command execution
