# RBAC: Roles and Permissions for Service Developers

Globular's RBAC system is one of its strongest architectural advantages. You annotate your proto file with authorization requirements, and the platform enforces them automatically — in the interceptor, before your handler ever runs. Your handler code has no authorization logic whatsoever. New services get full, auditable, role-based access control for free.

This document explains the full model, walks through building a complete service with RBAC, covers advanced patterns, and shows how to design roles that an operator can actually manage.

---

## Why This Matters

Most frameworks put authorization inside handlers:

```go
// What you DON'T write in Globular:
func (s *server) DeleteAsset(ctx context.Context, req *pb.DeleteAssetRequest) (*pb.DeleteAssetResponse, error) {
    caller := auth.CallerFrom(ctx)
    if !caller.HasPermission("inventory.asset.delete") {
        return nil, status.Error(codes.PermissionDenied, "not allowed")
    }
    // ... actual handler logic
}
```

This creates invisible policy — authorization logic scattered across hundreds of handlers, untestable without running the full service, and impossible to audit centrally. When you need to know "what can alice do?", you have to read every handler.

In Globular, authorization is declared once in the proto file and enforced once in the interceptor:

```protobuf
rpc DeleteAsset(DeleteAssetRequest) returns (DeleteAssetResponse) {
    option (globular.auth.authz) = {
        action: "inventory.asset.delete"
        permission: "delete"
        resource_template: "/inventory/assets/{asset_id}"
        default_role_hint: "admin"
    };
}
```

The platform reads this annotation at startup, enforces it on every request, and records every decision. An operator can query the live RBAC service to understand exactly what any subject is and isn't allowed to do — without reading source code.

---

## Core Concepts

### Actions

An **action key** is a stable, human-readable string that identifies what a caller is doing. Convention: `<service>.<resource>.<verb>`.

```
inventory.asset.read
inventory.asset.create
inventory.asset.update
inventory.asset.delete
inventory.category.list
inventory.report.generate
```

Action keys are what roles grant. They are intentionally decoupled from gRPC method names — your method can be renamed, moved to a different service, or split into two without invalidating any role bindings. The action key is the stable contract.

**Wildcards**: The permission system supports wildcard matching:

| Pattern | Matches |
|---|---|
| `/*` | Every action in every service |
| `inventory.*` | Every action in the inventory service |
| `inventory.asset.*` | Every action on the inventory.asset resource |
| `inventory.asset.read` | Exactly this action |

`globular-admin` holds `/*`. A service admin role typically holds `inventory.*`. A viewer role holds individual read actions.

### Resources

A **resource path** is a hierarchical URL-like string that identifies the thing being acted on:

```
/inventory
/inventory/assets
/inventory/assets/laptop-001
/inventory/categories
/inventory/categories/electronics
```

Resource paths enable **permission inheritance**: access to a parent grants access to all children. A subject with `write` on `/inventory/assets` can write any asset. A subject with `write` on `/inventory/assets/laptop-001` can only write that one asset.

Resource paths also enable **ownership**: a subject that creates a resource becomes its owner and has full permissions on it regardless of role bindings.

### The Two-Tier Model

Globular uses **two complementary permission models** that work together:

**Tier 1 — Action-based (roles):** "Does this subject's role permit this action?" This is fast and coarse-grained. It answers: "Can alice call `DeleteAsset` at all?"

**Tier 2 — Resource-based (permissions):** "Does this subject have permission on this specific resource path?" This is precise and fine-grained. It answers: "Can alice delete *this* asset specifically?"

The interceptor checks tier 1 first (role → action match). If that passes, it checks tier 2 (subject → resource permission). Both must pass. This means:
- Giving someone a role opens the capability class
- Resource permissions can further restrict to specific instances
- An explicit deny at tier 2 blocks access even if tier 1 passes

---

## A Complete Example: Inventory Service

Let's build an inventory service from scratch with full RBAC.

### Step 1: Design the Resource Hierarchy

```
/inventory                          # Service root
├── /inventory/assets               # Asset collection
│   └── /inventory/assets/{id}     # Specific asset instance
└── /inventory/categories           # Category collection
    └── /inventory/categories/{id} # Specific category instance
```

Design principles:
- Keep it shallow — 2–3 levels is usually enough
- Mirror your domain objects — one path level per domain aggregate
- Consistent naming — plural nouns for collections, IDs for instances

### Step 2: Define Action Keys

Map each gRPC method to an action key:

