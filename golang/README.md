<p align="center">
  <img src="logo.png" alt="Globular Logo" width="200"/>
</p>

<h1 align="center">Globular Services</h1>

<p align="center">
  <strong>A comprehensive microservices platform for building self-hosted applications</strong>
</p>

<p align="center">
  <a href="#architecture">Architecture</a> •
  <a href="#services">Services</a> •
  <a href="#getting-started">Getting Started</a> •
  <a href="#deployment">Deployment</a>
</p>

---

## Overview

Globular is a self-hosted application platform that provides a complete suite of microservices for building modern, distributed applications. Built on gRPC, it offers efficient, typed inter-service communication with support for streaming, authentication, and role-based access control.

### Key Features

- **Polyglot Persistence** - Multiple database backends (MongoDB, SQL, ScyllaDB, LevelDB, BadgerDB, etcd)
- **Event-Driven Architecture** - Pub/sub messaging for loose coupling between services
- **Cluster-First Design** - Distributed deployment with orchestrated updates
- **Full-Text Search** - Built-in search indexing for content discovery
- **Media Processing** - Video/audio conversion, streaming, and yt-dlp integration
- **Identity & Access Control** - Authentication with fine-grained RBAC permissions
- **External Integration** - LDAP, SMTP, DNS, and more

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Globular Platform                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                    INFRASTRUCTURE LAYER                             │    │
│  │                                                                      │    │
│  │   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌───────────┐  │    │
│  │   │Authentication│  │    Event    │  │     Log     │  │    DNS    │  │    │
│  │   │   Service   │  │   Service   │  │   Service   │  │  Service  │  │    │
│  │   └─────────────┘  └─────────────┘  └─────────────┘  └───────────┘  │    │
│  │                                                                      │    │
│  │   ┌─────────────┐  ┌─────────────┐                                  │    │
│  │   │  Cluster    │  │    Node     │                                  │    │
│  │   │ Controller  │  │    Agent    │                                  │    │
│  │   └─────────────┘  └─────────────┘                                  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                      DATA LAYER                                      │    │
│  │                                                                      │    │
│  │   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌───────────┐  │    │
│  │   │ Persistence │  │   Storage   │  │   Search    │  │    SQL    │  │    │
│  │   │   Service   │  │   Service   │  │   Service   │  │  Service  │  │    │
│  │   └─────────────┘  └─────────────┘  └─────────────┘  └───────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                    APPLICATION LAYER                                 │    │
│  │                                                                      │    │
│  │   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌───────────┐  │    │
│  │   │    File     │  │ Repository  │  │  Resource   │  │  Catalog  │  │    │
│  │   │   Service   │  │   Service   │  │   Service   │  │  Service  │  │    │
│  │   └─────────────┘  └─────────────┘  └─────────────┘  └───────────┘  │    │
│  │                                                                      │    │
│  │   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌───────────┐  │    │
│  │   │    Blog     │  │Conversation │  │    Media    │  │   Title   │  │    │
│  │   │   Service   │  │   Service   │  │   Service   │  │  Service  │  │    │
│  │   └─────────────┘  └─────────────┘  └─────────────┘  └───────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                    INTEGRATION LAYER                                 │    │
│  │                                                                      │    │
│  │   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌───────────┐  │    │
│  │   │    LDAP     │  │    Mail     │  │  Monitoring │  │  Torrent  │  │    │
│  │   │   Service   │  │   Service   │  │   Service   │  │  Service  │  │    │
│  │   └─────────────┘  └─────────────┘  └─────────────┘  └───────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                    SECURITY LAYER                                    │    │
│  │                                                                      │    │
│  │   ┌─────────────┐  ┌─────────────┐                                  │    │
│  │   │    RBAC     │  │  Discovery  │                                  │    │
│  │   │   Service   │  │   Service   │                                  │    │
│  │   └─────────────┘  └─────────────┘                                  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Services

### Infrastructure Services

Core services that provide the foundation for all other services.

| Service | Description | Documentation |
|---------|-------------|---------------|
| [Authentication](authentication/README.md) | User identity, tokens, and credential management | [Details](authentication/README.md) |
| [Event](event/README.md) | Pub/sub event bus for inter-service communication | [Details](event/README.md) |
| [Log](log/README.md) | Centralized logging and audit trail | [Details](log/README.md) |
| [DNS](dns/README.md) | DNS record management (A, AAAA, CNAME, MX, etc.) | [Details](dns/README.md) |
| [Cluster Controller](clustercontroller/README.md) | Cluster orchestration and node management | [Details](clustercontroller/README.md) |
| [Node Agent](nodeagent/README.md) | Local node orchestration and plan execution | [Details](nodeagent/README.md) |

### Data Layer Services

Services that provide data persistence and retrieval capabilities.

| Service | Description | Documentation |
|---------|-------------|---------------|
| [Persistence](persistence/README.md) | Universal database abstraction (MongoDB, SQL, ScyllaDB) | [Details](persistence/README.md) |
| [Storage](storage/README.md) | Key-value store abstraction (LevelDB, BadgerDB, etcd) | [Details](storage/README.md) |
| [Search](search/README.md) | Full-text search indexing and retrieval | [Details](search/README.md) |
| [SQL](sql/README.md) | SQL database abstraction (MySQL, PostgreSQL, SQLite) | [Details](sql/README.md) |

### Application Services

Services for building application features.

