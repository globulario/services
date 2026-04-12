package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

const (
	stagingRoot      = "/var/lib/globular/staging"
	defaultPublisher = "core@globular.io"
)

// DeleteCacheArtifact removes the cached artifact (.tgz) at the deterministic
// staging path for a given package. This is the production-safe primitive for
// the auto-heal "delete_stale_cache" action.
//
// Validation:
//   - publisher_id and package_name are sanitized (no path traversal)
//   - the computed path must be inside /var/lib/globular/staging/
//   - only removes the specific file "latest.artifact", not arbitrary paths
//
// Idempotent: returns ok=true if the file was deleted OR was already absent.
func (srv *NodeAgentServer) DeleteCacheArtifact(ctx context.Context, req *node_agentpb.DeleteCacheArtifactRequest) (*node_agentpb.DeleteCacheArtifactResponse, error) {
	pkg := strings.TrimSpace(req.GetPackageName())
	pub := strings.TrimSpace(req.GetPublisherId())
	if pub == "" {
		pub = defaultPublisher
	}
	if pkg == "" {
		return &node_agentpb.DeleteCacheArtifactResponse{
			Ok:      false,
			Message: "package_name is required",
		}, nil
	}

	// Sanitize: reject path traversal attempts.
	if strings.Contains(pkg, "/") || strings.Contains(pkg, "..") ||
		strings.Contains(pub, "..") {
		return &node_agentpb.DeleteCacheArtifactResponse{
			Ok:      false,
			Message: fmt.Sprintf("invalid package_name or publisher_id: %q / %q", pkg, pub),
		}, nil
	}

	path := filepath.Join(stagingRoot, pub, pkg, "latest.artifact")

	// Final safety: verify the resolved path is still inside stagingRoot.
	absPath, err := filepath.Abs(path)
	if err != nil || !strings.HasPrefix(absPath, stagingRoot) {
		return &node_agentpb.DeleteCacheArtifactResponse{
			Ok:      false,
			Message: fmt.Sprintf("resolved path %q escapes staging root", absPath),
		}, nil
	}

	if err := os.Remove(absPath); err != nil {
		if os.IsNotExist(err) {
			return &node_agentpb.DeleteCacheArtifactResponse{
				Ok:      true,
				Message: "already absent",
				Path:    absPath,
			}, nil
		}
		return &node_agentpb.DeleteCacheArtifactResponse{
			Ok:      false,
			Message: fmt.Sprintf("remove failed: %v", err),
			Path:    absPath,
		}, nil
	}

	return &node_agentpb.DeleteCacheArtifactResponse{
		Ok:      true,
		Message: "deleted",
		Path:    absPath,
	}, nil
}
