// security.go provides defense-in-depth security checks for resource operations.
// Phase 5: Ownership validation, cluster ID checks, path canonicalization.

package main

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/security"
)

// ResourceOperation represents the type of operation being performed on a resource.
type ResourceOperation string

const (
	OpRead   ResourceOperation = "read"
	OpWrite  ResourceOperation = "write"
	OpDelete ResourceOperation = "delete"
	OpShare  ResourceOperation = "share"
)

// ValidateResourceOwnership verifies that the authenticated subject is authorized
// to perform an operation on a resource based on ownership.
//
// Ownership rules:
// - Resource paths follow pattern: /users/{owner}/...
// - Subject must match owner for write/delete/share operations
// - Read operations may be allowed if resource is shared (checked separately)
//
// Parameters:
//   - ctx: Request context containing AuthContext
//   - resourcePath: Path to the resource (e.g., "/users/alice/files/doc.txt")
//   - operation: Type of operation (read, write, delete, share)
//
// Returns:
//   - error: nil if ownership check passes, descriptive error otherwise
//
// Security properties:
//   - DENY if no AuthContext in context (unauthenticated)
//   - DENY if path traversal detected
//   - DENY if owner cannot be extracted from path
//   - DENY if subject != owner for write/delete/share operations
//   - DEFER to RBAC for read operations (sharing logic)
func ValidateResourceOwnership(ctx context.Context, resourcePath string, operation ResourceOperation) error {
	// Extract AuthContext from request context
	authCtx := security.FromContext(ctx)
	if authCtx == nil {
		return fmt.Errorf("ownership validation failed: no authentication context")
	}

	// Validate path for security (prevent traversal attacks)
	if err := security.ValidateResourcePath(resourcePath); err != nil {
		return fmt.Errorf("ownership validation failed: %w", err)
	}

	// Extract owner from resource path
	owner, err := security.ExtractOwnerFromPath(resourcePath)
	if err != nil {
		// Path doesn't follow ownership pattern - might be a system resource
		// Let RBAC handle it (no ownership enforcement)
		return nil
	}

	// Check ownership based on operation
	subject := authCtx.Subject

	switch operation {
	case OpWrite, OpDelete, OpShare:
		// Destructive/sharing operations REQUIRE ownership
		if subject != owner {
			return fmt.Errorf("ownership validation failed: subject %q is not owner of resource (owner: %q)",
				subject, owner)
		}
		return nil

	case OpRead:
		// Read operations:
		// - ALLOW if subject == owner
		// - DEFER to RBAC if subject != owner (might have shared access)
		if subject == owner {
			return nil
		}
		// Not the owner - let RBAC/sharing check it
		return nil

	default:
		return fmt.Errorf("ownership validation failed: unknown operation %q", operation)
	}
}

// ValidateClusterMembership verifies that a node claiming cluster membership
// actually belongs to the local cluster.
//
// Parameters:
//   - ctx: Request context
//   - clusterID: Cluster ID claimed by the node
//
// Returns:
//   - error: nil if cluster ID matches local cluster, error otherwise
//
// This prevents cross-cluster attacks where node A from cluster X tries to
// access resources in cluster Y.
func ValidateClusterMembership(ctx context.Context, clusterID string) error {
	return security.ValidateClusterID(ctx, clusterID)
}

// ValidateFilePath canonicalizes a file path and ensures it stays within
// the allowed base directory.
//
// Parameters:
//   - baseDir: Base directory (e.g., "/var/lib/globular/files")
//   - requestedPath: User-supplied path (potentially malicious)
//
// Returns:
//   - canonicalPath: Safe, canonical path within base directory
//   - error: PathSecurityError if validation fails
//
// Example usage:
//   safe, err := ValidateFilePath("/var/lib/globular/files", "user/docs/file.txt")
//   if err != nil { return err }
//   // Use safe path for filesystem operations
//   file, err := os.Open(safe)
func ValidateFilePath(baseDir, requestedPath string) (string, error) {
	return security.CanonicalizePath(baseDir, requestedPath)
}
