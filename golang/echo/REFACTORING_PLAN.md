# Echo Service Refactoring Plan - Phase 1

## Goal
Prove maintainability improvements while preserving behavior through composition-based refactoring.

## Current State Analysis

**Files:**
- `server.go` (501 lines) - Everything: struct, getters/setters, main(), TLS, lifecycle
- `echo.go` (45 lines) - Single RPC handler

**Problems:**
1. **God object**: 40+ fields mixing metadata, policy, TLS, runtime
2. **Unclear boundaries**: Echo-specific vs Globular boilerplate unclear
3. **Side effects**: main() does config loading, port allocation, TLS setup all at once
4. **Hard to test**: Cannot test components independently
5. **Boilerplate**: 30+ getter/setter methods

## Target State (Via Composition)

```
echo_server/
├── config.go          # Configuration loading, defaults, persistence
├── server.go          # gRPC wiring, service registration (main)
├── handlers.go        # Business logic (Echo RPC)
├── lifecycle.go       # Start/Stop hooks, readiness
├── server_test.go     # Unit tests (NEW)
└── echo.go            # DEPRECATED, becomes handlers.go
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
}

func LoadConfig(path string) (*Config, error)
func DefaultConfig() *Config
func (c *Config) Save(path string) error
func (c *Config) Validate() error
```

#### handlers.go (renamed from echo.go)
```go
// Echo RPC handler - pure business logic
func (s *Service) Echo(ctx context.Context, req *echopb.EchoRequest) (*echopb.EchoResponse, error)

// No dependency on giant server struct
// No side effects (config save moved to lifecycle)
```

#### lifecycle.go
```go
type Service struct {
    config     *Config
    grpcServer *grpc.Server
    logger     *slog.Logger
}

func New(config *Config, logger *slog.Logger) *Service
func (s *Service) Start() error
func (s *Service) Stop() error
func (s *Service) Ready() bool
```

#### server.go (main)
```go
func main() {
    // 1. Parse CLI flags
    // 2. Handle --describe, --health, --help (no side effects)
    // 3. Load config
    // 4. Create service
    // 5. Start service
    // 6. Wait for signal
    // 7. Graceful shutdown
}
```

## Implementation Steps

### Step 0: Freeze Current Behavior (THIS PR)
- [ ] Add unit tests for Echo() handler
- [ ] Add test for --describe output
- [ ] Add test for config persistence
- [ ] Add test for default values
- [ ] Ensure all tests pass with current code

### Step 1: Extract Config Component
- [ ] Create config.go with Config struct
- [ ] Move config-related fields from server to Config
- [ ] Add LoadConfig(), DefaultConfig(), Save(), Validate()
- [ ] Update server to use new Config type
- [ ] Run tests - must pass unchanged

### Step 2: Extract Handlers Component
- [ ] Rename echo.go to handlers.go
- [ ] Remove Config.Save() side effect from Echo()
- [ ] Move echo.go content to handlers.go
- [ ] Run tests - must pass unchanged

### Step 3: Extract Lifecycle Component
- [ ] Create lifecycle.go with Service type
- [ ] Move Start/Stop logic from main() to lifecycle
- [ ] Add Ready() health check
- [ ] Run tests - must pass unchanged

### Step 4: Clean Up Server.go
- [ ] Remove moved code from server.go
- [ ] Keep only main() and CLI flag handling
- [ ] Simplify main() to use new components
- [ ] Run tests - must pass unchanged

### Step 5: Remove Boilerplate
- [ ] Remove unnecessary getter/setter methods
- [ ] Direct field access where appropriate
- [ ] Run tests - must pass unchanged

## Success Criteria

**Must Have:**
- ✅ All tests pass (behavior preserved)
- ✅ Clearer ownership boundaries
- ✅ No side effects in constructors
- ✅ Deterministic config behavior
- ✅ Conformance tests still pass

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

## Rollback Strategy

Each step is independently committable and reversible:
- Step N fails tests → revert Step N, keep Step N-1
- Tests show regression → git revert specific commit
- Behavior changed → automated tests catch it

## Timeline

- Step 0 (Tests): 1 PR - ~2 hours
- Steps 1-5 (Refactor): 5 small PRs - ~1 day
- Total: 6 PRs over 2 days

## Related

- Phase 0: v1-invariants.md, conformance tests
- Phase 2: Extract shared primitives (after 3+ services refactored)
- Phase 3: Build/packaging modernization
