package main

import (
	"context"
	"log"

	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// enforceNodeScope validates that the caller principal is authorized to act on
// the given nodeID. This is the single enforcement point for own-node-only
// scope across all controller handlers that accept a node_id.
//
// Returns nil if the request should proceed, gRPC error if denied.
// SA deprecation warnings are logged and the request is allowed through.
func enforceNodeScope(ctx context.Context, nodeID, method string) error {
	authCtx := security.FromContext(ctx)
	if authCtx == nil {
		return nil // no auth context → let interceptor handle
	}
	if err := security.ValidateNodeOwnershipForMethod(authCtx.Subject, nodeID, method); err != nil {
		if security.IsSADeprecationWarning(err) {
			log.Printf("WARN %v", err)
			return nil // allow but warn
		}
		return status.Errorf(codes.PermissionDenied, "node ownership validation failed: %v", err)
	}
	return nil
}
