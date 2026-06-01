// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.ingress
// @awareness file_role=ingress_spec_type_definitions
// @awareness risk=medium
package ingress

// Mode represents the ingress entrypoint mode
type Mode string

const (
	// ModeVIPFailover enables keepalived VRRP-based VIP failover (active/passive)
	ModeVIPFailover Mode = "vip_failover"

	// ModeDisabled disables ingress entrypoint management
	ModeDisabled Mode = "disabled"

	// Future modes (not implemented in v1):
	// ModeVIPHAProxyPool Mode = "vip_haproxy_pool"  // VIP + HAProxy active/active
	// ModeBGP            Mode = "bgp"                // BGP / ECMP
)

// Spec defines the ingress entrypoint specification
type Spec struct {
	Version string `json:"version"` // "v1"
	Mode    Mode   `json:"mode"`

	// Generation increments on every authoritative controller publish.
	Generation int64 `json:"generation,omitempty"`

	// Checksum is a content checksum of the canonical spec payload.
	Checksum string `json:"checksum,omitempty"`

	// WrittenAtUnix is the controller write timestamp (unix seconds).
	WrittenAtUnix int64 `json:"written_at_unix,omitempty"`

	// WriterLeaderID identifies the controller leader that wrote this spec.
	WriterLeaderID string `json:"writer_leader_id,omitempty"`

	// Source identifies the authoritative writer ("cluster-controller").
	Source string `json:"source,omitempty"`

	// ExplicitDisabled must be true to disable keepalived.
	// Missing spec or mode=disabled with ExplicitDisabled=false is treated as
	// non-destructive and should trigger hold-last-known-good behavior.
	ExplicitDisabled bool `json:"explicit_disabled,omitempty"`

	// Reason is a human-readable controller reason for the current spec.
	Reason string `json:"reason,omitempty"`

	// Authoritative indicates the spec was written and validated by an active
	// cluster-controller leader with a known cluster topology (Case 02:
	// BOOTSTRAP_STATE_ESCAPED_TO_PRODUCTION). Specs without this marker are
	// tentative (bootstrap or operator-injected) — consumers apply them but
	// log a warning and continue using LKG for destructive decisions.
	Authoritative bool `json:"authoritative,omitempty"`

	// VIPFailover configuration (only used when Mode == ModeVIPFailover)
	VIPFailover *VIPFailoverSpec `json:"vip_failover,omitempty"`
}

// IsExplicitDisable returns true only when the spec carries a fully-qualified
// disable intent: mode=disabled, explicit_disabled=true, non-empty reason, and
// positive generation. Any ambiguous disable (missing reason, zero generation,
// or explicit_disabled=false) is treated as non-destructive — the runtime must
// hold its last-known-good configuration rather than stopping keepalived.
//
// This is the shared policy helper for Case 11 (UNGUARDED_RUNTIME_DESTRUCTIVE_ACTION).
func (s *Spec) IsExplicitDisable() bool {
	return s.Mode == ModeDisabled &&
		s.ExplicitDisabled &&
		s.Reason != "" &&
		s.Generation > 0
}

// VIPFailoverSpec defines keepalived VRRP configuration
type VIPFailoverSpec struct {
	// VIP is the virtual IP address, with or without CIDR
	// Examples: "10.0.0.250" or "10.0.0.250/24"
	VIP string `json:"vip"`

	// Interface is the default network interface to bind the VIP
	// Example: "eth0"
	// Can be overridden per-node via InterfaceOverride
	Interface string `json:"interface"`

	// InterfaceOverride is a map of node ID to interface name
	// Use this when nodes have different interface names (e.g., wlp5s0 vs eno1)
	// If a node is in this map, its value is used instead of Interface
	InterfaceOverride map[string]string `json:"interface_override,omitempty"`

	// VirtualRouterID is the VRRP router ID (1-255)
	VirtualRouterID int `json:"virtual_router_id"`

	// AdvertIntervalMs is the VRRP advertisement interval in milliseconds
	// Default: 1000 (1 second)
	AdvertIntervalMs int `json:"advert_interval_ms"`

	// AuthPass is the VRRP authentication password (optional)
	// If empty, no authentication is configured
	AuthPass string `json:"auth_pass,omitempty"`

	// Participants is the list of node IDs allowed to participate in VIP election
	// Only nodes in this list will run keepalived
	Participants []string `json:"participants"`

	// Priority is a map of node ID to keepalived priority
	// Higher priority nodes become MASTER
	// Example: {"n1": 120, "n2": 110}
	Priority map[string]int `json:"priority"`

	// Preempt determines if a higher-priority node should preempt MASTER state
	// Default: true
	Preempt bool `json:"preempt"`

	// CheckTCPPorts is a list of TCP ports to check for health gating
	// If any port check fails, the node will not keep MASTER state
	// Example: [443] to check if Envoy is listening
	CheckTCPPorts []int `json:"check_tcp_ports"`
}
