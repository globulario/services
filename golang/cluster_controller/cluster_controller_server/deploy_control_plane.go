package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/repository/repositorypb"
)

// DeployControlPlanePackage triggers a leader-aware rolling update for a
// control-plane service. Validates inputs, resolves the exact build from
// the repository, ensures this instance is the leader, then dispatches
// the release.apply.controller workflow asynchronously.
//
// Build resolution rules:
//   - build_number > 0: resolve exactly that build; fail if not found
//   - build_number == 0: resolve the latest published build for the version
//   - Response includes both requested and resolved build numbers
func (srv *server) DeployControlPlanePackage(ctx context.Context, req *cluster_controllerpb.DeployControlPlanePackageRequest) (*cluster_controllerpb.DeployControlPlanePackageResponse, error) {
	// ── Input validation ──
	pkgName := strings.TrimSpace(req.GetPackageName())
	if pkgName == "" {
		return reject("package_name is required"), nil
	}
	version := strings.TrimSpace(req.GetVersion())
	if version == "" {
		return reject("version is required"), nil
	}
	pkgKind := strings.ToUpper(strings.TrimSpace(req.GetPackageKind()))
	if pkgKind == "" {
		pkgKind = "SERVICE"
	}
	if pkgKind != "SERVICE" && pkgKind != "INFRASTRUCTURE" {
		return reject(fmt.Sprintf("package_kind must be SERVICE or INFRASTRUCTURE, got %q", pkgKind)), nil
	}

	// ── Leader check ──
	if !srv.isLeader() {
		return reject("this instance is not the leader — send to leader node"), nil
	}
	if srv.workflowClient == nil {
		return reject("workflow service not configured"), nil
	}

	// ── Resolve build from repository ──
	requestedBuild := req.GetBuildNumber()
	publisher := strings.TrimSpace(req.GetPublisher())
	if publisher == "" {
		publisher = "core@globular.io"
	}

	artifactKind := repositorypb.ArtifactKind_SERVICE
	if pkgKind == "INFRASTRUCTURE" {
		artifactKind = repositorypb.ArtifactKind_INFRASTRUCTURE
	}

	resolver := &ReleaseResolver{ArtifactKind: artifactKind}
	spec := &cluster_controllerpb.ServiceReleaseSpec{
		PublisherID:  publisher,
		ServiceName:  pkgName,
		Version:      version,
		BuildNumber:  requestedBuild,
	}

	resolved, err := resolver.Resolve(ctx, spec)
	if err != nil {
		return reject(fmt.Sprintf("build resolution failed: %v", err)), nil
	}

	resolvedBuild := resolved.BuildNumber
	if requestedBuild > 0 && resolvedBuild != requestedBuild {
		return reject(fmt.Sprintf("requested build %d but repository resolved build %d", requestedBuild, resolvedBuild)), nil
	}

	// ── Generate correlation ID (independent of build identity) ──
	correlationID := fmt.Sprintf("deploy-controlplane/%s@%s+%d/%d",
		pkgName, version, resolvedBuild, time.Now().UnixMilli())

	// ── Resolve identity context ──
	localIP := config.GetRoutableIPv4()
	var acceptedByNodeID, leaderNodeID string
	srv.lock("deploy-control-plane:identity")
	for id, node := range srv.state.Nodes {
		if node.PrimaryIP() == localIP {
			acceptedByNodeID = id
			if srv.isLeader() {
				leaderNodeID = id
			}
		}
	}
	srv.unlock()

	log.Printf("deploy-control-plane: accepted %s/%s@%s (requested_build=%d resolved_build=%d digest=%s) node=%s",
		pkgKind, pkgName, version, requestedBuild, resolvedBuild, resolved.Digest, acceptedByNodeID)

	// ── Set desired version in etcd so the controller tracks this as the intended state ──
	if err := srv.upsertOne(ctx, &cluster_controllerpb.DesiredService{
		ServiceId:   pkgName,
		Version:     version,
		BuildNumber: resolvedBuild,
	}); err != nil {
		log.Printf("deploy-control-plane: WARNING failed to set desired version: %v", err)
		// Non-fatal — the deploy still proceeds, operator can fix desired state later.
	}

	// ── Dispatch async — don't block the RPC ──
	go func() {
		err := srv.RunControllerDeployWorkflow(context.Background(), pkgName, pkgKind, version, resolvedBuild)
		if err != nil {
			log.Printf("deploy-control-plane: workflow FAILED for %s@%s+%d: %v", pkgName, version, resolvedBuild, err)
		} else {
			log.Printf("deploy-control-plane: workflow SUCCEEDED for %s@%s+%d", pkgName, version, resolvedBuild)
		}
	}()

	buildLabel := "latest"
	if requestedBuild > 0 {
		buildLabel = fmt.Sprintf("%d", requestedBuild)
	}

	return &cluster_controllerpb.DeployControlPlanePackageResponse{
		Accepted:            true,
		RunId:               correlationID,
		Status:              "ACCEPTED",
		Message:             fmt.Sprintf("rollout dispatched for %s/%s@%s build %s (resolved=%d, digest=%s)", pkgKind, pkgName, version, buildLabel, resolvedBuild, resolved.Digest[:12]),
		WorkflowName:        "release.apply.controller",
		AcceptedByNodeId:    acceptedByNodeID,
		CurrentLeaderNodeId: leaderNodeID,
	}, nil
}

func reject(reason string) *cluster_controllerpb.DeployControlPlanePackageResponse {
	return &cluster_controllerpb.DeployControlPlanePackageResponse{
		Accepted: false,
		Status:   "REJECTED",
		Error:    reason,
	}
}
