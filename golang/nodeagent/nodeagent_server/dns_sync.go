package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	dns_client "github.com/globulario/services/golang/dns/dns_client"
	"github.com/globulario/services/golang/security"
)

const (
	defaultDnsEndpoint           = "127.0.0.1:10033"
	dnsDefaultTTL                = 60
	defaultSessionTimeoutMinutes = 15
	dnsConnectRetryAttempts      = 10
	dnsConnectRetrySleep         = time.Second
	envDNSIPv4                   = "GLOBULAR_DNS_IPv4"
	envDNSIPv6                   = "GLOBULAR_DNS_IPv6"
	envDNSIface                  = "GLOBULAR_DNS_IFACE"
	dnsInitConfigPath            = "/var/lib/globular/dns/dns_init.json"
)

func (srv *NodeAgentServer) syncDNS(spec *clustercontrollerpb.ClusterNetworkSpec) error {
	if spec == nil {
		return fmt.Errorf("cluster network spec is required")
	}
	domain := normalizeDomain(spec.GetClusterDomain())
	if domain == "" {
		return fmt.Errorf("cluster domain is required")
	}

	alternates := normalizeAltDomains(spec.GetAlternateDomains())
	domains := dedupeStrings(append([]string{domain}, alternates...))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, err := dialDNSWithRetry(ctx, resolveDNSEndpoint(spec))
	if err != nil {
		return err
	}
	defer client.Close()

	token, err := makeDNSToken(srv.nodeID, client, spec)
	if err != nil {
		return err
	}

	if err := client.SetDomains(token, domains); err != nil {
		return fmt.Errorf("set domains: %w", err)
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "node"
	}

	ipv4, ipv6 := selectDNSIPs()
	if ipv4 == "" {
		return fmt.Errorf("unable to determine node IPv4 address (set %s or %s)", envDNSIPv4, envDNSIface)
	}

	hostFQDN := fmt.Sprintf("%s.%s", hostname, domain)
	gateway := fmt.Sprintf("gateway.%s", domain)

	if _, err := client.SetA(token, gateway, ipv4, dnsDefaultTTL); err != nil {
		return fmt.Errorf("set A %s: %w", gateway, err)
	}
	if _, err := client.SetA(token, hostFQDN, ipv4, dnsDefaultTTL); err != nil {
		return fmt.Errorf("set A %s: %w", hostFQDN, err)
	}

	if ipv6 != "" {
		if _, err := client.SetAAAA(token, gateway, ipv6, dnsDefaultTTL); err != nil {
			return fmt.Errorf("set AAAA %s: %w", gateway, err)
		}
		if _, err := client.SetAAAA(token, hostFQDN, ipv6, dnsDefaultTTL); err != nil {
			return fmt.Errorf("set AAAA %s: %w", hostFQDN, err)
		}
	}

	// Apply DNS init config if it exists (SOA, NS, glue records)
	if err := applyDNSInitConfig(client, token); err != nil {
		// Log but don't fail - the basic DNS records are already set
		// This is non-critical for node operation
		fmt.Printf("nodeagent: dns init config: %v\n", err)
	}

	return nil
}

// dnsInitConfig represents the DNS initialization configuration rendered by cluster controller.
type dnsInitConfig struct {
	Domain      string       `json:"domain"`
	SOA         dnsSOAConfig `json:"soa"`
	NSRecords   []dnsNSConfig `json:"ns_records"`
	GlueRecords []dnsGlueConfig `json:"glue_records"`
	IsPrimary   bool         `json:"is_primary"`
}

type dnsSOAConfig struct {
	Domain  string `json:"domain"`
	NS      string `json:"ns"`
	Mbox    string `json:"mbox"`
	Serial  uint32 `json:"serial"`
	Refresh uint32 `json:"refresh"`
	Retry   uint32 `json:"retry"`
	Expire  uint32 `json:"expire"`
	Minttl  uint32 `json:"minttl"`
	TTL     uint32 `json:"ttl"`
}

type dnsNSConfig struct {
	NS  string `json:"ns"`
	TTL uint32 `json:"ttl"`
}

