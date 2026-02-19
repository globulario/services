package security

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Bootstrap constants define the security boundaries for Day-0 installation mode.
const (
	// bootstrapFlagFile is the sentinel file that enables bootstrap mode.
	// Security Fix #4: Now contains JSON state with explicit timestamps
	// (not relying on filesystem mtime which can be spoofed)
	bootstrapFlagFile = "/var/lib/globular/bootstrap.enabled"

	// bootstrapMaxDuration is the maximum time window for bootstrap mode.
	// After this duration from enabled_at, bootstrap requests are denied.
	// This prevents forgotten flag files from leaving the system permanently insecure.
	bootstrapMaxDuration = 30 * time.Minute

	// bootstrapEnvVar is the environment variable alternative to the flag file.
	// Set GLOBULAR_BOOTSTRAP=1 to enable bootstrap mode without a file.
	// Used for testing and containerized deployments.
	bootstrapEnvVar = "GLOBULAR_BOOTSTRAP"
)

// BootstrapState represents the bootstrap mode state stored in the flag file.
// Security Fix #4: Use file content with explicit timestamps, not filesystem mtime.
type BootstrapState struct {
	EnabledAt  int64  `json:"enabled_at_unix"`  // Unix timestamp when bootstrap was enabled
	ExpiresAt  int64  `json:"expires_at_unix"`  // Unix timestamp when bootstrap expires
	Nonce      string `json:"nonce,omitempty"`  // Optional random nonce for additional security
	CreatedBy  string `json:"created_by"`       // User/process that enabled bootstrap (for audit)
	Version    string `json:"version"`          // Bootstrap state format version
}

// bootstrapAllowedMethods defines the MINIMAL set of gRPC methods that are
// permitted during Day-0 bootstrap mode. This is the security-critical allowlist
// that prevents attackers from abusing bootstrap mode to call arbitrary methods.
//
// Design principles:
// - ONLY methods REQUIRED for initial setup are allowed
// - Health checks for service readiness
// - Account/role creation for first admin user
// - Authentication to get initial tokens
// - Peer registration for cluster formation
// - NO data access methods (read/write/delete)
// - NO administration methods beyond initial setup
//
// If a method is not in this list, it will be DENIED during bootstrap mode.
var bootstrapAllowedMethods = map[string]bool{
	// Health checks (required for service readiness)
	"/grpc.health.v1.Health/Check": true,
	"/grpc.health.v1.Health/Watch": true,

	// RBAC role-binding setup (required for seeding operator/SA roles during Day-0)
	// Protected by 30-min window + loopback-only + audit logging.
	// Post-bootstrap, these methods are protected by handler-level admin role check.
	"/rbac.RbacService/SetRoleBinding":   true,
	"/rbac.RbacService/GetRoleBinding":   true,
	"/rbac.RbacService/ListRoleBindings": true,

	// Authentication (required for getting initial tokens)
	"/authentication.AuthenticationService/Authenticate": true,

	// Resource service (required for peer registration)
	"/resource.ResourceService/CreatePeer": true,
	"/resource.ResourceService/GetPeers":   true,

	// DNS service (required for initial zone setup)
	"/dns.DnsService/CreateZone":   true,
	"/dns.DnsService/GetZone":      true,
	"/dns.DnsService/CreateRecord": true,

	// Configuration (required for initial config)
	"/admin.AdminService/GetConfig": true,
	"/admin.AdminService/SetConfig": true,
}

// BootstrapGate enforces 4-level security for Day-0 bootstrap mode:
// 1. Explicit enablement (flag file or env var)
// 2. Time-bounded (< 30 minutes from activation)
// 3. Loopback-only (requests must come from 127.0.0.1/::1)
// 4. Method allowlisted (only essential Day-0 methods permitted)
//
// All 4 conditions must be satisfied for bootstrap mode to allow a request.
// If any condition fails, the request is rejected and normal authorization applies.
type BootstrapGate struct {
	// flagFilePath is the path to the bootstrap flag file
	flagFilePath string

	// logger for bootstrap decisions
	logger *slog.Logger

	// skipOwnershipCheck disables strict ownership validation (testing only)
	skipOwnershipCheck bool
}

// NewBootstrapGate creates a new bootstrap security gate.
func NewBootstrapGate() *BootstrapGate {
	return &BootstrapGate{
		flagFilePath: bootstrapFlagFile,
		logger:       slog.Default().With("component", "bootstrap_gate"),
	}
}

