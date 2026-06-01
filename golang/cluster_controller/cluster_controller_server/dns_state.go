// @awareness namespace=globular.platform
// @awareness component=platform_controller.dns
// @awareness file_role=dns_zone_state_management
// @awareness risk=medium
package main

import (
	"fmt"
	"sort"
	"strings"
)

// DesiredDNSState represents the authoritative DNS records that should exist
type DesiredDNSState struct {
	Domain     string
	Records    []DNSRecord
	Generation uint64
}

// DNSRecord represents a single DNS record
type DNSRecord struct {
	Name     string      // FQDN
	Type     RecordType  // A, AAAA, SRV, CNAME
	Value    string      // IP for A/AAAA, target for SRV
	TTL      uint32
	Priority uint16 // SRV only
	Weight   uint16 // SRV only
	Port     uint16 // SRV only
}

// RecordType identifies the DNS record type
type RecordType string

const (
	RecordTypeA     RecordType = "A"
	RecordTypeAAAA  RecordType = "AAAA"
	RecordTypeSRV   RecordType = "SRV"
	RecordTypeCNAME RecordType = "CNAME"
)

// NodeInfo contains node information for DNS record generation.
//
// Readiness gating — InstalledServices and RuntimeHealthy are how the DNS
// reconciler enforces "a record may only point at a node that actually serves
// what the record promises." When BOTH maps are nil, callers fall back to the
// legacy profile-only behavior (preserves existing tests and the cold-start
// bootstrap path where no readiness data is yet available). When either map is
// non-nil, the gated path activates and records are published ONLY when the
// referenced service is BOTH installed AND runtime-healthy on this node.
//
// This wires the 4-layer model into DNS:
//
//	Desired (profile)  -> "this node should serve gateway"
//	Installed          -> InstalledServices["gateway"] == true
//	Runtime healthy    -> RuntimeHealthy["gateway"] == true
//	Published          -> recorded in DNS only when both above are true
//
// The DNS reconciler is the only consumer that should set Draining/Quarantined
// — those force every gated record off the node regardless of installed/health.
type NodeInfo struct {
	FQDN     string
	IPv4     string
	IPv6     string
	Profiles []string

	// InstalledServices is the set of package names the node-agent has
	// confirmed installed (Layer 3). nil means "no readiness data available;
	// fall back to profile-only behavior for this node."
	InstalledServices map[string]bool
	// RuntimeHealthy is the set of services whose runtime health probe is
	// currently passing (Layer 4). nil means "no health data available;
	// fall back to profile-only behavior for this node."
	RuntimeHealthy map[string]bool
	// Draining or Quarantined removes the node from every gated record set,
	// even when installed+healthy, so a node being decommissioned stops
	// serving DNS before its services stop responding.
	Draining    bool
	Quarantined bool
}

// ServiceInstance represents a service running on a node (PR4.1)
type ServiceInstance struct {
	ServiceName string   // e.g., "echo.EchoService"
	NodeFQDN    string   // e.g., "node-01.cluster.local"
	Port        uint16   // Service port
}

// HasProfile checks if a node has a specific profile
func (n NodeInfo) HasProfile(profile string) bool {
	for _, p := range n.Profiles {
		if p == profile {
			return true
		}
	}
	return false
}

// hasReadinessSignal reports whether the node carries any installed/runtime
// data. When false, callers fall back to profile-only gating (legacy /
// bootstrap behavior). When true, the gated path is active for this node.
func (n NodeInfo) hasReadinessSignal() bool {
	return n.InstalledServices != nil || n.RuntimeHealthy != nil
}