| gRPC Method | Action Key | Verb |
|---|---|---|
| `ListAssets` | `inventory.asset.list` | read |
| `GetAsset` | `inventory.asset.read` | read |
| `CreateAsset` | `inventory.asset.create` | write |
| `UpdateAsset` | `inventory.asset.update` | write |
| `DeleteAsset` | `inventory.asset.delete` | delete |
| `ListCategories` | `inventory.category.list` | read |
| `GetCategory` | `inventory.category.read` | read |
| `CreateCategory` | `inventory.category.create` | write |
| `DeleteCategory` | `inventory.category.delete` | delete |
| `GenerateReport` | `inventory.report.generate` | admin |

### Step 3: Annotate the Proto File

```protobuf
syntax = "proto3";
package inventory;

import "proto/globular_auth.proto";

service InventoryService {

    // ─── Assets ─────────────────────────────────────────────────────────────

    rpc ListAssets(ListAssetsRequest) returns (stream Asset) {
        option (globular.auth.authz) = {
            action: "inventory.asset.list"
            permission: "read"
            resource_template: "/inventory/assets"
            default_role_hint: "viewer"
        };
    }

    rpc GetAsset(GetAssetRequest) returns (Asset) {
        option (globular.auth.authz) = {
            action: "inventory.asset.read"
            permission: "read"
            resource_template: "/inventory/assets/{asset_id}"
            default_role_hint: "viewer"
        };
    }

    rpc CreateAsset(CreateAssetRequest) returns (Asset) {
        option (globular.auth.authz) = {
            action: "inventory.asset.create"
            permission: "write"
            resource_template: "/inventory/assets"
            default_role_hint: "editor"
        };
    }

    rpc UpdateAsset(UpdateAssetRequest) returns (Asset) {
        option (globular.auth.authz) = {
            action: "inventory.asset.update"
            permission: "write"
            resource_template: "/inventory/assets/{asset_id}"
            default_role_hint: "editor"
        };
    }

    rpc DeleteAsset(DeleteAssetRequest) returns (DeleteAssetResponse) {
        option (globular.auth.authz) = {
            action: "inventory.asset.delete"
            permission: "delete"
            resource_template: "/inventory/assets/{asset_id}"
            default_role_hint: "admin"
        };
    }

    // ─── Reports ─────────────────────────────────────────────────────────────

    rpc GenerateReport(GenerateReportRequest) returns (Report) {
        option (globular.auth.authz) = {
            action: "inventory.report.generate"
            permission: "admin"
            resource_template: "/inventory/reports"
            default_role_hint: "manager"
        };
    }
}

// ─── Messages ─────────────────────────────────────────────────────────────────

message GetAssetRequest {
    string asset_id = 1 [(globular.auth.resource) = {
        kind: "asset"
        scope_anchor: true      // This field determines the resource path
    }];
}

message UpdateAssetRequest {
    string asset_id = 1 [(globular.auth.resource) = {
        kind: "asset"
        scope_anchor: true
    }];
    Asset asset = 2;
}

message DeleteAssetRequest {
    string asset_id = 1 [(globular.auth.resource) = {
        kind: "asset"
        scope_anchor: true
    }];
}
```

### Step 4: Run Code Generation

```bash
./generateCode.sh
```

This produces two files in your service's package directory:

**`permissions.generated.json`** — maps gRPC methods to RBAC requirements at runtime:

```json
{
  "version": "1",
  "service": "inventory.InventoryService",
  "permissions": [
    {
      "method": "/inventory.InventoryService/GetAsset",
      "action": "inventory.asset.read",
      "permission": "read",
      "resource_template": "/inventory/assets/{asset_id}"
    },
    {
      "method": "/inventory.InventoryService/DeleteAsset",
      "action": "inventory.asset.delete",
      "permission": "delete",
      "resource_template": "/inventory/assets/{asset_id}"
    }
  ]
}
```

**`cluster-roles.generated.json`** — maps generated roles to action sets:

```json
{
  "version": "2.0",
  "roles": {
    "inventory-viewer": [
      "inventory.asset.list",
      "inventory.asset.read"
    ],
    "inventory-editor": [
      "inventory.asset.list",
      "inventory.asset.read",
      "inventory.asset.create",
      "inventory.asset.update"
    ],
    "inventory-manager": [
      "inventory.asset.list",
      "inventory.asset.read",
      "inventory.asset.create",
      "inventory.asset.update",
      "inventory.report.generate"
    ],
    "inventory-admin": [
      "inventory.*"
    ]
  }
}
```

### Step 5: Seed the Roles

After deploying your service, an operator seeds the generated roles into the RBAC store:

