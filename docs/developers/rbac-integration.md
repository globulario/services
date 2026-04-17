# RBAC Integration

This page covers how to integrate Role-Based Access Control into your Globular microservice. It explains the annotation model, how permissions are enforced at runtime, built-in roles, and how to design resource hierarchies for your service.

## How RBAC Works in Globular

RBAC enforcement in Globular is declarative — you annotate your proto file with permission requirements, and the platform enforces them automatically. Your handler code does not need any authorization logic.

The flow:
1. You define `(globular.auth.authz)` annotations on each RPC in your proto file
2. Code generation extracts these annotations into permission descriptor files
3. At runtime, the gRPC interceptor reads the descriptors
4. For each incoming request, the interceptor:
   a. Extracts the caller's identity from the JWT token or mTLS certificate
   b. Resolves the resource path by substituting request field values into the template
   c. Queries the RBAC service for the caller's permissions on that resource
   d. Allows or denies the request before your handler runs

## Annotating Your Proto File

### Method-Level Authorization

Every RPC should have an `(globular.auth.authz)` annotation:

```protobuf
import "proto/globular_auth.proto";

rpc GetAsset(GetAssetRequest) returns (Asset) {
    option (globular.auth.authz) = {
        action: "inventory.asset.read"
        permission: "read"
        resource_template: "/inventory/assets/{asset_id}"
        default_role_hint: "viewer"
    };
}
```

**Fields**:

| Field | Purpose | Example |
|-------|---------|---------|
| `action` | Stable action key for permission matching. Convention: `<service>.<resource>.<verb>` | `"inventory.asset.read"` |
| `permission` | Permission level: `read`, `write`, `delete`, or `admin` | `"read"` |
| `resource_template` | Resource path with `{field}` placeholders. Resolved from request message fields at runtime. | `"/inventory/assets/{asset_id}"` |
| `default_role_hint` | Default minimum role. Used by the code generator to assign this action to built-in roles. | `"viewer"`, `"editor"`, `"manager"`, `"admin"` |

### Field-Level Resource Metadata

Request fields that identify a resource should be annotated:

```protobuf
message GetAssetRequest {
    string asset_id = 1 [(globular.auth.resource) = {
        kind: "asset"
        scope_anchor: true
    }];
}
```

**Fields**:
- `kind`: The type of resource this field identifies (e.g., "asset", "connection", "item")
- `scope_anchor`: If `true`, this field is the primary scope anchor for RBAC resolution. The interceptor uses scope anchors to determine the resource hierarchy.

### Collection vs Instance Resources

Distinguish between collection operations (list, create) and instance operations (get, update, delete):

```protobuf
// Collection: /inventory/assets (no {asset_id})
rpc ListAssets(...) returns (...) {
    option (globular.auth.authz) = {
        action: "inventory.asset.list"
        permission: "read"
        resource_template: "/inventory/assets"
        collection_template: "/inventory/assets"
    };
}

// Instance: /inventory/assets/{asset_id}
rpc GetAsset(...) returns (...) {
    option (globular.auth.authz) = {
        action: "inventory.asset.read"
        permission: "read"
        resource_template: "/inventory/assets/{asset_id}"
    };
}
```

## Resource Hierarchy

### Designing Resource Paths

Resource paths in Globular are hierarchical, similar to URL paths. A well-designed hierarchy enables:
- **Inheritance**: Permission on a parent path grants permission on all children
- **Scoping**: Operators can grant narrow permissions on specific resources
- **Ownership**: Resource owners automatically have all permissions

Example hierarchy for an inventory service:

```
/inventory                          # Service root
/inventory/assets                   # Asset collection
/inventory/assets/{asset_id}        # Specific asset
/inventory/categories               # Category collection
/inventory/categories/{category_id} # Specific category
/inventory/reports                  # Report collection
```

### Permission Inheritance

A user with `write` permission on `/inventory/assets` automatically has `write` permission on `/inventory/assets/{any_asset_id}`. This means:
- Granting access at the collection level covers all instances
- Granting access at a specific instance only covers that instance

### Ownership Chain

The RBAC service supports ownership-based access. If a resource has an owner, the owner has full permissions on it:

```go
// The RBAC service checks:
// 1. Is the caller the owner of /inventory/assets/asset-123?
// 2. Is the caller the owner of /inventory/assets (parent)?
// 3. Is the caller the owner of /inventory (grandparent)?
// If any level returns true, access is granted.
```

## Built-In Roles

The platform defines built-in roles that cover common access patterns:

