# Service Refactoring Migration Guide

## Overview

This guide walks you through refactoring an existing Globular service to use the shared primitives from Phase 1-2 refactoring. The proven pattern has been applied to Echo, Discovery, and Repository services with 100% test pass rate maintained throughout.

**Time Estimate:** 2-3 hours per service (depending on complexity)

**Prerequisites:**
- Existing service with tests
- Familiarity with the service's business logic
- Go 1.18+ (for generics and modern features)

---

## The Pattern - 6 Steps

### Overview of Steps

1. **Extract Config Component** - Separate configuration from server struct
2. **Extract Handlers** - Pure business logic functions
3. **Use Shared Lifecycle** - Replace local lifecycle with `globular.NewLifecycleManager()`
4. **Use Shared CLI Helpers** - Replace CLI flag handling
5. **Use Shared Config Helpers** - Replace config persistence/validation
6. **Cleanup & Validation** - Remove duplicates, run tests, document

**Goal:** Reduce main() from 150+ lines to ~40-50 lines while maintaining all functionality.

---

## Step 1: Extract Config Component

**Goal:** Create a separate `config.go` file with the Config struct and related methods.

### 1.1 Create `config.go`

Create a new file `config.go` in your service's directory:

```go
package main

import (
    "github.com/globulario/services/golang/globular_service"
)

// Config holds the [ServiceName] service configuration.
type Config struct {
    // Core service identity
    ID          string   `json:"Id"`
    Name        string   `json:"Name"`
    Domain      string   `json:"Domain"`
    Address     string   `json:"Address"`
    Port        int      `json:"Port"`
    Proxy       int      `json:"Proxy"`
    Protocol    string   `json:"Protocol"`
    Version     string   `json:"Version"`
    PublisherID string   `json:"PublisherId"`
    Description string   `json:"Description"`
    Keywords    []string `json:"Keywords"`

    // Service discovery
    Repositories []string `json:"Repositories"`
    Discoveries  []string `json:"Discoveries"`

    // Dependencies
    Dependencies []string `json:"Dependencies"`

    // Policy & Operations
    AllowAllOrigins bool   `json:"AllowAllOrigins"`
    AllowedOrigins  string `json:"AllowedOrigins"`
    KeepUpToDate    bool   `json:"KeepUpToDate"`
    KeepAlive       bool   `json:"KeepAlive"`

    // TLS configuration
    TLS struct {
        Enabled            bool   `json:"TLS"`
        CertFile           string `json:"CertFile"`
        KeyFile            string `json:"KeyFile"`
        CertAuthorityTrust string `json:"CertAuthorityTrust"`
    } `json:"TLS"`

    // Permissions
    Permissions []any `json:"Permissions"`

    // ⚠️ Add your service-specific fields here
    // Example: Root string `json:"Root"` // For repository
}
```

### 1.2 Add DefaultConfig()

```go
// DefaultConfig returns a Config with [ServiceName] service defaults.
func DefaultConfig() *Config {
    cfg := &Config{
        Name:        "myservice.MyService",
        Port:        10000, // ⚠️ Use your service's default port
        Proxy:       10001,
        Protocol:    "grpc",
        Version:     "0.0.1",
        PublisherID: "localhost",
        Description: "Your service description",
        Keywords:    []string{"keyword1", "keyword2"},

        Repositories: []string{},
        Discoveries:  []string{},
        Dependencies: []string{}, // ⚠️ Add your dependencies

        AllowAllOrigins: true,
        AllowedOrigins:  "",
        KeepUpToDate:    true,
        KeepAlive:       true,

        Permissions: []any{}, // ⚠️ Add your permissions
    }

    cfg.TLS.Enabled = false

    // Set domain and address from environment
    cfg.Domain, cfg.Address = globular_service.GetDefaultDomainAddress(cfg.Port)

    return cfg
}
```

### 1.3 Add Validation