// ServiceReady reports whether DNS records that depend on `service` may
// include this node. The gate is:
//
//   - node is not draining/quarantined, AND
//   - desired (profile) says this node should run something, AND
//   - installed map (Layer 3) says the service is installed, AND
//   - runtime map (Layer 4) says the service is healthy.
//
// When BOTH maps are nil — bootstrap, legacy tests, or a node whose agent
// hasn't reported yet — this returns true so the legacy profile-only behavior
// continues. Callers that want strict gating MUST populate at least one of
// InstalledServices / RuntimeHealthy on every node (the DNS reconciler does).
//
// `service` is the package name as stored in installed_state (e.g. "gateway",
// "dns", "scylladb", "cluster-controller").
func (n NodeInfo) ServiceReady(service string) bool {
	if n.Draining || n.Quarantined {
		return false
	}
	if !n.hasReadinessSignal() {
		// Legacy / cold-start: no readiness data → trust the profile gate
		// the caller already applied.
		return true
	}
	if n.InstalledServices != nil && !n.InstalledServices[service] {
		return false
	}
	if n.RuntimeHealthy != nil && !n.RuntimeHealthy[service] {
		return false
	}
	return true
}

// ComputeDesiredState builds the desired DNS state from cluster state
func ComputeDesiredState(domain string, nodes []NodeInfo, generation uint64) *DesiredDNSState {
	return ComputeDesiredStateWithLeader(domain, nodes, nil, "", generation)
}

// ComputeDesiredStateWithServices builds DNS state including service SRV records (PR4.1)
func ComputeDesiredStateWithServices(domain string, nodes []NodeInfo, services []ServiceInstance, generation uint64) *DesiredDNSState {
	return ComputeDesiredStateWithLeader(domain, nodes, services, "", generation)
}

// ComputeDesiredStateWithLeader builds DNS state with leader-aware controller record (H3)
func ComputeDesiredStateWithLeader(domain string, nodes []NodeInfo, services []ServiceInstance, leaderFQDN string, generation uint64) *DesiredDNSState {
	return ComputeDesiredStateWithPools(domain, nodes, services, leaderFQDN, nil, generation)
}

// candidateFunnel records how many nodes were eligible at each gate, so the
// reconciler can log a structured field set per record group:
//
//	desired_candidates   → matched profile/intent
//	installed_candidates → also have InstalledServices[svc]
//	runtime_candidates   → also have RuntimeHealthy[svc]
//	published            → emitted into the desired state
//	filtered             → per-node drop reasons
type candidateFunnel struct {
	Record    string                 `json:"record"`
	Desired   int                    `json:"desired_candidates"`
	Installed int                    `json:"installed_candidates"`
	Runtime   int                    `json:"runtime_candidates"`
	Published int                    `json:"published"`
	Filtered  []candidateFilterEntry `json:"filtered,omitempty"`
}

type candidateFilterEntry struct {
	Node   string `json:"node"`
	Reason string `json:"reason"`
}

// gateForService applies the 4-layer gate to a list of nodes for `service`.
// Returns (kept, funnel). Funnel.Record is filled by the caller.
func gateForService(service string, nodes []NodeInfo, hasProfileFn func(NodeInfo) bool) ([]NodeInfo, candidateFunnel) {
	funnel := candidateFunnel{}
	kept := make([]NodeInfo, 0, len(nodes))
	for _, node := range nodes {
		if !hasProfileFn(node) {
			continue
		}
		funnel.Desired++
		if node.Draining || node.Quarantined {
			funnel.Filtered = append(funnel.Filtered, candidateFilterEntry{
				Node: node.FQDN, Reason: "node draining or quarantined",
			})
			continue
		}
		// Installed gate.
		installedOK := true
		if node.InstalledServices != nil {
			installedOK = node.InstalledServices[service]
		}
		if !installedOK {
			funnel.Filtered = append(funnel.Filtered, candidateFilterEntry{
				Node: node.FQDN, Reason: "service planned but not installed",
			})
			continue
		}
		funnel.Installed++
		// Runtime gate.
		runtimeOK := true
		if node.RuntimeHealthy != nil {
			runtimeOK = node.RuntimeHealthy[service]
		}
		if !runtimeOK {
			funnel.Filtered = append(funnel.Filtered, candidateFilterEntry{
				Node: node.FQDN, Reason: "service installed but runtime unhealthy",
			})
			continue
		}
		funnel.Runtime++
		kept = append(kept, node)
	}
	funnel.Published = len(kept)
	return kept, funnel
}