```bash
globular rbac seed --force
```

This reads all `cluster-roles.generated.json` files from installed services and writes the role-to-action mappings into the RBAC service's ScyllaDB store. After seeding, operators can assign these roles to users:

```bash
globular rbac bind --subject alice --role inventory-viewer
globular rbac bind --subject bob --role inventory-editor
globular rbac bind --subject charlie --role inventory-admin
```

### Step 6: Your Handler Needs No Auth Logic

```go
func (s *server) DeleteAsset(ctx context.Context, req *pb.DeleteAssetRequest) (*pb.DeleteAssetResponse, error) {
    // If we're here, the interceptor already checked:
    // - Is the caller authenticated?
    // - Does the caller's role allow inventory.asset.delete?
    // - Does the caller have permission on /inventory/assets/{req.AssetId}?
    // All three must pass. We just do the work.

    if err := s.store.Delete(req.AssetId); err != nil {
        return nil, status.Errorf(codes.Internal, "delete failed: %v", err)
    }
    return &pb.DeleteAssetResponse{}, nil
}
```

---

## Annotation Reference

### `(globular.auth.authz)` — Method annotation

Every RPC should have this annotation. If it's missing, the interceptor will deny the request by default (when running in strict mode).

| Field | Type | Required | Description |
|---|---|---|---|
| `action` | string | yes | Stable action key. Convention: `<service>.<resource>.<verb>`. Must match `[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+` |
| `permission` | string | yes | Required permission level: `read`, `write`, `delete`, or `admin` |
| `resource_template` | string | yes | Resource path template with `{field_name}` placeholders. Resolved from the request message at runtime |
| `default_role_hint` | string | recommended | Maps this action to a generated role tier: `viewer`, `editor`, `manager`, or `admin` |

**Role hint tiers:**

| Hint | Generated role | Typical actions |
|---|---|---|
| `viewer` | `<service>-viewer` | read, list |
| `editor` | `<service>-editor` | create, update (includes viewer) |
| `manager` | `<service>-manager` | advanced write, generate reports (includes editor) |
| `admin` | `<service>-admin` | delete, configure, full service access |

### `(globular.auth.resource)` — Field annotation

Marks a request field as a resource identifier. Used by the interceptor to resolve `{field_name}` placeholders in resource templates.

| Field | Type | Description |
|---|---|---|
| `kind` | string | Resource type (e.g., `"asset"`, `"category"`, `"report"`) |
| `scope_anchor` | bool | If `true`, this field is the primary scope for RBAC resolution. Only one field per message should be the anchor |

### Collection vs. Instance Resource Templates

Distinguish list/create (collection) from get/update/delete (instance):

```protobuf
// Collection operation — no ID in the path
rpc ListAssets(ListAssetsRequest) returns (stream Asset) {
    option (globular.auth.authz) = {
        action: "inventory.asset.list"
        permission: "read"
        resource_template: "/inventory/assets"   // No {asset_id}
    };
}

// Instance operation — ID in the path
rpc GetAsset(GetAssetRequest) returns (Asset) {
    option (globular.auth.authz) = {
        action: "inventory.asset.read"
        permission: "read"
        resource_template: "/inventory/assets/{asset_id}"   // Has {asset_id}
    };
}
```

This matters for resource-level permissions. A user with permission on `/inventory/assets` can list all assets. A user with permission on `/inventory/assets/laptop-001` can only access that instance. Both paths are valid RBAC targets.

---

## Permission Inheritance and Ownership

### Inheritance in Practice

```
Permission granted on:        Also applies to:
/inventory                    /inventory/assets, /inventory/assets/*, /inventory/categories/*
/inventory/assets             /inventory/assets/laptop-001, /inventory/assets/server-002
/inventory/assets/laptop-001  (only that path)
```

Use this to grant a team broad access to a department's resources without enumerating every item:

```bash
# Grant the hardware team write access to everything in the hardware category
globular rbac set-permission \
  --subject hardware-team \
  --resource "/inventory/categories/hardware" \
  --permission write
```

### Ownership Chain

When a user creates a resource, they can be set as its owner. An owner has full permissions on the resource and all children, regardless of role:

```go
// In your CreateAsset handler, after the resource is created:
// The interceptor can automatically add resource ownership if the
// AddResourceOwner RPC is called. Or your service can call it explicitly:
rbacClient.AddResourceOwner(ctx, &rbacpb.AddResourceOwnerRequest{
    Path:    fmt.Sprintf("/inventory/assets/%s", newAsset.Id),
    Subject: authCtx.Subject,
})
```

