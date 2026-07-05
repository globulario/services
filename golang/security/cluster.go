// @awareness namespace=globular.platform
// @awareness component=platform_security.cluster_validator
// @awareness file_role=cluster_id_validation_prevents_cross_cluster_impersonation_empty_or_mismatch_denied
// @awareness implements=globular.platform:intent.security.cluster_id_validates_request_origin
// @awareness risk=critical
package security

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"google.golang.org/grpc/metadata"
)

// ClusterValidator provides cluster ID validation for cross-cluster security.
// Prevents nodes from one cluster from impersonating nodes in another cluster.
type ClusterValidator struct {
	localClusterID string
}

// NewClusterValidator creates a new cluster validator with the local cluster ID.
func NewClusterValidator() (*ClusterValidator, error) {
	// Get local cluster ID from config
	// This should be a stable identifier for this cluster (e.g., domain, UUID, etc.)
	domain, err := config.GetDomain()
	if err != nil {
		return nil, fmt.Errorf("failed to get local domain for cluster validation: %w", err)
	}

	// For now, use domain as cluster ID
	// In a multi-cluster setup, this would be a globally unique cluster UUID
	return &ClusterValidator{
		localClusterID: domain,
	}, nil
}

// ValidateClusterID verifies that a claimed cluster ID matches the local cluster.
// This prevents cross-cluster attacks where a node from cluster A tries to
// access resources in cluster B by claiming to be a member.
//
// Parameters:
//   - ctx: Request context (for future use with distributed config)
//   - claimedClusterID: The cluster ID claimed by the requester
//
// Returns:
//   - error: nil if validation passes, error describing the issue otherwise
//
// Security properties:
//   - DENY if claimedClusterID is empty (no cluster ID provided)
//   - DENY if claimedClusterID != localClusterID (cross-cluster attempt)
//   - ALLOW if claimedClusterID == localClusterID (same cluster)
func (cv *ClusterValidator) ValidateClusterID(ctx context.Context, claimedClusterID string) error {
	// Empty cluster ID is not allowed
	if claimedClusterID == "" {
		return fmt.Errorf("cluster ID validation failed: no cluster ID provided")
	}

	// Check if claimed cluster matches local cluster
	if claimedClusterID != cv.localClusterID {
		return fmt.Errorf("cluster ID validation failed: claimed cluster %q does not match local cluster %q",
			claimedClusterID, cv.localClusterID)
	}

	// Validation passed
	return nil
}

// GetLocalClusterID returns the local cluster ID.
func (cv *ClusterValidator) GetLocalClusterID() string {
	return cv.localClusterID
}

// validatorTTL controls how long the cached validator is trusted before
// re-reading the domain from config. This prevents a bad first read
// (e.g. before config.json is written) from being stuck forever.
const validatorTTL = 2 * time.Minute

var (
	validatorMu   sync.RWMutex
	validatorInst *ClusterValidator
	validatorAt   time.Time
)

// refreshValidator re-reads the domain from config and updates the cached
// validator if the domain has changed or was previously unset.
func refreshValidator() (*ClusterValidator, error) {
	cv, err := NewClusterValidator()
	if err != nil {
		return nil, err
	}
	validatorMu.Lock()
	validatorInst = cv
	validatorAt = time.Now()
	validatorMu.Unlock()
	return cv, nil
}

// getValidator returns the cached validator, refreshing it if the TTL has
// expired or if no validator exists yet.
func getValidator() (*ClusterValidator, error) {
	validatorMu.RLock()
	cv := validatorInst
	at := validatorAt
	validatorMu.RUnlock()

	if cv != nil && time.Since(at) < validatorTTL {
		return cv, nil
	}

	// TTL expired or no validator — refresh from config.
	return refreshValidator()
}

// ValidateClusterID is a package-level convenience function that uses the
// default cluster validator.
//
// LEGACY (domain-based request-origin check): cluster MEMBERSHIP is now validated
// by the opaque minted UUID via ValidateClusterMembership, which the gRPC
// interceptor enforces. This domain-origin validator has no production callers;
// it is retained pending a paired removal that also updates the intent
// security.cluster_id_validates_request_origin. Do NOT wire new membership checks
// through it — use ValidateClusterMembership.
func ValidateClusterID(ctx context.Context, claimedClusterID string) error {
	cv, err := getValidator()
	if err != nil {
		return fmt.Errorf("failed to initialize cluster validator: %w", err)
	}
	return cv.ValidateClusterID(ctx, claimedClusterID)
}

