package interceptors

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/peer"
)

// PolicyVersion is the version of the authorization policy in effect.
// Security Fix #10: Track policy version for audit correlation.
//
// This can be set via:
// 1. Build-time: -ldflags "-X interceptors.PolicyVersion=$(git rev-parse --short HEAD)"
// 2. Runtime: Set via config or environment variable
var PolicyVersion = "unknown"

// AuditDecision represents a single authorization decision for audit logging.
// All authorization decisions (allow/deny) MUST be logged for security compliance.
//
// Security Fix #10: Enhanced with additional fields for security analysis:
// - policy_version: Correlate decisions with policy changes
// - decision_latency_ms: Detect performance anomalies
// - remote_addr: Track source of requests
// - Never logs raw tokens (privacy/security)
// - Never samples denials (security requirement)
//
// Decision reasons:
// - "bootstrap_bypass": Allowed due to Day-0 bootstrap mode
// - "allowlist": Allowed due to unauthenticated method allowlist
// - "no_rbac_mapping": Allowed because method has no RBAC resource mapping (permissive default)
// - "rbac_granted": Allowed by RBAC permission check
// - "rbac_denied": Denied by RBAC permission check
// - "bootstrap_expired": Denied because bootstrap time window expired
// - "bootstrap_remote": Denied because bootstrap request from non-loopback
// - "bootstrap_method_blocked": Denied because method not in bootstrap allowlist
// - "cluster_id_missing": Denied because cluster_id required after initialization
// - "cluster_id_mismatch": Denied because cluster_id doesn't match local cluster
type AuditDecision struct {
	// When was this decision made
	Timestamp time.Time `json:"timestamp"`

	// Security Fix #10: Policy version tracking
	PolicyVersion string `json:"policy_version"` // Git SHA or semantic version

	// Who made the request
	Subject       string `json:"subject"`        // Identity (e.g., "dave", "sa")
	PrincipalType string `json:"principal_type"` // "user", "application", "node", "admin", "anonymous"
	AuthMethod    string `json:"auth_method"`    // "jwt", "mtls", "apikey", "none"
	IsLoopback    bool   `json:"is_loopback"`    // Request from 127.0.0.1/::1

	// Security Fix #10: Network context
	RemoteAddr string `json:"remote_addr"` // Source IP:port (NEVER includes auth tokens)

	// What was requested
	GRPCMethod   string `json:"grpc_method"`   // Full method name (e.g., "/dns.DnsService/CreateZone")
	ResourcePath string `json:"resource_path"` // Resource being accessed (if available)
	Permission   string `json:"permission"`    // Permission required (e.g., "write", "delete")

	// Decision outcome
	Allowed bool   `json:"allowed"` // true = allowed, false = denied
	Reason  string `json:"reason"`  // One of the decision reasons listed above

	// Security Fix #10: Performance tracking
	DecisionLatencyMs int64 `json:"decision_latency_ms"` // Time from request start to decision (milliseconds)

	// Context (for debugging/forensics)
	ClusterID  string `json:"cluster_id,omitempty"`  // Cluster that made the request
	Bootstrap  bool   `json:"bootstrap,omitempty"`   // Was bootstrap mode active
	CallerIP   string `json:"caller_ip,omitempty"`   // Source IP (if available) [DEPRECATED: use remote_addr]
	CallSource string `json:"call_source,omitempty"` // "local", "remote", "peer"
}

// extractRemoteAddr safely extracts the remote address from gRPC context.
// Security Fix #10: Extract source IP for audit logging.
//
// CRITICAL: This function NEVER logs auth tokens or credentials.
// Only network addresses (IP:port) are extracted.
func extractRemoteAddr(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok {
		addr := p.Addr.String()

		// Clean up for logging (strip any credentials if somehow included)
		// Should never happen, but defense in depth
		if strings.Contains(addr, "@") {
			// Format like "user:pass@host:port" - strip credentials
			parts := strings.SplitN(addr, "@", 2)
			if len(parts) == 2 {
				addr = parts[1] // Keep only host:port
			}
		}

		return addr
	}
	return "unknown"
}

