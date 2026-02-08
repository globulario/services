# Repository Service Refactoring Plan - Phase 1

## ✅ STATUS: COMPLETE

**All 6 steps completed successfully!**
- 17 tests passing (6 config + 6 lifecycle + 5 baseline)
- main() reduced from 175 to 47 lines (~73% reduction)
- Clean component separation (Config, Handlers, Lifecycle)
- Modernized codebase (interface{} → any)
- Following proven Echo/Discovery pattern

## Goal
Apply proven Echo/Discovery refactoring pattern to Repository service to establish final patterns before extraction.

## Current State Analysis

**Files:**
- `server.go` (623 lines) - Everything: struct, getters/setters, main(), TLS, lifecycle
- `repository.go` (~200 lines) - RPC handlers for package management

**Problems:**
1. **God object**: 40+ fields mixing metadata, policy, TLS, runtime
2. **Unclear boundaries**: Repository-specific vs Globular boilerplate unclear
3. **Side effects**: main() does config loading, port allocation, TLS setup all at once
4. **Hard to test**: Cannot test components independently
5. **Boilerplate**: 30+ getter/setter methods

## Target State (Via Composition)

```
repository_server/
├── config.go          # Configuration loading, defaults, persistence
├── server.go          # gRPC wiring, service registration (main)
├── handlers.go        # Business logic (package management RPCs)
├── lifecycle.go       # Start/Stop hooks, readiness
├── server_test.go     # Unit tests
├── handlers_test.go   # Handler tests (if needed)
├── lifecycle_test.go  # Lifecycle tests
└── repository.go      # DEPRECATED, becomes handlers.go
```

### Component Responsibilities

#### config.go
```go
type Config struct {
    // Core service identity
    ID          string
    Name        string
    Domain      string
    Port        int

    // TLS configuration
    TLS struct {
        Enabled   bool
        CertFile  string
        KeyFile   string
        CAFile    string
    }

    // Service metadata
    Description string
    Version     string
    Keywords    []string
    Dependencies []string
}

func LoadConfig(path string) (*Config, error)
func DefaultConfig() *Config
func (c *Config) Save(path string) error
func (c *Config) Validate() error
```

#### handlers.go (renamed from repository.go)
```go
// Repository RPC handlers - pure business logic
// Package management, artifact storage, version control

// No dependency on giant server struct
// No side effects (config save moved to lifecycle if needed)
```

#### lifecycle.go
```go
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

#### server.go (main)
```go
func main() {
    // 1. Parse CLI flags
    // 2. Handle --describe, --health, --help (no side effects)
    // 3. Load config
    // 4. Create service
    // 5. Start service using lifecycle manager
}

// Helper functions extracted:
func initializeServerDefaults() *server
func handleInformationalFlags(srv *server, args []string) bool
func allocatePortIfNeeded(srv *server, args []string) error
func loadRuntimeConfig(srv *server)
func setupGrpcService(srv *server)
```

## Implementation Steps

### Step 0: Freeze Current Behavior ✅ COMPLETE
- [x] Add unit tests for repository handlers
- [x] Add test for --describe output
- [x] Add test for default values
- [x] Ensure all tests pass with current code
- **Result:** 5 baseline tests passing (commit: initial)

### Step 1: Extract Config Component ✅ COMPLETE
- [x] Create config.go with Config struct
- [x] Move config-related fields from server to Config
- [x] Add LoadConfig(), DefaultConfig(), Save(), Validate()
- [x] Update server to reference Config
- [x] Run tests - must pass unchanged
- **Result:** 11 tests passing (6 config + 5 baseline) (commit: a6962d80)

### Step 2: Extract Handlers Component ✅ COMPLETE
- [x] Rename repository.go to handlers.go
- [x] Verify no Save() side effects in handlers
- [x] Run tests - must pass unchanged
- **Result:** 11 tests passing (commit: e0ed048e)

### Step 3: Extract Lifecycle Component ✅ COMPLETE
- [x] Create lifecycle.go with lifecycleManager type
- [x] Move Start/Stop logic from main() to lifecycle
- [x] Add Ready() health check
- [x] Run tests - must pass unchanged
- **Result:** 17 tests passing (6 config + 6 lifecycle + 5 baseline) (commit: 0f9aaafa)

### Step 4: Clean Up Server.go ✅ COMPLETE
- [x] Extract helper functions from main()
- [x] Simplify main() to use new components
- [x] Use lifecycleManager.Start()
- [x] Run tests - must pass unchanged
- **Result:** 17 tests passing, main() reduced from 175 to 47 lines (commit: 4c8d6fa3)

### Step 5: Remove Boilerplate ✅ COMPLETE
- [x] Improve documentation
- [x] Modernize []interface{} → []any
- [x] Run tests - must pass unchanged
- **Result:** 17 tests passing, Phase 1 complete! (commit: cfd0da42)

## Success Criteria

**Must Have:**
- ✅ All tests pass (behavior preserved)
- ✅ Clearer ownership boundaries
- ✅ No side effects in constructors
- ✅ Deterministic config behavior

**Nice to Have:**
- Reduced line count
- Less code duplication
- Easier to understand flow

## Anti-Patterns to Avoid

❌ **DON'T:**
- Create a base "ServiceFramework" embedded struct
- Add codegen for getters/setters
- Change external API behavior
- Break existing clients
- Skip tests between steps

✅ **DO:**
- Use composition over inheritance
- Extract small, focused helpers
- Preserve all existing behavior
- Run tests after each step
- Commit frequently
- Follow exact Echo/Discovery pattern

## Comparison with Echo/Discovery Pattern

**Similarities:**
- Same server struct pattern
- Same main() initialization flow
- Same getter/setter boilerplate
- Same --describe/--health flags

**Differences:**
- Repository manages artifact storage
- Package versioning logic
- May have additional dependencies

**Key Learning:**
With 3 services refactored (Echo + Discovery + Repository), we can
extract shared primitives with confidence.

## Rollback Strategy

Each step is independently committable and reversible:
- Step N fails tests → revert Step N, keep Step N-1
- Tests show regression → git revert specific commit
- Behavior changed → automated tests catch it

## Timeline

- Step 0 (Tests): 1 commit - ~1 hour
- Steps 1-5 (Refactor): 5 small commits - ~2-3 hours
- Total: 6 commits over half day

## Related

- Echo service refactoring (completed - 19 tests, all passing)
- Discovery service refactoring (completed - 19 tests, all passing)
- Phase 2: Extract shared primitives (PR6 - after this)
