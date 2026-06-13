// Package infra_truth implements the infrastructure truth plane for external
// infrastructure components managed by Globular (ScyllaDB first; etcd, Envoy and
// MinIO are designed to fit later without redesign).
//
// The core principle: a process being active is never sufficient evidence that
// infrastructure is correct. Every component is represented as five things —
//
//  1. Desired state      — what Globular intends (with provenance)
//  2. Rendered config     — the config file as an artifact, parsed
//  3. Runtime truth       — what the daemon actually believes (native API)
//  4. Lifecycle FSM state — where the component is in its join/health lifecycle
//  5. Violations          — typed problems with owner-targeted remediation
//
// Ownership (see docs/awareness): the controller owns desired state, the package
// renderer/node-agent owns the rendered config artifact, the node-agent attests
// and observes, and the native component API owns runtime truth. Config files are
// artifacts, not authority — repair must target the owner that generated the bad
// config, never a manual file edit.
//
// This package computes desired state for comparison only; it NEVER writes
// desired state to etcd (enforces invariant infra.heartbeat_observer_only_not_authority).
package infra_truth

import (
	"net"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// Component names. Phase 1 implements only ScyllaDB.
const (
	ComponentScylla = "scylladb"
	ComponentAll    = "all"
)

// Default on-disk location of the rendered ScyllaDB config.
const ScyllaConfigPath = "/etc/scylla/scylla.yaml"

// Severity strings carried on violations/config fields. These mirror the
// cluster-doctor severity vocabulary so the doctor can map them 1:1.
const (
	SeverityCritical = "CRITICAL"
	SeverityError    = "ERROR"
	SeverityWarn     = "WARN"
	SeverityInfo     = "INFO"
)

// Bootstrap intent values for desired state.
const (
	BootstrapFirstNode = "first-node"
	BootstrapJoining   = "joining-node"
)

// Desired-state provenance sources (recorded in InfraDesiredState.Source and
// surfaced in the probe result's desired["source"] so an AI agent can see where
// Globular's expectation came from).
const (
	SourceComputedFromMembership  = "computed_from_cluster_membership"
	SourceDesiredStateUnavailable = "desired_state_unavailable"
)

// InfraDesiredState is what Globular intends for one infrastructure component on
// one node. It is internal-only: it is projected into InfraProbeResult.desired
// (a string map) and used by attestation. It is NEVER persisted to etcd in
// Phase 1 — provenance fields make the derivation auditable instead.
type InfraDesiredState struct {
	Component     string
	NodeID        string
	ClusterID     string
	Source        string // one of the Source* constants
	SourceVersion string // optional version/generation of the source
	GeneratedAt   int64  // unix seconds when this desired state was computed

	ExpectedListenAddresses    []string
	ExpectedAdvertiseAddresses []string
	ExpectedPeers              []string // all expected cluster members (may include self)
	ExpectedSeeds              []string // expected seed addresses
	ExpectedClusterName        string
	BootstrapIntent            string // BootstrapFirstNode | BootstrapJoining
}

// ScyllaRenderedConfig is the parsed /etc/scylla/scylla.yaml — the rendered
// config artifact. Empty string means the field was absent from the file.
type ScyllaRenderedConfig struct {
	Path                string
	Present             bool
	ClusterName         string
	ListenAddress       string
	RPCAddress          string
	BroadcastAddress    string
	BroadcastRPCAddress string
	APIAddress          string
	Seeds               []string
}

// ScyllaRuntimeState is the native-API observed truth. Every field is best
// effort: a field left zero/empty means "not observed", recorded in Errors.
type ScyllaRuntimeState struct {
	DaemonActive      bool
	RESTAPIReady      bool
	CQLReady          bool
	OperationMode     string  // e.g. STARTING, JOINING, NORMAL, DECOMMISSIONED
	BootstrapProgress float64 // streaming completion percent (0..100), -1 if unknown
	GossipLive        int     // count of live gossip endpoints, -1 if unknown
	ObservedPeers     []string
	HostID            string
	SchemaVersion     string
	Group0Status      string
	Errors            []string // partial-failure evidence — non-empty != whole probe failed
}

// isLoopback reports whether addr is a loopback address or hostname.
// "0.0.0.0"/"::" (the unspecified address) is handled separately because it is a
// valid bind-all for rpc_address but forbidden as a cluster-facing listen
// address.
func isLoopback(addr string) bool {
	a := stripQuotes(addr)
	if a == "" {
		return false
	}
	if strings.EqualFold(a, "localhost") {
		return true
	}
	if ip := net.ParseIP(a); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// isUnspecified reports whether addr is the "any"/unspecified address.
func isUnspecified(addr string) bool {
	if ip := net.ParseIP(stripQuotes(addr)); ip != nil {
		return ip.IsUnspecified()
	}
	return false
}

// newViolation is a small constructor so call sites stay terse.
func newViolation(id, severity, message, evidence, remediation string) *cluster_controllerpb.InfraViolation {
	return &cluster_controllerpb.InfraViolation{
		Id:          id,
		Severity:    severity,
		Message:     message,
		Evidence:    evidence,
		Remediation: remediation,
	}
}

// stripQuotes trims surrounding whitespace and YAML-style quotes from a scalar.
func stripQuotes(s string) string {
	return strings.Trim(strings.TrimSpace(s), `"'`)
}
