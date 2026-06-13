package infra_truth

import (
	"fmt"
	"strings"
)

// ScyllaDesiredInputs carries the raw cluster facts the node-agent has gathered
// (from etcd membership + local config) into the pure desired-state builder. The
// node-agent populates this; the builder stays free of etcd/config dependencies
// so it is unit-testable. Provenance is recorded so an AI agent can audit where
// Globular's expectation came from.
type ScyllaDesiredInputs struct {
	NodeID    string
	ClusterID string
	LocalIP   string // this node's cluster-facing address (StableIP, never the VIP)

	ClusterName string
	Peers       []string // all expected cluster members, may include self
	Seeds       []string // expected seed addresses

	// BootstrapIntentOverride, when non-empty, wins over the membership-derived
	// guess (e.g. the node-agent knows bootstrap.enabled is set).
	BootstrapIntentOverride string

	SourceVersion string // optional generation/version of the source data
	Now           int64  // unix seconds; injected so tests are deterministic
}

// BuildScyllaDesiredState derives the intended ScyllaDB state for this node with
// explicit provenance. It returns an error when the minimum facts (node id and
// local IP) are missing — the caller turns that into an explicit
// infra.desired_state_unavailable violation rather than silently skipping.
func BuildScyllaDesiredState(in ScyllaDesiredInputs) (*InfraDesiredState, error) {
	if strings.TrimSpace(in.NodeID) == "" {
		return nil, fmt.Errorf("cannot build scylla desired state: node id is empty")
	}
	if strings.TrimSpace(in.LocalIP) == "" {
		return nil, fmt.Errorf("cannot build scylla desired state: local IP is empty")
	}

	ds := &InfraDesiredState{
		Component:                  ComponentScylla,
		NodeID:                     in.NodeID,
		ClusterID:                  in.ClusterID,
		Source:                     SourceComputedFromMembership,
		SourceVersion:              in.SourceVersion,
		GeneratedAt:                in.Now,
		ExpectedListenAddresses:    []string{in.LocalIP},
		ExpectedAdvertiseAddresses: []string{in.LocalIP},
		ExpectedPeers:              dedupExcludingEmpty(in.Peers),
		ExpectedSeeds:              dedupExcludingEmpty(in.Seeds),
		ExpectedClusterName:        strings.TrimSpace(in.ClusterName),
		BootstrapIntent:            deriveBootstrapIntent(in),
	}
	return ds, nil
}

// deriveBootstrapIntent decides first-node vs joining-node. An explicit override
// wins. Otherwise: if no non-self peer and no non-self seed is expected, this is
// the first node bootstrapping a fresh ring; any non-self peer/seed means it is
// joining an existing cluster.
func deriveBootstrapIntent(in ScyllaDesiredInputs) string {
	if o := strings.TrimSpace(in.BootstrapIntentOverride); o != "" {
		return o
	}
	if hasNonSelf(in.Peers, in.LocalIP) || hasNonSelf(in.Seeds, in.LocalIP) {
		return BootstrapJoining
	}
	return BootstrapFirstNode
}

// hasNonSelf reports whether addrs contains any entry that is not self.
func hasNonSelf(addrs []string, self string) bool {
	self = stripQuotes(self)
	for _, a := range addrs {
		if a := stripQuotes(a); a != "" && a != self {
			return true
		}
	}
	return false
}

func dedupExcludingEmpty(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range in {
		s = stripQuotes(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// desiredMap projects the desired state into the InfraProbeResult.desired map.
// The "source" key makes provenance visible to AI agents.
func (d *InfraDesiredState) desiredMap() map[string]string {
	return map[string]string{
		"source":             d.Source,
		"source_version":     d.SourceVersion,
		"node_id":            d.NodeID,
		"cluster_id":         d.ClusterID,
		"cluster_name":       d.ExpectedClusterName,
		"bootstrap_intent":   d.BootstrapIntent,
		"expected_listen":    strings.Join(d.ExpectedListenAddresses, ","),
		"expected_advertise": strings.Join(d.ExpectedAdvertiseAddresses, ","),
		"expected_peers":     strings.Join(d.ExpectedPeers, ","),
		"expected_seeds":     strings.Join(d.ExpectedSeeds, ","),
		"generated_at_unix":  fmt.Sprintf("%d", d.GeneratedAt),
	}
}
