# Phase 2: Extract Shared Primitives - Implementation Plan

## Status: Planning

## Goal
Extract common patterns from Echo, Discovery, and Repository services into reusable shared primitives.

## Context

**Phase 1 Complete - 3 Services Refactored:**
- ✅ Echo: 19 tests, main() 146→40 lines (73% reduction)
- ✅ Discovery: 19 tests, main() 166→40 lines (76% reduction)
- ✅ Repository: 17 tests, main() 175→47 lines (73% reduction)

**Proven Pattern Established:**
1. Config component: Configuration management with validation
2. Lifecycle component: Service lifecycle with health checks
3. Main helpers: 8 helper functions to simplify main()
4. Server struct: Organized fields with clear sections

## Analysis of Common Code

### 1. Config Component Pattern

**Common across all 3 services:**

```go
type Config struct {
    // Core identity
    ID, Name, Domain, Address, Port, Proxy, Protocol, Version string/int

    // Service metadata
    Description, Keywords, Repositories, Discoveries, Dependencies

    // Policy
    AllowAllOrigins, AllowedOrigins, KeepUpToDate, KeepAlive

    // Runtime state
    Process, ProxyProcess, State, LastError, ModTime

    // TLS
    TLS struct { Enabled, CertFile, KeyFile, CAFile }

    // Permissions
    Permissions []any
}

func DefaultConfig() *Config
func (c *Config) Validate() error
func (c *Config) SaveToFile(path string) error
func LoadFromFile(path string) (*Config, error)
func (c *Config) Clone() *Config
```

**Service-specific fields:**
- Echo: (none - pure defaults)
- Discovery: (none - pure defaults)
- Repository: `Root string` (package storage directory)

**Extraction Strategy:**
- Create `globular_service/config.go` with base Config struct
- Services can embed/compose with service-specific fields
- Or: Use generic Config with `Extensions map[string]any` for service-specific

### 2. Lifecycle Component Pattern

**Identical across all 3 services:**

```go
type ServiceLifecycle interface {
    Start() error
    Stop() error
    Ready() bool
    Health() error
}

type lifecycleManager struct {
    srv    *server
    logger *slog.Logger
}

func newLifecycleManager(srv *server, logger *slog.Logger) *lifecycleManager
func (lm *lifecycleManager) Start() error
func (lm *lifecycleManager) Stop() error
func (lm *lifecycleManager) Ready() bool
func (lm *lifecycleManager) Health() error
func (lm *lifecycleManager) GracefulShutdown(timeout time.Duration) error
func (lm *lifecycleManager) AwaitReady(timeout time.Duration) error
```

**Differences:**
- Tied to service-specific `*server` type
- Needs generic approach or interface-based design

**Extraction Strategy:**
- Create `globular_service/lifecycle.go` with generic lifecycle manager
- Use interface constraints or type parameters (Go 1.18+)
- Services implement minimal interface for lifecycle operations

### 3. Main Helper Functions Pattern

**Common across all 3 services:**

1. `initializeServerDefaults()` - Creates server with defaults (service-specific)
2. `handleInformationalFlags(srv, args)` - Processes CLI flags (identical)
3. `handleDescribeFlag(srv)` - Outputs JSON metadata (identical)
4. `handleHealthFlag(srv)` - Health check output (identical)
5. `parsePositionalArgs(srv, args)` - Extracts service_id, config_path (identical)
6. `allocatePortIfNeeded(srv, args)` - Port allocation (identical)
7. `loadRuntimeConfig(srv)` - Loads domain/address (identical)
8. `setupGrpcService(srv)` - Registers gRPC service (service-specific)

**Service-specific:**
- `initializeServerDefaults()` - Each service has unique defaults, permissions
- `setupGrpcService()` - Each service registers different proto definitions

**Common (100% identical):**
- `handleInformationalFlags()`, `handleDescribeFlag()`, `handleHealthFlag()`
- `parsePositionalArgs()`, `allocatePortIfNeeded()`, `loadRuntimeConfig()`

