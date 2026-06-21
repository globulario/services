package infra_truth

import (
	"fmt"
	"strings"
)

// envoyExpectedADSCluster is the static cluster name the Globular bootstrap wires
// the ADS stream to. The bootstrap renderer always names it "xds_cluster"; the
// ADS stream cannot connect unless this cluster is defined in static_resources.
const envoyExpectedADSCluster = "xds_cluster"

// EnvoyDesiredInputs carries the raw facts the node-agent gathered into the pure
// desired-state builder. Envoy is a per-node data plane (NOT clustered), so there
// is no peer set — desired state is mostly provenance plus this node's identity.
type EnvoyDesiredInputs struct {
	NodeID    string
	ClusterID string
	LocalIP   string

	SourceVersion string
	Now           int64
}

// BuildEnvoyDesiredState derives the intended Envoy data-plane state for this node
// with explicit provenance. It returns an error when the minimum facts (node id
// and local IP) are missing — the caller turns that into an explicit
// infra.desired_state_unavailable violation rather than silently skipping.
func BuildEnvoyDesiredState(in EnvoyDesiredInputs) (*InfraDesiredState, error) {
	if strings.TrimSpace(in.NodeID) == "" {
		return nil, fmt.Errorf("cannot build envoy desired state: node id is empty")
	}
	if strings.TrimSpace(in.LocalIP) == "" {
		return nil, fmt.Errorf("cannot build envoy desired state: local IP is empty")
	}

	return &InfraDesiredState{
		Component:                  ComponentEnvoy,
		NodeID:                     in.NodeID,
		ClusterID:                  in.ClusterID,
		Source:                     SourceComputedFromMembership,
		SourceVersion:              in.SourceVersion,
		GeneratedAt:                in.Now,
		ExpectedListenAddresses:    []string{in.LocalIP},
		ExpectedAdvertiseAddresses: []string{in.LocalIP},
		// Envoy is not clustered — no peers, no seeds. The ADS upstream is its only
		// "membership", attested structurally against envoyExpectedADSCluster.
		ExpectedClusterName: envoyExpectedADSCluster,
		BootstrapIntent:     BootstrapFirstNode,
	}, nil
}
