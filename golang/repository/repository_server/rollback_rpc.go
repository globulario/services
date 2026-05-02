package main

// rollback_rpc.go — Phase CLI-C public RPCs for installed-revision history
// and rollback candidate listing.
//
// The actual rollback execution (drain → snapshot configs → install target →
// verify health) is the node-agent's responsibility and is wired by the
// `package.rollback` workflow. This RPC surface lets the controller / CLI /
// AI executor enumerate viable targets and record results.

import (
	"context"
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *server) RecordInstalledRevision(ctx context.Context, req *repopb.RecordInstalledRevisionRequest) (*repopb.RecordInstalledRevisionResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	rev := req.GetRevision()
	if rev == nil {
		return nil, status.Error(codes.InvalidArgument, "revision is required")
	}
	if rev.GetPublisherId() == "" || rev.GetName() == "" || rev.GetPlatform() == "" {
		return nil, status.Error(codes.InvalidArgument, "publisher_id, name, platform are required")
	}
	if err := srv.saveInstalledRevision(ctx, rev); err != nil {
		return nil, status.Errorf(codes.Internal, "save revision: %v", err)
	}
	srv.publishAuditEvent(ctx, "repository.revision.record", map[string]any{
		"publisher_id": rev.GetPublisherId(),
		"name":         rev.GetName(),
		"version":      rev.GetVersion(),
		"build_id":     rev.GetBuildId(),
		"platform":     rev.GetPlatform(),
		"node_id":      rev.GetNodeId(),
		"action":       rev.GetAction(),
		"revision_id":  rev.GetRevisionId(),
	})
	return &repopb.RecordInstalledRevisionResponse{RevisionId: rev.GetRevisionId()}, nil
}

func (srv *server) ListInstalledRevisions(ctx context.Context, req *repopb.ListInstalledRevisionsRequest) (*repopb.ListInstalledRevisionsResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	pubID := strings.TrimSpace(req.GetPublisherId())
	name := strings.TrimSpace(req.GetName())
	platform := strings.TrimSpace(req.GetPlatform())
	if pubID == "" || name == "" || platform == "" {
		return nil, status.Error(codes.InvalidArgument, "publisher_id, name, platform are required")
	}
	limit := int(req.GetLimit())
	if limit == 0 {
		limit = 25
	}
	rows := srv.loadInstalledRevisions(ctx, pubID, name, platform, strings.TrimSpace(req.GetNodeId()), limit)
	return &repopb.ListInstalledRevisionsResponse{Revisions: rows}, nil
}

func (srv *server) ListRollbackCandidates(ctx context.Context, req *repopb.ListRollbackCandidatesRequest) (*repopb.ListRollbackCandidatesResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	pubID := strings.TrimSpace(req.GetPublisherId())
	name := strings.TrimSpace(req.GetName())
	platform := strings.TrimSpace(req.GetPlatform())
	if pubID == "" || name == "" || platform == "" {
		return nil, status.Error(codes.InvalidArgument, "publisher_id, name, platform are required")
	}
	limit := int(req.GetLimit())
	if limit == 0 {
		limit = 5
	}
	rows := srv.loadInstalledRevisions(ctx, pubID, name, platform, strings.TrimSpace(req.GetNodeId()), 0)

	resp := &repopb.ListRollbackCandidatesResponse{}
	if len(rows) == 0 {
		return resp, nil
	}
	// First row is current install (newest). Everything else is a candidate.
	current := rows[0]
	resp.CurrentRef = &repopb.ArtifactRef{
		PublisherId: current.GetPublisherId(),
		Name:        current.GetName(),
		Version:     current.GetVersion(),
		Platform:    current.GetPlatform(),
		Kind:        current.GetKind(),
	}
	for i := 1; i < len(rows) && len(resp.Candidates) < limit; i++ {
		r := rows[i]
		ref := &repopb.ArtifactRef{
			PublisherId: r.GetPublisherId(), Name: r.GetName(),
			Version: r.GetVersion(), Platform: r.GetPlatform(), Kind: r.GetKind(),
		}
		resp.Candidates = append(resp.Candidates, &repopb.RollbackCandidate{
			Revision:    r,
			TargetRef:   ref,
			Eligibility: srv.evaluateRollbackCandidate(ctx, ref, r.GetBuildNumber()),
		})
	}
	return resp, nil
}
