package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/clustercontroller/clustercontroller_server/internal/dnsprovider"
	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/config"
	dns_client "github.com/globulario/services/golang/dns/dns_client"
	Utility "github.com/globulario/utility"
)

const (
	dnsReconcileInterval = 30 * time.Second
	dnsReconcileTimeout  = 10 * time.Second
	dnsHealthCheckInterval = 30 * time.Second // PR7: Check DNS health every 30s
)

// DNSReconciler continuously syncs cluster state to DNS service(s) (PR7: multi-DNS)
type DNSReconciler struct {
	srv            *server
	dnsEndpoints   []string             // PR7: Support multiple DNS endpoints
	healthStatus   map[string]bool      // PR7: Track health of each DNS endpoint
	externalDNS    dnsprovider.Provider // PR8: External DNS provider
	lastGeneration uint64
	stopCh         chan struct{}
	healthStopCh   chan struct{} // PR7: Stop health checks

	// Metrics (PR10)
	reconcileTotal   uint64 // Total reconciliations attempted
	reconcileSuccess uint64 // Successful reconciliations
	reconcileFailure uint64 // Failed reconciliations
	lastReconcileAt  int64  // Unix timestamp of last reconciliation
	lastReconcileDur int64  // Duration of last reconciliation (nanoseconds)
}

// NewDNSReconciler creates a new DNS reconciler
// PR7: Now accepts multiple DNS endpoints for high availability
// Day-0 Security: Uses dynamic discovery instead of hardcoded 10033
func NewDNSReconciler(srv *server, dnsEndpoints []string) *DNSReconciler {
	if len(dnsEndpoints) == 0 {
		// Use dynamic discovery to find DNS service endpoint
		discovered := config.ResolveDNSGrpcEndpoint("127.0.0.1:10033")
		dnsEndpoints = []string{discovered}
		log.Printf("DNS reconciler: discovered DNS endpoint: %s", discovered)
	}

	healthStatus := make(map[string]bool)
	for _, endpoint := range dnsEndpoints {
		healthStatus[endpoint] = true // Assume healthy initially
	}

	return &DNSReconciler{
		srv:          srv,
		dnsEndpoints: dnsEndpoints,
		healthStatus: healthStatus,
		stopCh:       make(chan struct{}),
		healthStopCh: make(chan struct{}),
	}
}

// Start begins the reconciliation loop and health checks (PR7)
func (r *DNSReconciler) Start() {
	go r.reconcileLoop()
	go r.healthCheckLoop() // PR7: Start health monitoring
}

// Stop halts the reconciliation loop and health checks (PR7)
func (r *DNSReconciler) Stop() {
	close(r.stopCh)
	close(r.healthStopCh) // PR7: Stop health checks
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
	startTime := time.Now()
	atomic.AddUint64(&r.reconcileTotal, 1)

	defer func() {
		duration := time.Since(startTime)
		atomic.StoreInt64(&r.lastReconcileDur, int64(duration))
		atomic.StoreInt64(&r.lastReconcileAt, time.Now().Unix())
	}()

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

	// PR8: Configure external DNS provider (may have changed in spec)
	if err := r.ConfigureExternalDNS(spec); err != nil {
		log.Printf("dns reconciler: WARN - failed to configure external DNS: %v", err)
		// Continue with internal DNS reconciliation
	}

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

	// PR9: Check for domain migration
	domains := []string{spec.ClusterDomain}
	if r.isInMigration(spec) {
		// During migration, publish to both old and new domains
		domains = []string{spec.DomainMigration.OldDomain, spec.DomainMigration.NewDomain}
		log.Printf("dns reconciler: domain migration IN PROGRESS - publishing to both %s and %s",
			spec.DomainMigration.OldDomain, spec.DomainMigration.NewDomain)
	} else if r.shouldCleanupOldDomain(spec) {
		// Grace period expired, clean up old domain
		if err := r.cleanupOldDomain(spec.DomainMigration.OldDomain); err != nil {
			log.Printf("dns reconciler: WARN - failed to cleanup old domain %s: %v",
				spec.DomainMigration.OldDomain, err)
		}
	}

	// Apply desired state to DNS service for each domain
	ctx, cancel := context.WithTimeout(context.Background(), dnsReconcileTimeout)
	defer cancel()

	for _, domain := range domains {
		desired := ComputeDesiredStateWithServices(domain, nodeInfos, serviceInstances, generation)
		if err := r.applyDNSState(ctx, desired); err != nil {
			atomic.AddUint64(&r.reconcileFailure, 1)
			return fmt.Errorf("apply dns state for domain %s: %w", domain, err)
		}
	}

	// PR8: Publish to external DNS if enabled
	if err := r.publishExternalDNS(ctx, spec); err != nil {
		log.Printf("dns reconciler: WARN - external dns publish failed: %v", err)
		// Don't fail reconciliation on external DNS error
	}

	r.lastGeneration = generation
	atomic.AddUint64(&r.reconcileSuccess, 1)
	log.Printf("dns reconciler: SUCCESS - applied generation %d to %d domain(s)", generation, len(domains))
	return nil
}