**Extraction Strategy:**
- Extract 6 identical functions to `globular_service/cli_helpers.go`
- Keep 2 service-specific functions in each service's server.go
- Services call shared helpers from main()

### 4. Server Struct Pattern

**Common fields (identical across all services):**
```go
// Core Identity
Id, Mac, Name, Domain, Address, Path, Proto, Port, Proxy, Protocol, Version, PublisherID

// Service Metadata
Description, Keywords, Repositories, Discoveries, Dependencies

// Policy & Operations
AllowAllOrigins, AllowedOrigins, KeepUpToDate, KeepAlive, Permissions

// Runtime State
Process, ProxyProcess, ConfigPath, LastError, State, ModTime

// TLS Configuration
TLS, CertFile, KeyFile, CertAuthorityTrust

// gRPC
grpcServer *grpc.Server
```

**Service-specific fields:**
- Echo: (none)
- Discovery: (none)
- Repository: `Root string`, `Plaform string`, `Checksum string`

**Extraction Strategy:**
- Create `globular_service/base_service.go` with BaseService struct
- Services embed BaseService and add service-specific fields
- Getters/setters defined once on BaseService

## Extraction Approach

### Option A: Aggressive Extraction (Recommended)
**Extract immediately:**
1. CLI helpers (6 identical functions)
2. Lifecycle interface and generic manager
3. Config base struct with embedding

**Benefits:**
- Immediate code reduction across all services
- Establishes clean patterns for future services
- 6/8 helper functions eliminated per service

**Risks:**
- Need to ensure no behavioral changes
- May need interface design for lifecycle

### Option B: Conservative Extraction
**Extract in stages:**
1. Phase 2a: CLI helpers only (safest, 100% identical)
2. Phase 2b: Lifecycle component (needs generics)
3. Phase 2c: Config component (needs design for extensions)
4. Phase 2d: Base service struct (largest change)

**Benefits:**
- Lower risk per step
- Can validate each extraction independently
- Easier to rollback if issues

**Risks:**
- Slower progress
- More commits/PRs to manage

### Option C: Proof of Concept First
**Start with one service:**
1. Extract Echo service to use shared primitives
2. Validate all tests pass
3. Apply to Discovery
4. Apply to Repository

**Benefits:**
- Proves extraction works end-to-end
- Validates design decisions early
- Single service easier to rollback

**Risks:**
- Duplicate work if design needs changes

## Recommended Plan: Option A (Aggressive)

### Phase 2 Step 1: Extract CLI Helpers ✅ NEXT
**Create:** `globular_service/cli_helpers.go`

Extract 6 identical functions:
- `HandleInformationalFlags(srv Service, args []string, logger *slog.Logger) bool`
- `HandleDescribeFlag(srv Service, logger *slog.Logger)`
- `HandleHealthFlag(srv Service, logger *slog.Logger)`
- `ParsePositionalArgs(srv Service, args []string)`
- `AllocatePortIfNeeded(srv Service, args []string) error`
- `LoadRuntimeConfig(srv Service)`

Define minimal `Service` interface:
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
    GetConfigPath() string
    SetConfigPath(string)
    GetState() string
    SetState(string)
    // ... (subset needed by helpers)
}
```

Update all 3 services to import and use shared helpers.

**Expected Result:**
- Remove ~50 lines × 3 services = ~150 lines eliminated
- All 55 tests passing (19+19+17)

### Phase 2 Step 2: Extract Lifecycle Component
**Create:** `globular_service/lifecycle.go`

Use generics (Go 1.18+) or interface-based design:
```go
type LifecycleService interface {
    StartService() error
    StopService() error
    GetName() string
    GetId() string
    GetState() string
    SetState(string)
    GetPort() int
    GetGrpcServer() *grpc.Server
}

