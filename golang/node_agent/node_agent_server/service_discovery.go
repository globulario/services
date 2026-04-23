package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/netutil"
)

// discoverServiceAddr returns the address for a gRPC service.
//
// When the service is running locally, it returns the node's cert-valid
// FQDN with the service port (e.g. globule-nuc.globular.internal:10002).
// This ensures TLS verification succeeds — the cert SAN covers the FQDN,
// NOT localhost.
//
// When the service is not local, it returns the gateway address for mesh routing.
//
// All discovery uses etcd (via config package) as the source of truth.
// No environment variables, no hardcoded fallbacks.
func discoverServiceAddr(defaultLocalPort int) string {
	// Fast check: is the service running locally on this port?
	if isLocalPortOpen(defaultLocalPort) {
		// Return FQDN-based address that matches the cert SAN.
		host := localCertHostname()
		return fmt.Sprintf("%s:%d", host, defaultLocalPort)
	}

	// Not local — route through the gateway.
	if gw := discoverGatewayAddr(); gw != "" {
		return gw
	}

	// Last resort — use FQDN, never localhost.
	host := localCertHostname()
	return fmt.Sprintf("%s:%d", host, defaultLocalPort)
}

// localCertHostname returns the FQDN that matches this node's service
// certificate SAN: <hostname>.<domain> (e.g. globule-nuc.globular.internal).
//
// Domain is read from etcd via config.GetDomain() — the source of truth.
func localCertHostname() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "localhost"
	}
	domain, err := config.GetDomain()
	if err != nil || domain == "" {
		domain = netutil.DefaultClusterDomain()
	}
	return hostname + "." + domain
}

// discoverGatewayAddr resolves remote routing endpoint from etcd only.
// Address and port must come from service registry (source of truth).
// No DNS or hardcoded port fallback.
func discoverGatewayAddr() string {
	// Preferred: explicit gateway service endpoint from etcd.
	if addr := config.ResolveServiceAddr("gateway.GatewayService", ""); addr != "" {
		return addr
	}

	// Compatibility fallback: controller endpoint from etcd.
	if addr := config.ResolveServiceAddr("cluster_controller.ClusterControllerService", ""); addr != "" {
		return addr
	}
	return ""
}

func hostFromEndpoint(ep string) string {
	host, _, err := net.SplitHostPort(ep)
	if err != nil {
		return ""
	}
	if host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return "" // loopback doesn't help for remote discovery
	}
	return host
}

func isLocalPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
