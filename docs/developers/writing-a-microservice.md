# Writing a Microservice

This page walks through creating a new gRPC microservice for the Globular platform from scratch. It covers defining the proto contract, generating code, implementing the server, using shared primitives, writing tests, and integrating with the platform's service discovery, configuration, and lifecycle management.

---

## Why build on Globular instead of plain systemd + gRPC?

You can write a gRPC service in Go, deploy it as a systemd unit, and call it done. That works. But you will end up rebuilding the same infrastructure every time: service discovery, configuration management, TLS, auth, deployment pipelines, health monitoring. Most teams do this in fragments — a bit of Consul here, some Vault there, a Prometheus exporter bolted on, a shell script for deployments.

Globular gives you that infrastructure as a platform, and the deal is worth understanding before you build on it.

**What you get for free the moment your service implements the pattern:**

| Problem | Globular's answer |
|---------|-----------------|
| How does my service find other services? | etcd service registry. `config.ResolveLocalServiceAddr("auth.AuthenticationService")` returns the endpoint. |
| How does my service authenticate callers? | The interceptor chain does it. JWT verification, token extraction, identity propagation — zero code in your service. |
| How is access controlled? | RBAC annotations on your proto RPCs. One line per method. The enforcement interceptor handles the rest. |
| How is my service configured? | etcd. `config.GetServiceConfigByID()`. No env vars, no config files, no redeploy to change config. |
| How does it get deployed? | `globular deploy my-service --bump patch`. The desired-state model propagates it across all nodes automatically. |
| How do I know if it's running? | `globular cluster health`. The convergence model tracks installed vs. desired across all nodes. |
| How do I upgrade it across a 3-node cluster in order? | The workflow service orchestrates it. Each node gets a workflow run: FETCH → VERIFY → INSTALL → START → HEALTH_CHECK. |
| How do I recover a node that had my service installed? | The full-reseed recovery workflow reinstalls it from the artifact snapshot, in bootstrap order, with checksum verification. |
| What happens if a deployment partially fails? | The workflow fails at the exact step that broke, with classification. The partial_apply mechanism resumes from where it stopped. |
| How do other services know my service is healthy? | Node agent heartbeats include your service's systemd unit state. The gateway's xDS configuration updates automatically. |

**What the Globular way costs you:**

- Your service must be a gRPC server. REST-only services don't fit the model.
- Your API must be defined in protobuf. This is a constraint that is also a discipline.
- Configuration must come from etcd, not environment variables or files. Some third-party libraries resist this.
- You must package your service into a Globular package spec. This is a small YAML file, not a Dockerfile.

If those constraints are acceptable — and for most internal services they are — then the platform handles deployment, discovery, auth, RBAC, health, upgrades, and disaster recovery, and you write the business logic.

---

## Overview

A Globular microservice is a gRPC server that:
- Defines its API in a Protocol Buffer (`.proto`) file
- Implements the generated gRPC server interface in Go
- Uses shared primitives for lifecycle management, configuration, and health checks
- Registers itself in etcd for service discovery
- Integrates with the interceptor chain for authentication, RBAC, and audit logging

## Step 1: Define the Proto Contract

Create a `.proto` file in the `/proto/` directory. This is the authoritative API contract for your service.

