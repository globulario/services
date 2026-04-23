# Network and Routing

This page covers how Globular handles networking: the Envoy gateway, xDS-driven routing, DNS service, gRPC service discovery, gRPC-Web proxy for browser clients, and the TLS architecture.

## Network Architecture

Traffic in a Globular cluster flows through several layers:

```
External clients (browsers, CLI, mobile)
         │
         ▼
┌─────────────────────────┐
│  Envoy Gateway          │   Ports: 443 (HTTPS), 8443 (gRPC-Web)
│  - TLS termination      │
│  - xDS-driven routing   │
│  - Health checking      │
│  - Load balancing       │
└────────────┬────────────┘
             │
             ▼
┌─────────────────────────┐
│  gRPC Services          │   Each on its own port (10101, 10102, ...)
│  - Native gRPC (TLS)   │
│  - gRPC-Web proxy       │   Each service also has a proxy port (+1)
│  - Health endpoints     │
└────────────┬────────────┘
             │
             ▼
┌─────────────────────────┐
│  etcd                   │   Port: 2379 (client), 2380 (peer)
│  - Service registration │
│  - Configuration        │
│  - Endpoint discovery   │
└─────────────────────────┘
```

## Envoy Gateway

### What the Gateway Does

The Envoy gateway is the entry point for all external traffic. It provides:

- **TLS termination**: External connections are encrypted. The gateway decrypts and forwards to internal services.
- **gRPC-Web translation**: Browsers cannot use native gRPC. The gateway translates gRPC-Web requests (HTTP/2 or HTTP/1.1) to native gRPC for internal services.
- **Load balancing**: When a service runs on multiple nodes, the gateway distributes requests.
- **Health checking**: The gateway probes each backend and removes unhealthy instances from the rotation.

### xDS Routing

The gateway's routing configuration is not static — it's pushed dynamically via the **xDS protocol** (the same protocol used by Envoy in service mesh deployments). A Globular xDS server watches etcd for service registrations and pushes route updates to Envoy:

1. A service starts and registers its endpoint in etcd:
   ```
   /globular/services/authentication/instances/node-1 → { address: "192.168.1.10", port: 10101 }
   ```
2. The xDS server detects the new registration
3. xDS pushes a cluster/route/listener update to Envoy
4. Envoy adds the new backend to its routing table
5. Requests to `/authentication.AuthenticationService/*` are routed to the new backend

When a service stops or becomes unhealthy, the reverse happens — the xDS server removes the backend from the route.

### Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 443 | HTTPS | Primary external endpoint (TLS terminated) |
| 8443 | HTTPS | gRPC-Web endpoint (for browser clients) |

### Health Checking

Envoy performs active health checks on all backends:
- Probes the gRPC health endpoint at configurable intervals
- Backends that fail health checks are removed from the rotation
- When a backend recovers, it's automatically re-added
- Configurable thresholds for marking healthy/unhealthy

## DNS Service

The DNS service provides authoritative DNS resolution for the cluster.

### Zone Management

```bash
# Create a DNS zone
globular dns zone create --domain mycluster.local

# List zones
globular dns zones list
```

### Record Types

The DNS service supports standard record types:

| Type | Purpose | Example |
|------|---------|---------|
| A | IPv4 address | `controller.mycluster.local → 192.168.1.10` |
| AAAA | IPv6 address | `controller.mycluster.local → ::1` |
| CNAME | Alias | `db.mycluster.local → postgresql.mycluster.local` |
| MX | Mail exchange | `mycluster.local → mail.mycluster.local` |
| TXT | Text records | SPF, DKIM, verification |
| NS | Name server | Delegation records |
| SOA | Start of authority | Zone metadata |
| SRV | Service location | `_grpc._tcp.auth.mycluster.local → 192.168.1.10:10101` |
| CAA | Certificate authority authorization | CA restrictions |

### Managing Records

```bash
# Add an A record
globular dns record add --zone mycluster.local \
  --name controller --type A --value 192.168.1.10

# Add a wildcard record
globular dns record add --zone mycluster.local \
  --name "*.services" --type A --value 192.168.1.10

# Remove a record
globular dns record remove --zone mycluster.local \
  --name controller --type A
```

### Bootstrap Behavior

During cluster bootstrap, the DNS service automatically creates default records:
- `globular-gateway.<domain>` → node IP
- `controller.<domain>` → node IP
- `etcd.<domain>` → node IP

These provide initial service resolution before the full DNS configuration is in place.

### Storage

DNS records are stored in ScyllaDB (cluster-wide shared storage). This ensures all DNS instances across nodes serve the same records without synchronization issues.

### Port

The DNS service listens on:
- Port 10006 (gRPC management API)
- Port 53 (UDP/TCP DNS protocol, requires `CAP_NET_BIND_SERVICE`)

## Service Discovery

### How Services Find Each Other

Services discover each other through etcd. The pattern:

1. **Registration**: When a service starts, it writes its endpoint to etcd:
   ```
   /globular/services/{service_id}/config → { address, port, protocol, tls_config }
   /globular/services/{service_id}/instances/{node_key} → { endpoint }
   ```