```go
// Validate checks that required configuration fields are set correctly.
func (c *Config) Validate() error {
    // Validate common fields
    if err := globular_service.ValidateCommonFields(c.Name, c.Port, c.Proxy, c.Protocol, c.Version); err != nil {
        return err
    }

    // ⚠️ Add service-specific validation here
    // Example:
    // if len(c.Dependencies) == 0 {
    //     return fmt.Errorf("dependencies are required")
    // }

    return nil
}
```

### 1.4 Add File Operations

```go
// SaveToFile writes the configuration to a JSON file.
func (c *Config) SaveToFile(path string) error {
    return globular_service.SaveConfigToFile(c, path)
}

// LoadFromFile reads configuration from a JSON file.
func LoadFromFile(path string) (*Config, error) {
    cfg := &Config{}
    if err := globular_service.LoadConfigFromFile(path, cfg); err != nil {
        return nil, err
    }
    return cfg, nil
}
```

### 1.5 Add Clone Method

```go
// Clone creates a deep copy of the configuration.
func (c *Config) Clone() *Config {
    clone := &Config{
        ID:              c.ID,
        Name:            c.Name,
        Domain:          c.Domain,
        Address:         c.Address,
        Port:            c.Port,
        Proxy:           c.Proxy,
        Protocol:        c.Protocol,
        Version:         c.Version,
        PublisherID:     c.PublisherID,
        Description:     c.Description,
        Keywords:        globular_service.CloneStringSlice(c.Keywords),
        Repositories:    globular_service.CloneStringSlice(c.Repositories),
        Discoveries:     globular_service.CloneStringSlice(c.Discoveries),
        Dependencies:    globular_service.CloneStringSlice(c.Dependencies),
        AllowAllOrigins: c.AllowAllOrigins,
        AllowedOrigins:  c.AllowedOrigins,
        KeepUpToDate:    c.KeepUpToDate,
        KeepAlive:       c.KeepAlive,
    }

    // Deep copy TLS
    clone.TLS.Enabled = c.TLS.Enabled
    clone.TLS.CertFile = c.TLS.CertFile
    clone.TLS.KeyFile = c.TLS.KeyFile
    clone.TLS.CertAuthorityTrust = c.TLS.CertAuthorityTrust

    // ⚠️ Deep copy Permissions if needed (complex structures)
    // ⚠️ Deep copy service-specific fields

    return clone
}
```

### 1.6 Create Config Tests

Create `config_test.go` with tests for:
- DefaultConfig
- Validation (valid and invalid cases)
- File operations (SaveToFile, LoadFromFile)
- Clone (verify deep copy)
- Service-specific fields

**Template:**
```go
package main

import (
    "os"
    "path/filepath"
    "testing"
)

func TestDefaultConfig(t *testing.T) {
    cfg := DefaultConfig()
    if cfg == nil {
        t.Fatal("DefaultConfig returned nil")
    }
    if err := cfg.Validate(); err != nil {
        t.Errorf("Default config validation failed: %v", err)
    }
}

func TestConfigValidation(t *testing.T) {
    tests := []struct {
        name    string
        config  *Config
        wantErr bool
    }{
        {
            name:    "valid default config",
            config:  DefaultConfig(),
            wantErr: false,
        },
        {
            name: "missing name",
            config: &Config{
                Port:     10000,
                Proxy:    10001,
                Protocol: "grpc",
                Version:  "0.0.1",
            },
            wantErr: true,
        },
        // ⚠️ Add more validation test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}

func TestConfigFileOperations(t *testing.T) {
    cfg := DefaultConfig()
    tmpDir := t.TempDir()
    configPath := filepath.Join(tmpDir, "config.json")

    // Test SaveToFile
    if err := cfg.SaveToFile(configPath); err != nil {
        t.Fatalf("SaveToFile failed: %v", err)
    }

    // Test LoadFromFile
    loaded, err := LoadFromFile(configPath)
    if err != nil {
        t.Fatalf("LoadFromFile failed: %v", err)
    }

    if loaded.Name != cfg.Name {
        t.Errorf("Name mismatch: got %v, want %v", loaded.Name, cfg.Name)
    }
}

func TestConfigClone(t *testing.T) {
    original := DefaultConfig()
    clone := original.Clone()

    // Verify deep copy
    clone.Name = "modified"
    if original.Name == clone.Name {
        t.Error("Clone is not a deep copy (Name field shared)")
    }

    // Verify slice deep copy
    clone.Keywords = append(clone.Keywords, "new-keyword")
    if len(original.Keywords) == len(clone.Keywords) {
        t.Error("Clone is not a deep copy (Keywords slice shared)")
    }
}
```

