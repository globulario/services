package security

import (
	"context"
	"strings"
	"sync"
	"time"
)

// ---- cluster initialization gate -------------------------------------------

const clusterInitCacheTTL = 30 * time.Second

var (
	clusterInitCacheMu sync.RWMutex
	clusterInitCached  bool
	clusterInitValue   bool
	clusterInitAt      time.Time
)

// IsClusterInitialized returns true when Day-0 is complete and the cluster is
// operating in secured mode.  A cluster is considered initialized when:
//
//  1. It has a local cluster_id (domain is configured), AND
//  2. It is NOT currently in active bootstrap mode
//
// Results are cached for clusterInitCacheTTL to avoid repeated lookups.
// The cache is intentionally short-lived so that the transition from
// bootstrap → secured happens within one TTL period.
func IsClusterInitialized(ctx context.Context) (bool, error) {
	clusterInitCacheMu.RLock()
	if clusterInitCached && time.Since(clusterInitAt) < clusterInitCacheTTL {
		v := clusterInitValue
		clusterInitCacheMu.RUnlock()
		return v, nil
	}
	clusterInitCacheMu.RUnlock()

	initialized := computeIsClusterInitialized()

	clusterInitCacheMu.Lock()
	clusterInitCached = true
	clusterInitValue = initialized
	clusterInitAt = time.Now()
	clusterInitCacheMu.Unlock()

	return initialized, nil
}

// InvalidateClusterInitCache forces the next IsClusterInitialized call to
// recompute the value.  Call this when bootstrap mode is disabled.
func InvalidateClusterInitCache() {
	clusterInitCacheMu.Lock()
	clusterInitCached = false
	clusterInitCacheMu.Unlock()
}

func computeIsClusterInitialized() bool {
	// Condition 1: cluster must have a domain/cluster-id
	clusterID, err := GetLocalClusterID()
	if err != nil || clusterID == "" {
		return false
	}

	// Condition 2: must NOT be in active bootstrap mode
	// (bootstrap mode == Day-0 still in progress)
	if DefaultBootstrapGate.IsActive() {
		return false
	}

	return true
}

// ---- mutating RPC classifier ------------------------------------------------

// mutatingPrefixes are method-name prefixes that indicate a state-changing RPC.
var mutatingPrefixes = []string{
	"create", "update", "delete", "apply", "set", "add", "remove",
	"publish", "install", "uninstall", "upgrade", "scale",
	"issue", "revoke", "upload", "write", "put", "patch",
	"deploy", "rollback", "restart", "stop", "start",
	"grant", "revoke", "bind", "unbind",
	"approve", "reject", "archive",
}

// readOnlyPrefixes are method-name prefixes that indicate a read-only RPC.
// These are checked first so that ambiguous names default to mutating (fail closed).
var readOnlyPrefixes = []string{
	"get", "list", "watch", "read", "fetch", "query",
	"check", "status", "health", "metrics", "info",
	"search", "describe", "inspect", "resolve",
}

// IsMutatingRPC returns true when the full gRPC method name represents a
// state-changing (write/delete/apply) operation.
//
// The classification is based on the method name (last segment of the path):
//   - If the method starts with a read-only keyword → false
//   - If the method starts with or contains a mutating keyword → true
//   - Unknown methods default to true (fail closed: treat as mutating)
//
// This is used post-Day-0 to require authentication even for methods that
// lack an explicit RBAC mapping.
func IsMutatingRPC(fullMethod string) bool {
	if fullMethod == "" {
		return true // unknown → treat as mutating (fail closed)
	}

	// Extract just the method name: "/pkg.Service/MethodName" → "methodname"
	parts := strings.Split(fullMethod, "/")
	methodName := strings.ToLower(parts[len(parts)-1])
	if methodName == "" {
		return true
	}

	// Read-only fast-path
	for _, pfx := range readOnlyPrefixes {
		if strings.HasPrefix(methodName, pfx) {
			return false
		}
	}

	// Mutating check
	for _, pfx := range mutatingPrefixes {
		if strings.HasPrefix(methodName, pfx) {
			return true
		}
	}

	// Default: treat as mutating (fail closed)
	return true
}
