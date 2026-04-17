# Local-First Development

Globular does not require a cluster to be useful. A developer can run a single service on their laptop with `go run`, no etcd, no cluster, no infrastructure. As the project grows, the same service seamlessly scales from a local binary to a single-node deployment to a multi-node distributed cluster — without code changes.

This page explains the local-first principle, how services degrade gracefully without infrastructure, and how to develop Globular services at each scale.

## The Local-First Principle

Globular is designed around a progressive deployment model:

```
Level 0: go run            → Single service, no infrastructure, developer laptop
Level 1: Single node       → One machine, systemd, local etcd, basic services
Level 2: Small cluster     → 2-3 nodes, HA, convergence model active
Level 3: Production        → 3+ nodes, HA, backups, monitoring, external access
```

Each level adds capabilities without changing the service code. The same binary runs at every level. The difference is what infrastructure surrounds it.

**Why this matters:**
- Developers can iterate on service logic without running a cluster
- Testing doesn't require a full deployment
- CI can build and test services without etcd or a cluster
- Small teams can start with a single machine and grow
- Appliance products can ship as a single-node Globular instance

## Level 0: Single Service (`go run`)

At the simplest level, a Globular service is a Go binary that starts a gRPC server. You can run it directly:

```bash
cd golang/echo/echo_server
go run .
```

### What Happens Without Infrastructure

When a service starts and cannot reach etcd, it doesn't crash. It falls through a series of fallbacks:

```
1. Try etcd for configuration
   → etcd not available → continue
2. Try local seed file (/var/lib/globular/services/{id}.json)
   → file not found → continue
3. Try global config (/var/lib/globular/config.json)
   → file not found → continue
4. Use hardcoded defaults
   → port 10000, domain "globular.internal", protocol "grpc"
   → Service starts successfully
```

### What Works at Level 0

| Feature | Works | Notes |
|---------|-------|-------|
| gRPC serving | Yes | Binds to allocated port |
| Health check | Yes | gRPC health protocol registered |
| Request handling | Yes | All RPC handlers functional |
| `--version`, `--help` | Yes | CLI flags work without infrastructure |
| `--describe` | Yes | Prints service descriptor as JSON |
| Service discovery | No | No etcd to register in |
| RBAC enforcement | No | No RBAC service to query |
| mTLS | No | No certificates provisioned |
| Convergence | No | No controller to manage state |

### Running Multiple Services Locally

You can run several services on different ports:

```bash
# Terminal 1: Authentication service
cd golang/authentication/authentication_server
go run . --port 10101

# Terminal 2: RBAC service
cd golang/rbac/rbac_server
go run . --port 10104

# Terminal 3: Your custom service
cd golang/my_service/my_service_server
go run . --port 10300
```

Each service starts independently. They can't discover each other (no etcd), but you can connect to them directly by address:port for testing.

### Useful for

- Implementing and testing RPC handlers
- Running unit tests (`go test ./my_service/...`)
- Rapid iteration on business logic
- CI builds and test suites

## Level 1: Single Node (Bootstrapped)

When you bootstrap a single-node cluster, you get etcd, service discovery, RBAC, and the full platform on one machine.

```bash
# Start node agent
sudo systemctl start globular-node-agent

# Bootstrap
globular cluster bootstrap \
  --node localhost:11000 \
  --domain dev.local \
  --profile core
```

### What Changes at Level 1

| Feature | Level 0 | Level 1 |
|---------|---------|---------|
| Configuration | Hardcoded defaults | etcd (source of truth) |
| Service discovery | None | Automatic via etcd |
| Authentication | None | JWT tokens, password auth |
| Authorization | None | RBAC with interceptor chain |
| TLS | None | mTLS with internal CA |
| Package management | None | Repository + artifact lifecycle |
| Desired state | None | Convergence model active |
| Health monitoring | Local only | Doctor + health checks |

### When to Use Level 1

- Integration testing across services
- Development requiring authentication or RBAC
- Testing the full request lifecycle (auth → RBAC → handler → response)
- Building and publishing packages
- Validating workflow integration

### Level 1 on a Laptop

A single-node Globular cluster runs comfortably on a modern laptop:
- **RAM**: ~2 GB for core services + etcd
- **Disk**: ~5 GB for packages + etcd data
- **CPU**: Minimal (services are idle until receiving requests)

You can bootstrap a dev cluster in under 3 minutes and tear it down when done.

## Level 2: Small Cluster (2-3 Nodes)

Add one or two more machines (or VMs) and you get HA:

```bash
# Node 2 joins
globular cluster join --node node-2:11000 --controller node-1:12000 --join-token <token>
globular cluster requests approve <req-id> --profile core
```

### What Changes at Level 2

| Feature | Level 1 | Level 2 |
|---------|---------|---------|
| etcd | Single node | 3-node quorum (tolerates 1 failure) |
| Controller | Single instance | Leader + standby |
| Gateway | Single point of failure | keepalived VIP failover |
| MinIO | Single disk | Erasure coding across nodes |
| Service instances | 1 per service | Multiple (load balanced) |

### When to Use Level 2

- Testing HA scenarios (node failure, leader failover)
- Load testing with multiple service instances
- Validating the convergence model across nodes
- Pre-production staging

## Level 3: Production

Full production deployment with external access, backups, monitoring, and security hardening. See [Day-0/1/2 Operations](../operators/day-0-1-2-operations.md) for the complete timeline.

## How Services Handle Each Level

### Configuration Fallback Chain

Every Globular service uses the same initialization sequence, which works at every level:

