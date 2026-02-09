# Shared Service Primitives - Developer Guide

## Overview

This document describes the shared primitives available in `globular_service/` for building consistent, maintainable Globular services. These primitives were extracted from the Echo, Discovery, and Repository services in Phase 1-2 refactoring.

**Benefits:**
- Reduce code duplication across services
- Consistent patterns and behavior
- Easier maintenance and testing
- Faster service development

## Available Primitives

### 1. CLI Helpers (`cli_helpers.go`)

Common command-line interface helpers that process flags and arguments.

**File:** `golang/globular_service/cli_helpers.go` (143 lines)

#### Functions

**`HandleInformationalFlags(srv Service, args []string, logger *slog.Logger, printUsage func()) bool`**

Processes informational flags like `--version`, `--help`, `--describe`, `--health`.

**Returns:** `true` if an informational flag was handled (service should exit), `false` otherwise

**Usage:**
```go
func main() {
    srv := initializeServerDefaults()
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

    if globular.HandleInformationalFlags(srv, os.Args[1:], logger, printUsage) {
        return // Exit after handling flag
    }

    // Continue with normal startup...
}
```

**Note:** The `--debug` flag should be handled by your service BEFORE calling this function, as it affects logging configuration.

**`HandleDescribeFlag(srv Service, logger *slog.Logger)`**

Outputs service metadata as JSON and exits.

**`HandleHealthFlag(srv Service, logger *slog.Logger)`**

Performs health check and exits with status code.

**`ParsePositionalArgs(srv Service, args []string)`**

Extracts `service_id` and `config_path` from positional arguments.

**Format:** `<service_id> [config_path]`

**`AllocatePortIfNeeded(srv Service, args []string) error`**

Allocates a new port if `--port 0` is specified in arguments.

**`LoadRuntimeConfig(srv Service)`**

Loads domain and address from environment variables:
- `GLOBULAR_DOMAIN`
- `GLOBULAR_ADDRESS`

#### Interface Requirements

Your service's `*server` type must implement the `Service` interface (from `services.go`):

```go
type Service interface {
    GetId() string
    SetId(string)
    GetName() string
    GetDomain() string
    SetDomain(string)
    GetAddress() string
    SetAddress(string)
    GetPort() int
    SetPort(int)
    GetVersion() string
    GetProcess() int
    SetProcess(int)
    GetConfigurationPath() string
    SetConfigurationPath(string)
    GetState() string
    SetState(string)
    // ... (see services.go for complete interface)
}
```

---

### 2. Lifecycle Manager (`lifecycle.go`)

Generic lifecycle management for service startup, shutdown, and health checks.

**File:** `golang/globular_service/lifecycle.go` (199 lines)

#### Interface

Your service must implement `LifecycleService`:

```go
type LifecycleService interface {
    GetId() string
    GetName() string
    GetPort() int
    GetState() string
    SetState(string)
    StartService() error
    StopService() error
    GetGrpcServer() *grpc.Server
}
```

#### Usage

**1. Implement the interface in your server:**

```go
type server struct {
    // ... your fields ...
    grpcServer *grpc.Server
}

// Lifecycle methods
func (s *server) StartService() error {
    // Initialize your service (connect to databases, etc.)
    return nil
}

func (s *server) StopService() error {
    // Cleanup resources
    return nil
}

func (s *server) GetGrpcServer() *grpc.Server {
    return s.grpcServer
}

// Service interface getters/setters
func (s *server) GetId() string { return s.Id }
func (s *server) GetName() string { return s.Name }
// ... etc
```

**2. Create lifecycle manager in main():**

```go
func main() {
    srv := initializeServerDefaults()
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

    // ... handle flags, setup gRPC ...

    // Create lifecycle manager
    lifecycle := globular.NewLifecycleManager(srv, logger)

    // Start the service
    if err := lifecycle.Start(); err != nil {
        logger.Error("failed to start service", "error", err)
        os.Exit(1)
    }

    // Wait for termination signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan

    // Graceful shutdown
    if err := lifecycle.GracefulShutdown(30 * time.Second); err != nil {
        logger.Error("shutdown failed", "error", err)
        os.Exit(1)
    }
}
```

