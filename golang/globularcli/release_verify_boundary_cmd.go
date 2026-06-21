package main

// release_verify_boundary_cmd.go — `globular release verify-boundary`.
//
// Operator front door to the PR-16 release-boundary proof. It proves that a
// service's desired published artifact (by build_id) is the same artifact
// installed on a node and currently running, and that the process restarted
// after install. The proof mapping + evaluation live in
// release_boundary/boundarycheck — this command only supplies transport
// (owner-RPC closures) and renders the report. It never duplicates the proof
// logic and never reads etcd / Scylla / repository storage / the node
// filesystem / /proc directly.

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/release_boundary/boundarycheck"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	verifyBoundaryNode string
	verifyBoundaryJSON bool
)

var releaseVerifyBoundaryCmd = &cobra.Command{
	Use:   "verify-boundary <service> --node <node>",
	Short: "Verify the release boundary for one service on one node",
	Long: `Verify that a service's desired published artifact is the same artifact
installed on a node and currently running, using the PR-16 release-boundary
proof. Read-only — performs no repair and no mutation.

Assertions:
  A0 repository artifact intact
  A1 desired binds a PUBLISHED build
  A2 installed == published
  A3 running executable == published
  A4 process restarted after install (uses metadata["installed_at"])

Exits 0 only when the overall verdict is PROVEN. FAILED, INDETERMINATE,
NOT_APPLICABLE, or any collection error exits non-zero.

Pilot against an ordinary installed service. Self-hosted control-plane
services (repository, node-agent, cluster-controller, cluster-doctor) may
anchor install time to process start, making A4 permanently INDETERMINATE.

Examples:
  globular release verify-boundary globular/echo --node globule-ryzen
  globular release verify-boundary echo --node globule-dell --json`,
	Args: cobra.ExactArgs(1),
	RunE: runReleaseVerifyBoundary,
}

func init() {
	releaseVerifyBoundaryCmd.Flags().StringVar(&verifyBoundaryNode, "node", "", "Node to inspect installed + runtime evidence on (required)")
	releaseVerifyBoundaryCmd.Flags().BoolVar(&verifyBoundaryJSON, "json", false, "Emit JSON output for automation / AI executor")
	_ = releaseVerifyBoundaryCmd.MarkFlagRequired("node")
	releaseCmd.AddCommand(releaseVerifyBoundaryCmd)
}

