package security

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Bootstrap decision counters. Labels:
//
//	reason — the denial reason string (e.g. "bootstrap_expired") or "bootstrap_allowed"
//
// Counters are low-cardinality: the reason set is fixed and small.
var (
	bootstrapAllowedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "bootstrap",
		Name:      "allowed_total",
		Help:      "Total requests allowed by the bootstrap gate.",
	})

	bootstrapDeniedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "bootstrap",
		Name:      "denied_total",
		Help:      "Total requests denied by the bootstrap gate, labelled by reason.",
	}, []string{"reason"})
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

	// bootstrapEnvVar is kept for documentation — env var activation was removed
	// because it bypassed time-bounded expiry. Use the flag file exclusively.
	bootstrapEnvVar = "GLOBULAR_BOOTSTRAP" //nolint:unused
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
// - Node identity registration for cluster formation
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

	// Resource service (required for node identity during cluster formation)
	"/resource.ResourceService/UpsertNodeIdentity":  true,
	"/resource.ResourceService/GetNodeIdentity":     true,
	"/resource.ResourceService/ListNodeIdentities":  true,

	// DNS service (required for initial zone and record setup)
	"/dns.DnsService/CreateZone":   true,
	"/dns.DnsService/GetZone":      true,
	"/dns.DnsService/CreateRecord": true,
	"/dns.DnsService/SetDomains":   true,
	"/dns.DnsService/SetA":         true,
	"/dns.DnsService/SetAAAA":      true,
	"/dns.DnsService/SetSoa":       true,
	"/dns.DnsService/SetNs":        true,
	"/dns.DnsService/SetTXT":       true,
	"/dns.DnsService/GetDomains":   true,
	"/dns.DnsService/GetA":         true,

	// Repository service (required for publishing bootstrap artifacts)
	"/repository.PackageRepository/UploadArtifact":      true,
	"/repository.PackageRepository/GetArtifactManifest": true,
	"/repository.PackageRepository/ListArtifacts":       true,
	"/repository.PackageRepository/UploadBundle":        true,

	// Resource service (required for service registration during bootstrap)
	"/resource.ResourceService/SetPackageDescriptor":  true,
	"/resource.ResourceService/GetPackageDescriptor":  true,
	"/resource.ResourceService/GetPackagesDescriptor": true,
	"/resource.ResourceService/RegisterAccount":       true,
	"/resource.ResourceService/GetAccount":            true,
	"/resource.ResourceService/GetAccounts":           true,
	"/resource.ResourceService/SetAccountPassword":    true,

	// Configuration (required for initial config)
	"/admin.AdminService/GetConfig": true,
	"/admin.AdminService/SetConfig": true,

	// Event service (required for internal event propagation during bootstrap;
	// services publish lifecycle events on startup)
	"/event.EventService/Publish":   true,
	"/event.EventService/Subscribe": true,
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
		g.logger.Warn("bootstrap deny: expired",
			"reason", "bootstrap_expired",
			"max_duration", bootstrapMaxDuration,
			"method", authCtx.GRPCMethod,
			"subject", authCtx.Subject,
		)
		bootstrapDeniedTotal.WithLabelValues("bootstrap_expired").Inc()
		return false, "bootstrap_expired"
	}

	// Gate 3: Loopback-only
	// Bootstrap requests MUST originate from localhost.
	// This prevents remote attackers from exploiting Day-0 mode.
	if !authCtx.IsLoopback {
		g.logger.Warn("bootstrap deny: non-loopback source",
			"reason", "bootstrap_remote",
			"method", authCtx.GRPCMethod,
			"subject", authCtx.Subject,
		)
		bootstrapDeniedTotal.WithLabelValues("bootstrap_remote").Inc()
		return false, "bootstrap_remote"
	}

	// Gate 4: Method allowlist
	// Only ESSENTIAL Day-0 methods are permitted in bootstrap mode.
	// This limits the attack surface during the vulnerable initial setup.
	if !g.isMethodAllowed(authCtx.GRPCMethod) {
		g.logger.Warn("bootstrap deny: method not in allowlist",
			"reason", "bootstrap_method_blocked",
			"method", authCtx.GRPCMethod,
			"subject", authCtx.Subject,
		)
		bootstrapDeniedTotal.WithLabelValues("bootstrap_method_blocked").Inc()
		return false, "bootstrap_method_blocked"
	}

	// All 4 gates passed - allow bootstrap access.
	// Logged at Info (not Debug) because every allow during the bootstrap window
	// is a security-relevant event worth capturing in normal log levels.
	g.logger.Info("bootstrap allow",
		"reason", "bootstrap_allowed",
		"method", authCtx.GRPCMethod,
		"subject", authCtx.Subject,
		"enabled_via", enabledReason,
	)
	bootstrapAllowedTotal.Inc()
	return true, "bootstrap_allowed"
}

