# Workflow Integration

This page covers how services integrate with Globular's workflow system: implementing backup hooks, providing health check endpoints, participating in the convergence model, and understanding how workflows interact with your service during deployment, upgrades, and maintenance.

## Why Workflow Integration Exists

In most deployment systems, the operator manages service lifecycle externally — a CI pipeline pushes binaries, an Ansible playbook restarts services, a Prometheus alert fires when something is down. Knowledge about what the service needs (what data to back up, how long it takes to initialize, when it is safe to stop) lives outside the service, in scripts and runbooks.

Globular inverts this. The service is the authority on its own lifecycle. It declares what data it manages. It controls when it is healthy. It signals when it is safe to stop. The workflow system acts as the coordinator, but the decisions about readiness, data ownership, and safe shutdown come from the service itself.

This has a practical consequence: when you add a new data directory, you don't update a backup policy file somewhere else — you update `PrepareBackup`. When your service needs 30 seconds to load its index before serving, you don't add a sleep to the deploy script — you block in `StartService`. The service and its operational contract live in the same place.

## How Workflows Interact with Services

Every Globular service participates in the workflow system, even if it doesn't implement any custom workflow logic. During deployment and upgrades, workflows:

1. **Fetch** your package from the repository
2. **Install** the binary on the target node
3. **Configure** the service (write etcd keys, update systemd unit)
4. **Start** the systemd unit
5. **Verify** the service is healthy via the gRPC health endpoint

Your service participates in steps 4 and 5 automatically — by starting correctly and responding to health checks.

The separation between START and VERIFY matters. "The process launched" and "the service is ready to handle traffic" are different facts. A service can start and immediately enter a crash loop, or start and spend 20 seconds loading data before it can serve. VERIFY is the checkpoint that confirms the cluster actually gained a working service — not just that systemd launched a process.

## Health Check Integration

### gRPC Health Protocol

Every Globular service automatically registers the gRPC Health Check protocol (defined in `grpc.health.v1`). The Lifecycle Manager handles this:

```go
// The lifecycle manager automatically registers:
// grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
```

The health check is used by:
- **Workflow VERIFY phase**: After starting your service, the workflow probes the health endpoint to confirm it's operational
- **Envoy gateway**: Continuous health checking for load balancing decisions
- **Cluster Doctor**: Invariant checking for runtime health
- **Node Agent heartbeat**: Reports unit health state to the controller

### Custom Health Logic

If your service has custom health requirements (database connection, external dependency), implement them in the `StartService` method:

```go
func (s *server) StartService() error {
    // Connect to database
    db, err := connectDatabase(s.config.DatabaseEndpoint)
    if err != nil {
        return fmt.Errorf("database connection failed: %w", err)
    }
    s.db = db

    // The health check will pass only after StartService returns nil
    return nil
}
```

If `StartService` returns an error, the gRPC health check reports NOT_SERVING, and the workflow VERIFY phase fails with `FailureClass: SYSTEMD`.

### Health Check Timeout

The workflow VERIFY phase has a configurable timeout for the health check. If your service takes time to initialize (loading data, warming caches), it must complete initialization before the timeout expires.