type dnsGlueConfig struct {
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
	TTL      uint32 `json:"ttl"`
}

// applyDNSInitConfig reads the DNS init config file and applies SOA, NS, and glue records.
// This is called after basic DNS sync to set up authoritative DNS records.
func applyDNSInitConfig(client *dns_client.Dns_Client, token string) error {
	data, err := os.ReadFile(dnsInitConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No init config - this is fine, node may not have DNS profile
			return nil
		}
		return fmt.Errorf("read dns init config: %w", err)
	}

	var config dnsInitConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse dns init config: %w", err)
	}

	if config.Domain == "" {
		return fmt.Errorf("dns init config: empty domain")
	}

	// Only the primary node should set SOA and NS records to avoid conflicts
	if !config.IsPrimary {
		// Non-primary nodes only set their own glue records
		for _, glue := range config.GlueRecords {
			if glue.Hostname != "" && glue.IP != "" {
				ttl := glue.TTL
				if ttl == 0 {
					ttl = 3600
				}
				if _, err := client.SetA(token, glue.Hostname, glue.IP, ttl); err != nil {
					return fmt.Errorf("set glue A %s: %w", glue.Hostname, err)
				}
			}
		}
		return nil
	}

	// Primary node sets SOA, NS, and all glue records
	soa := config.SOA
	if soa.NS != "" {
		ttl := soa.TTL
		if ttl == 0 {
			ttl = 3600
		}
		if err := client.SetSoa(token, config.Domain,
			soa.NS, soa.Mbox, soa.Serial, soa.Refresh, soa.Retry, soa.Expire, soa.Minttl, ttl); err != nil {
			return fmt.Errorf("set SOA %s: %w", config.Domain, err)
		}
	}

	// Set NS records
	for _, ns := range config.NSRecords {
		if ns.NS != "" {
			ttl := ns.TTL
			if ttl == 0 {
				ttl = 3600
			}
			if err := client.SetNs(token, config.Domain, ns.NS, ttl); err != nil {
				return fmt.Errorf("set NS %s: %w", ns.NS, err)
			}
		}
	}

	// Set glue records (A records for nameservers)
	for _, glue := range config.GlueRecords {
		if glue.Hostname != "" && glue.IP != "" {
			ttl := glue.TTL
			if ttl == 0 {
				ttl = 3600
			}
			if _, err := client.SetA(token, glue.Hostname, glue.IP, ttl); err != nil {
				return fmt.Errorf("set glue A %s: %w", glue.Hostname, err)
			}
		}
	}

	return nil
}

func dialDNSWithRetry(ctx context.Context, endpoint string) (*dns_client.Dns_Client, error) {
	var lastErr error
	backoff := 100 * time.Millisecond
	deadline, hasDeadline := ctx.Deadline()
	for attempt := 0; attempt < 8; attempt++ {
		client, err := dns_client.NewDnsService_Client(endpoint, "dns.DnsService")
		if err == nil {
			return client, nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("dns client (%s): %w", endpoint, ctx.Err())
		default:
		}

		sleep := backoff
		if hasDeadline {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return nil, fmt.Errorf("dns client (%s): deadline exceeded: %w", endpoint, ctx.Err())
			}
			if sleep > remaining {
				sleep = remaining
			}
		}
		timer := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("dns client (%s): %w", endpoint, ctx.Err())
		case <-timer.C:
		}
		if backoff < 500*time.Millisecond {
			backoff *= 2
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("unable to connect to dns at %s", endpoint)
	}
	return nil, fmt.Errorf("dns client (%s): %w", endpoint, lastErr)
}

func resolveDNSEndpoint(spec *clustercontrollerpb.ClusterNetworkSpec) string {
	if v := strings.TrimSpace(os.Getenv("GLOBULAR_DNS_ENDPOINT")); v != "" {
		return v
	}
	// spec override can be added later
	return defaultDnsEndpoint
}

func normalizeDomain(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "*.")
	s = strings.TrimSuffix(s, ".")
	return s
}