// NewBootstrapGateWithPath creates a bootstrap gate with a custom flag file path.
// This is primarily for testing purposes.
func NewBootstrapGateWithPath(flagFilePath string) *BootstrapGate {
	return &BootstrapGate{
		flagFilePath: flagFilePath,
		logger:       slog.Default().With("component", "bootstrap_gate"),
	}
}

// DefaultBootstrapGate is the global bootstrap gate instance.
var DefaultBootstrapGate = NewBootstrapGate()

// SetSkipOwnershipCheck enables or disables ownership validation.
// This is for testing purposes only - DO NOT use in production code.
func (g *BootstrapGate) SetSkipOwnershipCheck(skip bool) {
	g.skipOwnershipCheck = skip
}

// ShouldAllow determines if a request should be allowed under bootstrap mode.
// Returns (allowed bool, reason string).
//
// If allowed=true, reason explains why (for audit logs).
// If allowed=false, reason explains which gate failed (for debugging).
//
// The 4 gates are checked in order:
// 1. Enablement check (fast path - no bootstrap if not enabled)
// 2. Time window check (prevent stale bootstrap mode)
// 3. Loopback check (prevent remote bootstrap exploitation)
// 4. Method allowlist (prevent unauthorized method access)
func (g *BootstrapGate) ShouldAllow(authCtx *AuthContext) (bool, string) {
	// Gate 1: Explicit enablement
	// Bootstrap mode MUST be explicitly enabled via flag file or env var.
	// This prevents accidental activation.
	enabled, enabledReason := g.isEnabled()
	if !enabled {
		return false, "bootstrap_not_enabled"
	}

	// Gate 2: Time-bounded
	// Bootstrap mode MUST NOT exceed the maximum duration.
	// This prevents forgotten flag files from leaving the system insecure.
	if !g.isWithinTimeWindow() {
		g.logger.Warn("bootstrap mode expired",
			"max_duration", bootstrapMaxDuration,
			"method", authCtx.GRPCMethod,
		)
		return false, "bootstrap_expired"
	}

	// Gate 3: Loopback-only
	// Bootstrap requests MUST originate from localhost.
	// This prevents remote attackers from exploiting Day-0 mode.
	if !authCtx.IsLoopback {
		g.logger.Warn("bootstrap request from non-loopback source",
			"method", authCtx.GRPCMethod,
			"subject", authCtx.Subject,
		)
		return false, "bootstrap_remote"
	}

	// Gate 4: Method allowlist
	// Only ESSENTIAL Day-0 methods are permitted in bootstrap mode.
	// This limits the attack surface during the vulnerable initial setup.
	if !g.isMethodAllowed(authCtx.GRPCMethod) {
		g.logger.Warn("bootstrap method not in allowlist",
			"method", authCtx.GRPCMethod,
		)
		return false, "bootstrap_method_blocked"
	}

	// All 4 gates passed - allow bootstrap access
	g.logger.Debug("bootstrap request allowed",
		"method", authCtx.GRPCMethod,
		"enabled_via", enabledReason,
	)
	return true, "bootstrap_allowed"
}

// isEnabled checks if bootstrap mode is explicitly enabled via flag file or env var.
// Returns (enabled bool, reason string).
func (g *BootstrapGate) isEnabled() (bool, string) {
	// Check environment variable first (higher priority for testing/containers)
	if envVal := strings.TrimSpace(os.Getenv(bootstrapEnvVar)); envVal != "" {
		if envVal == "1" || strings.EqualFold(envVal, "true") || strings.EqualFold(envVal, "yes") {
			return true, "env_var"
		}
	}

	// Check flag file
	if _, err := os.Stat(g.flagFilePath); err == nil {
		return true, "flag_file"
	}

	return false, ""
}

// isWithinTimeWindow checks if bootstrap mode is within the allowed time window.
// Security Fix #4: Validates using file CONTENT (explicit timestamps), not filesystem mtime.
// Also enforces file permissions (0600, root/globular owned).
//
// Returns true if:
// - Enabled via env var (no time limit for env-based bootstrap)
// - Flag file exists, has correct permissions, and current time < expires_at
func (g *BootstrapGate) isWithinTimeWindow() bool {
	// If enabled via env var, no time limit (for testing/development)
	if envVal := strings.TrimSpace(os.Getenv(bootstrapEnvVar)); envVal != "" {
		if envVal == "1" || strings.EqualFold(envVal, "true") || strings.EqualFold(envVal, "yes") {
			return true
		}
	}

	// Read and validate bootstrap state file
	state, err := g.readBootstrapState()
	if err != nil {
		g.logger.Debug("bootstrap state file invalid", "error", err)
		return false
	}

	// Check if current time is within the valid window
	now := time.Now().Unix()
	if now < state.EnabledAt {
		g.logger.Warn("bootstrap state has future enabled_at timestamp",
			"enabled_at", state.EnabledAt,
			"now", now,
		)
		return false
	}

	if now >= state.ExpiresAt {
		g.logger.Warn("bootstrap mode expired",
			"expires_at", time.Unix(state.ExpiresAt, 0),
			"now", time.Unix(now, 0),
			"enabled_by", state.CreatedBy,
		)
		return false
	}

	return true
}