After this, the creator can always read/update/delete their own asset, even if their role doesn't explicitly grant it.

### Deny Rules

Deny rules override allows, enabling fine-grained exceptions:

```bash
# Charlie has inventory-editor globally...
globular rbac bind --subject charlie --role inventory-editor

# ...but is explicitly denied access to the confidential hardware inventory
globular rbac set-permission \
  --subject charlie \
  --resource "/inventory/categories/confidential-hardware" \
  --permission deny
```

Charlie can update any asset except those under `/inventory/categories/confidential-hardware`. The deny at the resource level wins over the role-based allow.

---

## Role Design Patterns

### Pattern 1: The Four-Tier Service Role

Most services fit a four-tier model generated from `default_role_hint`:

```
<service>-viewer   → read, list
<service>-editor   → create, update (+ viewer)
<service>-manager  → advanced ops, reports (+ editor)
<service>-admin    → delete, configure (+ all above)
```

The code generator creates all four automatically. Operators assign the appropriate tier per user.

### Pattern 2: Functional Roles (cross-service)

When users need a consistent capability across multiple services (e.g., "all read access"), define a cross-service role in `/etc/globular/policy/rbac/cluster-roles.json`:

```json
{
  "roles": {
    "my-readonly-analyst": [
      "inventory.asset.read",
      "inventory.asset.list",
      "warehouse.stock.read",
      "warehouse.stock.list",
      "analytics.report.read"
    ]
  }
}
```

This is more precise than assigning three separate viewer roles and makes it clear this is a purpose-built access pattern.

### Pattern 3: Service Account Roles

Internal services that call other services need roles too. Design these to be minimal — grant only the specific actions the service needs:

```json
{
  "roles": {
    "inventory-sync-sa": [
      "inventory.asset.read",
      "inventory.asset.list",
      "warehouse.stock.read"
    ]
  }
}
```

Bind the role to the service's identity:

```bash
globular rbac bind --subject inventory-sync-service --role inventory-sync-sa
```

The sync service calls other services with its Ed25519 service token, which carries its identity. The receiving service's interceptor validates the token and checks RBAC with the service's subject.

### Pattern 4: Scoped Admin (per-team)

If you want `alice` to administer only the inventory service but not the whole cluster:

```bash
globular rbac bind --subject alice --role inventory-admin
# alice gets: inventory.*
# alice does NOT get: cluster_controller.*, repository.*, rbac.*, etc.
```

Alice can do anything in the inventory service but cannot touch other services. This is fundamentally safer than assigning `globular-admin`.

---

## Service-to-Service Authorization

When your service calls another service, it needs a token. Globular generates short-lived Ed25519 service tokens automatically:

```go
import "github.com/globulario/services/golang/security"

// Get the current service token (cached, refreshed automatically, 5-min TTL)
token, err := security.GetServiceToken()
if err != nil {
    return nil, fmt.Errorf("get service token: %w", err)
}

// Attach it to the outgoing call
ctx := metadata.AppendToOutgoingContext(ctx, "token", token)
resp, err := otherServiceClient.DoSomething(ctx, req)
```

The token carries the service's identity (its MAC-derived subject). The receiving service's interceptor validates the token and checks RBAC using that subject.

**Important**: Service tokens have a 5-minute TTL and are scoped to the peer MAC address as the audience. They cannot be replayed to a different service or after they expire.

---

## Runtime Enforcement Details

### How the Interceptor Resolves Resource Paths

For a request with `resource_template: "/inventory/assets/{asset_id}"` and `asset_id: "laptop-001"`:

1. The interceptor reads `asset_id` from the request message via reflection
2. It substitutes `{asset_id}` → `"laptop-001"`
3. The resolved path is `/inventory/assets/laptop-001`
4. This path is passed to `ValidateAccess(subject, "/inventory/assets/laptop-001", "delete")`

The RBAC service then checks:
1. Does `subject` own `/inventory/assets/laptop-001`? (ownership chain walk)
2. Is `subject` explicitly denied on `/inventory/assets/laptop-001` or any parent? (deny check)
3. Is `subject` explicitly allowed on `/inventory/assets/laptop-001` or any parent? (allow check)

Ownership and deny checks take precedence over role-based allows.

### The RBAC Fallback

If the RBAC service is temporarily unavailable (restart, network partition), the interceptor falls back to the local `cluster-roles.generated.json`:

1. Load the file from disk (already in memory if previously loaded)
2. Check if the caller's roles (cached in the interceptor's role cache) include the required action
3. Skip the resource-level check (unavailable without the RBAC service)

