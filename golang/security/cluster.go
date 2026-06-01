package security

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
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
func ValidateClusterID(ctx context.Context, claimedClusterID string) error {
	cv, err := getValidator()
	if err != nil {
		return fmt.Errorf("failed to initialize cluster validator: %w", err)
	}
	return cv.ValidateClusterID(ctx, claimedClusterID)
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