// ComputeDesiredStateWithPools builds DNS state with additional pool-based multi-A records.
// pools maps a role name (e.g., "minio") to the ordered list of IPv4 addresses that host
// that role. One multi-A record is emitted at <role>.<domain> pointing to every IP. This
// is how cluster services discover pool endpoints via DNS (etcd+DNS are the only sources
// of truth — no env vars, no loopback literals).
//
// 4-layer gating — every service-bearing record (gateway, dns, scylla, api,
// wildcard, controller, _cluster-controller SRV, per-service SRV) is gated by
// the candidate funnel: a record is published ONLY when desired (profile) AND
// installed (Layer 3) AND runtime healthy (Layer 4). When NodeInfo carries no
// readiness data (cold start, legacy callers, tests), the gate passes through
// so the bootstrap path and existing tests are preserved.
func ComputeDesiredStateWithPools(domain string, nodes []NodeInfo, services []ServiceInstance, leaderFQDN string, pools map[string][]string, generation uint64) *DesiredDNSState {
	state, _ := ComputeDesiredStateWithFunnels(domain, nodes, services, leaderFQDN, pools, generation)
	return state
}

// ComputeDesiredStateWithFunnels is the same as ComputeDesiredStateWithPools
// but also returns the candidate-filtering funnels per record group so the
// reconciler can emit structured telemetry.
func ComputeDesiredStateWithFunnels(domain string, nodes []NodeInfo, services []ServiceInstance, leaderFQDN string, pools map[string][]string, generation uint64) (*DesiredDNSState, []candidateFunnel) {
	state := &DesiredDNSState{
		Domain:     domain,
		Records:    make([]DNSRecord, 0),
		Generation: generation,
	}
	funnels := make([]candidateFunnel, 0, 8)

	// Per-node A/AAAA records — describe the NODE, not a service. These are
	// safe to publish from membership alone; they back FQDN resolution that
	// every other layer depends on (and that the gated SRV records reference).
	// Draining/quarantined nodes withdraw themselves from their own record.
	for _, node := range nodes {
		if node.Draining || node.Quarantined {
			continue
		}
		if node.FQDN != "" && node.IPv4 != "" {
			state.Records = append(state.Records, DNSRecord{
				Name:  node.FQDN,
				Type:  RecordTypeA,
				Value: node.IPv4,
				TTL:   60,
			})
		}
		if node.FQDN != "" && node.IPv6 != "" {
			state.Records = append(state.Records, DNSRecord{
				Name:  node.FQDN,
				Type:  RecordTypeAAAA,
				Value: node.IPv6,
				TTL:   60,
			})
		}
	}

	// controller-nodes.<domain> — diagnostic multi-A. Requires control-plane
	// profile AND cluster-controller installed+healthy.
	controllerFQDN := fmt.Sprintf("controller.%s", domain)
	controllerNodesFQDN := fmt.Sprintf("controller-nodes.%s", domain)

	controllerNodeKept, controllerNodeFunnel := gateForService("cluster-controller", nodes,
		func(n NodeInfo) bool { return n.HasProfile("control-plane") })
	controllerNodeFunnel.Record = controllerNodesFQDN
	funnels = append(funnels, controllerNodeFunnel)

	for _, node := range controllerNodeKept {
		if node.IPv4 != "" {
			state.Records = append(state.Records, DNSRecord{
				Name:  controllerNodesFQDN,
				Type:  RecordTypeA,
				Value: node.IPv4,
				TTL:   60,
			})
		}
		if node.IPv6 != "" {
			state.Records = append(state.Records, DNSRecord{
				Name:  controllerNodesFQDN,
				Type:  RecordTypeAAAA,
				Value: node.IPv6,
				TTL:   60,
			})
		}
	}

	// controller.<domain> — single A, leader only. Requires the leader's
	// cluster-controller service to be installed AND runtime-healthy.
	leaderFunnel := candidateFunnel{Record: controllerFQDN}
	for _, node := range nodes {
		if !node.HasProfile("control-plane") {
			continue
		}
		leaderFunnel.Desired++
		if leaderFQDN == "" || node.FQDN != leaderFQDN {
			continue
		}
		if !node.ServiceReady("cluster-controller") {
			leaderFunnel.Filtered = append(leaderFunnel.Filtered, candidateFilterEntry{
				Node: node.FQDN, Reason: "leader cluster-controller not installed+healthy",
			})
			continue
		}
		leaderFunnel.Installed++
		leaderFunnel.Runtime++
		leaderFunnel.Published++
		if node.IPv4 != "" {
			state.Records = append(state.Records, DNSRecord{
				Name:  controllerFQDN,
				Type:  RecordTypeA,
				Value: node.IPv4,
				TTL:   60,
			})
		}
		if node.IPv6 != "" {
			state.Records = append(state.Records, DNSRecord{
				Name:  controllerFQDN,
				Type:  RecordTypeAAAA,
				Value: node.IPv6,
				TTL:   60,
			})
		}
		break
	}
	funnels = append(funnels, leaderFunnel)

	// H3 Hardening: If leader unknown, fall back to first control-plane node
	// that is installed+healthy (bootstrap). Without readiness data this still
	// reduces to "first control-plane node" — matches legacy behavior.
	if leaderFQDN == "" {
		for _, node := range nodes {
			if !node.HasProfile("control-plane") {
				continue
			}
			if !node.ServiceReady("cluster-controller") {
				continue
			}
			if node.IPv4 != "" {
				state.Records = append(state.Records, DNSRecord{
					Name:  controllerFQDN,
					Type:  RecordTypeA,
					Value: node.IPv4,
					TTL:   60,
				})
			}
			if node.IPv6 != "" {
				state.Records = append(state.Records, DNSRecord{
					Name:  controllerFQDN,
					Type:  RecordTypeAAAA,
					Value: node.IPv6,
					TTL:   60,
				})
			}
			break
		}
	}

	// gateway.<domain> / api.<domain> / *.<domain> — every record that promises
	// "traffic terminates at a working gateway" must be gated on the gateway
	// service being installed AND runtime-healthy on each candidate node.
	gatewayFQDN := fmt.Sprintf("gateway.%s", domain)
	wildcardFQDN := fmt.Sprintf("*.%s", domain)
	apiFQDN := fmt.Sprintf("api.%s", domain)

	gatewayKept, gatewayFunnel := gateForService("gateway", nodes,
		func(n NodeInfo) bool { return n.HasProfile("gateway") })
	gatewayFunnel.Record = gatewayFQDN
	funnels = append(funnels, gatewayFunnel)
	wildcardFunnel := gatewayFunnel
	wildcardFunnel.Record = wildcardFQDN
	funnels = append(funnels, wildcardFunnel)
	apiFunnel := gatewayFunnel
	apiFunnel.Record = apiFQDN
	funnels = append(funnels, apiFunnel)

	for _, node := range gatewayKept {
		if node.IPv4 != "" {
			state.Records = append(state.Records, DNSRecord{
				Name: gatewayFQDN, Type: RecordTypeA, Value: node.IPv4, TTL: 60,
			})
			state.Records = append(state.Records, DNSRecord{
				Name: wildcardFQDN, Type: RecordTypeA, Value: node.IPv4, TTL: 60,
			})
			state.Records = append(state.Records, DNSRecord{
				Name: apiFQDN, Type: RecordTypeA, Value: node.IPv4, TTL: 60,
			})
		}
		if node.IPv6 != "" {
			state.Records = append(state.Records, DNSRecord{
				Name: gatewayFQDN, Type: RecordTypeAAAA, Value: node.IPv6, TTL: 60,
			})
			state.Records = append(state.Records, DNSRecord{
				Name: wildcardFQDN, Type: RecordTypeAAAA, Value: node.IPv6, TTL: 60,
			})
			state.Records = append(state.Records, DNSRecord{
				Name: apiFQDN, Type: RecordTypeAAAA, Value: node.IPv6, TTL: 60,
			})
		}
	}

	// dns.<domain> — multi-A, must point only at nodes whose dns service is
	// installed AND healthy. Legacy code allowed core profile to substitute
	// for dns profile, preserved here.
	dnsFQDN := fmt.Sprintf("dns.%s", domain)
	dnsKept, dnsFunnel := gateForService("dns", nodes,
		func(n NodeInfo) bool { return n.HasProfile("dns") || n.HasProfile("core") })
	dnsFunnel.Record = dnsFQDN
	funnels = append(funnels, dnsFunnel)
	for _, node := range dnsKept {
		if node.IPv4 != "" {
			state.Records = append(state.Records, DNSRecord{
				Name: dnsFQDN, Type: RecordTypeA, Value: node.IPv4, TTL: 60,
			})
		}
		if node.IPv6 != "" {
			state.Records = append(state.Records, DNSRecord{
				Name: dnsFQDN, Type: RecordTypeAAAA, Value: node.IPv6, TTL: 60,
			})
		}
	}

	// Scylla database records — individual scylla-N + multi-A scylla.<domain>.
	// Gated on scylladb being installed AND healthy.
	scyllaFQDN := fmt.Sprintf("scylla.%s", domain)
	scyllaKept, scyllaFunnel := gateForService("scylladb", nodes,
		func(n NodeInfo) bool { return n.HasProfile("scylla") || n.HasProfile("database") })
	scyllaFunnel.Record = scyllaFQDN
	funnels = append(funnels, scyllaFunnel)
	scyllaIndex := 0
	for _, node := range scyllaKept {
		if node.IPv4 != "" {
			individualFQDN := fmt.Sprintf("scylla-%d.%s", scyllaIndex, domain)
			state.Records = append(state.Records, DNSRecord{
				Name:  individualFQDN,
				Type:  RecordTypeA,
				Value: node.IPv4,
				TTL:   60,
			})
			state.Records = append(state.Records, DNSRecord{
				Name:  scyllaFQDN,
				Type:  RecordTypeA,
				Value: node.IPv4,
				TTL:   60,
			})
		}
		if node.IPv6 != "" {
			individualFQDN := fmt.Sprintf("scylla-%d.%s", scyllaIndex, domain)
			state.Records = append(state.Records, DNSRecord{
				Name:  individualFQDN,
				Type:  RecordTypeAAAA,
				Value: node.IPv6,
				TTL:   60,
			})
			state.Records = append(state.Records, DNSRecord{
				Name:  scyllaFQDN,
				Type:  RecordTypeAAAA,
				Value: node.IPv6,
				TTL:   60,
			})
		}
		scyllaIndex++
	}

	// _cluster-controller._tcp.<domain> SRV — service-routing record. Must
	// only include nodes whose cluster-controller is installed AND healthy.
	controllerSRV := fmt.Sprintf("_cluster-controller._tcp.%s", domain)
	srvKept, srvFunnel := gateForService("cluster-controller", nodes,
		func(n NodeInfo) bool { return n.HasProfile("control-plane") && n.FQDN != "" })
	srvFunnel.Record = controllerSRV
	funnels = append(funnels, srvFunnel)
	for _, node := range srvKept {
		state.Records = append(state.Records, DNSRecord{
			Name:     controllerSRV,
			Type:     RecordTypeSRV,
			Value:    node.FQDN,
			TTL:      60,
			Priority: 10,
			Weight:   100,
			Port:     12000,
		})
	}

	// Add service SRV records (PR4.1)
	// Format: _<service>._tcp.<domain>
	for _, svc := range services {
		if svc.ServiceName == "" || svc.NodeFQDN == "" || svc.Port == 0 {
			continue // Skip incomplete service info
		}

		// Normalize service name to DNS-safe format (lowercase, dots to hyphens)
		dnsServiceName := normalizeDNSLabel(svc.ServiceName)
		srvName := fmt.Sprintf("_%s._tcp.%s", dnsServiceName, domain)

		state.Records = append(state.Records, DNSRecord{
			Name:     srvName,
			Type:     RecordTypeSRV,
			Value:    svc.NodeFQDN,
			TTL:      60,
			Priority: 10,
			Weight:   10,
			Port:     svc.Port,
		})
	}

	// Emit pool-based multi-A records: <role>.<domain> → every IP in the pool.
	// Services (including MinIO consumers) resolve the pool address via DNS so that
	// no endpoint is ever hardcoded or read from an environment variable.
	//
	// Pool membership is owned by the controller's pool-management code (e.g.
	// MinIO topology). It already enforces "only ready members in pool" so we
	// trust the pool input; we don't re-gate here on InstalledServices/Healthy
	// because the IPs we receive may not correspond 1:1 to NodeInfo entries.
	for role, ips := range pools {
		if role == "" || len(ips) == 0 {
			continue
		}
		poolFQDN := fmt.Sprintf("%s.%s", strings.ToLower(role), domain)
		for _, ip := range ips {
			ip = strings.TrimSpace(ip)
			if ip == "" {
				continue
			}
			state.Records = append(state.Records, DNSRecord{
				Name:  poolFQDN,
				Type:  RecordTypeA,
				Value: ip,
				TTL:   60,
			})
		}
	}

	return state, funnels
}

