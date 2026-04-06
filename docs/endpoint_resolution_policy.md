# Endpoint Resolution Policy

Status: adopted
Scope: service-to-service gRPC dialing inside a Globular cluster.

## Why

Historically every service carried its own ad-hoc rules for turning an
endpoint string into a gRPC dial target:

- cluster-doctor had its own `normalizeLoopback` in `config.go`.
- cluster-controller had its own SNI extraction in `agentclient.go`.
- node-agent had yet another SNI branch in `heartbeat.go`.
- cluster-doctor's event client was hardcoded to `127.0.0.1:10102`
  with no normalization at all.

Because the rules disagreed, the same endpoint string would dial in
one place and fail TLS verification in another. The most common
symptom was:

> `x509: cannot validate certificate for 127.0.0.1 because it doesn't
> contain any IP SANs` — service certs only cover `DNS:localhost`,
> not the loopback IP literals.

This policy fixes that by making endpoint resolution a single
deterministic function.

## The single resolver

`golang/config/endpoint_resolver.go` exports:

```go
type DialTarget struct {
    Address    string // TLS-safe "host:port" to pass to grpc.NewClient
    ServerName string // TLS ServerName/SNI to verify the peer cert against
    // ...
}

func ResolveDialTarget(endpoint string) DialTarget
func NormalizeLoopback(endpoint string) string
func IsLoopbackEndpoint(endpoint string) bool
```

**Every service-to-service dialer MUST use `ResolveDialTarget`.** No
caller may re-implement host/port parsing, loopback rewriting, or
SNI extraction. If the resolver is wrong, fix it there — not in the
caller.

### Rules the resolver enforces

| Input                               | `.Address`               | `.ServerName`          |
|-------------------------------------|--------------------------|------------------------|
| `127.0.0.1:12000`                   | `localhost:12000`        | `localhost`            |
| `[::1]:12000`                       | `localhost:12000`        | `localhost`            |
| `localhost:12000`                   | `localhost:12000`        | `localhost`            |
| `controller.globular.internal:12000`| passthrough              | `controller.globular.internal` |
| `10.0.0.63:12000`                   | passthrough              | `10.0.0.63`            |

Key point: loopback **IP literals** are rewritten to `localhost`
because service certs carry `DNS:localhost` in their SAN set, not
the IP. A non-loopback IP literal (`10.0.0.63`) is **not** rewritten
— if you dial one, you must issue the service cert with a matching
`IP:` SAN, or configure a mesh endpoint with a DNS name.

## Local-only vs service-to-service

Not every loopback use is a bug. The policy distinguishes two classes
of endpoint:

### Local-only endpoints (loopback IP is fine)

These are intentionally on `127.0.0.1` and do not cross the trust
boundary. The resolver is still useful here, but the loopback IP
itself is safe.

Examples:
- Envoy admin interface on `127.0.0.1:9901`
- Node-local Prometheus scrape sockets
- Unix-domain-style debug endpoints bound only to loopback

Rule: if the endpoint is TLS-terminated by the process itself AND
the connection never leaves the host, you may use `127.0.0.1`
directly. The resolver will still return `ServerName: "localhost"`
if you ask for it — safe default.

### Service-to-service endpoints (must be cert-valid)

Any endpoint that terminates a cluster service TLS connection MUST
be dialed through `ResolveDialTarget`. This covers:

- cluster-doctor → cluster-controller
- cluster-doctor → workflow-service
- cluster-doctor → node-agent (via controller-provided endpoints)
- cluster-controller → node-agent
- node-agent → cluster-controller
- any gRPC client loaded from `globular_client` (already handled)

Rule: the `ServerName` you pass to `tls.Config` MUST be the
`.ServerName` field from the DialTarget — never the raw endpoint,
never an IP literal.

## Migration status

Touched in this session:

| File                                                                    | Uses resolver? |
|-------------------------------------------------------------------------|----------------|
| `cluster_doctor/cluster_doctor_server/config.go`                        | yes            |
| `cluster_doctor/cluster_doctor_server/server.go` (controller + wf dial) | yes            |
| `cluster_doctor/cluster_doctor_server/server.go` (event client addr)    | yes (NormalizeLoopback) |
| `cluster_doctor/cluster_doctor_server/node_agent_dialer.go`             | yes            |
| `cluster_controller/cluster_controller_server/agentclient.go`           | yes            |
| `node_agent/node_agent_server/heartbeat.go`                             | yes            |

Other dialers remain on their existing paths for this session; they
should migrate to the resolver the next time they are touched.

## How to add a new service-to-service dialer

```go
import "github.com/globulario/services/golang/config"

target := config.ResolveDialTarget(endpoint)
creds := credentials.NewTLS(&tls.Config{
    ServerName: target.ServerName,
    RootCAs:    clusterCAPool, // from config.GetTLSFile("", "", "ca.crt")
})
conn, err := grpc.NewClient(target.Address, grpc.WithTransportCredentials(creds))
```

That is the full recipe. Do not add fallback hostname logic, do not
re-parse the endpoint, do not special-case loopback. If the resolver
needs a new behaviour, teach it there and update this doc.