2. **Resolution**: When a service needs to call another service, it queries etcd for the target's endpoint:
   ```go
   // Resolved from etcd, never hardcoded
   endpoint := config.ResolveServiceEndpoint("authentication")
   conn := grpc.Dial(endpoint, ...)
   ```

3. **Address normalization**: The endpoint resolver normalizes addresses for TLS compatibility:
   - `127.0.0.1:10101` → `localhost:10101` (rewrites loopback IPs to "localhost" for TLS SAN matching)
   - `192.168.1.10:10101` → passthrough (already hostname or routable IP)

### Hard Rules

Globular enforces strict rules about service discovery:

- **No hardcoded addresses**: Services must resolve endpoints from etcd. No `localhost`, no `127.0.0.1`, no constants.
- **No environment variables**: Service configuration comes from etcd. `os.Getenv` is not used for service endpoints.
- **No hardcoded ports**: All service ports come from etcd. Standard protocol ports (443, 53, 2379) are exceptions — they're protocol definitions, not service configuration.
- **Bind to 0.0.0.0**: Services bind to all interfaces, never to loopback. This ensures cross-node reachability.

These rules ensure that services work correctly in single-node, multi-node, and network-partitioned environments without code changes.

## gRPC-Web Proxy

### Browser Client Support

Browsers cannot use native gRPC (it requires HTTP/2 trailers, which browsers don't support for direct connections). Globular provides gRPC-Web support through two mechanisms:

**1. Envoy Gateway**: The primary mechanism. Envoy translates gRPC-Web requests to native gRPC. Browsers connect to the gateway on port 8443, and Envoy handles the protocol translation.

**2. Per-Service Proxy**: Each Globular service also runs a gRPC-Web reverse proxy on a companion port (service port + 1). This is used for direct service access during development or when the gateway is not available.

Content types supported:
- `application/grpc-web+proto` (binary protobuf)
- `application/grpc-web+json` (JSON encoding)

### TypeScript Client Library

Globular provides a TypeScript client library in the `typescript/` directory. It includes generated gRPC-Web clients for all services:

```
typescript/
├── authentication/
│   ├── authentication_pb.js         # Message types
│   └── authentication_grpc_web_pb.js # gRPC-Web client
├── rbac/
│   ├── rbac_pb.js
│   └── rbac_grpc_web_pb.js
└── ... (all services)
```

## TLS Architecture

### Mandatory TLS

All gRPC communication in Globular uses TLS. There is no plaintext gRPC option.

**Server TLS configuration** (applied to every service):
- Server certificate and key loaded from etcd-configured paths
- CA certificate from cluster configuration
- Client certificate requested (optional — JWT is the primary auth method)
- If a client certificate is presented, it's verified against the cluster CA

**gRPC keepalive settings**:
- Keepalive time: 30 seconds
- Keepalive timeout: 5 seconds
- Max connection idle: 2 minutes
- Max concurrent streams: 1,000,000

### Certificate Chain

```
Cluster CA (root of trust)
    │
    ├── Server certificates (one per node)
    │   └── SANs: node hostname, node IPs, "localhost"
    │
    └── Client certificates (for mTLS)
        └── CN: node identity or service account
```

### TLS Endpoint Resolution

When resolving a service endpoint for a TLS connection, the endpoint resolver applies special handling:

- **Loopback rewriting**: `127.0.0.1:PORT` → `localhost:PORT`. This is necessary because TLS certificates have SANs for DNS names (like "localhost"), not for IP literals. Connecting to `127.0.0.1` with a certificate that has SAN `localhost` would fail TLS verification.
- **Server name**: The TLS server name is set to the hostname portion of the endpoint, used for certificate verification.

## Practical Scenarios

### Scenario 1: Exposing a Service Externally

A new service needs to be accessible from outside the cluster:

```bash
# 1. The service registers in etcd when it starts (automatic)

# 2. The xDS server detects the registration and pushes routes to Envoy

# 3. External clients can now access the service:
#    - Native gRPC: via port 443 (TLS)
#    - gRPC-Web: via port 8443 (for browsers)

# 4. Add a DNS record for the service (optional, for friendly names)
globular dns record add --zone mycluster.local \
  --name myservice --type A --value <gateway-ip>
```

### Scenario 2: Diagnosing Connectivity Issues

A service cannot reach another service:

```bash
# 1. Check if the target service is registered in etcd
# (use MCP etcd tools or CLI)

# 2. Check if the target service is healthy
globular cluster health

# 3. Check DNS resolution
globular dns query --name <service>.mycluster.local

# 4. Check network connectivity
# From the source node:
# curl or grpcurl to the target endpoint

# 5. Check TLS certificates
globular node certificate-status --node <node>:11000
# Verify certificates are valid and not expired
```

### Scenario 3: Adding Custom DNS Records

Setting up DNS for an application:

```bash
# Create records for the application
globular dns record add --zone myapp.example.com \
  --name "@" --type A --value <gateway-ip>
globular dns record add --zone myapp.example.com \
  --name "api" --type A --value <gateway-ip>
globular dns record add --zone myapp.example.com \
  --name "www" --type CNAME --value "myapp.example.com"
```

## What's Next

- [Certificate Lifecycle](certificate-lifecycle.md): Certificate provisioning, rotation, and management
- [Writing a Microservice](../developers/writing-a-microservice.md): Build services that integrate with the network stack