type LifecycleManager[T LifecycleService] struct {
    srv    T
    logger *slog.Logger
}
```

Or simpler interface-based:
```go
type LifecycleManager struct {
    srv    LifecycleService
    logger *slog.Logger
}
```

**Expected Result:**
- Remove lifecycle.go from each service (~200 lines × 3 = ~600 lines)
- Replace with single shared implementation (~200 lines)
- Net reduction: ~400 lines
- All 55 tests passing

### Phase 2 Step 3: Extract Config Component
**Create:** `globular_service/config.go`

Base config with extension mechanism:
```go
type BaseConfig struct {
    // All common fields
    Id, Name, Domain, Address, Port, Proxy, Protocol, Version, ...

    // Extension for service-specific fields
    Extensions map[string]any
}

func (c *BaseConfig) GetExtension(key string) (any, bool)
func (c *BaseConfig) SetExtension(key string, value any)
```

Or: Keep config.go in services, use shared validation logic.

**Expected Result:**
- Reduce duplication in config structs
- Shared validation, save/load logic
- All 55 tests passing

### Phase 2 Step 4: Document and Validate
**Create:** Documentation for using shared primitives
**Validate:** Run all service tests, integration tests
**Create:** Migration guide for other services

## Success Criteria

**Must Have:**
- ✅ All 55 tests passing (19+19+17) across all services
- ✅ No behavior changes
- ✅ Significant code reduction (target: 500+ lines eliminated)
- ✅ Clean, documented shared primitives
- ✅ Pattern established for future services

**Nice to Have:**
- Generic solution for lifecycle (Go 1.18+ features)
- Base service struct with embedding
- Config extension mechanism
- Migration guide for remaining services

## File Structure After Extraction

```
golang/
├── globular_service/
│   ├── services.go (existing)
│   ├── cli_helpers.go (NEW - Phase 2 Step 1)
│   ├── lifecycle.go (NEW - Phase 2 Step 2)
│   ├── config.go (NEW - Phase 2 Step 3)
│   └── base_service.go (OPTIONAL - Phase 2 Step 4)
├── echo/echo_server/
│   ├── server.go (simplified, uses shared helpers)
│   ├── handlers.go (unchanged)
│   ├── lifecycle.go (REMOVED - uses shared)
│   ├── config.go (simplified or REMOVED)
│   └── *_test.go (unchanged, all passing)
├── discovery/discovery_server/
│   ├── server.go (simplified, uses shared helpers)
│   ├── handlers.go (unchanged)
│   ├── lifecycle.go (REMOVED - uses shared)
│   ├── config.go (simplified or REMOVED)
│   └── *_test.go (unchanged, all passing)
└── repository/repository_server/
    ├── server.go (simplified, uses shared helpers)
    ├── handlers.go (unchanged)
    ├── lifecycle.go (REMOVED - uses shared)
    ├── config.go (simplified or REMOVED)
    └── *_test.go (unchanged, all passing)
```

## Anti-Patterns to Avoid

❌ **DON'T:**
- Break any tests during extraction
- Change external behavior
- Create overly complex abstractions
- Extract too much at once without validation
- Skip tests between steps

✅ **DO:**
- Extract 100% identical code first
- Keep service-specific code in services
- Run all tests after each step
- Document extraction decisions
- Commit after each successful step
- Prefer composition over inheritance

## Rollback Strategy

Each step is independently reversible:
- Step N fails tests → revert Step N, keep previous steps
- Can extract CLI helpers without lifecycle
- Can extract lifecycle without config
- Git history allows surgical rollbacks

## Timeline Estimate

- Step 1 (CLI Helpers): ~2-3 hours (straightforward extraction)
- Step 2 (Lifecycle): ~3-4 hours (needs interface design)
- Step 3 (Config): ~2-3 hours (needs extension mechanism)
- Step 4 (Docs): ~1 hour
- **Total:** ~8-11 hours (1-2 days)

## Related Work

- Phase 1: Echo refactoring (PR3)
- Phase 1: Discovery refactoring (PR4)
- Phase 1: Repository refactoring (PR5)
- Phase 3: Apply pattern to remaining services (future)

## Decision Log

### 2026-02-08: Initial Planning
- Reviewed all 3 refactored services
- Identified common patterns
- Chose Option A (Aggressive) for faster progress
- Prioritized CLI helpers as first step (100% identical, low risk)
