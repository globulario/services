// workflow_repo_sync.go wires the repository actor actions for the controller
// and provides runSyncUpstreamWorkflow — the entry point for executing
// repository.sync.upstream via the centralized workflow service.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/repository/repository_client"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/workflow/engine"
)

// buildRepositoryConfig returns the RepositoryConfig wired to the live
// repository service. Used for both the default router (fallback) and
// per-run routers created by runSyncUpstreamWorkflow.
func (srv *server) buildRepositoryConfig() engine.RepositoryConfig {
	return engine.RepositoryConfig{
		// PublishBootstrapArtifacts is not triggered from the controller;
		// it runs from the node-agent during Day-0. No-op here.
		PublishBootstrapArtifacts: nil,

		SyncUpstream: func(ctx context.Context, sourceName, releaseTag string, dryRun bool, only []string) (map[string]any, error) {
			repoAddr := config.ResolveLocalServiceAddr("repository.PackageRepository")
			if repoAddr == "" {
				return nil, fmt.Errorf("repository service not found in registry")
			}
			rc, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
			if err != nil {
				return nil, fmt.Errorf("repository client: %w", err)
			}
			defer rc.Close()

			resp, err := rc.SyncFromUpstream(sourceName, releaseTag, dryRun, only)
			if err != nil {
				return nil, fmt.Errorf("SyncFromUpstream: %w", err)
			}

			summary := map[string]any{
				"source_name": sourceName,
				"release_tag": releaseTag,
				"dry_run":     dryRun,
				"imported":    resp.Imported,
				"skipped":     resp.Skipped,
				"rejected":    resp.Rejected,
				"failed":      resp.Failed,
			}
			log.Printf("repository.sync.upstream: source=%s tag=%s imported=%d skipped=%d rejected=%d failed=%d dry_run=%v",
				sourceName, releaseTag, resp.Imported, resp.Skipped, resp.Rejected, resp.Failed, dryRun)
			return summary, nil
		},
	}
}

// runSyncUpstreamWorkflow triggers repository.sync.upstream via the
// centralized workflow service. Returns the sync result summary from
// the workflow's output.
func (srv *server) runSyncUpstreamWorkflow(ctx context.Context, req *repositorypb.SyncFromUpstreamRequest) (map[string]any, error) {
	corrID := fmt.Sprintf("repo-sync-%s-%s", req.SourceName, req.ReleaseTag)

	router := engine.NewRouter()
	engine.RegisterRepositoryActions(router, srv.buildRepositoryConfig())

	inputs := map[string]any{
		"source_name": req.SourceName,
		"release_tag": req.ReleaseTag,
		"dry_run":     req.DryRun,
	}
	if len(req.Only) > 0 {
		only := make([]any, len(req.Only))
		for i, v := range req.Only {
			only[i] = v
		}
		inputs["only"] = only
	}

	resp, err := srv.executeWorkflowCentralized(ctx, "repository.sync.upstream", corrID, inputs, router)
	if err != nil {
		return nil, err
	}
	if resp.Status == "FAILED" {
		return nil, fmt.Errorf("repository.sync.upstream workflow failed: %s", resp.Error)
	}
	return map[string]any{"status": resp.Status}, nil
}
