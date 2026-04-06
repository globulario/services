package config

import (
	"net"
	"strings"
)

// DialTarget is the single authoritative description of how to dial a
// gRPC endpoint across the cluster. It is produced by ResolveDialTarget
// and consumed directly by callers — no caller should re-implement any
// of the fields below.
//
// Why this exists:
//   - Every service used to carry its own ad-hoc host-rewriting rules
//     (127.0.0.1 ↔ localhost, SNI extraction, host:port parsing). Those
//     rewrites disagreed with each other and caused TLS verification
//     bugs (service certs cover DNS:localhost, not the loopback IP).
//   - Centralising the rules makes connectivity boring and predictable.
//
// See docs/endpoint_resolution_policy.md for the policy this implements.
type DialTarget struct {
	// Address is the gRPC dial target ("host:port"). It is always
	// TLS-dialable against the cluster CA: loopback IP literals have
	// been rewritten to "localhost" to match the service cert SAN.
	Address string

	// ServerName is the TLS ServerName / SNI value to verify the peer
	// certificate against. Callers MUST set this on their tls.Config
	// (or use grpc credentials that honour it). It is the host part
	// of Address, never an IP literal.
	ServerName string

	// WasLoopbackRewritten is true when the original endpoint used a
	// loopback IP literal that was rewritten to "localhost". Primarily
	// useful for diagnostics / tests.
	WasLoopbackRewritten bool
}

// ResolveDialTarget is the canonical entry point every service-to-service
// dialer should use. Given a raw endpoint string ("host:port", optionally
// with a loopback IP literal), it returns a DialTarget whose Address is
// safe to dial with TLS against the cluster CA and whose ServerName is
// the cert-valid hostname to verify.
//
// Rules:
//   - "127.0.0.1:P" / "::1:P" / "[::1]:P" → Address="localhost:P", ServerName="localhost"
//   - "localhost:P"                        → passthrough, ServerName="localhost"
//   - "host.example:P"                     → passthrough, ServerName="host.example"
//   - bare "host" (no port)                → Address=host, ServerName=host
//
// An empty endpoint returns a zero DialTarget; the caller should treat
// that as a misconfiguration.
func ResolveDialTarget(endpoint string) DialTarget {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return DialTarget{}
	}
	host, port := splitHostPortLoose(endpoint)
	rewrote := false
	if isLoopbackLiteral(host) {
		host = "localhost"
		rewrote = true
	}
	address := host
	if port != "" {
		address = net.JoinHostPort(host, port)
	}
	return DialTarget{
		Address:              address,
		ServerName:           host,
		WasLoopbackRewritten: rewrote,
	}
}

// NormalizeLoopback is a thin convenience wrapper around ResolveDialTarget
// that returns just the TLS-safe address. Existing call sites that only
// need the string form can use this without rewriting their plumbing.
func NormalizeLoopback(endpoint string) string {
	return ResolveDialTarget(endpoint).Address
}

// IsLoopbackEndpoint reports whether endpoint targets the local host
// (127.0.0.1, ::1, or "localhost"). Service-to-service callers that
// want different treatment for local vs remote endpoints should key
// off this, not off substring matches.
func IsLoopbackEndpoint(endpoint string) bool {
	host, _ := splitHostPortLoose(strings.TrimSpace(endpoint))
	if host == "localhost" {
		return true
	}
	return isLoopbackLiteral(host)
}

// splitHostPortLoose handles "host:port", "[ipv6]:port", bare "host",
// and bare "[ipv6]". net.SplitHostPort rejects inputs without a port,
// so we fall back manually for bare hosts.
func splitHostPortLoose(s string) (host, port string) {
	if s == "" {
		return "", ""
	}
	if h, p, err := net.SplitHostPort(s); err == nil {
		return h, p
	}
	// No port — could be "host" or "[::1]".
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	return s, ""
}

func isLoopbackLiteral(host string) bool {
	if host == "" {
		return false
	}
	if host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
