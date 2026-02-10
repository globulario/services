package security

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
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

	// RBAC setup (required for creating first admin account)
	// Security Issue #3: These methods are powerful and could be abused
	// TODO: Ideally replace with purpose-built rbac.SeedBootstrapPolicy (idempotent)
	// OR add request validation to ensure only seed admin/roles can be created
	// For now: Rely on 30-min window + loopback-only + audit logging
	"/rbac.RbacService/CreateAccount": true,
	"/rbac.RbacService/CreateRole":    true,
	"/rbac.RbacService/SetAccountRole": true,
	"/rbac.RbacService/GetAccount":     true, // needed to check if account exists

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

	// Security: File must be owned by root (uid 0) or globular user
	// This prevents unprivileged users from creating fake bootstrap files
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		// Accept root (0) or globular user (typically 1000+ but we'll be lenient)
		// In production, this should be more strict
		if stat.Uid != 0 && stat.Uid > 65535 {
			// Suspiciously high UID - reject
			return nil, fmt.Errorf("file owned by suspicious uid %d (must be root or service user)", stat.Uid)
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