// isEnabled checks if bootstrap mode is explicitly enabled via flag file.
// Returns (enabled bool, reason string).
//
// NOTE: Environment variable support (GLOBULAR_BOOTSTRAP=1) was removed because
// it bypassed the time-bounded expiry, leading to permanently insecure clusters
// when baked into systemd units. Use the flag file exclusively — it has JSON
// timestamps, 30-minute expiry, ownership checks, and auto-cleanup.
func (g *BootstrapGate) isEnabled() (bool, string) {
	// Check flag file — the only supported activation mechanism.
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
		// Clean up the stale flag file so it doesn't cause confusion during filesystem audits.
		if removeErr := os.Remove(g.flagFilePath); removeErr != nil && !os.IsNotExist(removeErr) {
			g.logger.Warn("failed to remove expired bootstrap flag", "path", g.flagFilePath, "error", removeErr)
		} else {
			g.logger.Info("removed expired bootstrap flag file", "path", g.flagFilePath)
		}
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

// bootstrapAllowedSubjects is the explicit set of service-account subjects
// that are permitted to write RBAC bindings via bootstrap path 2 (non-loopback,
// authenticated service principal). This must remain small — adding a subject
// here grants it admin-equivalent power during the 30-minute Day-0 window.
var bootstrapAllowedSubjects = map[string]bool{
	"globular-node-agent":  true,
	"globular-controller":  true,
	"globular-gateway":     true,
}

// IsBootstrapSubject reports whether subject is an explicitly allowed bootstrap
// service account. Used by handler-level bootstrap path 2 in the RBAC service.
func IsBootstrapSubject(subject string) bool {
	return bootstrapAllowedSubjects[subject]
}

// EnableBootstrapGate writes a properly-formatted bootstrap flag file at the
// default path (/var/lib/globular/bootstrap.enabled). The file:
//   - Contains JSON with explicit timestamps (required by readBootstrapState)
//   - Is written with mode 0600 (required by readBootstrapState)
//   - Should be chowned to the globular service user after writing (see note below)
//
// NOTE: This function writes the file as the calling process's user. If the
// calling process is root (e.g. the node agent), the file will be root-owned.
// The BootstrapGate allows both root-owned and globular-owned files. Services
// running as the globular user can read root-owned 0600 files only if the
// service UID matches root's supplementary groups — which is normally not the
// case. Callers that run as root should chown the file to the globular user
// after calling EnableBootstrapGate, or use os.Chown with the globular UID.
func EnableBootstrapGate(ttl time.Duration, createdBy string) error {
	return writeBootstrapStateTo(bootstrapFlagFile, ttl, createdBy)
}

// writeBootstrapStateTo is the internal implementation used by EnableBootstrapGate
// and by tests (which write to a temp path).
func writeBootstrapStateTo(path string, ttl time.Duration, createdBy string) error {
	now := time.Now()
	state := BootstrapState{
		EnabledAt: now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
		CreatedBy: createdBy,
		Version:   "1",
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal bootstrap state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create bootstrap dir: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
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
