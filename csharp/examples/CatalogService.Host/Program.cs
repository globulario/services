using Globular.Runtime;
using Globular.Runtime.Authorization;

var builder = WebApplication.CreateBuilder(args);

// ── gRPC ─────────────────────────────────────────────────────────────
builder.Services.AddGrpc(options =>
{
    // Register the Globular authorization interceptor.
    // Enforces RBAC on every gRPC call (method -> action -> RBAC validation).
    options.Interceptors.Add<GlobularAuthorizationInterceptor>();
});

// ── Globular managed runtime ─────────────────────────────────────────
// Core runtime: config binding, context, manifest, health, startup pipeline.
builder.Services.AddGlobularRuntime(builder.Configuration);
builder.Services.AddGlobularHealth(builder.Configuration);

// Authorization: real RBAC client, real etcd publisher, managed startup.
// In managed mode (default), this registers:
//   - GlobularRbacClient (real gRPC calls to RBAC service)
//   - GlobularEtcdClient + EtcdServiceStatePublisher (real etcd state publication)
//   - ManagedStartupCoordinator (orchestrated startup sequence)
//   - GlobularAuthorizationInterceptor (fail-closed RBAC enforcement)
//
// For local development, pass AuthorizationMode.Development:
//   builder.Services.AddGlobularAuthorization("catalog", builder.Configuration,
//       AuthorizationMode.Development);
builder.Services.AddGlobularAuthorization("catalog", builder.Configuration);

// Discovery: registers service with Globular cluster management plane.
builder.Services.AddGlobularDiscovery(builder.Configuration);

var app = builder.Build();

// ── Endpoints ────────────────────────────────────────────────────────
// Health endpoint: GET /health
app.MapGlobularHealth();

// Management endpoint: GET /_globular/effective-state
// Returns the resolved runtime state as JSON (authz mode, RBAC status, etc.)
app.MapGlobularManagement();

// Map gRPC services
// app.MapGrpcService<CatalogServiceImpl>();

app.Run();

// ── appsettings.json example ─────────────────────────────────────────
// {
//   "Globular": {
//     "Service": {
//       "Name": "catalog",
//       "Version": "1.0.0",
//       "Domain": "globular.internal",
//       "ProtoPackage": "catalog",
//       "ProtoService": "CatalogService"
//     },
//     "Network": {
//       "GrpcPort": 10200,
//       "GrpcBindAddress": "0.0.0.0",
//       "TlsEnabled": true
//     },
//     "Rbac": {
//       "Address": "https://localhost:10104",
//       "Timeout": "00:00:03",
//       "Domain": "globular.internal"
//     },
//     "Etcd": {
//       "Endpoint": "https://127.0.0.1:2379",
//       "CaCertPath": "/var/lib/globular/pki/ca.crt",
//       "ClientCertPath": "/var/lib/globular/pki/issued/services/service.crt",
//       "ClientKeyPath": "/var/lib/globular/pki/issued/services/service.key"
//     },
//     "Discovery": {
//       "Enabled": true
//     }
//   }
// }