// applyDNSState applies the desired DNS state to all healthy DNS services (PR7: fanout)
func (r *DNSReconciler) applyDNSState(ctx context.Context, desired *DesiredDNSState) error {
	healthyEndpoints := r.getHealthyEndpoints()
	if len(healthyEndpoints) == 0 {
		return fmt.Errorf("no healthy DNS endpoints available")
	}

	log.Printf("dns reconciler: applying to %d DNS endpoints (healthy: %d/%d)",
		len(healthyEndpoints), len(healthyEndpoints), len(r.dnsEndpoints))

	// PR7: Fan out to all healthy DNS instances
	var lastErr error
	successCount := 0

	for _, endpoint := range healthyEndpoints {
		if err := r.applyToDNSInstance(ctx, endpoint, desired); err != nil {
			log.Printf("dns reconciler: WARN - failed to apply to %s: %v", endpoint, err)
			lastErr = err
			// Mark endpoint as unhealthy
			r.healthStatus[endpoint] = false
		} else {
			successCount++
		}
	}

	if successCount == 0 {
		return fmt.Errorf("failed to apply to any DNS instance: %w", lastErr)
	}

	if successCount < len(healthyEndpoints) {
		log.Printf("dns reconciler: partial success (%d/%d instances updated)", successCount, len(healthyEndpoints))
	}

	return nil
}