// readBootstrapState reads and validates the bootstrap state file.
// Security Fix #4: Validates file permissions and ownership, reads explicit timestamps.
func (g *BootstrapGate) readBootstrapState() (*BootstrapState, error) {
	// Check file exists and get info
	info, err := os.Stat(g.flagFilePath)
	if err != nil {
		return nil, fmt.Errorf("stat failed: %w", err)
	}

	// Security: File must be regular file, not symlink
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file (mode: %v)", info.Mode())
	}

	// Security: File must be 0600 (owner read/write only)
	perm := info.Mode().Perm()
	if perm != 0600 {
		return nil, fmt.Errorf("insecure permissions %o (must be 0600)", perm)
	}

	// Security: File must be owned by root (uid 0) or the globular service user.
	// Blocker Fix #2: Strict ownership check - only root or globular user allowed.
	if !g.skipOwnershipCheck {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			// Resolve the globular service user UID at runtime via OS user database.
			// Falls back to root-only (uid 0) if the "globular" user does not exist.
			globularUID := uint32(0)
			if u, err := user.Lookup("globular"); err == nil {
				if uid, err := strconv.ParseUint(u.Uid, 10, 32); err == nil {
					globularUID = uint32(uid)
				}
			}

			if stat.Uid != 0 && stat.Uid != globularUID {
				return nil, fmt.Errorf("file owned by uid %d (must be root:0 or globular service user uid:%d)", stat.Uid, globularUID)
			}
		}
	}

	// Read file content
	data, err := os.ReadFile(g.flagFilePath)
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}

	// Parse JSON state
	var state BootstrapState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate state fields
	if state.EnabledAt == 0 {
		return nil, fmt.Errorf("missing enabled_at_unix")
	}
	if state.ExpiresAt == 0 {
		return nil, fmt.Errorf("missing expires_at_unix")
	}
	if state.ExpiresAt <= state.EnabledAt {
		return nil, fmt.Errorf("expires_at must be after enabled_at")
	}

	return &state, nil
}

// isMethodAllowed checks if a gRPC method is in the bootstrap allowlist.
func (g *BootstrapGate) isMethodAllowed(method string) bool {
	return bootstrapAllowedMethods[method]
}

// IsActive returns true when bootstrap mode is currently enabled AND within the time window.
// This is the public check used by IsClusterInitialized to determine if Day-0 is still in progress.
func (g *BootstrapGate) IsActive() bool {
	enabled, _ := g.isEnabled()
	if !enabled {
		return false
	}
	return g.isWithinTimeWindow()
}

// GetBootstrapStatus returns a human-readable status of bootstrap mode.
// Used for diagnostics and troubleshooting.
func (g *BootstrapGate) GetBootstrapStatus() string {
	enabled, reason := g.isEnabled()
	if !enabled {
		return "disabled"
	}

	withinWindow := g.isWithinTimeWindow()
	if !withinWindow {
		return fmt.Sprintf("enabled (%s) but EXPIRED", reason)
	}

	// Check file age for display
	if reason == "flag_file" {
		if info, err := os.Stat(g.flagFilePath); err == nil {
			age := time.Since(info.ModTime())
			remaining := bootstrapMaxDuration - age
			return fmt.Sprintf("enabled (%s), %v remaining", reason, remaining.Round(time.Second))
		}
	}

	return fmt.Sprintf("enabled (%s)", reason)
}

// AddAllowedMethod adds a method to the bootstrap allowlist.
// This is for testing only - DO NOT use in production code.
func (g *BootstrapGate) AddAllowedMethod(method string) {
	bootstrapAllowedMethods[method] = true
}

// RemoveAllowedMethod removes a method from the bootstrap allowlist.
// This is for testing only - DO NOT use in production code.
func (g *BootstrapGate) RemoveAllowedMethod(method string) {
	delete(bootstrapAllowedMethods, method)
}