### 1.7 Run Tests

```bash
go test -v
```

All existing tests should still pass. You've only extracted the config, not changed behavior.

---

## Step 2: Extract Handlers

**Goal:** Move business logic to a separate `handlers.go` file with pure functions.

### 2.1 Identify Business Logic

Look for gRPC handler methods in your server:

```go
func (s *server) MyRpcMethod(ctx context.Context, req *pb.MyRequest) (*pb.MyResponse, error) {
    // This is business logic - move to handlers.go
}
```

### 2.2 Create `handlers.go`

```go
package main

import (
    "context"
    "log/slog"

    pb "github.com/globulario/services/golang/myservice/myservicepb"
)

// handlers.go - Business logic for the MyService service
//
// This file contains the gRPC handler implementations. All functions are pure
// and testable, with side effects (logging, errors) explicitly returned.
//
// Phase 1 Step 2: Extracted from server struct for clean separation of concerns.

// MyRpcMethod handles the MyRpcMethod RPC call.
func (s *server) MyRpcMethod(ctx context.Context, req *pb.MyRequest) (*pb.MyResponse, error) {
    slog.Info("handling rpc request",
        "service", s.Name,
        "id", s.Id,
        "method", "MyRpcMethod",
        "param", req.SomeField)

    // ⚠️ Move your business logic here

    slog.Info("rpc response sent",
        "service", s.Name,
        "id", s.Id)

    return &pb.MyResponse{
        Result: "processed",
    }, nil
}

// ⚠️ Add more handlers as needed
```

### 2.3 Create Handler Tests

Create `handlers_test.go`:

```go
package main

import (
    "context"
    "testing"

    pb "github.com/globulario/services/golang/myservice/myservicepb"
)

func TestMyRpcMethod(t *testing.T) {
    srv := initializeServerDefaults()

    tests := []struct {
        name    string
        request *pb.MyRequest
        wantErr bool
    }{
        {
            name:    "valid request",
            request: &pb.MyRequest{SomeField: "test"},
            wantErr: false,
        },
        // ⚠️ Add more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            resp, err := srv.MyRpcMethod(context.Background(), tt.request)
            if (err != nil) != tt.wantErr {
                t.Errorf("MyRpcMethod() error = %v, wantErr %v", err, tt.wantErr)
            }
            if !tt.wantErr && resp == nil {
                t.Error("Expected response, got nil")
            }
        })
    }
}
```

### 2.4 Run Tests

```bash
go test -v
```

All tests should still pass.

---

## Step 3: Use Shared Lifecycle

**Goal:** Replace local lifecycle management with `globular.NewLifecycleManager()`.

### 3.1 Implement LifecycleService Interface

In `server.go`, add lifecycle methods to your server:

```go
// Lifecycle methods for globular.NewLifecycleManager()

// StartService initializes the service.
// This is called by the lifecycle manager before starting the gRPC server.
func (s *server) StartService() error {
    slog.Info("initializing service", "name", s.Name, "id", s.Id)

    // ⚠️ Add your service initialization here
    // Examples:
    // - Connect to databases
    // - Initialize caches
    // - Create directories
    // - Validate configuration

    return nil
}

// StopService cleans up resources.
// This is called by the lifecycle manager after stopping the gRPC server.
func (s *server) StopService() error {
    slog.Info("cleaning up service", "name", s.Name, "id", s.Id)

    // ⚠️ Add your cleanup here
    // Examples:
    // - Close database connections
    // - Flush caches
    // - Save state

    return nil
}

// GetGrpcServer returns the gRPC server instance.
func (s *server) GetGrpcServer() *grpc.Server {
    return s.grpcServer
}
```

### 3.2 Update main()

