package main

import (
	"fmt"
	"net"
	"strings"
)

// resolveGRPCAddr normalises a CLI address parameter into a host:port string
// suitable for gRPC dialing.
//
// Accepted input forms:
//
//	ip:port                            (e.g. "10.0.0.8:12000")              → used verbatim
//	hostname:port                      (e.g. "globular.internal:443")        → used verbatim
//	node-fqdn:port                     (e.g. "globule-nuc.globular.internal:12000") → used verbatim
//	bare ip                            (e.g. "10.0.0.8")                    → :443 appended
//	cluster fqdn                       (e.g. "globular.internal")            → :443 appended
//	node fqdn                          (e.g. "globule-nuc.globular.internal") → :443 appended
//
// Port 443 is the Envoy service-mesh TLS ingress port. It is a standard
// protocol port and may be hardcoded per project rules (standard protocol
// ports are not service-config). Envoy routes by gRPC service path, so any
// service is reachable at host:443 without knowing its direct port.
//
// An explicit port always overrides the default, allowing direct connections
// that bypass the mesh (useful for bootstrap, join, and debug operations).
// isLoopbackHost reports whether host is a loopback address or hostname.
func isLoopbackHost(host string) bool {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func resolveGRPCAddr(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("address is empty")
	}

	// net.SplitHostPort succeeds when the input already contains a port
	// component (either "host:port" or "[ipv6]:port").
	host, port, err := net.SplitHostPort(input)
	if err == nil {
		if host == "" {
			return "", fmt.Errorf("address %q: missing host", input)
		}
		if isLoopbackHost(host) {
			return "", fmt.Errorf("address %q: loopback addresses are not valid cluster endpoints — use a routable IP or hostname", input)
		}
		return net.JoinHostPort(host, port), nil
	}

	// No port present. The input is a bare IP address or a bare hostname.
	if isLoopbackHost(input) {
		return "", fmt.Errorf("address %q: loopback addresses are not valid cluster endpoints — use a routable IP or hostname", input)
	}
	// Append port 443 (Envoy mesh ingress).
	return net.JoinHostPort(input, "443"), nil
}
