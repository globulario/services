package main

// tools_release.go — PR-16 Phase 2: the release_verify_boundary MCP tool.
//
// Thin front door over release_boundary/boundarycheck: it supplies transport
// (owner-RPC closures using the MCP client pool + auth) and returns the shared
// report. All proof mapping + evaluation lives in boundarycheck — this file
// forks no proof logic.
//
// Read-only (intent:awareness.mcp_bridge_exposes_safe_tools_only): no repair,
// no mutation, no storage reads. Every truth source is the owning actor's typed
// RPC (meta.storage_is_not_semantic_authority). RPC errors are surfaced via the
// report's collection_errors, never absorbed (meta.connection_errors_must_not_
// be_absorbed).

import (
	"context"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/release_boundary/boundarycheck"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func registerReleaseTools(s *server) {

	// ── release_verify_boundary ─────────────────────────────────────────
	s.register(toolDef{
		Name:        "release_verify_boundary",
		Description: "Verify that a service's desired published artifact is the same artifact installed on a node and currently running, using the PR-16 release-boundary proof evaluator. Read-only: proves the build_id published by the repository equals what is installed and running, and that the process restarted after install. Returns PROVEN / FAILED / INDETERMINATE / NOT_APPLICABLE per assertion (A0..A4).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"service_id": {Type: "string", Description: "Service identifier, e.g. \"globular/echo\" or \"echo\". Matched against desired-state ServiceId."},
				"node_id":    {Type: "string", Description: "The node to inspect installed + runtime evidence on."},
			},
			Required: []string{"service_id", "node_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		serviceID := getStr(args, "service_id")
		if serviceID == "" {
			serviceID = getStr(args, "service_name")
		}
		nodeID := getStr(args, "node_id")
		if nodeID == "" {
			nodeID = getStr(args, "node_name")
		}

		report, ev := boundarycheck.Run(ctx, s.releaseBoundaryFetchers(ctx, nodeID), serviceID, nodeID)
		return boundarycheck.ReportToMap(report, ev), nil
	})
}

// releaseBoundaryFetchers wires the boundarycheck transport closures to the MCP
// client pool. Each closure dials its owner via s.clients, wraps auth, and sets
// its own timeout. The node-agent endpoint is resolved once; a resolution
// failure is carried into the installed/runtime closures so it surfaces as a
// named collection error rather than being absorbed.
func (s *server) releaseBoundaryFetchers(ctx context.Context, nodeID string) boundarycheck.Fetchers {
	naEndpoint, naErr := s.resolveNodeAgentEndpoint(ctx, nodeID)

	nodeAgent := func(cctx context.Context) (node_agentpb.NodeAgentServiceClient, error) {
		if naErr != nil {
			return nil, naErr
		}
		conn, err := s.clients.get(cctx, naEndpoint)
		if err != nil {
			return nil, err
		}
		return node_agentpb.NewNodeAgentServiceClient(conn), nil
	}

	return boundarycheck.Fetchers{
		Desired: func(cctx context.Context) ([]*cluster_controllerpb.DesiredService, error) {
			conn, err := s.clients.get(cctx, controllerEndpoint())
			if err != nil {
				return nil, err
			}
			callCtx, cancel := context.WithTimeout(authCtx(cctx), 10*time.Second)
			defer cancel()
			resp, err := cluster_controllerpb.NewClusterControllerServiceClient(conn).GetDesiredState(callCtx, &emptypb.Empty{})
			if err != nil {
				return nil, err
			}
			return resp.GetServices(), nil
		},
		Manifest: func(cctx context.Context, ref *repositorypb.ArtifactRef, buildNumber int64) (*repositorypb.ArtifactManifest, error) {
			conn, err := s.clients.get(cctx, repositoryEndpoint())
			if err != nil {
				return nil, err
			}
			callCtx, cancel := context.WithTimeout(authCtx(cctx), 10*time.Second)
			defer cancel()
			resp, err := repositorypb.NewPackageRepositoryClient(conn).GetArtifactManifest(callCtx, &repositorypb.GetArtifactManifestRequest{
				Ref:         ref,
				BuildNumber: buildNumber,
			})
			if err != nil {
				return nil, err
			}
			return resp.GetManifest(), nil
		},
		Verify: func(cctx context.Context, ref *repositorypb.ArtifactRef, buildID string) (*repositorypb.VerifyArtifactResponse, error) {
			conn, err := s.clients.get(cctx, repositoryEndpoint())
			if err != nil {
				return nil, err
			}
			callCtx, cancel := context.WithTimeout(authCtx(cctx), 10*time.Second)
			defer cancel()
			return repositorypb.NewPackageRepositoryClient(conn).VerifyArtifact(callCtx, &repositorypb.VerifyArtifactRequest{
				Ref:             ref,
				BuildId:         buildID,
				VerifyDigest:    true,
				IncludeLedger:   true,
				IncludeManifest: true,
				IncludeBlob:     true,
			})
		},
		Installed: func(cctx context.Context, nodeID, kind, name string) (*node_agentpb.InstalledPackage, error) {
			client, err := nodeAgent(cctx)
			if err != nil {
				return nil, err
			}
			callCtx, cancel := context.WithTimeout(authCtx(cctx), 10*time.Second)
			defer cancel()
			resp, err := client.GetInstalledPackage(callCtx, &node_agentpb.GetInstalledPackageRequest{
				NodeId: nodeID, Kind: kind, Name: name,
			})
			if err != nil {
				return nil, err
			}
			return resp.GetPackage(), nil
		},
		Runtime: func(cctx context.Context, nodeID, serviceName string) ([]*node_agentpb.ServiceRuntimeProof, error) {
			client, err := nodeAgent(cctx)
			if err != nil {
				return nil, err
			}
			callCtx, cancel := context.WithTimeout(authCtx(cctx), 10*time.Second)
			defer cancel()
			resp, err := client.GetServiceRuntimeProof(callCtx, &node_agentpb.GetServiceRuntimeProofRequest{
				NodeId: nodeID, ServiceName: serviceName,
			})
			if err != nil {
				return nil, err
			}
			return resp.GetProofs(), nil
		},
	}
}
