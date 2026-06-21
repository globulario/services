package collector

// release_boundary.go — PR-19: collect PR-16 release-boundary proofs for an
// allowlist of ordinary services, for consumption by the
// release.boundary_unproven rule.
//
// This is read-only (cluster_doctor.observer_only_never_writes_etcd): it runs
// the shared boundarycheck verifier, which reads every truth source through its
// owning actor's typed RPC (four_layer.truth_read_via_owner_rpc_not_direct_
// storage). It performs no repair and no writes. The proof mapping/evaluation
// is NOT forked here — it reuses release_boundary/boundarycheck.

import (
	"context"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/release_boundary"
	"github.com/globulario/services/golang/release_boundary/boundarycheck"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ReleaseBoundaryReport is the stored result of a release-boundary proof for one
// (service, node), produced by the collector and consumed by the
// release.boundary_unproven rule.
type ReleaseBoundaryReport struct {
	Service          string
	Node             string
	Report           release_boundary.Report
	CollectionErrors map[string]string
	ProvenanceGitSHA string
}

// DefaultReleaseBoundaryAllowlist is the conservative, opt-in set of ordinary
// services the doctor verifies. Start small and expand only as services are
// validated for release-boundary proof. Self-hosted control-plane services are
// excluded by releaseBoundarySelfHosted (their install timestamps can be
// PID-start anchored, making A4 ambiguous — see Phase 1.5).
var DefaultReleaseBoundaryAllowlist = []string{"event"}

// releaseBoundarySelfHosted services are never release-boundary-checked by the
// doctor, even if added to an allowlist.
var releaseBoundarySelfHosted = map[string]bool{
	"repository":         true,
	"node-agent":         true,
	"cluster-controller": true,
	"cluster-doctor":     true,
}

// collectReleaseBoundary runs the shared boundarycheck verifier for each
// allowlisted ordinary service that is actually installed on this node, using
// the collector's existing owner-RPC clients. A service not installed on this
// node is skipped (it is not this node's boundary to prove). Requires a
// repository client; returns nil otherwise.
func (c *Collector) collectReleaseBoundary(ctx context.Context, nodeID string, agent node_agentpb.NodeAgentServiceClient, allowlist []string) []*ReleaseBoundaryReport {
	if c.repoClient == nil || c.controllerClient == nil || agent == nil {
		return nil
	}

	var out []*ReleaseBoundaryReport
	for _, svc := range allowlist {
		if releaseBoundarySelfHosted[svc] {
			continue
		}
		// Installed-presence gate: only prove services actually installed on
		// this node, so absent-elsewhere services don't generate INDETERMINATE
		// noise on nodes that never hosted them.
		if !c.serviceInstalledOnNode(ctx, agent, nodeID, svc) {
			continue
		}

		f := c.releaseBoundaryFetchers(agent)
		report, ev := boundarycheck.Run(ctx, f, svc, nodeID, boundarycheck.Options{})
		prov := ""
		if ev.Manifest != nil {
			prov = ev.Manifest.GetProvenance().GetBuildCommit()
		}
		out = append(out, &ReleaseBoundaryReport{
			Service:          svc,
			Node:             nodeID,
			Report:           report,
			CollectionErrors: ev.CollectionErrors,
			ProvenanceGitSHA: prov,
		})
	}
	return out
}

// serviceInstalledOnNode reports whether the named SERVICE package is installed
// on the node (used as the presence gate). A query failure is treated as "not
// installed" for gating purposes — the absence does not become a finding, and a
// genuinely installed-but-broken service is still caught by the full proof when
// the gate passes.
func (c *Collector) serviceInstalledOnNode(ctx context.Context, agent node_agentpb.NodeAgentServiceClient, nodeID, service string) bool {
	resp, err := agent.GetInstalledPackage(ctx, &node_agentpb.GetInstalledPackageRequest{
		NodeId: nodeID,
		Kind:   "SERVICE",
		Name:   boundaryServiceShortName(service),
	})
	if err != nil {
		return false
	}
	return resp.GetPackage() != nil
}

// releaseBoundaryFetchers wires boundarycheck's transport to the collector's
// owner-RPC clients. Every closure performs one typed owner RPC.
func (c *Collector) releaseBoundaryFetchers(agent node_agentpb.NodeAgentServiceClient) boundarycheck.Fetchers {
	return boundarycheck.Fetchers{
		Desired: func(ctx context.Context) ([]*cluster_controllerpb.DesiredService, error) {
			resp, err := c.controllerClient.GetDesiredState(ctx, &emptypb.Empty{})
			if err != nil {
				return nil, err
			}
			return resp.GetServices(), nil
		},
		Resolve: func(ctx context.Context, req *repopb.ResolveArtifactRequest) (*repopb.ArtifactManifest, error) {
			resp, err := c.repoClient.ResolveArtifact(ctx, req)
			if err != nil {
				return nil, err
			}
			return resp.GetManifest(), nil
		},
		Verify: func(ctx context.Context, ref *repopb.ArtifactRef, buildID string) (*repopb.VerifyArtifactResponse, error) {
			return c.repoClient.VerifyArtifact(ctx, &repopb.VerifyArtifactRequest{
				Ref:             ref,
				BuildId:         buildID,
				VerifyDigest:    true,
				IncludeLedger:   true,
				IncludeManifest: true,
				IncludeBlob:     true,
			})
		},
		Installed: func(ctx context.Context, nodeID, kind, name string) (*node_agentpb.InstalledPackage, error) {
			resp, err := agent.GetInstalledPackage(ctx, &node_agentpb.GetInstalledPackageRequest{
				NodeId: nodeID, Kind: kind, Name: name,
			})
			if err != nil {
				return nil, err
			}
			return resp.GetPackage(), nil
		},
		Runtime: func(ctx context.Context, nodeID, serviceName string) ([]*node_agentpb.ServiceRuntimeProof, error) {
			resp, err := agent.GetServiceRuntimeProof(ctx, &node_agentpb.GetServiceRuntimeProofRequest{
				NodeId: nodeID, ServiceName: serviceName,
			})
			if err != nil {
				return nil, err
			}
			return resp.GetProofs(), nil
		},
	}
}

// boundaryServiceShortName strips an optional "publisher/" prefix.
func boundaryServiceShortName(serviceID string) string {
	for i := len(serviceID) - 1; i >= 0; i-- {
		if serviceID[i] == '/' {
			return serviceID[i+1:]
		}
	}
	return serviceID
}