func runReleaseVerifyBoundary(cmd *cobra.Command, args []string) error {
	serviceID := args[0]
	nodeID := verifyBoundaryNode
	if nodeID == "" {
		return fmt.Errorf("--node is required")
	}

	ctx := context.Background()

	// Controller — desired state + node-agent endpoint resolution.
	ctrlConn, err := controllerClient()
	if err != nil {
		return fmt.Errorf("connect to cluster controller: %w", err)
	}
	defer ctrlConn.Close()
	ctrl := cluster_controllerpb.NewClusterControllerServiceClient(ctrlConn)

	// Repository (reuses the shared CLI repo client helper).
	repo, err := newRepoClient()
	if err != nil {
		return err
	}
	defer repo.Close()

	// Node-agent — resolve the target node's agent endpoint, then dial it.
	// A resolution/dial failure is carried into the installed/runtime closures
	// so it surfaces as a named collection error rather than aborting. The
	// resolved canonical node_id (UUID) is what the node-agent keys its
	// installed-state by, so RPCs use it even when --node was a hostname.
	na, canonicalNodeID, naErr := dialNodeAgentForNode(ctx, ctrl, nodeID)
	if na != nil {
		defer na.close()
	}
	if canonicalNodeID != "" {
		nodeID = canonicalNodeID
	}

	fetchers := boundarycheck.Fetchers{
		Desired: func(cctx context.Context) ([]*cluster_controllerpb.DesiredService, error) {
			callCtx, cancel := context.WithTimeout(cctx, 10*time.Second)
			defer cancel()
			resp, err := ctrl.GetDesiredState(callCtx, &emptypb.Empty{})
			if err != nil {
				return nil, err
			}
			return resp.GetServices(), nil
		},
		Manifest: func(_ context.Context, ref *repopb.ArtifactRef, buildNumber int64) (*repopb.ArtifactManifest, error) {
			return repo.GetArtifactManifest(ref, buildNumber)
		},
		Verify: func(_ context.Context, ref *repopb.ArtifactRef, buildID string) (*repopb.VerifyArtifactResponse, error) {
			return repo.VerifyArtifact(&repopb.VerifyArtifactRequest{
				Ref:             ref,
				BuildId:         buildID,
				VerifyDigest:    true,
				IncludeLedger:   true,
				IncludeManifest: true,
				IncludeBlob:     true,
			})
		},
		Installed: func(cctx context.Context, nodeID, kind, name string) (*node_agentpb.InstalledPackage, error) {
			if naErr != nil {
				return nil, naErr
			}
			callCtx, cancel := context.WithTimeout(cctx, 10*time.Second)
			defer cancel()
			resp, err := na.client.GetInstalledPackage(callCtx, &node_agentpb.GetInstalledPackageRequest{
				NodeId: nodeID, Kind: kind, Name: name,
			})
			if err != nil {
				return nil, err
			}
			return resp.GetPackage(), nil
		},
		Runtime: func(cctx context.Context, nodeID, serviceName string) ([]*node_agentpb.ServiceRuntimeProof, error) {
			if naErr != nil {
				return nil, naErr
			}
			callCtx, cancel := context.WithTimeout(cctx, 10*time.Second)
			defer cancel()
			resp, err := na.client.GetServiceRuntimeProof(callCtx, &node_agentpb.GetServiceRuntimeProofRequest{
				NodeId: nodeID, ServiceName: serviceName,
			})
			if err != nil {
				return nil, err
			}
			return resp.GetProofs(), nil
		},
	}

	report, ev := boundarycheck.Run(ctx, fetchers, serviceID, nodeID)

	if verifyBoundaryJSON {
		emitJSON(boundarycheck.ReportToMap(report, ev))
	} else {
		fmt.Print(boundarycheck.FormatReport(report, ev))
	}

	// Exit non-zero unless PROVEN with no collection errors.
	if boundarycheck.ExitCode(report.Verdict) != 0 || len(ev.CollectionErrors) > 0 {
		os.Exit(2)
	}
	return nil
}

// nodeAgentConn bundles a node-agent client with its connection for cleanup.
type nodeAgentConn struct {
	client node_agentpb.NodeAgentServiceClient
	close  func()
}

// dialNodeAgentForNode resolves the node's agent endpoint via the controller's
// ListNodes (matching --node against either node_id or hostname) and dials it.
// Returns the canonical node_id alongside the connection. Returns a non-nil
// error (not absorbed) when the node is unknown or has no registered endpoint.
func dialNodeAgentForNode(ctx context.Context, ctrl cluster_controllerpb.ClusterControllerServiceClient, nodeRef string) (*nodeAgentConn, string, error) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := ctrl.ListNodes(callCtx, &cluster_controllerpb.ListNodesRequest{})
	if err != nil {
		return nil, "", fmt.Errorf("ListNodes: %w", err)
	}
	var endpoint, canonicalID string
	for _, n := range resp.GetNodes() {
		if n.GetNodeId() == nodeRef ||
			n.GetIdentity().GetHostname() == nodeRef ||
			n.GetIdentity().GetNodeName() == nodeRef {
			endpoint = n.GetAgentEndpoint()
			canonicalID = n.GetNodeId()
			break
		}
	}
	if endpoint == "" {
		return nil, "", fmt.Errorf("node %s not found or has no agent endpoint", nodeRef)
	}
	conn, err := dialGRPC(endpoint)
	if err != nil {
		return nil, canonicalID, fmt.Errorf("dial node agent %s (%s): %w", nodeRef, endpoint, err)
	}
	return &nodeAgentConn{
		client: node_agentpb.NewNodeAgentServiceClient(conn),
		close:  func() { conn.Close() },
	}, canonicalID, nil
}
