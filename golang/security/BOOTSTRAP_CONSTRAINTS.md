# Bootstrap Mode Security Constraints

## Overview

Bootstrap mode is a temporary security bypass during Day-0 installation. It MUST be strictly constrained to prevent attackers from exploiting it.

**Current Implementation**: 4-Gate Security Model (Security Fix #4)
**Future Enhancement**: Seed-Only Bootstrap Policy

---

## Current 4-Gate Model

Bootstrap mode allows requests ONLY when ALL 4 gates pass:

### Gate 1: Explicit Enablement
- **Requirement**: Flag file `/var/lib/globular/bootstrap.enabled` exists OR `GLOBULAR_BOOTSTRAP=1` env var set
- **Format**: JSON state file with explicit timestamps (not filesystem mtime)
- **Permissions**: File MUST be 0600 and owned by root/globular user
- **Purpose**: Prevents accidental activation

### Gate 2: Time-Bounded
- **Requirement**: Current time < `expires_at_unix` timestamp in state file
- **Duration**: 30 minutes maximum from `enabled_at_unix`
- **Purpose**: Prevents forgotten flag files from leaving system permanently insecure

### Gate 3: Loopback-Only
- **Requirement**: Request MUST originate from 127.0.0.1 or ::1
- **Detection**: Uses `peer.FromContext()` to extract source IP
- **Purpose**: Prevents remote attackers from exploiting Day-0 mode

### Gate 4: Method Allowlist
- **Requirement**: Only ESSENTIAL Day-0 methods permitted
- **Allowed Methods**:
  - `/grpc.health.v1.Health/Check` - Health checks
  - `/rbac.RbacService/CreateAccount` - First admin account
  - `/rbac.RbacService/CreateRole` - Initial roles
  - `/rbac.RbacService/SetAccountRole` - Bind admin role
  - `/rbac.RbacService/GetAccount` - Check if account exists
  - `/authentication.AuthenticationService/Authenticate` - Get initial tokens
  - `/resource.ResourceService/CreatePeer` - Cluster formation
  - `/resource.ResourceService/GetPeers` - Cluster enumeration
  - `/dns.DnsService/CreateZone` - Initial DNS setup
  - `/dns.DnsService/GetZone` - Check zone exists
  - `/dns.DnsService/CreateRecord` - Bootstrap DNS records
  - `/admin.AdminService/GetConfig` - Read initial config
  - `/admin.AdminService/SetConfig` - Write initial config
- **Purpose**: Limits attack surface to only what's needed for Day-0

---

## Security Issue #3: CreateAccount/CreateRole Too Powerful

**Problem**: Generic `CreateAccount` and `CreateRole` methods in bootstrap allowlist can be abused to create arbitrary privileged accounts, not just the initial admin.

**Current Mitigation**:
- 30-minute time window (Gate 2)
- Loopback-only access (Gate 3)
- Audit logging of all bootstrap requests
- Warning comment in code (bootstrap.go lines 61-65)

**Recommended Enhancement**: Seed-Only Bootstrap Policy

---

## Proposed: Seed-Only Bootstrap Policy (Production-Ready)

### Option A: Idempotent Seed RPC

Replace generic methods with purpose-built idempotent RPC:

```protobuf
message SeedBootstrapPolicyRequest {
  // Fixed seed objects (cannot be customized)
  bool create_admin_account = 1;  // Create "sa" account if not exists
  bool create_admin_role = 2;     // Create "globular-admin" role if not exists
  bool bind_admin_role = 3;       // Bind sa → globular-admin if not already bound
}

message SeedBootstrapPolicyResponse {
  bool admin_account_created = 1;
  bool admin_role_created = 2;
  bool role_binding_created = 3;
  repeated string warnings = 4;  // "already exists" warnings
}
```

**Advantages**:
- Idempotent (safe to call multiple times)
- Cannot create arbitrary accounts/roles
- Clear intent (bootstrap only)
- Easy to audit ("bootstrap seed was called")

**Implementation** (`rbac.RbacService`):
```go
func (s *server) SeedBootstrapPolicy(ctx context.Context, req *SeedBootstrapPolicyRequest) (*SeedBootstrapPolicyResponse, error) {
    // Only callable during bootstrap mode
    authCtx := security.FromContext(ctx)
    if !authCtx.IsBootstrap {
        return nil, status.Error(codes.PermissionDenied, "seed_bootstrap_policy only available during bootstrap")
    }

    // Idempotent operations
    resp := &SeedBootstrapPolicyResponse{}

    if req.CreateAdminAccount {
        // Check if "sa" exists
        if _, err := s.getAccount("sa"); err != nil {
            // Create sa account
            resp.AdminAccountCreated = true
        } else {
            resp.Warnings = append(resp.Warnings, "admin account already exists")
        }
    }

    if req.CreateAdminRole {
        // Check if "globular-admin" exists
        if _, err := s.getRole("globular-admin"); err != nil {
            // Create globular-admin role with /* wildcard
            resp.AdminRoleCreated = true
        } else {
            resp.Warnings = append(resp.Warnings, "admin role already exists")
        }
    }

    if req.BindAdminRole {
        // Check if binding exists
        // Create binding if not
        resp.RoleBindingCreated = true
    }

    return resp, nil
}
```

### Option B: Request Validation

Keep generic methods but add strict validation:

```go
func (s *server) CreateAccount(ctx context.Context, req *CreateAccountRequest) (*CreateAccountResponse, error) {
    authCtx := security.FromContext(ctx)

    // During bootstrap, ONLY allow creating "sa" account
    if authCtx.IsBootstrap {
        if req.Name != "sa" {
            return nil, status.Errorf(codes.PermissionDenied,
                "bootstrap mode only allows creating 'sa' account, got '%s'", req.Name)
        }
    }

    // Normal creation logic...
}

func (s *server) CreateRole(ctx context.Context, req *CreateRoleRequest) (*CreateRoleResponse, error) {
    authCtx := security.FromContext(ctx)

    // During bootstrap, ONLY allow creating "globular-admin" role
    if authCtx.IsBootstrap {
        if req.Name != "globular-admin" {
            return nil, status.Errorf(codes.PermissionDenied,
                "bootstrap mode only allows creating 'globular-admin' role, got '%s'", req.Name)
        }

        // Verify role has /* wildcard (not arbitrary permissions)
        if len(req.Actions) != 1 || req.Actions[0] != "/*" {
            return nil, status.Error(codes.PermissionDenied,
                "bootstrap admin role must have exactly one action: '/*'")
        }
    }

    // Normal creation logic...
}
```

**Advantages**:
- No new RPC methods needed
- Inline validation (easy to review)
- Clear error messages for misuse

**Disadvantages**:
- Less idempotent (still fails if objects exist)
- Mixed concerns (bootstrap + normal logic in same method)

---

## Recommended Implementation

**Use Option A (Idempotent Seed RPC)** for production:

1. **Create new RPC**: `rbac.RbacService/SeedBootstrapPolicy`
2. **Update bootstrap allowlist**: Replace `CreateAccount`, `CreateRole`, `SetAccountRole` with `SeedBootstrapPolicy`
3. **Update installer**: Call `SeedBootstrapPolicy` instead of individual calls
4. **Keep old methods for normal operation**: Once bootstrap complete, use normal RBAC methods

**Benefits**:
- Idempotent (safe to run multiple times)
- Cannot be abused to create arbitrary power
- Clear separation of concerns (bootstrap vs normal)
- Easy to audit (single method call in logs)

---

## Migration Path

### Phase 1: Add Seed RPC (No Breaking Changes)
```bash
# Add to rbacpb/rbac.proto
service RbacService {
    rpc SeedBootstrapPolicy(SeedBootstrapPolicyRequest) returns (SeedBootstrapPolicyResponse);
    // ... existing methods
}

# Implement in rbac_server/server.go
func (s *server) SeedBootstrapPolicy(...) {...}
```

### Phase 2: Update Installer (Parallel Support)
```bash
# install-day0.sh calls both (for backwards compatibility)
globularcli rbac seed-bootstrap-policy || {
    # Fallback to old method
    globularcli rbac create-account --name sa
    globularcli rbac create-role --name globular-admin
    globularcli rbac set-account-role --account sa --role globular-admin
}
```

### Phase 3: Switch Bootstrap Allowlist
```go
// golang/security/bootstrap.go
var bootstrapAllowedMethods = map[string]bool{
    // ... health checks, etc.

    // NEW: Idempotent seed method
    "/rbac.RbacService/SeedBootstrapPolicy": true,

    // OLD: Remove after migration
    // "/rbac.RbacService/CreateAccount": true,
    // "/rbac.RbacService/CreateRole": true,
    // "/rbac.RbacService/SetAccountRole": true,
}
```

### Phase 4: Deprecate Old Methods in Bootstrap
```go
// After N releases, remove old methods from allowlist
// They remain available for normal (non-bootstrap) operation
```

---

## Testing

### Test: Bootstrap Cannot Create Arbitrary Accounts
```go
func TestBootstrap_OnlySeedAccountsAllowed(t *testing.T) {
    // Enable bootstrap mode
    // Try to create non-sa account
    // Verify: DENIED
}
```

### Test: Seed Policy is Idempotent
```go
func TestBootstrap_SeedPolicyIdempotent(t *testing.T) {
    // Call SeedBootstrapPolicy
    // Call again
    // Verify: Both succeed, second returns "already exists" warnings
}
```

### Test: Seed Policy Only in Bootstrap
```go
func TestBootstrap_SeedPolicyOnlyDuringBootstrap(t *testing.T) {
    // Disable bootstrap
    // Try to call SeedBootstrapPolicy
    // Verify: DENIED
}
```

---

## Security Properties

### Current (4-Gate Model)
- ✅ Time-bounded (30 min)
- ✅ Loopback-only
- ✅ Method allowlist
- ⚠️ Can create arbitrary accounts/roles (mitigated by time window + loopback)

### Future (Seed-Only Policy)
- ✅ Time-bounded (30 min)
- ✅ Loopback-only
- ✅ Method allowlist
- ✅ Can ONLY create seed admin account/role (cannot abuse)
- ✅ Idempotent (safe to retry)

---

## References

- Security Fix #4: `golang/security/bootstrap.go`
- Bootstrap allowlist: `bootstrapAllowedMethods` map (lines 55-85)
- Bootstrap gate logic: `BootstrapGate.ShouldAllow()` (lines 125-172)
- Installer usage: `globular-installer/scripts/install-day0.sh`
- Security Issue #3: Todo file, lines 41-67