```protobuf
// proto/inventory.proto
syntax = "proto3";

package inventory;

option go_package = "github.com/globulario/services/golang/inventory/inventorypb";

import "proto/globular_auth.proto";

// InventoryService manages physical asset tracking.
service InventoryService {

    // CreateAsset registers a new physical asset.
    rpc CreateAsset(CreateAssetRequest) returns (CreateAssetResponse) {
        option (globular.auth.authz) = {
            action: "inventory.asset.create"
            permission: "write"
            resource_template: "/inventory/assets"
            default_role_hint: "editor"
        };
    }

    // GetAsset retrieves an asset by ID.
    rpc GetAsset(GetAssetRequest) returns (Asset) {
        option (globular.auth.authz) = {
            action: "inventory.asset.read"
            permission: "read"
            resource_template: "/inventory/assets/{asset_id}"
            default_role_hint: "viewer"
        };
    }

    // ListAssets returns all assets matching a filter.
    rpc ListAssets(ListAssetsRequest) returns (ListAssetsResponse) {
        option (globular.auth.authz) = {
            action: "inventory.asset.list"
            permission: "read"
            resource_template: "/inventory/assets"
            default_role_hint: "viewer"
        };
    }

    // UpdateAsset modifies an existing asset.
    rpc UpdateAsset(UpdateAssetRequest) returns (Asset) {
        option (globular.auth.authz) = {
            action: "inventory.asset.update"
            permission: "write"
            resource_template: "/inventory/assets/{asset_id}"
            default_role_hint: "editor"
        };
    }

    // DeleteAsset removes an asset.
    rpc DeleteAsset(DeleteAssetRequest) returns (DeleteAssetResponse) {
        option (globular.auth.authz) = {
            action: "inventory.asset.delete"
            permission: "delete"
            resource_template: "/inventory/assets/{asset_id}"
            default_role_hint: "admin"
        };
    }
}

message Asset {
    string id = 1;
    string name = 2;
    string category = 3;
    string location = 4;
    string serial_number = 5;
    string assigned_to = 6;
    int64 created_at = 7;
    int64 updated_at = 8;
    map<string, string> metadata = 9;
}

message CreateAssetRequest {
    string name = 1;
    string category = 2;
    string location = 3;
    string serial_number = 4;
    map<string, string> metadata = 5;
}

message CreateAssetResponse {
    Asset asset = 1;
}

message GetAssetRequest {
    string asset_id = 1 [(globular.auth.resource) = { kind: "asset", scope_anchor: true }];
}

message ListAssetsRequest {
    string category_filter = 1;
    string location_filter = 2;
    int32 limit = 3;
    int32 offset = 4;
}

message ListAssetsResponse {
    repeated Asset assets = 1;
    int32 total = 2;
}

message UpdateAssetRequest {
    string asset_id = 1 [(globular.auth.resource) = { kind: "asset", scope_anchor: true }];
    string name = 2;
    string category = 3;
    string location = 4;
    string assigned_to = 5;
    map<string, string> metadata = 6;
}

message DeleteAssetRequest {
    string asset_id = 1 [(globular.auth.resource) = { kind: "asset", scope_anchor: true }];
}

message DeleteAssetResponse {
    bool success = 1;
}
```

**Key elements**:
- Import `globular_auth.proto` for RBAC annotations
- Every RPC has an `(globular.auth.authz)` option specifying action, permission level, resource template, and default role
- Request fields that identify a resource use `(globular.auth.resource)` with `scope_anchor: true`
- The resource template uses `{field_name}` placeholders that are resolved from request fields at runtime

## Step 2: Generate Code

Run the code generation script:

```bash
./generateCode.sh
```

This generates:
- `golang/inventory/inventorypb/inventory.pb.go` — Message types
- `golang/inventory/inventorypb/inventory_grpc.pb.go` — gRPC server and client interfaces
- `typescript/inventory/inventory_pb.js` — TypeScript message types
- `typescript/inventory/inventory_grpc_web_pb.js` — TypeScript gRPC-Web client
- RBAC permission descriptors extracted from the authz annotations

## Step 3: Implement the Server

Create the server directory structure:

```
golang/inventory/
├── inventorypb/              # Generated (don't edit)
│   ├── inventory.pb.go
│   └── inventory_grpc.pb.go
├── inventory_client/
│   └── client.go             # Client helper (optional)
└── inventory_server/
    ├── server.go             # Server struct + gRPC registration
    ├── config.go             # Configuration
    ├── handlers.go           # Business logic
    └── handlers_test.go      # Tests
```

### server.go

```go
package main

import (
    "log"

    "github.com/globulario/services/golang/globular_service"
    "github.com/globulario/services/golang/inventory/inventorypb"
    "google.golang.org/grpc"
)

// server implements inventorypb.InventoryServiceServer
type server struct {
    inventorypb.UnimplementedInventoryServiceServer

    // Embed the Globular service for shared lifecycle
    *globular_service.BaseService

    // Service-specific state
    config *InventoryConfig
    // store  *AssetStore  // your data access layer
}

func main() {
    // Handle --version, --help, --describe, --health flags
    globular_service.HandleInformationalFlags("inventory", "0.0.1")

    // Parse positional arguments: service_id, config_path
    serviceID, configPath := globular_service.ParsePositionalArgs()

    // Create server instance
    srv := &server{}

    // Load configuration
    cfg, err := LoadConfig(serviceID, configPath)
    if err != nil {
        log.Fatalf("failed to load config: %v", err)
    }
    srv.config = cfg

    // Create the lifecycle manager
    lm := globular_service.NewLifecycleManager(srv, cfg.Port)

    // Register gRPC service
    lm.RegisterService(func(gs *grpc.Server) {
        inventorypb.RegisterInventoryServiceServer(gs, srv)
    })

    // Start serving (blocks until shutdown)
    if err := lm.Serve(); err != nil {
        log.Fatalf("server failed: %v", err)
    }
}

// Implement globular_service.Service interface
func (s *server) GetId() string      { return s.config.ID }
func (s *server) SetId(id string)    { s.config.ID = id }
func (s *server) GetName() string    { return "inventory" }
func (s *server) GetPort() int       { return s.config.Port }
func (s *server) GetState() string   { return s.config.State }
func (s *server) SetState(st string) { s.config.State = st }
func (s *server) GetDomain() string  { return s.config.Domain }
func (s *server) GetVersion() string { return "0.0.1" }
// ... additional interface methods
```