This means during a RBAC service restart, tier 1 (action-based) continues to work but tier 2 (resource-specific permissions) is skipped. Roles keep the cluster running; fine-grained resource permissions are temporarily unenforced. The system logs this state clearly.

### Deny-by-Default

If a gRPC method has no `(globular.auth.authz)` annotation and no permission descriptor entry, the interceptor can be configured to deny all requests for that method. This is the `GLOBULAR_DENY_UNMAPPED=1` mode. In production, all methods should be annotated. The CI test `TestReflection_RBACCoverageWithDenyByDefault` verifies this.

---

## Testing RBAC in Your Service

### Test 1: Verify the annotations load

```go
func TestPermissionsLoad(t *testing.T) {
    reg, err := policy.LoadAndRegisterPermissions("inventory")
    if err != nil {
        t.Fatalf("load permissions: %v", err)
    }
    if reg.PermissionCount == 0 {
        t.Error("no permissions loaded — check permissions.generated.json")
    }
    t.Logf("loaded %d permissions, %d action mappings",
        reg.PermissionCount, reg.ActionMappingCount)
}
```

### Test 2: Verify action resolution

```go
func TestActionResolution(t *testing.T) {
    resolver := policy.GlobalResolver()

    cases := []struct {
        method     string
        wantAction string
    }{
        {"/inventory.InventoryService/GetAsset", "inventory.asset.read"},
        {"/inventory.InventoryService/DeleteAsset", "inventory.asset.delete"},
    }

    for _, c := range cases {
        p, ok := resolver.Resolve(c.method)
        if !ok {
            t.Errorf("no mapping for %s", c.method)
            continue
        }
        if p.Action != c.wantAction {
            t.Errorf("Resolve(%s) = %q, want %q", c.method, p.Action, c.wantAction)
        }
    }
}
```

### Test 3: Verify the generated roles cover the expected actions

```go
func TestGeneratedRoles(t *testing.T) {
    roles, fromFile, err := policy.LoadServiceRoles("inventory")
    if err != nil || !fromFile {
        t.Skip("cluster-roles.generated.json not found")
    }

    var viewerRole *policy.ServiceRole
    for _, r := range roles {
        if r.Name == "inventory-viewer" {
            viewerRole = &r
            break
        }
    }
    if viewerRole == nil {
        t.Fatal("inventory-viewer role not generated")
    }

    // Viewer should have read, not write
    actions := make(map[string]bool)
    for _, a := range viewerRole.Actions {
        actions[a] = true
    }
    if !actions["inventory.asset.read"] {
        t.Error("inventory-viewer missing inventory.asset.read")
    }
    if actions["inventory.asset.delete"] {
        t.Error("inventory-viewer should NOT have inventory.asset.delete")
    }
}
```

---

## Overriding Generated Roles

The generated roles are defaults. Operators can override them without modifying your service code:

```
Priority order (highest wins):
/etc/globular/policy/rbac/cluster-roles.json        ← admin overrides
/var/lib/globular/policy/rbac/cluster-roles.json    ← package defaults
/var/lib/globular/policy/<service>/cluster-roles.generated.json  ← generated
```

This lets operators tighten or expand role actions for their specific deployment without patching your service package. Your generated roles are the recommended defaults; operators adapt them to their organization.

---

## Checklist: Adding RBAC to a New Service

- [ ] Design resource path hierarchy (2–3 levels, plural nouns)
- [ ] Define action keys for every RPC (`<service>.<resource>.<verb>`)
- [ ] Add `(globular.auth.authz)` annotation to every RPC
- [ ] Add `(globular.auth.resource)` annotation to scope-anchor fields
- [ ] Use `default_role_hint` consistently (viewer/editor/manager/admin)
- [ ] Run `./generateCode.sh` and commit the generated JSON files
- [ ] Verify `permissions.generated.json` has an entry for every RPC
- [ ] Add a test that verifies all methods resolve to an action key
- [ ] After deploying, run `globular rbac seed` to load the new roles

---

## See Also

- [Access Control: Roles and Permissions (Operators)](../operators/rbac-permissions.md) — Operator guide: assigning roles, CLI usage, built-in role reference
- [Writing a Microservice](writing-a-microservice.md) — Service primitives, lifecycle, registration
- [RBAC Integration (quick reference)](rbac-integration.md) — Annotation reference and runtime details
- [proto/rbac.proto](../../proto/rbac.proto) — Full protobuf API for the RBAC service