#### Lifecycle Manager Methods

**`NewLifecycleManager(srv LifecycleService, logger *slog.Logger) *LifecycleManager`**

Creates a new lifecycle manager.

**`Start() error`**

Starts the service:
1. Calls `srv.StartService()` for initialization
2. Starts the gRPC server in a goroutine
3. Sets state to "running"

**`Stop() error`**

Stops the service:
1. Gracefully stops the gRPC server
2. Calls `srv.StopService()` for cleanup
3. Sets state to "stopped"

**`Ready() bool`**

Returns `true` if the service is running.

**`Health() error`**

Performs health check, returns error if unhealthy.

**`GracefulShutdown(timeout time.Duration) error`**

Attempts graceful shutdown with timeout. If timeout is exceeded, forces shutdown.

**`AwaitReady(timeout time.Duration) error`**

Waits for the service to become ready, returns error if timeout is exceeded.

---

### 3. Config Helpers (`config_helpers.go`)

Common configuration management operations.

**File:** `golang/globular_service/config_helpers.go` (119 lines)

#### Functions

**`SaveConfigToFile(cfg any, path string) error`**

Writes a configuration struct to a JSON file.

**Features:**
- Creates directory if it doesn't exist
- Pretty-prints JSON with indentation
- Returns descriptive errors

**Usage:**
```go
func (c *Config) SaveToFile(path string) error {
    return globular.SaveConfigToFile(c, path)
}
```

**`LoadConfigFromFile(path string, cfg any) error`**

Reads a configuration struct from a JSON file.

**Usage:**
```go
func LoadFromFile(path string) (*Config, error) {
    cfg := &Config{}
    if err := globular.LoadConfigFromFile(path, cfg); err != nil {
        return nil, err
    }
    return cfg, nil
}
```

**`ValidateCommonFields(name string, port, proxy int, protocol, version string) error`**

Validates common configuration fields present in all services.

**Checks:**
- Name is not empty
- Port is between 1 and 65535
- Proxy port is between 1 and 65535
- Protocol is not empty
- Version is not empty

**Usage:**
```go
func (c *Config) Validate() error {
    // Validate common fields first
    if err := globular.ValidateCommonFields(c.Name, c.Port, c.Proxy, c.Protocol, c.Version); err != nil {
        return err
    }

    // Add service-specific validation
    if len(c.Dependencies) == 0 {
        return fmt.Errorf("dependencies are required")
    }

    return nil
}
```

**`CloneStringSlice(src []string) []string`**

Creates a deep copy of a string slice. Handles `nil` slices correctly.

**Usage:**
```go
func (c *Config) Clone() *Config {
    clone := *c
    clone.Keywords = globular.CloneStringSlice(c.Keywords)
    clone.Repositories = globular.CloneStringSlice(c.Repositories)
    return &clone
}
```

**`GetDefaultDomainAddress(port int) (domain string, address string)`**

Returns default domain and address values from environment variables.

**Environment Variables:**
- `GLOBULAR_DOMAIN` - defaults to "localhost" if not set
- `GLOBULAR_ADDRESS` - defaults to "localhost:port" if not set

**Usage:**
```go
func DefaultConfig() *Config {
    cfg := &Config{
        Name:     "myservice.MyService",
        Port:     10000,
        Protocol: "grpc",
        Version:  "0.0.1",
        // ...
    }

    // Set domain and address from environment
    cfg.Domain, cfg.Address = globular.GetDefaultDomainAddress(cfg.Port)

    return cfg
}
```

---

## Design Patterns

### Composition Over Inheritance

These primitives use **composition** rather than inheritance:

❌ **Not this (inheritance):**
```go
type server struct {
    BaseService  // Embedded base struct
    // service-specific fields
}
```

✅ **This (composition):**
```go
type server struct {
    // Service fields as before
    Name   string
    Port   int
    // ...
}

// Use shared helpers via functions
func main() {
    srv := &server{/* ... */}

    // Use shared helpers
    globular.HandleInformationalFlags(srv, args, logger, printUsage)
    lifecycle := globular.NewLifecycleManager(srv, logger)
}
```