// applyToDNSInstance applies state to a single DNS instance (PR7)
func (r *DNSReconciler) applyToDNSInstance(ctx context.Context, endpoint string, desired *DesiredDNSState) error {
	client, err := dns_client.NewDnsService_Client(endpoint, "dns.DnsService")
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
	recordCount := map[RecordType]int{}
	for _, rec := range desired.Records {
		switch rec.Type {
		case RecordTypeA:
			if _, err := client.SetA(token, rec.Name, rec.Value, rec.TTL); err != nil {
				log.Printf("dns reconciler: WARN - failed to set A record %s -> %s on %s: %v", rec.Name, rec.Value, endpoint, err)
			} else {
				recordCount[RecordTypeA]++
			}
		case RecordTypeAAAA:
			if _, err := client.SetAAAA(token, rec.Name, rec.Value, rec.TTL); err != nil {
				log.Printf("dns reconciler: WARN - failed to set AAAA record %s -> %s on %s: %v", rec.Name, rec.Value, endpoint, err)
			} else {
				recordCount[RecordTypeAAAA]++
			}
		case RecordTypeSRV:
			// PR4.1: Create SRV record
			if err := r.setSRVRecord(client, token, rec); err != nil {
				log.Printf("dns reconciler: WARN - failed to set SRV record %s -> %s:%d on %s: %v", rec.Name, rec.Value, rec.Port, endpoint, err)
			} else {
				recordCount[RecordTypeSRV]++
			}
		}
	}
	log.Printf("dns reconciler: applied to %s: A=%d, AAAA=%d, SRV=%d records",
		endpoint, recordCount[RecordTypeA], recordCount[RecordTypeAAAA], recordCount[RecordTypeSRV])

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

// healthCheckLoop periodically checks DNS endpoint health (PR7)
func (r *DNSReconciler) healthCheckLoop() {
	ticker := time.NewTicker(dnsHealthCheckInterval)
	defer ticker.Stop()

	log.Printf("dns reconciler: starting health check loop (interval=%v)", dnsHealthCheckInterval)

	for {
		select {
		case <-ticker.C:
			r.checkAllEndpoints()
		case <-r.healthStopCh:
			log.Printf("dns reconciler: health check stopped")
			return
		}
	}
}

// checkAllEndpoints checks health of all DNS endpoints (PR7)
func (r *DNSReconciler) checkAllEndpoints() {
	for _, endpoint := range r.dnsEndpoints {
		healthy := r.checkEndpointHealth(endpoint)

		// Update health status and log changes
		previousHealth := r.healthStatus[endpoint]
		r.healthStatus[endpoint] = healthy

		if previousHealth != healthy {
			if healthy {
				log.Printf("dns reconciler: endpoint %s is now HEALTHY", endpoint)
			} else {
				log.Printf("dns reconciler: endpoint %s is now UNHEALTHY", endpoint)
			}
		}
	}
}

// checkEndpointHealth checks if a single DNS endpoint is healthy (PR7)
func (r *DNSReconciler) checkEndpointHealth(endpoint string) bool {
	// Try to connect and perform a simple operation
	client, err := dns_client.NewDnsService_Client(endpoint, "dns.DnsService")
	if err != nil {
		return false
	}
	defer client.Close()

	// Try to get domains as a health check
	_, err = client.GetDomains()
	return err == nil
}

// getHealthyEndpoints returns a list of currently healthy DNS endpoints (PR7)
func (r *DNSReconciler) getHealthyEndpoints() []string {
	healthy := make([]string, 0, len(r.dnsEndpoints))

	for _, endpoint := range r.dnsEndpoints {
		if r.healthStatus[endpoint] {
			healthy = append(healthy, endpoint)
		}
	}

	return healthy
}

// ConfigureExternalDNS sets up external DNS provider from cluster network spec (PR8)
func (r *DNSReconciler) ConfigureExternalDNS(spec *clustercontrollerpb.ClusterNetworkSpec) error {
	// Close existing provider if any
	if r.externalDNS != nil {
		r.externalDNS.Close()
		r.externalDNS = nil
	}

	if spec == nil || spec.ExternalDns == nil || !spec.ExternalDns.Enabled {
		log.Printf("external dns: DISABLED")
		return nil
	}

	cfg := spec.ExternalDns
	providerCfg := dnsprovider.Config{
		Provider:        cfg.Provider,
		Domain:          cfg.Domain,
		TTL:             int(cfg.Ttl),
		AllowPrivateIPs: cfg.AllowPrivateIps,
		ProviderConfig:  cfg.ProviderConfig,
	}

	provider, err := dnsprovider.New(providerCfg)
	if err != nil {
		return fmt.Errorf("create external dns provider: %w", err)
	}

	r.externalDNS = provider
	log.Printf("external dns: ENABLED (provider=%s, domain=%s, publish=%v)", cfg.Provider, cfg.Domain, cfg.Publish)
	return nil
}

// publishExternalDNS publishes selected records to external DNS (PR8)
func (r *DNSReconciler) publishExternalDNS(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
	if r.externalDNS == nil || spec.ExternalDns == nil || !spec.ExternalDns.Enabled {
		return nil // External DNS not enabled
	}

	cfg := spec.ExternalDns
	if len(cfg.Publish) == 0 {
		return nil // Nothing to publish
	}

	r.srv.mu.Lock()
	nodesMap := r.srv.state.Nodes
	r.srv.mu.Unlock()

	ttl := int(cfg.Ttl)
	if ttl <= 0 {
		ttl = 300 // Default 5 minutes
	}

	// Publish each requested endpoint
	for _, endpoint := range cfg.Publish {
		switch endpoint {
		case "gateway":
			if err := r.publishGateway(ctx, cfg.Domain, nodesMap, ttl); err != nil {
				log.Printf("external dns: WARN - failed to publish gateway: %v", err)
			}
		case "controller":
			if err := r.publishController(ctx, cfg.Domain, nodesMap, ttl); err != nil {
				log.Printf("external dns: WARN - failed to publish controller: %v", err)
			}
		default:
			log.Printf("external dns: WARN - unknown endpoint type: %s", endpoint)
		}
	}

	return nil
}

// publishGateway publishes gateway.<domain> record (PR8)
func (r *DNSReconciler) publishGateway(ctx context.Context, domain string, nodesMap map[string]*nodeState, ttl int) error {
	// Find nodes with gateway profile
	var ips []net.IP
	for _, node := range nodesMap {
		if node == nil {
			continue
		}
		hasGateway := false
		for _, profile := range node.Profiles {
			if profile == "gateway" {
				hasGateway = true
				break
			}
		}
		if hasGateway && len(node.Identity.Ips) > 0 {
			ip := net.ParseIP(node.Identity.Ips[0])
			if ip != nil {
				ips = append(ips, ip)
			}
		}
	}

	if len(ips) == 0 {
		log.Printf("external dns: no gateway nodes found")
		return nil
	}

	name := fmt.Sprintf("gateway.%s", domain)
	if err := r.externalDNS.UpsertA(ctx, name, ips, ttl); err != nil {
		return fmt.Errorf("upsert gateway A record: %w", err)
	}

	log.Printf("external dns: published %s -> %v", name, ips)
	return nil
}

// publishController publishes controller.<domain> record (PR8)
func (r *DNSReconciler) publishController(ctx context.Context, domain string, nodesMap map[string]*nodeState, ttl int) error {
	// Find nodes with controller profile
	var ips []net.IP
	for _, node := range nodesMap {
		if node == nil {
			continue
		}
		hasController := false
		for _, profile := range node.Profiles {
			if profile == "controller" {
				hasController = true
				break
			}
		}
		if hasController && len(node.Identity.Ips) > 0 {
			ip := net.ParseIP(node.Identity.Ips[0])
			if ip != nil {
				ips = append(ips, ip)
			}
		}
	}

	if len(ips) == 0 {
		log.Printf("external dns: no controller nodes found")
		return nil
	}

	name := fmt.Sprintf("controller.%s", domain)
	if err := r.externalDNS.UpsertA(ctx, name, ips, ttl); err != nil {
		return fmt.Errorf("upsert controller A record: %w", err)
	}

	log.Printf("external dns: published %s -> %v", name, ips)
	return nil
}

// isInMigration checks if domain migration is currently in progress (PR9)
func (r *DNSReconciler) isInMigration(spec *clustercontrollerpb.ClusterNetworkSpec) bool {
	if spec == nil || spec.DomainMigration == nil {
		return false
	}
	return spec.DomainMigration.State == clustercontrollerpb.DomainMigration_MIGRATION_IN_PROGRESS
}

// shouldCleanupOldDomain checks if grace period has expired and old domain should be cleaned up (PR9)
func (r *DNSReconciler) shouldCleanupOldDomain(spec *clustercontrollerpb.ClusterNetworkSpec) bool {
	if spec == nil || spec.DomainMigration == nil {
		return false
	}

	migration := spec.DomainMigration
	if migration.State != clustercontrollerpb.DomainMigration_MIGRATION_IN_PROGRESS {
		return false
	}

	// Check if grace period has expired
	gracePeriod := migration.GracePeriodSeconds
	if gracePeriod == 0 {
		gracePeriod = 3600 // Default 1 hour
	}

	elapsed := time.Now().Unix() - migration.StartedAt
	return elapsed > int64(gracePeriod)
}

// cleanupOldDomain removes DNS records for the old domain (PR9)
func (r *DNSReconciler) cleanupOldDomain(oldDomain string) error {
	log.Printf("dns reconciler: cleaning up old domain %s (grace period expired)", oldDomain)

	// For now, just log - actual cleanup would require tracking which records to delete
	// In a full implementation, we would:
	// 1. List all records for old domain
	// 2. Delete them from DNS service
	// 3. Mark migration as completed

	log.Printf("dns reconciler: TODO - implement old domain cleanup for %s", oldDomain)
	return nil
}

// ReconcilerMetrics holds metrics about DNS reconciliation (PR10)
type ReconcilerMetrics struct {
	Total          uint64        // Total reconciliations attempted
	Success        uint64        // Successful reconciliations
	Failure        uint64        // Failed reconciliations
	LastAt         time.Time     // Last reconciliation timestamp
	LastDuration   time.Duration // Duration of last reconciliation
	CurrentGen     uint64        // Current generation
	EndpointsTotal int           // Total DNS endpoints
	EndpointsHealthy int         // Healthy DNS endpoints
}

// GetMetrics returns current reconciler metrics (PR10)
func (r *DNSReconciler) GetMetrics() ReconcilerMetrics {
	healthyCount := 0
	for _, healthy := range r.healthStatus {
		if healthy {
			healthyCount++
		}
	}

	return ReconcilerMetrics{
		Total:            atomic.LoadUint64(&r.reconcileTotal),
		Success:          atomic.LoadUint64(&r.reconcileSuccess),
		Failure:          atomic.LoadUint64(&r.reconcileFailure),
		LastAt:           time.Unix(atomic.LoadInt64(&r.lastReconcileAt), 0),
		LastDuration:     time.Duration(atomic.LoadInt64(&r.lastReconcileDur)),
		CurrentGen:       r.lastGeneration,
		EndpointsTotal:   len(r.dnsEndpoints),
		EndpointsHealthy: healthyCount,
	}
}