Replace manual lifecycle management with the shared manager:

**Before:**
```go
func main() {
    // ... lots of lifecycle code ...

    lis, err := net.Listen("tcp", fmt.Sprintf(":%d", srv.Port))
    // ...

    go func() {
        if err := srv.grpcServer.Serve(lis); err != nil {
            // ...
        }
    }()

    // ... manual shutdown handling ...
}
```

**After:**
```go
func main() {
    srv := initializeServerDefaults()
    logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))

    // Handle CLI flags
    if globular.HandleInformationalFlags(srv, os.Args[1:], logger, printUsage) {
        return
    }

    // Setup gRPC service
    setupGrpcService(srv)

    // Create lifecycle manager
    lifecycle := globular.NewLifecycleManager(srv, logger)

    // Start the service
    if err := lifecycle.Start(); err != nil {
        logger.Error("failed to start service", "error", err)
        os.Exit(1)
    }

    logger.Info("service started successfully",
        "name", srv.Name,
        "port", srv.Port,
        "id", srv.Id)

    // Wait for termination signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan

    logger.Info("received shutdown signal")

    // Graceful shutdown
    if err := lifecycle.GracefulShutdown(30 * time.Second); err != nil {
        logger.Error("shutdown failed", "error", err)
        os.Exit(1)
    }

    logger.Info("service stopped gracefully")
}
```

### 3.3 Remove Old Lifecycle Code

Delete or comment out:
- Local `lifecycleManager` struct
- Local lifecycle methods
- Manual gRPC server startup code
- Manual shutdown handling code

### 3.4 Update Tests

