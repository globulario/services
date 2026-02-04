package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	dns_client "github.com/globulario/services/golang/dns/dns_client"
	Utility "github.com/globulario/utility"
)

const (
	dnsReconcileInterval = 30 * time.Second
	dnsReconcileTimeout  = 10 * time.Second
)

// DNSReconciler continuously syncs cluster state to DNS service
type DNSReconciler struct {
	srv            *server
	dnsEndpoint    string
	lastGeneration uint64
	stopCh         chan struct{}
}

// NewDNSReconciler creates a new DNS reconciler
func NewDNSReconciler(srv *server, dnsEndpoint string) *DNSReconciler {
	if dnsEndpoint == "" {
		dnsEndpoint = "127.0.0.1:10033"
	}
	return &DNSReconciler{
		srv:         srv,
		dnsEndpoint: dnsEndpoint,
		stopCh:      make(chan struct{}),
	}
}

// Start begins the reconciliation loop
func (r *DNSReconciler) Start() {
	go r.reconcileLoop()
}

// Stop halts the reconciliation loop
func (r *DNSReconciler) Stop() {
	close(r.stopCh)
}

// reconcileLoop runs periodic reconciliation
func (r *DNSReconciler) reconcileLoop() {
	ticker := time.NewTicker(dnsReconcileInterval)
	defer ticker.Stop()

	log.Printf("dns reconciler: starting loop (interval=%v)", dnsReconcileInterval)

	// Initial reconciliation
	if err := r.reconcile(); err != nil {
		log.Printf("dns reconciler: initial reconciliation failed: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := r.reconcile(); err != nil {
				log.Printf("dns reconciler: reconciliation error: %v (will retry in %v)", err, dnsReconcileInterval)
			}
		case <-r.stopCh:
			log.Printf("dns reconciler: stopped")
			return
		}
	}
}

// reconcile performs a single reconciliation cycle
func (r *DNSReconciler) reconcile() error {
	r.srv.mu.Lock()
	spec := r.srv.state.ClusterNetworkSpec
	generation := r.srv.state.NetworkingGeneration
	nodes := r.srv.state.Nodes
	r.srv.mu.Unlock()

	if spec == nil || spec.ClusterDomain == "" {
		log.Printf("dns reconciler: skipping (no cluster network spec)")
		return nil // No DNS config yet
	}

	// Check if generation changed
	if generation == r.lastGeneration {
		// No log on unchanged generation (would spam logs every 30s)
		return nil // No changes
	}

	log.Printf("dns reconciler: generation changed %d -> %d, reconciling...", r.lastGeneration, generation)

	// Build desired DNS state from cluster state
	nodeInfos := make([]NodeInfo, 0, len(nodes))
	nodeByFQDN := make(map[string]string) // FQDN -> node FQDN for service routing
	for _, node := range nodes {
		info := NodeInfo{
			FQDN:     node.AdvertiseFqdn,
			Profiles: node.Profiles,
		}
		if len(node.Identity.Ips) > 0 {
			info.IPv4 = node.Identity.Ips[0]
		}
		nodeInfos = append(nodeInfos, info)
		if node.AdvertiseFqdn != "" {
			nodeByFQDN[node.AdvertiseFqdn] = node.AdvertiseFqdn
		}
	}

	// Fetch service instances from etcd (PR4.1)
	serviceInstances := r.fetchServiceInstances(spec.ClusterDomain, nodeByFQDN)
	if len(serviceInstances) > 0 {
		log.Printf("dns reconciler: discovered %d service instances for SRV records", len(serviceInstances))
	}

	desired := ComputeDesiredStateWithServices(spec.ClusterDomain, nodeInfos, serviceInstances, generation)

	// Apply desired state to DNS service
	ctx, cancel := context.WithTimeout(context.Background(), dnsReconcileTimeout)
	defer cancel()

	if err := r.applyDNSState(ctx, desired); err != nil {
		return fmt.Errorf("apply dns state: %w", err)
	}

	r.lastGeneration = generation
	log.Printf("dns reconciler: SUCCESS - applied generation %d (%d records for domain %s)", generation, len(desired.Records), spec.ClusterDomain)
	return nil
}