| Role | Permissions | Use Case |
|------|------------|----------|
| `globular-admin` | `/*` (all resources, all permissions) | Cluster administrators |
| `globular-publisher` | Artifact upload and lifecycle management | CI/CD pipelines |
| `globular-operator` | Manage releases, desired state, domains | Day-2 operators |
| `globular-controller-sa` | Read/apply state (no publish) | Cluster Controller internal |
| `globular-node-agent-sa` | Report status, execute plans | Node Agent internal |

### Custom Roles

The code generator creates role-to-action mappings based on the `default_role_hint` field:

```protobuf
// This RPC is added to the "viewer" role's permitted actions
rpc GetAsset(...) returns (...) {
    option (globular.auth.authz) = {
        default_role_hint: "viewer"  // viewer, editor, manager, or admin
    };
}
```

Generated roles (in `cluster-roles.generated.json`):
- `inventory-viewer`: Can call RPCs with `default_role_hint: "viewer"` (read operations)
- `inventory-editor`: Can call viewer RPCs + `default_role_hint: "editor"` RPCs (create, update)
- `inventory-admin`: Can call all RPCs including `default_role_hint: "admin"` (delete, configure)

### Wildcard Permissions

The permission matching system supports wildcards:

```json
{
  "globular-admin": ["/*"],
  "inventory-editor": ["inventory.*", "inventory.asset.create", "inventory.asset.update"],
  "inventory-viewer": ["inventory.asset.read", "inventory.asset.list"]
}
```

- `/*` — matches all actions
- `inventory.*` — matches all actions starting with `inventory.`
- `inventory.asset.read` — matches exactly this action

## Runtime Enforcement

### Interceptor Flow

For each incoming gRPC request:

1. **Extract identity**: JWT → PrincipalID, or mTLS → CN
2. **Look up method mapping**: Find the RBAC descriptor for this gRPC method
3. **Resolve resource path**: Substitute `{field}` placeholders from the request message
   - For `resource_template: "/inventory/assets/{asset_id}"` and a request with `asset_id: "abc123"`
   - Resolved path: `/inventory/assets/abc123`
4. **Check RBAC**: Call the RBAC service with (subject, resource_path, permission)
5. **Allow or deny**: Based on the RBAC response

### RBAC Service Fallback

If the RBAC service is unreachable (startup, network issue), the interceptor falls back to local cluster-roles:
1. Load `cluster-roles.generated.json` from disk
2. Check if any of the caller's roles grant the required action
3. This prevents a circular dependency at startup (services need RBAC, RBAC needs etcd, etcd needs TLS...)

### Deny Overrides Allow

If a subject has both an explicit allow and an explicit deny on the same resource, the deny wins. This enables fine-grained exceptions.

## Code Generation

When you run `./generateCode.sh`, the `authzgen` tool:

1. Reads all proto files with `(globular.auth.authz)` annotations
2. Extracts action, permission, resource_template, and default_role_hint for each RPC
3. Generates `permissions.generated.json` — maps gRPC methods to their RBAC requirements
4. Generates `cluster-roles.generated.json` — maps roles to permitted actions

These files are loaded by the interceptor at service startup.

### Overriding Generated Roles

Administrators can override generated role permissions by placing a custom file at:
```
/etc/globular/policy/rbac/cluster-roles.json
```

This file takes precedence over the generated file. It's useful for:
- Adding custom roles not derived from proto annotations
- Modifying default role assignments
- Adding organization-specific permissions

## Practical Scenarios

### Scenario 1: Granting Access to a New User

A new team member needs read access to the inventory service:

```bash
# Create the user account
globular auth create-account --username alice --type user

# Bind the viewer role
globular rbac bind --subject alice --role inventory-viewer

# Alice can now:
# - GetAsset (read)
# - ListAssets (read)
# But NOT:
# - CreateAsset (write — requires inventory-editor)
# - DeleteAsset (delete — requires inventory-admin)
```

### Scenario 2: Service-to-Service Authorization

Your inventory service needs to call the persistence service:

```bash
# The inventory service runs with a service account
# When it calls persistence.PersistenceService/FindOne:
# 1. Inventory generates a service token (5-min TTL)
# 2. Sends the token in the gRPC metadata
# 3. Persistence interceptor validates the token
# 4. Persistence checks RBAC for the inventory service account
# 5. The service account needs the appropriate role for persistence operations
```

### Scenario 3: Resource-Scoped Permissions

Grant a user access to only specific assets:

```bash
# Grant write access to asset "laptop-001" only
globular rbac set-permission \
  --subject bob \
  --resource "/inventory/assets/laptop-001" \
  --permission write

# Bob can update laptop-001 but not other assets
```

## What's Next

- [Application Deployment Model](developers/application-deployment.md): Deploy web applications
- [Workflow Integration](developers/workflow-integration.md): Custom workflow steps
