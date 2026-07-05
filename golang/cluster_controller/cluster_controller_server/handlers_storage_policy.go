package main

import (
	"context"
	"log"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	config "github.com/globulario/services/golang/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// handlers_storage_policy.go — operator-facing declaration of the cluster
// storage-durability policy (config.StoragePolicy). The controller is the sole
// owner of /globular/system/storage-policy; these RPCs are the only sanctioned
// write path. Degraded storage is an explicit, audited operator choice — it is
// never inferred from the current node count (intent:degraded_is_explicit_not_hidden).

// SetStoragePolicy validates and persists the declared cluster storage policy
// through the governed owner-write path. A degraded profile is rejected unless
// allow_degraded is explicitly set, and durable+allow_degraded is a rejected
// contradiction (config.StoragePolicy.Validate).
func (srv *server) SetStoragePolicy(ctx context.Context, req *cluster_controllerpb.SetStoragePolicyRequest) (*cluster_controllerpb.SetStoragePolicyResponse, error) {
	profile := config.StorageProfile(strings.ToLower(strings.TrimSpace(req.GetProfile())))
	if profile == "" {
		profile = config.StorageProfileDurable
	}

	// Read current policy for generation continuity (never nil — durable default).
	current, err := config.LoadStoragePolicy(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "load current storage policy: %v", err)
	}

	p := &config.StoragePolicy{
		Profile:       profile,
		AllowDegraded: req.GetAllowDegraded(),
		Generation:    current.Generation + 1,
		DeclaredAt:    time.Now(),
		DeclaredBy:    strings.TrimSpace(req.GetDeclaredBy()),
		Reason:        strings.TrimSpace(req.GetReason()),
	}
	if err := p.Validate(); err != nil {
		// Validate enforces "degraded requires explicit allow_degraded" and rejects
		// durable+allow_degraded — surface as InvalidArgument, not a silent accept.
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := config.SaveStoragePolicy(ctx, p); err != nil {
		return nil, status.Errorf(codes.Internal, "persist storage policy: %v", err)
	}

	// Audit the declaration loudly — degraded durability must be visible, never
	// silently absorbed.
	log.Printf("storage-policy: DECLARED profile=%s allow_degraded=%v is_degraded=%v min_storage_nodes=%d generation=%d declared_by=%q reason=%q (intent:degraded_is_explicit_not_hidden)",
		p.Profile, p.AllowDegraded, p.IsDegraded(), p.MinStorageNodes(), p.Generation, p.DeclaredBy, p.Reason)
	srv.emitClusterEvent("controller.storage_policy_declared", map[string]interface{}{
		"profile":           string(p.Profile),
		"allow_degraded":    p.AllowDegraded,
		"is_degraded":       p.IsDegraded(),
		"min_storage_nodes": p.MinStorageNodes(),
		"generation":        p.Generation,
		"declared_by":       p.DeclaredBy,
	})

	return &cluster_controllerpb.SetStoragePolicyResponse{
		Profile:         string(p.Profile),
		AllowDegraded:   p.AllowDegraded,
		Generation:      p.Generation,
		MinStorageNodes: int32(p.MinStorageNodes()),
		IsDegraded:      p.IsDegraded(),
	}, nil
}

// GetStoragePolicy returns the declared cluster storage policy, resolving to the
// durable default when none has been declared.
func (srv *server) GetStoragePolicy(ctx context.Context, req *cluster_controllerpb.GetStoragePolicyRequest) (*cluster_controllerpb.GetStoragePolicyResponse, error) {
	p, err := config.LoadStoragePolicy(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "load storage policy: %v", err)
	}
	return &cluster_controllerpb.GetStoragePolicyResponse{
		Profile:         string(p.Profile),
		AllowDegraded:   p.AllowDegraded,
		Generation:      p.Generation,
		MinStorageNodes: int32(p.MinStorageNodes()),
		IsDegraded:      p.IsDegraded(),
		DeclaredBy:      p.DeclaredBy,
		Reason:          p.Reason,
	}, nil
}