| Service | Description | Documentation |
|---------|-------------|---------------|
| [File](file/README.md) | File system operations and document management | [Details](file/README.md) |
| [Repository](repository/README.md) | Artifact storage and version management | [Details](repository/README.md) |
| [Resource](resource/README.md) | Package, role, and account management | [Details](resource/README.md) |
| [Catalog](catalog/README.md) | Inventory and product catalog management | [Details](catalog/README.md) |
| [Blog](blog/README.md) | Blogging platform with content management | [Details](blog/README.md) |
| [Conversation](conversation/README.md) | Real-time messaging and group conversations | [Details](conversation/README.md) |
| [Media](media/README.md) | Video/audio processing and streaming | [Details](media/README.md) |
| [Title](title/README.md) | Media metadata database | [Details](title/README.md) |

### Integration Services

Services for integrating with external systems.

| Service | Description | Documentation |
|---------|-------------|---------------|
| [LDAP](ldap/README.md) | LDAP directory integration and sync | [Details](ldap/README.md) |
| [Mail](mail/README.md) | Email delivery system (SMTP) | [Details](mail/README.md) |
| [Monitoring](monitoring/README.md) | Time-series metrics (Prometheus integration) | [Details](monitoring/README.md) |
| [Torrent](torrent/README.md) | Torrent download management | [Details](torrent/README.md) |

### Security Services

Services for access control and governance.

| Service | Description | Documentation |
|---------|-------------|---------------|
| [RBAC](rbac/README.md) | Role-based access control and permissions | [Details](rbac/README.md) |
| [Discovery](discovery/README.md) | Service and application discovery/publishing | [Details](discovery/README.md) |

### Utility Services

| Service | Description | Documentation |
|---------|-------------|---------------|
| [Echo](echo/README.md) | Simple request-response testing | [Details](echo/README.md) |

---

## Service Dependencies

```
                    ┌──────────────────┐
                    │  Authentication  │
                    └────────┬─────────┘
                             │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
         ▼                   ▼                   ▼
    ┌─────────┐        ┌─────────┐        ┌─────────┐
    │  Event  │        │   RBAC  │        │   Log   │
    └────┬────┘        └────┬────┘        └─────────┘
         │                  │
         │    ┌─────────────┴─────────────┐
         │    │                           │
         ▼    ▼                           ▼
    ┌───────────────┐              ┌─────────────┐
    │  Persistence  │              │  Resource   │
    │    Storage    │              │  Repository │
    │    Search     │              │  Discovery  │
    └───────┬───────┘              └──────┬──────┘
            │                             │
            │    ┌────────────────────────┘
            │    │
            ▼    ▼
    ┌───────────────────────────────────────────┐
    │           Application Services            │
    │  (File, Media, Blog, Catalog, etc.)       │
    └───────────────────────────────────────────┘
```

---

## Getting Started

### Prerequisites

- Go 1.21+
- Protocol Buffers compiler (protoc)
- gRPC tools

### Building Services

```bash
# Build all services
cd golang
go build ./...

# Build a specific service
go build ./authentication/authentication_server

# Run tests
go test ./...
```

### Running a Service

Each service can be run standalone or as part of a cluster:

```bash
# Standalone mode
./authentication_server --port 10101

# With configuration file
./authentication_server --config /etc/globular/authentication.json
```

---

## Deployment

### Single Node

For development or small deployments:

```bash
# Bootstrap a single-node cluster
globular cluster bootstrap \
  --node=localhost:11000 \
  --domain=myserver.local \
  --profile=core
```

### Multi-Node Cluster

For production deployments:

```bash
# On first node: Bootstrap
globular cluster bootstrap \
  --node=localhost:11000 \
  --domain=prod.example.com \
  --profile=core

# Create join token
globular cluster token create --expires=24h

# On additional nodes: Join
globular cluster join \
  --controller=192.168.1.10:12000 \
  --join-token=<token>

# Approve join requests
globular cluster requests approve <id> --profile=core
```

See the [Cluster Controller documentation](clustercontroller/README.md) for detailed setup instructions.

---

## Configuration

### Environment Variables

Common environment variables used across services:

| Variable | Description | Default |
|----------|-------------|---------|
| `GLOBULAR_DATA_PATH` | Data storage directory | `/var/lib/globular` |
| `GLOBULAR_CONFIG_PATH` | Configuration directory | `/etc/globular` |
| `GLOBULAR_LOG_LEVEL` | Log verbosity | `INFO` |

### Service Ports

Default port assignments:

| Service | Default Port |
|---------|-------------|
| Authentication | 10101 |
| Event | 10102 |
| File | 10103 |
| RBAC | 10104 |
| Resource | 10105 |
| Discovery | 10106 |
| DNS | 10107 |
| Persistence | 10108 |
| Storage | 10109 |
| Search | 10110 |
| Node Agent | 11000 |
| Cluster Controller | 12000 |

---

## Communication Protocol

All services communicate via **gRPC** with Protocol Buffers serialization:

- **Unary RPCs** - Simple request/response
- **Server Streaming** - Server sends multiple responses
- **Client Streaming** - Client sends multiple requests
- **Bidirectional Streaming** - Both directions

### Authentication Flow

```
┌────────┐                    ┌──────────────────┐
│ Client │                    │  Authentication  │
└───┬────┘                    │     Service      │
    │                         └────────┬─────────┘
    │  1. Authenticate(user, pass)     │
    │─────────────────────────────────▶│
    │                                  │
    │  2. Token                        │
    │◀─────────────────────────────────│
    │                                  │
    │  3. Request + Token              │
    │─────────────────────────────────▶│ Other Service
    │                                  │
    │  4. ValidateToken(token)         │
    │                                  │────▶ Auth Service
    │                                  │◀────
    │  5. Response                     │
    │◀─────────────────────────────────│
```

---

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Write tests
5. Submit a pull request

### Code Style

- Follow Go conventions
- Use `gofmt` for formatting
- Write meaningful commit messages
- Include tests for new features

---

## License

This project is licensed under the MIT License - see the LICENSE file for details.

---

<p align="center">
  <sub>Built with Go and gRPC</sub>
</p>