### config.go

```go
package main

import (
    "github.com/globulario/services/golang/globular_service"
)

// InventoryConfig holds service-specific configuration
type InventoryConfig struct {
    ID     string `json:"id"`
    Port   int    `json:"port"`
    State  string `json:"state"`
    Domain string `json:"domain"`

    // Service-specific config
    DatabaseEndpoint string `json:"database_endpoint,omitempty"`
    MaxPageSize      int    `json:"max_page_size,omitempty"`
}

func LoadConfig(serviceID, configPath string) (*InventoryConfig, error) {
    cfg := &InventoryConfig{
        ID:          serviceID,
        Port:        10300, // default port
        State:       "stopped",
        MaxPageSize: 100,
    }

    // Load from file or etcd
    if configPath != "" {
        if err := globular_service.LoadConfigFromFile(configPath, cfg); err != nil {
            return nil, err
        }
    }

    // Validate required fields
    if err := globular_service.ValidateCommonFields(cfg.ID, cfg.Port, "grpc", "0.0.1"); err != nil {
        return nil, err
    }

    return cfg, nil
}

func (c *InventoryConfig) Save(path string) error {
    return globular_service.SaveConfigToFile(path, c)
}
```

### handlers.go

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/globulario/services/golang/inventory/inventorypb"
    "github.com/google/uuid"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

func (s *server) CreateAsset(ctx context.Context, req *inventorypb.CreateAssetRequest) (*inventorypb.CreateAssetResponse, error) {
    if req.Name == "" {
        return nil, status.Error(codes.InvalidArgument, "name is required")
    }

    asset := &inventorypb.Asset{
        Id:           uuid.New().String(),
        Name:         req.Name,
        Category:     req.Category,
        Location:     req.Location,
        SerialNumber: req.SerialNumber,
        Metadata:     req.Metadata,
        CreatedAt:    time.Now().Unix(),
        UpdatedAt:    time.Now().Unix(),
    }

    // Store the asset (your data layer)
    // if err := s.store.Put(ctx, asset); err != nil {
    //     return nil, status.Errorf(codes.Internal, "failed to store asset: %v", err)
    // }

    return &inventorypb.CreateAssetResponse{Asset: asset}, nil
}