Remove lifecycle-specific tests (they're now tested in the shared primitive). Keep integration tests that verify your service's StartService/StopService work correctly.

### 3.5 Run Tests

```bash
go test -v
```

---

## Step 4: Use Shared CLI Helpers

**Goal:** Replace CLI flag handling with `globular.HandleInformationalFlags()`.

### 4.1 Simplify main()

**Before:**
```go
func main() {
    srv := initializeServerDefaults()

    // Manual flag handling (50+ lines)
    for i, arg := range os.Args[1:] {
        if arg == "--version" {
            // ...
        } else if arg == "--help" {
            // ...
        }
        // ... more flags ...
    }
}
```

**After:**
```go
func main() {
    srv := initializeServerDefaults()
    logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))

    // Handle informational flags (version, help, describe, health)
    if globular.HandleInformationalFlags(srv, os.Args[1:], logger, printUsage) {
        return
    }

    // Parse positional arguments (service_id, config_path)
    globular.ParsePositionalArgs(srv, os.Args[1:])

    // Allocate port if needed (--port 0)
    if err := globular.AllocatePortIfNeeded(srv, os.Args[1:]); err != nil {
        logger.Error("failed to allocate port", "error", err)
        os.Exit(1)
    }

    // Load runtime config from environment
    globular.LoadRuntimeConfig(srv)

    // Continue with service setup...
}
```

### 4.2 Create printUsage()

```go
func printUsage() {
    fmt.Println("Usage: myservice_server [OPTIONS] [service_id] [config_path]")
    fmt.Println()
    fmt.Println("Options:")
    fmt.Println("  --version          Show version and exit")
    fmt.Println("  --help             Show this help message")
    fmt.Println("  --describe         Show service metadata as JSON")
    fmt.Println("  --health           Perform health check")
    fmt.Println("  --debug            Enable debug logging")
    fmt.Println("  --port <port>      Override service port")
    fmt.Println()
    fmt.Println("Arguments:")
    fmt.Println("  service_id         Service instance identifier")
    fmt.Println("  config_path        Path to configuration file")
}
```

### 4.3 Run Tests

```bash
go test -v
```

---

## Step 5: Use Shared Config Helpers

**Goal:** Simplify config.go using shared helper functions.

You've already done this in Step 1 if you followed the templates. If not, update:

- `SaveToFile()` → use `globular.SaveConfigToFile()`
- `LoadFromFile()` → use `globular.LoadConfigFromFile()`
- `Validate()` → use `globular.ValidateCommonFields()`
- `DefaultConfig()` → use `globular.GetDefaultDomainAddress()`
- `Clone()` → use `globular.CloneStringSlice()`

See Step 1 for details.

---

## Step 6: Cleanup & Validation

**Goal:** Remove duplicates, modernize code, and validate everything works.

### 6.1 Modernize Code

Replace `interface{}` with `any`:

```bash
# Find all interface{} usages
grep -r "interface{}" .

# Replace with any (Go 1.18+)
# Update manually or use sed
```

### 6.2 Remove Duplicates

- Remove old lifecycle.go file (if exists)
- Remove old CLI helper functions (if exists)
- Remove unused imports

### 6.3 Run All Tests

```bash
go test -v ./...
```

**Expected:** All tests should pass with no changes to test output.

### 6.4 Update Documentation

Add comments to your server.go:

```go
// server.go - Main server implementation for MyService
//
// Phase 1 refactoring: This file uses shared primitives from globular_service:
// - CLI helpers (HandleInformationalFlags, ParsePositionalArgs, etc.)
// - Lifecycle manager (NewLifecycleManager)
// - Config helpers (SaveConfigToFile, ValidateCommonFields, etc.)
//
// Business logic is in handlers.go, configuration in config.go.
```

### 6.5 Verify main() Simplification

Your main() should now be ~40-50 lines instead of 150+:

```go
func main() {
    // 1. Initialize
    srv := initializeServerDefaults()
    logger := slog.New(...)

    // 2. Handle CLI flags
    if globular.HandleInformationalFlags(...) { return }
    globular.ParsePositionalArgs(...)
    globular.AllocatePortIfNeeded(...)
    globular.LoadRuntimeConfig(...)

    // 3. Setup gRPC
    setupGrpcService(srv)

    // 4. Start service
    lifecycle := globular.NewLifecycleManager(...)
    lifecycle.Start()

    // 5. Wait for shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan

    // 6. Graceful shutdown
    lifecycle.GracefulShutdown(30 * time.Second)
}
```

---

## Validation Checklist

Before considering the refactoring complete:

- [ ] All tests passing (`go test -v`)
- [ ] No behavioral changes (service works identically)
- [ ] main() reduced from 150+ lines to ~40-50 lines
- [ ] Config extracted to config.go
- [ ] Handlers extracted to handlers.go
- [ ] Using `globular.NewLifecycleManager()`
- [ ] Using `globular.HandleInformationalFlags()` and other CLI helpers
- [ ] Using config helpers (SaveConfigToFile, ValidateCommonFields, etc.)
- [ ] Code modernized (`interface{}` → `any`)
- [ ] Documentation updated
- [ ] Committed with descriptive message

---

## Common Issues

### Issue: Tests failing after extraction

**Solution:** Ensure you haven't changed any business logic. The refactoring should be purely structural.

### Issue: Imports not resolving

**Solution:** Run `go mod tidy` to update dependencies.

### Issue: Interface not satisfied

**Solution:** Ensure your server implements all methods required by `LifecycleService` and `Service` interfaces. Check method signatures match exactly.

### Issue: Config not loading from file

**Solution:** Verify your Config struct has correct JSON tags and LoadFromFile uses a pointer: `cfg := &Config{}`.

---

## Results

**Expected improvements:**
- 60-75% reduction in main() size
- Clear separation of concerns (config, handlers, server)
- Consistent patterns across all services
- Easier testing and maintenance
- Faster development for new services

**Example (Repository service):**
- Before: main() 175 lines
- After: main() 47 lines
- Reduction: 73%

---

## Next Steps

After refactoring your service:

1. Commit with descriptive message (see examples in git log)
2. Update service documentation
3. Consider refactoring another service
4. Share learnings with the team

---

## Support

- See [SHARED_PRIMITIVES.md](./SHARED_PRIMITIVES.md) for detailed API documentation
- Check Echo, Discovery, or Repository services for complete examples
- Review git history for refactoring commit messages
- Open an issue on GitHub for questions

---

**Happy Refactoring!** 🚀
