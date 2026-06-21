package infra_truth

import (
	"fmt"
	"strings"
)

// EtcdDesiredInputs carries the raw cluster facts the node-agent gathered (from
// local identity + the rendered etcd_endpoints file / etcd membership) into the
// pure desired-state builder. The node-agent populates this; the builder stays
// free of etcd/config dependencies so it is unit-testable. Provenance is recorded
// so an AI agent can audit where Globular's expectation came from.
type EtcdDesiredInputs struct {
	NodeID    string
	ClusterID string
	LocalIP   string // this node's cluster-facing address (StableIP, never the VIP)

	// Peers is every expected etcd member's cluster-facing host (may include
	// self), derived from the controller-rendered etcd endpoints list.
	Peers []string

	// ClusterToken is the expected initial-cluster-token — the fixed
	// bootstrap-immutable constant (config.EtcdClusterToken), not a value derived
	// from cluster_id. Empty means "no authoritative value wired".
	ClusterToken string

	// BootstrapIntentOverride, when non-empty, wins over the membership-derived
	// guess (e.g. the node-agent knows bootstrap.enabled is set).
	BootstrapIntentOverride string

	SourceVersion string // optional generation/version of the source data
	Now           int64  // unix seconds; injected so tests are deterministic
}

// BuildEtcdDesiredState derives the intended etcd state for this node with
// explicit provenance. It returns an error when the minimum facts (node id and
// local IP) are missing — the caller turns that into an explicit
// infra.desired_state_unavailable violation rather than silently skipping.
func BuildEtcdDesiredState(in EtcdDesiredInputs) (*InfraDesiredState, error) {
	if strings.TrimSpace(in.NodeID) == "" {
		return nil, fmt.Errorf("cannot build etcd desired state: node id is empty")
	}
	if strings.TrimSpace(in.LocalIP) == "" {
		return nil, fmt.Errorf("cannot build etcd desired state: local IP is empty")
	}

	peers := dedupExcludingEmpty(in.Peers)
	ds := &InfraDesiredState{
		Component:                  ComponentEtcd,
		NodeID:                     in.NodeID,
		ClusterID:                  in.ClusterID,
		Source:                     SourceComputedFromMembership,
		SourceVersion:              in.SourceVersion,
		GeneratedAt:                in.Now,
		ExpectedListenAddresses:    []string{in.LocalIP},
		ExpectedAdvertiseAddresses: []string{in.LocalIP},
		ExpectedPeers:              peers,
		// etcd has no separate seed concept — the initial-cluster IS the peer set.
		ExpectedSeeds:       peers,
		ExpectedClusterName: strings.TrimSpace(in.ClusterToken),
		BootstrapIntent:     deriveEtcdBootstrapIntent(in),
	}
	return ds, nil
}

// deriveEtcdBootstrapIntent decides first-node vs joining-node. An explicit
// override wins. Otherwise: any non-self expected peer means this node is joining
// an existing cluster; only self means it is bootstrapping a fresh ring.
func deriveEtcdBootstrapIntent(in EtcdDesiredInputs) string {
	if o := strings.TrimSpace(in.BootstrapIntentOverride); o != "" {
		return o
	}
	if hasNonSelf(in.Peers, in.LocalIP) {
		return BootstrapJoining
	}
	return BootstrapFirstNode
}