**Benefits:**
- Services maintain full control over their structure
- No hidden behavior from base classes
- Clear dependencies and interfaces
- Easy to understand and test

### Interface-Based Design

Shared primitives use **minimal interfaces**:

```go
// CLI helpers only need basic getters/setters
type Service interface {
    GetId() string
    SetId(string)
    // ... minimal interface
}

// Lifecycle only needs lifecycle operations
type LifecycleService interface {
    StartService() error
    StopService() error
    GetGrpcServer() *grpc.Server
    // ... minimal interface
}
```

**Benefits:**
- Services implement only what they need
- Clear contracts
- Easy to test with mocks
- Flexible service implementations

---

## Testing

### Unit Testing Shared Primitives

The shared primitives are tested in their own test files:

- `cli_helpers_test.go` - Tests for CLI helper functions
- `lifecycle_test.go` - Tests for lifecycle manager
- `config_helpers_test.go` - Tests for config helpers (if created)

### Integration Testing in Services

Each service should test that it correctly uses the shared primitives:

```go
func TestLifecycleIntegration(t *testing.T) {
    srv := initializeServerDefaults()
    logger := slog.New(slog.NewTextHandler(io.Discard, nil))

    lifecycle := globular.NewLifecycleManager(srv, logger)

    // Test Start
    if err := lifecycle.Start(); err != nil {
        t.Fatalf("Start failed: %v", err)
    }

    // Test Ready
    if !lifecycle.Ready() {
        t.Error("Service should be ready after Start")
    }

    // Test Stop
    if err := lifecycle.Stop(); err != nil {
        t.Fatalf("Stop failed: %v", err)
    }
}
```

---

## Best Practices

### 1. Service Structure

Organize your service files consistently:

```
myservice/myservice_server/
├── server.go          # Server struct, main(), helper functions
├── config.go          # Config struct, validation, persistence
├── handlers.go        # Business logic (pure functions)
├── server_test.go     # Server tests
├── config_test.go     # Config tests
└── handlers_test.go   # Handler tests
```

### 2. Use Shared Helpers Consistently

- ✅ Always use `globular.HandleInformationalFlags()` for CLI flags
- ✅ Always use `globular.NewLifecycleManager()` for lifecycle management
- ✅ Always use config helpers in your config.go file

### 3. Service-Specific Code

Keep service-specific code in your service:

- Custom initialization in `StartService()`
- Custom cleanup in `StopService()`
- Service-specific validation in `Validate()`
- Business logic in `handlers.go`

### 4. Documentation

Document service-specific behavior:

```go
// StartService initializes the repository's package storage.
// It creates the Root directory if it doesn't exist and validates
// that it's writable.
func (s *server) StartService() error {
    // Service-specific initialization
    if err := os.MkdirAll(s.Root, 0755); err != nil {
        return fmt.Errorf("failed to create root directory: %w", err)
    }
    return nil
}
```

---

## Migration Checklist

See [MIGRATION_GUIDE.md](./MIGRATION_GUIDE.md) for step-by-step instructions on refactoring an existing service to use these shared primitives.

---

## Examples

See the following services for complete examples:

- **Echo** (`echo/echo_server/`) - Simple service with minimal configuration
- **Discovery** (`discovery/discovery_server/`) - Medium complexity with service-specific validation
- **Repository** (`repository/repository_server/`) - Service-specific fields (Root) and simple validation

---

## Contributing

When adding new shared primitives:

1. Extract only code that is **100% identical** across multiple services
2. Use **minimal interfaces** - services implement only what they need
3. Preserve **service-specific behavior** in services, not in shared code
4. Add **comprehensive tests** for the shared primitive
5. Update this documentation with usage examples
6. Update the migration guide if needed

---

## Questions?

For questions or issues with shared primitives:

1. Check existing service implementations (Echo, Discovery, Repository)
2. Review test files for usage examples
3. Check git history for refactoring decisions
4. Open an issue on GitHub

---

**Last Updated:** 2026-02-08
**Phase:** 2 (Steps 1-3 Complete)
**Services Using Primitives:** Echo, Discovery, Repository (35 tests passing)
