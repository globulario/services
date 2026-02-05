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

// NodeInfo contains node information for DNS record generation
type NodeInfo struct {
	FQDN     string
	IPv4     string
	IPv6     string
	Profiles []string
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

// ComputeDesiredState builds the desired DNS state from cluster state
func ComputeDesiredState(domain string, nodes []NodeInfo, generation uint64) *DesiredDNSState {
	return ComputeDesiredStateWithServices(domain, nodes, nil, generation)
}

// ComputeDesiredStateWithServices builds DNS state including service SRV records (PR4.1)
func ComputeDesiredStateWithServices(domain string, nodes []NodeInfo, services []ServiceInstance, generation uint64) *DesiredDNSState {
	state := &DesiredDNSState{
		Domain:     domain,
		Records:    make([]DNSRecord, 0),
		Generation: generation,
	}

	// Add node A/AAAA records
	for _, node := range nodes {
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

	// Add controller A/AAAA (points to all control-plane nodes) - PR3
	controllerFQDN := fmt.Sprintf("controller.%s", domain)
	for _, node := range nodes {
		if node.HasProfile("control-plane") && node.IPv4 != "" {
			state.Records = append(state.Records, DNSRecord{
				Name:  controllerFQDN,
				Type:  RecordTypeA,
				Value: node.IPv4,
				TTL:   60,
			})
		}
		if node.HasProfile("control-plane") && node.IPv6 != "" {
			state.Records = append(state.Records, DNSRecord{
				Name:  controllerFQDN,
				Type:  RecordTypeAAAA,
				Value: node.IPv6,
				TTL:   60,
			})
		}
	}

	// Add gateway A/AAAA (points to all gateway nodes)
	gatewayFQDN := fmt.Sprintf("gateway.%s", domain)
	for _, node := range nodes {
		if node.HasProfile("gateway") && node.IPv4 != "" {
			state.Records = append(state.Records, DNSRecord{
				Name:  gatewayFQDN,
				Type:  RecordTypeA,
				Value: node.IPv4,
				TTL:   60,
			})
		}
		if node.HasProfile("gateway") && node.IPv6 != "" {
			state.Records = append(state.Records, DNSRecord{
				Name:  gatewayFQDN,
				Type:  RecordTypeAAAA,
				Value: node.IPv6,
				TTL:   60,
			})
		}
	}

	// Add Scylla database records (Day-0 Security)
	// Individual scylla-N records for each node, plus scylla.domain multi-A
	scyllaFQDN := fmt.Sprintf("scylla.%s", domain)
	scyllaIndex := 0
	for _, node := range nodes {
		if node.HasProfile("scylla") || node.HasProfile("database") {
			// Individual node record: scylla-0.domain, scylla-1.domain, etc.
			if node.IPv4 != "" {
				individualFQDN := fmt.Sprintf("scylla-%d.%s", scyllaIndex, domain)
				state.Records = append(state.Records, DNSRecord{
					Name:  individualFQDN,
					Type:  RecordTypeA,
					Value: node.IPv4,
					TTL:   60,
				})
				// Multi-A record pointing to all scylla nodes
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
				// Multi-AAAA record pointing to all scylla nodes
				state.Records = append(state.Records, DNSRecord{
					Name:  scyllaFQDN,
					Type:  RecordTypeAAAA,
					Value: node.IPv6,
					TTL:   60,
				})
			}
			scyllaIndex++
		}
	}

	// Add cluster-controller SRV record
	controllerSRV := fmt.Sprintf("_cluster-controller._tcp.%s", domain)
	for _, node := range nodes {
		if node.HasProfile("control-plane") && node.FQDN != "" {
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

	return state
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
