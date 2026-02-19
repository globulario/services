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

	// VIPFailover configuration (only used when Mode == ModeVIPFailover)
	VIPFailover *VIPFailoverSpec `json:"vip_failover,omitempty"`
}

// VIPFailoverSpec defines keepalived VRRP configuration
type VIPFailoverSpec struct {
	// VIP is the virtual IP address, with or without CIDR
	// Examples: "10.0.0.250" or "10.0.0.250/24"
	VIP string `json:"vip"`

	// Interface is the network interface to bind the VIP
	// Example: "eth0"
	Interface string `json:"interface"`

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
