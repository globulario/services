package security

import (
	"context"
	"fmt"

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

// DefaultClusterValidator is the global cluster validator instance.
// Initialized on first use.
var defaultValidator *ClusterValidator

// ValidateClusterID is a package-level convenience function that uses the
// default cluster validator.
func ValidateClusterID(ctx context.Context, claimedClusterID string) error {
	if defaultValidator == nil {
		var err error
		defaultValidator, err = NewClusterValidator()
		if err != nil {
			return fmt.Errorf("failed to initialize cluster validator: %w", err)
		}
	}

	return defaultValidator.ValidateClusterID(ctx, claimedClusterID)
}

// GetLocalClusterID returns the local cluster ID using the default validator.
func GetLocalClusterID() (string, error) {
	if defaultValidator == nil {
		var err error
		defaultValidator, err = NewClusterValidator()
		if err != nil {
			return "", fmt.Errorf("failed to initialize cluster validator: %w", err)
		}
	}

	return defaultValidator.GetLocalClusterID(), nil
}