func normalizeAltDomains(domains []string) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, d := range domains {
		d = normalizeDomain(d)
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	return out
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func firstIPv4() string {
	ips := gatherIPs()
	if len(ips) == 0 {
		return ""
	}
	return ips[0]
}

func firstIPv6() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ipv6 := ip.To16(); ipv6 != nil && ipv6.To4() == nil {
				return ipv6.String()
			}
		}
	}
	return ""
}

func parseIPv4(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ""
}

func parseIPv6(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return ""
	}
	if ip.To16() == nil || ip.To4() != nil {
		return ""
	}
	return ip.String()
}

func ifaceIPv4(ifaceName string) string {
	ifaceName = strings.TrimSpace(ifaceName)
	if ifaceName == "" {
		return ""
	}
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return ""
	}
	if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
		return ""
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsLoopback() {
			continue
		}
		if v4 := ip.To4(); v4 != nil {
			if v4[0] == 169 && v4[1] == 254 { // skip link-local
				continue
			}
			return v4.String()
		}
	}
	return ""
}

func ifaceIPv6(ifaceName string) string {
	ifaceName = strings.TrimSpace(ifaceName)
	if ifaceName == "" {
		return ""
	}
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return ""
	}
	if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
		return ""
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsLoopback() {
			continue
		}
		if v6 := ip.To16(); v6 != nil && v6.To4() == nil {
			text := strings.ToLower(v6.String())
			if strings.HasPrefix(text, "fe80:") {
				continue
			}
			return v6.String()
		}
	}
	return ""
}

// selectDNSIPs picks IPv4/IPv6 for DNS records with override/env/interface options and fallbacks.
// On multi-NIC nodes, set GLOBULAR_DNS_IFACE=enp3s0 or explicit GLOBULAR_DNS_IPv4.
func selectDNSIPs() (string, string) {
	if v4 := parseIPv4(os.Getenv(envDNSIPv4)); v4 != "" {
		return v4, parseIPv6(os.Getenv(envDNSIPv6))
	}
	v6Override := parseIPv6(os.Getenv(envDNSIPv6))

	if ifn := strings.TrimSpace(os.Getenv(envDNSIface)); ifn != "" {
		v4 := ifaceIPv4(ifn)
		v6 := ifaceIPv6(ifn)
		if v4 != "" {
			if v6Override != "" {
				return v4, v6Override
			}
			return v4, v6
		}
	}

	v4 := firstIPv4()
	v6 := firstIPv6()
	if v6Override != "" {
		v6 = v6Override
	}
	return v4, v6
}

func dnsAdminEmail(spec *clustercontrollerpb.ClusterNetworkSpec) string {
	if spec == nil {
		return ""
	}
	email := strings.TrimSpace(spec.GetAdminEmail())
	if email == "" || !strings.Contains(email, "@") {
		domain := strings.TrimSpace(spec.GetClusterDomain())
		if domain == "" {
			return ""
		}
		return "admin@" + domain
	}
	return email
}

func makeDNSToken(nodeID string, client *dns_client.Dns_Client, spec *clustercontrollerpb.ClusterNetworkSpec) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("dns: spec required")
	}
	domain := normalizeDomain(spec.GetClusterDomain())
	if domain == "" {
		return "", fmt.Errorf("dns: empty cluster domain")
	}
	if envTk := strings.TrimSpace(os.Getenv("GLOBULAR_DNS_TOKEN")); envTk != "" {
		return envTk, nil
	}
	id := strings.TrimSpace(nodeID)
	if id == "" {
		id = strings.TrimSpace(os.Getenv("NODE_AGENT_NODE_ID"))
	}
	if id == "" && client != nil {
		id = strings.TrimSpace(client.GetMac())
	}
	if id == "" {
		return "", fmt.Errorf("dns: empty node identity for token")
	}
	adminEmail := dnsAdminEmail(spec)
	tk, err := security.GenerateToken(defaultSessionTimeoutMinutes, id, "sa", "", adminEmail, domain)
	if err != nil {
		return "", fmt.Errorf("dns: generate token: %w", err)
	}
	if strings.TrimSpace(tk) == "" {
		return "", fmt.Errorf("dns: generated empty token")
	}
	return tk, nil
}