// LogAuthzDecision logs an authorization decision to structured logs.
// This is the SINGLE POINT where all authz decisions are logged - do not add
// additional logging elsewhere or we'll lose consistency.
//
// Security Fix #10: Enhanced audit logging with:
// - Policy version tracking (for correlation with policy changes)
// - Decision latency measurement (for performance analysis)
// - Remote address logging (for forensics)
// - Never logs raw tokens (privacy/security requirement)
// - Never samples denials (security requirement - ALL denials logged)
//
// Parameters:
//   - ctx: gRPC context (for extracting remote address and latency)
//   - authCtx: Authentication context (identity + security properties)
//   - allowed: true if access was granted, false if denied
//   - reason: Short code explaining why (see AuditDecision.Reason for valid values)
//   - resourcePath: Resource being accessed (empty string if not applicable)
//   - permission: Permission required (e.g., "read", "write", "delete")
//   - startTime: When the request started (for latency calculation)
//
// The function is safe to call with nil authCtx (will log as anonymous).
func LogAuthzDecision(
	ctx context.Context,
	authCtx *security.AuthContext,
	allowed bool,
	reason string,
	resourcePath string,
	permission string,
	startTime time.Time,
) {
	// Security Fix #10: Calculate decision latency
	now := time.Now().UTC()
	latencyMs := now.Sub(startTime).Milliseconds()

	// Build audit decision struct
	decision := AuditDecision{
		Timestamp:         now,
		PolicyVersion:     PolicyVersion,                // Security Fix #10
		DecisionLatencyMs: latencyMs,                    // Security Fix #10
		RemoteAddr:        extractRemoteAddr(ctx),       // Security Fix #10
		Allowed:           allowed,
		Reason:            reason,
		ResourcePath:      resourcePath,
		Permission:        permission,
	}

	// Extract identity from AuthContext (handle nil case)
	if authCtx != nil {
		decision.Subject = authCtx.Subject
		decision.PrincipalType = authCtx.PrincipalType
		decision.AuthMethod = authCtx.AuthMethod
		decision.IsLoopback = authCtx.IsLoopback
		decision.GRPCMethod = authCtx.GRPCMethod
		decision.ClusterID = authCtx.ClusterID
		decision.Bootstrap = authCtx.IsBootstrap

		// Determine call source
		if authCtx.IsLoopback {
			decision.CallSource = "local"
		} else if authCtx.ClusterID != "" {
			decision.CallSource = "peer"
		} else {
			decision.CallSource = "remote"
		}

		// Backward compatibility: populate deprecated CallerIP field
		if decision.RemoteAddr != "" && decision.RemoteAddr != "unknown" {
			host, _, err := net.SplitHostPort(decision.RemoteAddr)
			if err == nil {
				decision.CallerIP = host
			} else {
				decision.CallerIP = decision.RemoteAddr
			}
		}
	} else {
		// No auth context - treat as anonymous
		decision.Subject = "anonymous"
		decision.PrincipalType = "anonymous"
		decision.AuthMethod = "none"
		decision.CallSource = "unknown"
	}

	// Security Fix #10: ALL denials logged at WARN (never sampled)
	// Allowed decisions logged at INFO (can be sampled if volume too high)
	level := slog.LevelInfo
	if !allowed {
		// CRITICAL: Denied decisions MUST NEVER be sampled
		// These are security-relevant events that indicate:
		// - Attack attempts
		// - Misconfigurations
		// - Access policy violations
		level = slog.LevelWarn
	}

	// Convert to JSON for easy parsing (single-line)
	// Security Fix #10: Ensure no tokens are logged
	// AuditDecision struct does not contain token fields
	// extractRemoteAddr explicitly strips credentials
	jsonBytes, err := json.Marshal(decision)
	if err != nil {
		// Shouldn't happen, but don't fail the request if logging breaks
		slog.Error("failed to marshal audit decision",
			"error", err,
			"subject", decision.Subject,
			"method", decision.GRPCMethod,
		)
		return
	}

	// Log with structured fields for filtering/indexing
	slog.Log(context.Background(), level, "authz_decision",
		slog.String("audit", string(jsonBytes)),
		slog.String("subject", decision.Subject),
		slog.String("method", decision.GRPCMethod),
		slog.Bool("allowed", decision.Allowed),
		slog.String("reason", decision.Reason),
		slog.Int64("latency_ms", latencyMs),             // Security Fix #10
		slog.String("policy_version", PolicyVersion),    // Security Fix #10
	)
}

// LogAuthzDecisionSimple is a convenience wrapper for cases where we don't have
// resource/permission info or need to calculate latency.
// Deprecated: Use LogAuthzDecision with full context for better audit trail.
func LogAuthzDecisionSimple(authCtx *security.AuthContext, allowed bool, reason string) {
	// Use background context and zero start time (latency will be ~0ms)
	LogAuthzDecision(context.Background(), authCtx, allowed, reason, "", "", time.Now())
}