func (s *server) GetAsset(ctx context.Context, req *inventorypb.GetAssetRequest) (*inventorypb.Asset, error) {
    if req.AssetId == "" {
        return nil, status.Error(codes.InvalidArgument, "asset_id is required")
    }

    // Retrieve from data layer
    // asset, err := s.store.Get(ctx, req.AssetId)
    // if err != nil {
    //     return nil, status.Errorf(codes.NotFound, "asset %s not found", req.AssetId)
    // }

    return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (s *server) ListAssets(ctx context.Context, req *inventorypb.ListAssetsRequest) (*inventorypb.ListAssetsResponse, error) {
    limit := int(req.Limit)
    if limit <= 0 || limit > s.config.MaxPageSize {
        limit = s.config.MaxPageSize
    }

    // Query data layer with filters
    // assets, total, err := s.store.List(ctx, req.CategoryFilter, req.LocationFilter, limit, int(req.Offset))

    return &inventorypb.ListAssetsResponse{
        Assets: nil, // replace with actual results
        Total:  0,
    }, nil
}

func (s *server) UpdateAsset(ctx context.Context, req *inventorypb.UpdateAssetRequest) (*inventorypb.Asset, error) {
    if req.AssetId == "" {
        return nil, status.Error(codes.InvalidArgument, "asset_id is required")
    }
    return nil, status.Error(codes.Unimplemented, fmt.Sprintf("update %s not implemented", req.AssetId))
}

func (s *server) DeleteAsset(ctx context.Context, req *inventorypb.DeleteAssetRequest) (*inventorypb.DeleteAssetResponse, error) {
    if req.AssetId == "" {
        return nil, status.Error(codes.InvalidArgument, "asset_id is required")
    }
    return nil, status.Error(codes.Unimplemented, fmt.Sprintf("delete %s not implemented", req.AssetId))
}
```

## Step 4: Build and Test

### Build

```bash
cd golang
go build ./inventory/inventory_server
```

### Run Locally

```bash
./inventory_server --port 10300
# Or with full flags:
./inventory_server inventory_service_001 /path/to/config.json
```

### Test

```go
// handlers_test.go
package main

import (
    "context"
    "testing"

    "github.com/globulario/services/golang/inventory/inventorypb"
)

func TestCreateAsset(t *testing.T) {
    srv := &server{
        config: &InventoryConfig{MaxPageSize: 100},
    }

    resp, err := srv.CreateAsset(context.Background(), &inventorypb.CreateAssetRequest{
        Name:     "Laptop",
        Category: "hardware",
        Location: "office-1",
    })
    if err != nil {
        t.Fatalf("CreateAsset failed: %v", err)
    }
    if resp.Asset.Name != "Laptop" {
        t.Errorf("expected name 'Laptop', got '%s'", resp.Asset.Name)
    }
    if resp.Asset.Id == "" {
        t.Error("expected non-empty asset ID")
    }
}

func TestCreateAsset_MissingName(t *testing.T) {
    srv := &server{
        config: &InventoryConfig{MaxPageSize: 100},
    }

    _, err := srv.CreateAsset(context.Background(), &inventorypb.CreateAssetRequest{})
    if err == nil {
        t.Fatal("expected error for missing name")
    }
}
```

Run tests:
```bash
cd golang
go test ./inventory/inventory_server -v
```

## Step 5: Shared Primitives

### CLI Helpers

`HandleInformationalFlags` adds standard flags to every service binary:

```bash
inventory_server --version    # "inventory 0.0.1"
inventory_server --help       # Usage information
inventory_server --describe   # JSON service descriptor
inventory_server --health     # Check health endpoint
```

### Lifecycle Manager

`NewLifecycleManager` handles:
- gRPC server creation with TLS
- Interceptor chain (auth, RBAC, audit)
- Prometheus metrics registration
- gRPC health check registration
- RBAC permission loading
- Graceful shutdown on SIGTERM

### Port Allocation

If the configured port is in use, the lifecycle manager automatically tries fallback ports (up to 5 attempts). The actual bound port is registered in etcd.

```bash
# Request dynamic port allocation
inventory_server --port 0
```

### Configuration from etcd

In production, service configuration comes from etcd, not local files:

```go
// The lifecycle manager reads config from etcd automatically
// Config keys: /globular/services/inventory/config
```

## Step 6: RBAC Integration

The proto annotations define what permissions each RPC requires. At runtime:

1. The gRPC interceptor intercepts each request
2. It looks up the RBAC mapping for the method (generated from proto annotations)
3. It resolves the resource path by substituting request field values into the template
4. It checks the caller's permissions against the RBAC service
5. The request is allowed or denied before your handler code runs

Your handler code does not need to implement any authorization logic — it's handled entirely by the interceptor.

## Practical Example: Full Service Integration

After implementing your service:

```bash
# 1. Generate code
./generateCode.sh

# 2. Build
cd golang && go build ./inventory/inventory_server && cd ..

# 3. Create package spec
cat > specs/inventory_service.yaml << 'EOF'
name: inventory
version: 0.0.1
publisher: myteam@example.com
platform: linux_amd64
kind: SERVICE
profiles:
  - custom
priority: 60
dependencies:
  - etcd
  - authentication
  - rbac
EOF

# 4. Package
globular pkg build --spec specs/inventory_service.yaml --root packages/payload/inventory/ --version 0.0.1

# 5. Publish
globular pkg publish globular-inventory-0.0.1-linux_amd64-1.tgz

# 6. Deploy
globular services desired set inventory 0.0.1 --publisher myteam@example.com

# 7. Monitor
globular services desired list
```

## What's Next

- [Service Packaging](developers/service-packaging.md): Detailed packaging guide
- [RBAC Integration](developers/rbac-integration.md): Deep dive into authorization
