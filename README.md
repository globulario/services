# Globular Services

Microservices platform for self-hosted distributed applications. Built on gRPC with Protocol Buffers, running as native Linux binaries under systemd. etcd as the single source of truth. Workflow-driven convergence across a 4-layer state model (Repository → Desired → Installed → Runtime).

## Architecture

```
Proto definitions (proto/)
    ↓ generateCode.sh
Go services (golang/) + TypeScript clients (typescript/)
    ↓ build-all-packages.sh
Package archives (.tgz)
    ↓ globular-installer
Running cluster (systemd + etcd + Envoy)
```

## Repository Structure

```
services/
├── proto/                     # 38 Protocol Buffer definitions (source of truth)
├── golang/                    # All Go microservices (33 services + CLI + MCP)
│   ├── cluster_controller/    # Cluster management, desired state, workflows
│   ├── node_agent/            # Node-local executor, package management
│   ├── workflow/              # Centralized workflow execution engine
│   ├── repository/            # Package artifact registry (MinIO-backed)
│   ├── authentication/        # JWT token management
│   ├── rbac/                  # Role-based access control
│   ├── dns/                   # Authoritative DNS + zone management
│   ├── monitoring/            # Prometheus adapter
│   ├── backup_manager/        # Distributed backup orchestration
│   ├── cluster_doctor/        # Health analysis + auto-heal
│   ├── ai_memory/             # Persistent AI knowledge (ScyllaDB)
│   ├── ai_executor/           # AI diagnosis + remediation
│   ├── ai_watcher/            # Event-driven incident detection
│   ├── ai_router/             # Dynamic routing policies
│   ├── compute/               # Distributed batch job execution
│   ├── domain/                # External domain + ACME cert management
│   ├── globularcli/           # CLI tool
│   ├── mcp/                   # Model Context Protocol server (129+ tools)
│   ├── globular_service/      # Shared service primitives
│   ├── globular_client/       # Shared client primitives
│   ├── interceptors/          # gRPC auth, RBAC, audit middleware
│   ├── config/                # etcd-backed configuration
│   ├── security/              # TLS, PKI, JWT, Ed25519 keystore
│   └── ...                    # 15+ additional services
├── typescript/                # TypeScript gRPC-Web client library
├── generated/                 # Generated specs, packages, policies
├── generateCode.sh            # Proto → Go/TypeScript code generation
└── build-all-packages.sh      # Full package build pipeline
```

## Quick Start

### Build from Source

```bash
# Requires: Go 1.24+, protoc, protoc-gen-go

# Generate code and build all services
bash generateCode.sh

# Build all packages (infrastructure + services)
bash build-all-packages.sh
```

### Run Tests

```bash
cd golang
go test ./... -race
```

### Build a Specific Service

```bash
cd golang
go build ./echo/echo_server
```

## Services

### Control Plane

| Service | Port | Description |
|---------|------|-------------|
| Cluster Controller | 12000 | Cluster management, desired state, membership, workflow dispatch |
| Node Agent | 11000 | Local executor, package tracking, service control |
| Workflow Service | 10004 | Centralized workflow execution and tracking |
| Cluster Doctor | 12005 | Health analysis, drift detection, auto-heal |

### Infrastructure

| Service | Port | Description |
|---------|------|-------------|
| etcd | 2379/2380 | Distributed configuration and state store |
| MinIO | 9000 | Object storage (packages, backups, artifacts) |
| Envoy Gateway | 443/8443 | TLS termination, xDS routing, gRPC-Web |
| Prometheus | 9090 | Metrics collection |
| Alertmanager | 9093 | Alert routing |
| ScyllaDB | 9042 | High-throughput data (AI memory, DNS storage) |

### Core Services

| Service | Port | Description |
|---------|------|-------------|
| Authentication | 10101 | JWT tokens, password management |
| RBAC | 10104 | Permission enforcement |
| Event | 10102 | Publish-subscribe event bus |
| File | 10103 | File management |
| DNS | 10006 | Authoritative DNS, zone management |
| Discovery | 10029 | Service discovery, install plans |
| Repository | — | Package artifact registry |
| Resource | — | Package descriptors, accounts, groups |
| Log | 10100 | Centralized logging |

### AI Services

| Service | Port | Description |
|---------|------|-------------|
| AI Memory | 10200 | Persistent knowledge store (ScyllaDB) |
| AI Watcher | 10210 | Event monitoring, incident detection |
| AI Router | 10220 | Dynamic routing policy computation |
| AI Executor | 10230 | Incident diagnosis and remediation |

### Operational

| Service | Port | Description |
|---------|------|-------------|
| Monitoring | 10019 | Prometheus API adapter |
| Backup Manager | 10040 | Backup orchestration |
| MCP Server | 10260 | AI agent interface (129+ diagnostic tools) |
| Domain Reconciler | — | External domain + ACME cert management (runs in controller) |

### Application Services

| Service | Description |
|---------|-------------|
| Persistence | MongoDB access layer |
| Storage | Key-value store (BadgerDB) |
| Search | Full-text search (Bleve) |
| Media | Audio/video management |
| Title | Metadata service |
| Mail | SMTP email |
| LDAP | LDAP authentication provider |
| SQL | SQL database access |
| Blog | CMS engine |
| Conversation | Chat management |
| Catalog | Component catalog |
| Torrent | Torrent downloads |

## Documentation

Full documentation is in [`docs/`](docs/index.md):

- [Getting Started](docs/getting-started.md) — From zero to running cluster
- [Architecture](docs/operators/architecture-overview.md) — How components interact
- [Day-0/1/2 Operations](docs/operators/day-0-1-2-operations.md) — Complete lifecycle
- [Building from Source](docs/operators/building-from-source.md) — Build process
- [AI Layer](docs/ai/ai-overview.md) — AI services, rules, and agent model
- [Developer Guide](docs/developers/local-first.md) — Local-first development

## Key Design Principles

- **etcd is the single source of truth** — no environment variables, no hardcoded addresses
- **Workflow-driven convergence** — all state changes go through the workflow engine
- **4-layer state model** — Repository → Desired → Installed → Runtime (never collapsed)
- **Native binaries under systemd** — no containers required
- **Local-first** — services run standalone with `go run`, no cluster needed for development

## License

See [LICENSE](LICENSE) for details.
