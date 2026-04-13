# Create a Service

## Service Structure

Every Go service follows this layout:

```
<service_name>/
  <service_name>pb/       # Generated protobuf code
  <service_name>_client/  # Client library
  <service_name>_server/
    server.go             # Lifecycle, gRPC registration
    config.go             # Configuration
    handlers.go           # Business logic
    *_test.go             # Tests
```

## Step 1: Define the Proto Contract

Create `proto/<service>.proto`:

```protobuf
syntax = "proto3";
package <service>;
option go_package = "github.com/globulario/services/golang/<service>/<service>pb";

service <ServiceName>Service {
  rpc MyMethod(MyRequest) returns (MyResponse);
}
```

Generate code: `./generateCode.sh`

## Step 2: Implement the Server

The server implements two interfaces:
- `Service` (getters/setters for Name, Port, Domain, etc.)
- `LifecycleService` (`StartService()`, `StopService()`, `GetGrpcServer()`)

Use shared primitives from `globular_service/`:

```go
func main() {
    srv := initializeServerDefaults()
    globular.HandleInformationalFlags(srv, os.Args[1:], logger, printUsage)
    globular.ParsePositionalArgs(srv, os.Args[1:])
    globular.AllocatePortIfNeeded(srv, os.Args[1:])
    globular.LoadRuntimeConfig(srv)

    if err := srv.Init(); err != nil { os.Exit(1) }
    setupGrpcService(srv)

    lm := globular.NewLifecycleManager(srv, logger)
    lm.Start()
}
```

## Step 3: Create Package Spec

Create `generated/specs/<service>_service.yaml` (see build-packages.md).

## Step 4: Build and Deploy

```bash
go build -o /tmp/<service>_server ./<service>/<service>_server/
globular pkg build --spec generated/specs/<service>_service.yaml --root /tmp/payload
globular pkg publish --file /tmp/out/<service>.tgz
globular services desired set <service> 0.0.1
```

## Key Patterns

- Use `config.ResolveServiceAddr()` for service discovery
- Use `config.GetEtcdClient()` for state storage
- Register RBAC permissions in `policy.GlobalResolver().Register()`
- Use `interceptors.AllowUnauthenticated()` for public endpoints
- Embed `workflowpb.UnimplementedWorkflowActorServiceServer` if the service handles workflow callbacks