```go
// Simplified from globular_service/services.go
func InitService(srv Service) {
    // 1. Try etcd (Level 1+)
    if cfg, err := loadFromEtcd(srv.GetId()); err == nil {
        apply(srv, cfg)
        return
    }

    // 2. Try local seed file (Level 0 with manual config)
    if cfg, err := loadFromFile("/var/lib/globular/services/" + srv.GetId() + ".json"); err == nil {
        apply(srv, cfg)
        return
    }

    // 3. Try global config (Level 0 with minimal setup)
    if cfg, err := loadFromFile("/var/lib/globular/config.json"); err == nil {
        apply(srv, cfg)
        return
    }

    // 4. Use defaults (Level 0, bare minimum)
    // Service already has defaults from initializeServerDefaults()
}
```

### Port Allocation

Services allocate ports without conflicting:

```bash
# Explicit port
my_service_server --port 10300

# Auto-allocate (finds a free port)
my_service_server --port 0
# Or just:
my_service_server
# Uses default or allocates from the port allocator
```

The port allocator uses a local file-based registry to prevent conflicts when running multiple services on the same machine.

### Domain and Address

At Level 0, services default to:
- **Domain**: `globular.internal` (from `netutil.DefaultClusterDomain()`)
- **Address**: `0.0.0.0` (all interfaces)

At Level 1+, these come from etcd:
- **Domain**: The cluster domain (e.g., `dev.local`, `globular.internal`)
- **Address**: The node's routable IP

## Development Workflow

### Write → Test → Deploy Cycle

```bash
# 1. Write code
vim golang/my_service/my_service_server/handlers.go

# 2. Test locally (Level 0)
cd golang
go test ./my_service/... -v -race

# 3. Run and test manually (Level 0)
go run ./my_service/my_service_server --port 10300
# In another terminal: grpcurl -plaintext localhost:10300 list

# 4. Test with cluster (Level 1)
# Bootstrap if not done: globular cluster bootstrap ...
go build -o /usr/local/bin/my_service_server ./my_service/my_service_server
sudo systemctl restart my_service

# 5. Package and publish (Level 1+)
globular pkg build --spec specs/my_service_service.yaml --root payload/ --version 0.0.1
globular pkg publish globular-my_service-0.0.1-linux_amd64-1.tgz

# 6. Deploy to cluster (Level 2+)
globular services desired set my_service 0.0.1
```

### Using Local Seed Files

If you need configuration at Level 0 without etcd, create a seed file:

```bash
mkdir -p /var/lib/globular/services/
cat > /var/lib/globular/services/my_service_001.json << 'EOF'
{
  "Id": "my_service_001",
  "Name": "my_service",
  "Port": 10300,
  "Domain": "localhost",
  "Protocol": "grpc",
  "Version": "0.0.1"
}
EOF

# Service will load this on startup
my_service_server my_service_001
```

### Testing Without TLS

At Level 0, TLS is not configured and services accept plaintext gRPC. You can test with standard tools:

```bash
# List available RPCs
grpcurl -plaintext localhost:10300 list

# Call an RPC
grpcurl -plaintext -d '{"name": "test"}' localhost:10300 my_service.MyService/GetItem
```

At Level 1+, TLS is mandatory. Use the cluster's CA for testing:

```bash
grpcurl -cacert /var/lib/globular/pki/ca.crt localhost:10300 list
```

## Designing for Local-First

When writing a new service, keep these principles in mind:

### Don't Fail on Missing Infrastructure

```go
// GOOD: Graceful fallback
endpoint, err := config.ResolveServiceEndpoint("database")
if err != nil {
    // Use a default or skip the feature
    slog.Warn("database not available, using in-memory store", "error", err)
    return newInMemoryStore()
}

// BAD: Hard crash
endpoint := config.MustResolveServiceEndpoint("database")  // panics without etcd
```

### Use Feature Detection, Not Level Detection

```go
// GOOD: Check if the feature is available
if rbacClient != nil {
    // Check permissions
} else {
    // Skip RBAC (development mode)
}

// BAD: Check what "level" we're running at
if os.Getenv("GLOBULAR_MODE") == "production" {
    // Check permissions
}
```

### Keep Business Logic Independent

```go
// GOOD: Handler doesn't care about infrastructure
func (s *server) CreateItem(ctx context.Context, req *pb.CreateItemRequest) (*pb.Item, error) {
    // Pure business logic — works at any level
    item := &pb.Item{Id: uuid.New().String(), Name: req.Name}
    return s.store.Put(ctx, item)
}

// BAD: Handler depends on cluster state
func (s *server) CreateItem(ctx context.Context, req *pb.CreateItemRequest) (*pb.Item, error) {
    // Reaches into etcd directly — only works at Level 1+
    etcdClient.Put(ctx, "/items/"+id, data)
}
```

### Provide In-Memory Alternatives

For services that need a data store, provide an in-memory implementation for Level 0:

```go
type Store interface {
    Put(ctx context.Context, item *Item) error
    Get(ctx context.Context, id string) (*Item, error)
}

// Production: ScyllaDB, MongoDB, etc.
type scyllaStore struct { ... }

// Development: In-memory map
type memoryStore struct {
    items map[string]*Item
    mu    sync.RWMutex
}
```

## What's Next

- [Writing a Microservice](developers/writing-a-microservice.md) — Complete service development guide
- [Service Packaging](developers/service-packaging.md) — Package for deployment
- [Day-0/1/2 Operations](../operators/day-0-1-2-operations.md) — From bootstrap to production
