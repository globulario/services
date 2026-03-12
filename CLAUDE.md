# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Globular Services is a microservices platform for building self-hosted distributed applications. Built on gRPC with Protocol Buffers, it provides 28+ microservices across multiple languages (primarily Go, with TypeScript client support).

## Build Commands

```bash
# Build all Go services
cd golang && go build ./...

# Build a specific service
cd golang && go build ./authentication/authentication_server

# Run all tests
cd golang && go test ./... -race -coverprofile=coverage.out

# Run tests for a specific package
cd golang && go test ./echo/echo_server -v

# Lint (via CI)
golangci-lint run --timeout=5m

# Generate protobuf code from .proto files
./generateCode.sh

# Build all packages (infrastructure + services)
./build-all-packages.sh
```

## Project Structure

```
services/
├── golang/                     # PRIMARY - All Go microservices
│   ├── <service_name>/         # Each service has its own directory
│   │   ├── <service_name>pb/   # Generated protobuf code
│   │   ├── <service_name>_client/
│   │   └── <service_name>_server/
│   ├── globular_service/       # Shared service primitives (lifecycle, CLI, config)
│   ├── globular_client/        # Shared client primitives
│   ├── interceptors/           # gRPC interceptors (auth, audit, RBAC)
│   ├── config/                 # Configuration management (etcd backend)
│   └── go.mod                  # Go 1.24.5
├── typescript/                 # TypeScript web client library
├── proto/                      # Protocol buffer definitions (*.proto)
├── generated/                  # Generated specs and packages
└── build-all-packages.sh       # Full build script
```

## Service Architecture

Each Go service follows this structure:
```
service_name_server/
├── server.go       # Main server + gRPC registration
├── config.go       # Config struct, validation, persistence
├── handlers.go     # Business logic (refactored pattern)
├── *_test.go       # Tests
```

### Service Implementation Pattern

Services use shared primitives from `globular_service/`:

1. **CLI Helpers** - `globular.HandleInformationalFlags()`, `globular.ParsePositionalArgs()`
2. **Lifecycle Manager** - `globular.NewLifecycleManager()` for startup/shutdown
3. **Config Helpers** - `globular.SaveConfigToFile()`, `globular.ValidateCommonFields()`

Services implement two interfaces:
- `Service` interface (getters/setters for Name, Port, Domain, etc.)
- `LifecycleService` interface (`StartService()`, `StopService()`, `GetGrpcServer()`)

See `golang/MIGRATION_GUIDE.md` and `golang/SHARED_PRIMITIVES.md` for details.

## 4-Layer State Model

The platform tracks each package across 4 state layers:

| Layer | Source | Owner |
|-------|--------|-------|
| **Artifact** | Repository catalog (`repository.PackageRepository`) | `pkg publish` / `ensure-bootstrap-artifacts.sh` |
| **Desired Release** | Controller etcd (`/globular/resources/DesiredRelease/…`) | `globular services desired set` / `seed` |
| **Installed Observed** | Node Agent etcd (`/globular/nodes/{id}/packages/…`) | Node Agent (auto-populated from systemd) |
| **Runtime Health** | systemd + gRPC health checks | Gateway / admin metrics |

Status vocabulary (design-doc-aligned):
- **Installed** — desired == installed, converged
- **Planned** — desired set, not yet installed
- **Available** — in repo, no desired release
- **Drifted** — installed version differs from desired
- **Unmanaged** — installed without a desired-state entry
- **Missing in repo** — desired/installed but artifact not in repository
- **Orphaned** — in repo, not desired, not installed

CLI tools: `globular services repair [--dry-run]`, `globular services seed`

## Key Dependencies

- `google.golang.org/grpc` v1.78.0 - gRPC framework
- `go.etcd.io/etcd/client/v3` v3.5.14 - Distributed configuration
- `go.mongodb.org/mongo-driver` v1.16.0 - MongoDB
- `github.com/minio/minio-go/v7` - Object storage
- `github.com/prometheus/client_golang` - Metrics

## Protocol Buffers

Proto files are in `/proto/`. After modifying a `.proto` file:
```bash
./generateCode.sh   # Regenerates Go + TypeScript code
```

## Testing

- Unit tests alongside source files (`*_test.go`)
- Integration tests in server directories
- Test utilities in `golang/testutil/`

## CLI Tool (globularcli)

Located in `golang/globularcli/`. Commands include:
```bash
globular cluster bootstrap    # Initialize first node
globular cluster join         # Add nodes to cluster
globular cluster token create # Create join tokens
globular pkg build            # Build service packages
```

## Default Ports

- Authentication: 10101
- Event: 10102
- File: 10103
- RBAC: 10104
- Node Agent: 11000
- Cluster Controller: 12000

## Security Constraints (Makefile)

The Makefile enforces security checks:
- `clustercontroller_server` must NOT use `os/exec`, `syscall`, or `systemctl`
- `nodeagent_server` can only use `os/exec` within `internal/supervisor/`

Run checks: `make check-services`