If your service needs a long startup time:
- Ensure the binary starts quickly (don't block `main()`)
- Load data asynchronously
- Report SERVING only after initialization is complete

## Backup Hooks

Services that manage data can participate in the backup system by implementing the `BackupHookService`. When the Backup Manager runs a cluster backup, it calls `PrepareBackup` on every service that implements this hook.

The design here is intentional inversion of control. The Backup Manager does not know which paths each service writes to — the service declares them. This means a new service that stores data in `/var/lib/globular/inventory/` is automatically backed up as soon as it implements `PrepareBackup`. No backup policy configuration, no Ansible vars file to update. The backup coverage is a consequence of the code, not a separate operational artifact.

### Implementing BackupHookService

```protobuf
// From proto/backup_hook.proto

service BackupHookService {
    rpc PrepareBackup(PrepareBackupRequest) returns (PrepareBackupResponse);
    rpc FinalizeBackup(FinalizeBackupRequest) returns (FinalizeBackupResponse);
}
```

### PrepareBackup

Called before the backup starts. Your service should:
1. Declare its local datasets (what data it manages)
2. Optionally quiesce writes (pause mutations to ensure consistency)

```go
func (s *server) PrepareBackup(ctx context.Context, req *backuppb.PrepareBackupRequest) (*backuppb.PrepareBackupResponse, error) {
    // Declare datasets
    entries := []*backuppb.ServiceDataEntry{
        {
            Name:            "inventory-data",
            Path:            "/var/lib/globular/inventory/",
            DataClass:       backuppb.DataClass_AUTHORITATIVE,
            Scope:           "cluster",
            SizeBytes:       s.getDataSize(),
            BackupByDefault: true,
            RestoreByDefault: true,
            RebuildSupported: false,
        },
        {
            Name:            "inventory-cache",
            Path:            "/var/lib/globular/inventory/cache/",
            DataClass:       backuppb.DataClass_CACHE,
            Scope:           "node",
            BackupByDefault: false,
            RebuildSupported: true,
        },
    }

    // Optionally quiesce (pause writes)
    s.quiesce()

    return &backuppb.PrepareBackupResponse{
        Entries: entries,
    }, nil
}
```

**Data classes** reflect a real trade-off between backup cost and recovery cost:
- `AUTHORITATIVE`: This is the primary copy. There is no other source. If it's not backed up, it cannot be recovered. Must be backed up.
- `REBUILDABLE`: Can be reconstructed from other sources (re-indexed from raw data, recomputed from events), but doing so takes time. Backing it up is optional — it trades backup storage for faster recovery.
- `CACHE`: Ephemeral. Loss is acceptable; the service rebuilds it on next start. Not backed up by default.

Declaring the right data class matters for disaster recovery. A cluster with 100 GB of `CACHE` data that gets labeled `AUTHORITATIVE` wastes backup storage and slows restores. A service that labels its primary database `REBUILDABLE` when it can't actually rebuild it will be unrecoverable after a disaster.

**Scope**:
- `"node"`: Data is specific to this node (local state, cache)
- `"cluster"`: Data is shared across the cluster (database tables, shared config)

### FinalizeBackup

Called after the backup completes (success or failure). Resume normal operations:

```go
func (s *server) FinalizeBackup(ctx context.Context, req *backuppb.FinalizeBackupRequest) (*backuppb.FinalizeBackupResponse, error) {
    // Resume writes
    s.unquiesce()

    return &backuppb.FinalizeBackupResponse{}, nil
}
```

### Registering the Hook

Register the backup hook in your server's gRPC registration:

```go
lm.RegisterService(func(gs *grpc.Server) {
    inventorypb.RegisterInventoryServiceServer(gs, srv)
    backuppb.RegisterBackupHookServiceServer(gs, srv)  // Register backup hook
})
```

The Backup Manager discovers backup hooks automatically via etcd service registration.

## Service Configuration Patterns

### Reading Configuration from etcd

In production, service configuration comes from etcd. The pattern:

```go
// The lifecycle manager reads config from etcd on startup
// Your service receives configuration through the config loading mechanism

func LoadConfig(serviceID, configPath string) (*InventoryConfig, error) {
    cfg := &InventoryConfig{}

    // Primary: load from etcd via the config system
    // Fallback: load from local file (bootstrap, development)
    if configPath != "" {
        if err := globular_service.LoadConfigFromFile(configPath, cfg); err != nil {
            return nil, err
        }
    }

    return cfg, nil
}
```

### Watching for Configuration Changes

Services can watch etcd keys for configuration changes and react dynamically:

```go
// Watch for config changes
go func() {
    watchChan := etcdClient.Watch(ctx, "/globular/services/inventory/config")
    for resp := range watchChan {
        for _, ev := range resp.Events {
            // Reload configuration
            s.reloadConfig(ev.Kv.Value)
        }
    }
}()
```

## Service Registration

### Automatic Registration

The Lifecycle Manager automatically registers your service in etcd when it starts:

```
/globular/services/inventory/config → { address: "0.0.0.0", port: 10300, protocol: "grpc" }
/globular/services/inventory/instances/node-abc123 → { endpoint: "192.168.1.10:10300" }
```

When the service stops, the registration is removed (or marked as inactive).

### Discovery by Other Services

Other services find your service through etcd:

```go
// Another service wants to call the inventory service
endpoint := config.ResolveServiceEndpoint("inventory")
// Returns: "192.168.1.10:10300" (from etcd)

conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(creds))
client := inventorypb.NewInventoryServiceClient(conn)
```

## Graceful Shutdown

The Lifecycle Manager handles graceful shutdown on SIGTERM:

1. Mark the service as NOT_SERVING in the health check
2. Stop accepting new requests
3. Wait for in-flight requests to complete (with timeout)
4. Close the gRPC server
5. Call your `StopService()` method for cleanup

Graceful shutdown matters specifically during upgrades. When the convergence model installs a new version, it restarts the service. If the old version is killed mid-request, the client receives an error and may retry against the new version — which is a different binary that may have different state. The NOT_SERVING signal tells Envoy to drain traffic before the process exits, so in-flight requests complete against the version that received them.

`StopService()` is a separate hook from process exit because cleanup — closing database connections, flushing caches, releasing locks — must happen within the graceful shutdown window, before the process is forcibly killed. If cleanup is in a `defer` or init function, it may not run in time.

Implement `StopService()` for your custom cleanup:

```go
func (s *server) StopService() error {
    // Close database connections
    if s.db != nil {
        s.db.Close()
    }

    // Flush caches
    if s.cache != nil {
        s.cache.Flush()
    }

    return nil
}
```

## Workflow Lifecycle Summary

Here's how your service interacts with workflows throughout its lifecycle:

### Deployment
```
1. DECISION: Controller resolves your package from the repository
2. FETCH:    Node Agent downloads your .tgz from MinIO
3. INSTALL:  Binary extracted to /usr/local/bin/<name>_server
4. CONFIGURE: Service config written to etcd
5. START:    systemctl start <name> → your main() runs
6. VERIFY:   Health check probed → SERVING response
7. COMPLETE: InstalledPackage record written to etcd
```

### Upgrade
```
1. DECISION: Controller detects version drift
2. FETCH:    Node Agent downloads the new version
3. INSTALL:  New binary replaces old at /usr/local/bin/
4. CONFIGURE: Config updated in etcd if needed
5. START:    systemctl restart <name>
   → StopService() on old version
   → main() of new version
6. VERIFY:   Health check confirms new version is serving
7. COMPLETE: InstalledPackage record updated
```

### Backup
```
1. PrepareBackup: Your service declares datasets and quiesces
2. Providers run: restic/etcd/minio/scylla backup your declared paths
3. FinalizeBackup: Your service resumes normal operations
```

### Health Monitoring
```
Continuous:
- Envoy probes gRPC health → routes traffic to healthy instances
- Node Agent reports unit state in heartbeat
- Doctor evaluates running-state invariant
```

## Practical Scenarios

### Scenario 1: Service with Database

A service that manages its own data and participates in backups:

```go
func (s *server) StartService() error {
    // Connect to database (endpoint from etcd, not hardcoded)
    dbEndpoint := s.config.DatabaseEndpoint // resolved from etcd
    db, err := connectDB(dbEndpoint)
    if err != nil {
        return err // health check will fail
    }
    s.db = db
    return nil
}

func (s *server) StopService() error {
    return s.db.Close()
}

func (s *server) PrepareBackup(ctx context.Context, req *backuppb.PrepareBackupRequest) (*backuppb.PrepareBackupResponse, error) {
    s.db.SetReadOnly(true) // quiesce
    return &backuppb.PrepareBackupResponse{
        Entries: []*backuppb.ServiceDataEntry{{
            Name: "inventory-db", Path: "/var/lib/inventory/data/",
            DataClass: backuppb.DataClass_AUTHORITATIVE, Scope: "cluster",
            BackupByDefault: true, RestoreByDefault: true,
        }},
    }, nil
}
```

### Scenario 2: Stateless Service

A service with no local state needs no special integration — the default health check and lifecycle management are sufficient:

```go
func main() {
    globular_service.HandleInformationalFlags("calculator", "0.0.1")
    serviceID, configPath := globular_service.ParsePositionalArgs()

    srv := &server{}
    cfg, _ := LoadConfig(serviceID, configPath)

    lm := globular_service.NewLifecycleManager(srv, cfg.Port)
    lm.RegisterService(func(gs *grpc.Server) {
        calculatorpb.RegisterCalculatorServiceServer(gs, srv)
    })
    lm.Serve()
}
// That's it — health checks, TLS, interceptors, and metrics are automatic
```

## What's Next

- [Writing a Microservice](developers/writing-a-microservice.md): Complete service development guide
- [Service Packaging](developers/service-packaging.md): Package format and build process