// applyDNSState applies the desired DNS state to the DNS service
func (r *DNSReconciler) applyDNSState(ctx context.Context, desired *DesiredDNSState) error {
	client, err := dns_client.NewDnsService_Client(r.dnsEndpoint, "dns.DnsService")
	if err != nil {
		return fmt.Errorf("connect dns: %w", err)
	}
	defer client.Close()

	// Generate DNS token (using cluster-controller identity)
	token, err := r.generateDNSToken(ctx, client, desired.Domain)
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}

	// Set domains
	if err := client.SetDomains(token, []string{desired.Domain}); err != nil {
		return fmt.Errorf("set domains: %w", err)
	}

	// Apply records
	log.Printf("dns reconciler: applying %d records to DNS service", len(desired.Records))
	recordCount := map[RecordType]int{}
	for _, rec := range desired.Records {
		switch rec.Type {
		case RecordTypeA:
			if _, err := client.SetA(token, rec.Name, rec.Value, rec.TTL); err != nil {
				log.Printf("dns reconciler: WARN - failed to set A record %s -> %s: %v", rec.Name, rec.Value, err)
			} else {
				recordCount[RecordTypeA]++
			}
		case RecordTypeAAAA:
			if _, err := client.SetAAAA(token, rec.Name, rec.Value, rec.TTL); err != nil {
				log.Printf("dns reconciler: WARN - failed to set AAAA record %s -> %s: %v", rec.Name, rec.Value, err)
			} else {
				recordCount[RecordTypeAAAA]++
			}
		case RecordTypeSRV:
			// PR4.1: Create SRV record
			// Format: SetSrv(token, name, priority, weight, port, target, ttl)
			if err := r.setSRVRecord(client, token, rec); err != nil {
				log.Printf("dns reconciler: WARN - failed to set SRV record %s -> %s:%d: %v", rec.Name, rec.Value, rec.Port, err)
			} else {
				recordCount[RecordTypeSRV]++
			}
		}
	}
	log.Printf("dns reconciler: applied A=%d, AAAA=%d, SRV=%d records", recordCount[RecordTypeA], recordCount[RecordTypeAAAA], recordCount[RecordTypeSRV])

	return nil
}

// generateDNSToken creates an authentication token for DNS service
func (r *DNSReconciler) generateDNSToken(ctx context.Context, client *dns_client.Dns_Client, domain string) (string, error) {
	// Use cluster-controller as the identity
	// This is a placeholder - actual implementation depends on security package
	return "cluster-controller-token", nil
}

// fetchServiceInstances retrieves service instances from etcd for SRV records (PR4.1)
func (r *DNSReconciler) fetchServiceInstances(clusterDomain string, nodeByFQDN map[string]string) []ServiceInstance {
	services, err := config.GetServicesConfigurations()
	if err != nil {
		log.Printf("dns reconciler: WARN - failed to fetch services for SRV: %v", err)
		return nil
	}

	instances := make([]ServiceInstance, 0)
	for _, svc := range services {
		// Extract service name
		name := strings.TrimSpace(Utility.ToString(svc["Name"]))
		if name == "" {
			continue
		}

		// Extract port
		port := Utility.ToInt(svc["Port"])
		if port == 0 {
			port = Utility.ToInt(svc["Proxy"])
		}
		if port == 0 || port > 65535 {
			continue // Skip services without valid ports
		}

		// Extract node FQDN from Address field
		// Expected formats:
		//   - "node-01.cluster.local:8080" (FQDN with port)
		//   - "node-01.cluster.local" (FQDN without port)
		//   - "10.0.1.101:8080" (IP - skip for SRV)
		addr := strings.TrimSpace(Utility.ToString(svc["Address"]))
		nodeFQDN := r.extractNodeFQDN(addr, clusterDomain, nodeByFQDN)
		if nodeFQDN == "" {
			continue // Skip services without proper FQDN
		}

		instances = append(instances, ServiceInstance{
			ServiceName: name,
			NodeFQDN:    nodeFQDN,
			Port:        uint16(port),
		})
	}

	return instances
}

// extractNodeFQDN extracts the node FQDN from a service address (PR4.1)
// Returns empty string if address is not a valid FQDN in the cluster domain
func (r *DNSReconciler) extractNodeFQDN(addr, clusterDomain string, nodeByFQDN map[string]string) string {
	if addr == "" {
		return ""
	}

	// Strip port if present
	host := addr
	if idx := strings.Index(addr, ":"); idx >= 0 {
		host = addr[:idx]
	}

	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}

	// Check if it's an IP address (skip for SRV - we only want FQDNs)
	if strings.Contains(host, ".") && !strings.Contains(host, clusterDomain) {
		// Looks like IP or external domain - skip
		return ""
	}

	// Check if host is a known node FQDN
	if _, ok := nodeByFQDN[host]; ok {
		return host
	}

	// Check if host ends with cluster domain
	if strings.HasSuffix(host, "."+clusterDomain) {
		return host
	}

	return ""
}

// setSRVRecord sets a DNS SRV record (PR4.1)
func (r *DNSReconciler) setSRVRecord(client *dns_client.Dns_Client, token string, rec DNSRecord) error {
	return client.SetSrv(token, rec.Name, uint32(rec.Priority), uint32(rec.Weight), uint32(rec.Port), rec.Value, rec.TTL)
}