// ValidateClusterMembership verifies cluster membership by the opaque membership
// UUID — the ONLY membership credential. Fail-closed: a request is admitted iff
// its claimed cluster_uid equals the local minted UUID. An empty claim, a
// mismatch, or an unavailable local UUID is denied — never fall back to the
// domain, never fail open. The domain is NOT a membership credential; it remains
// only the DNS/storage/workflow namespace (config.GetDomain()).
func ValidateClusterMembership(claimedUID string) error {
	if claimedUID == "" {
		return fmt.Errorf("cluster membership validation failed: no cluster_uid provided")
	}
	localUID, err := GetLocalClusterUID()
	if err != nil {
		// Fail-closed: without the local minted identity we cannot verify membership.
		return fmt.Errorf("cluster membership validation failed: local cluster uid unavailable: %w", err)
	}
	if claimedUID != localUID {
		return fmt.Errorf("cluster membership validation failed: cluster_uid mismatch")
	}
	return nil
}

// GetLocalClusterID returns the local cluster ID using the default validator.
func GetLocalClusterID() (string, error) {
	cv, err := getValidator()
	if err != nil {
		return "", fmt.Errorf("failed to initialize cluster validator: %w", err)
	}
	return cv.GetLocalClusterID(), nil
}

// InvalidateClusterValidator forces the next GetLocalClusterID / ValidateClusterID
// call to re-read from config. Call this after changing the local domain.
func InvalidateClusterValidator() {
	validatorMu.Lock()
	validatorInst = nil
	validatorMu.Unlock()
}

// OverrideLocalClusterID temporarily sets the local cluster ID to the given
// value for the duration of a test, and registers a cleanup function to
// restore the original state.
//
// This function is intended for testing only.
func OverrideLocalClusterID(t interface{ Cleanup(func()) }, clusterID string) {
	validatorMu.Lock()
	saved := validatorInst
	validatorInst = &ClusterValidator{localClusterID: clusterID}
	validatorAt = time.Now()
	validatorMu.Unlock()

	InvalidateClusterInitCache()
	t.Cleanup(func() {
		validatorMu.Lock()
		validatorInst = saved
		validatorMu.Unlock()
		InvalidateClusterInitCache()
	})
}

// ── Cluster membership UUID (opaque identity) — read-through cache ────────────
//
// The membership UUID is minted once by the controller into
// /globular/system/cluster/id (config.ClusterMembershipIDKey) and is IMMUTABLE,
// so once read it is cached for the life of the process. This is the cluster's
// opaque MEMBERSHIP identity — deliberately NOT the domain (config.GetDomain()
// remains the DNS/storage/workflow namespace).
//
// Identity-migration status: ACTIVE. This UUID is the sole membership credential —
// ValidateClusterMembership (below) validates against it, fail-closed, and the
// gRPC interceptor enforces it on every request. The domain-based ValidateClusterID
// is legacy request-origin validation, retained only until the paired intent
// (security.cluster_id_validates_request_origin) is updated to name the UUID gate.
var (
	clusterUIDMu  sync.RWMutex
	clusterUIDVal string
)

// GetLocalClusterUID returns the cluster's opaque membership UUID, read from the
// controller-owned authority (config.ReadClusterMembershipID). It NEVER derives
// from, coerces, or defaults to the domain.
//
// Fail-closed: absence (config.ErrClusterMembershipIDAbsent) or a transport error
// is returned and NOT cached, so a not-yet-minted cluster retries on the next
// call until the controller mints it. A caller doing additive dual-emit must
// treat an error as "omit the UUID" — NEVER as "fall back to the domain".
func GetLocalClusterUID() (string, error) {
	clusterUIDMu.RLock()
	v := clusterUIDVal
	clusterUIDMu.RUnlock()
	if v != "" {
		return v, nil
	}
	id, err := config.ReadClusterMembershipID(context.Background())
	if err != nil {
		return "", err
	}
	clusterUIDMu.Lock()
	clusterUIDVal = id
	clusterUIDMu.Unlock()
	return id, nil
}

// AppendClusterUIDMetadata appends the opaque membership UUID (cluster_uid) to the
// outgoing gRPC metadata when it has been minted, so EVERY internal caller carries
// the membership badge — not just globular_client. Call it right after emitting
// cluster_id so membership semantics never depend on the transport (mTLS) exemption.
//
// Best-effort and idempotent: omit on absence (never fall back to the domain),
// skip if a cluster_uid is already set.
func AppendClusterUIDMetadata(ctx context.Context) context.Context {
	if md, ok := metadata.FromOutgoingContext(ctx); ok && len(md.Get("cluster_uid")) > 0 {
		return ctx
	}
	if uid, err := GetLocalClusterUID(); err == nil && uid != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "cluster_uid", uid)
	}
	return ctx
}

// invalidateClusterUIDForTest resets the membership-UUID cache (tests only).
func invalidateClusterUIDForTest() {
	clusterUIDMu.Lock()
	clusterUIDVal = ""
	clusterUIDMu.Unlock()
}