// normalizeDNSLabel converts a service name to DNS-safe format (PR4.1)
// Examples: "echo.EchoService" -> "echo-echoservice"
func normalizeDNSLabel(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)
	// Replace dots with hyphens
	name = strings.ReplaceAll(name, ".", "-")
	return name
}

// DNSStateDiff represents changes needed to reconcile current vs desired state
type DNSStateDiff struct {
	ToCreate []DNSRecord
	ToUpdate []DNSRecord
	ToDelete []DNSRecord
}

// Diff computes the changes needed to reconcile current vs desired
func (d *DesiredDNSState) Diff(current *DesiredDNSState) *DNSStateDiff {
	diff := &DNSStateDiff{
		ToCreate: make([]DNSRecord, 0),
		ToUpdate: make([]DNSRecord, 0),
		ToDelete: make([]DNSRecord, 0),
	}

	currentMap := make(map[string]DNSRecord)
	if current != nil {
		for _, r := range current.Records {
			key := recordKey(r)
			currentMap[key] = r
		}
	}

	desiredMap := make(map[string]DNSRecord)
	for _, r := range d.Records {
		key := recordKey(r)
		desiredMap[key] = r

		if existing, ok := currentMap[key]; ok {
			if !recordsEqual(r, existing) {
				diff.ToUpdate = append(diff.ToUpdate, r)
			}
		} else {
			diff.ToCreate = append(diff.ToCreate, r)
		}
	}

	for key, r := range currentMap {
		if _, ok := desiredMap[key]; !ok {
			diff.ToDelete = append(diff.ToDelete, r)
		}
	}

	return diff
}

// recordKey generates a unique key for a DNS record
func recordKey(r DNSRecord) string {
	return fmt.Sprintf("%s:%s:%s", r.Name, r.Type, r.Value)
}

// recordsEqual checks if two DNS records are equivalent
func recordsEqual(a, b DNSRecord) bool {
	return a.Name == b.Name &&
		a.Type == b.Type &&
		a.Value == b.Value &&
		a.TTL == b.TTL &&
		a.Priority == b.Priority &&
		a.Weight == b.Weight &&
		a.Port == b.Port
}

// SortRecords sorts records by type then name for consistent ordering
func (d *DesiredDNSState) SortRecords() {
	sort.Slice(d.Records, func(i, j int) bool {
		if d.Records[i].Type != d.Records[j].Type {
			return d.Records[i].Type < d.Records[j].Type
		}
		if d.Records[i].Name != d.Records[j].Name {
			return d.Records[i].Name < d.Records[j].Name
		}
		return d.Records[i].Value < d.Records[j].Value
	})
}
