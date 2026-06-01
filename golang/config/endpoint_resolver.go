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

// MustResolveDialTarget is like ResolveDialTarget but returns an error
// when the endpoint is empty or produces an ambiguous/invalid result.
// Use this at service startup to fail fast on misconfiguration instead
// of discovering the problem at runtime.
func MustResolveDialTarget(endpoint string) (DialTarget, error) {
	dt := ResolveDialTarget(endpoint)
	if dt.Address == "" {
		return dt, &EndpointError{Endpoint: endpoint, Reason: "empty or whitespace-only endpoint"}
	}
	if dt.ServerName == "" {
		return dt, &EndpointError{Endpoint: endpoint, Reason: "could not extract hostname for TLS verification"}
	}
	return dt, nil
}

// EndpointError is returned when an endpoint string cannot be resolved
// to a valid DialTarget. The error message is explicit about what went
// wrong so operators can fix configuration without guessing.
type EndpointError struct {
	Endpoint string
	Reason   string
}

func (e *EndpointError) Error() string {
	return "endpoint resolution failed: " + e.Reason + " (endpoint=" + e.Endpoint + ")"
}

// ValidateRemoteAddr returns an error if addr is a loopback address.
// Loopback addresses (127.0.0.1, ::1, localhost) are never valid for
// inter-node cluster communication. Call this at every address boundary
// where an address arrives from outside (user input, etcd, gRPC fields).
func ValidateRemoteAddr(addr string) error {
	host, _ := splitHostPortLoose(strings.TrimSpace(addr))
	if host == "" {
		return nil // empty is a separate validation concern
	}
	if host == "localhost" || isLoopbackLiteral(host) {
		return &EndpointError{
			Endpoint: addr,
			Reason:   "loopback address rejected — this is a cluster, not a local server; use a routable IP or hostname",
		}
	}
	return nil
}

// ValidateLANAddress is a stricter version of ValidateRemoteAddr for places
// that must speak canonical inter-node LAN identity: rollout gates, agent
// config reconcilers, cluster registration, etc.
//
// Rejects:
//   - loopback (127.0.0.0/8, ::1, "localhost") — local-only, not addressable from peers
//   - unspecified (0.0.0.0, ::) — placeholder, not a real address
//   - link-local (169.254.0.0/16, fe80::/10) — single-segment only
//   - multicast (224.0.0.0/4, ff00::/8) — never valid as a node identity
//   - Docker default bridge 172.17.0.0/16 — picked accidentally when an
//     interface scanner iterates docker0 before the real LAN NIC; the node
//     becomes invisible to peers and to scylla-manager because nothing
//     outside this host can route to 172.17.x.
//
// Accepts:
//   - other RFC1918 ranges (10/8, 172.16-31/12 except 172.17/16, 192.168/16)
//   - public/routable IPs (some clusters legitimately use them)
//   - bare hostnames (deferred — let downstream resolution handle it)
//
// Call this at every place where a node-identity address arrives from
// auto-detection or user input, before persisting or starting a service
// that depends on it. The history that motivates this check:
//
//   - 2026-05-20: node_agent's heartbeat reconciler wrote
//     "scylla.api_address: 172.17.0.1" into scylla-manager-agent.yaml
//     because nodeRoutableIP() briefly returned docker0's IP. With YAML
//     last-wins this silently rerouted the agent to an unreachable host
//     and broke cluster registration with a misleading "TLS EOF" error.
func ValidateLANAddress(addr string) error {
	host, _ := splitHostPortLoose(strings.TrimSpace(addr))
	if host == "" {
		return nil // empty is a separate validation concern
	}
	if host == "localhost" {
		return &EndpointError{
			Endpoint: addr,
			Reason:   "localhost rejected — not a routable LAN identity",
		}
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// Hostname — defer to downstream resolution.
		return nil
	}
	if ip.IsLoopback() {
		return &EndpointError{
			Endpoint: addr,
			Reason:   "loopback address (127/8, ::1) rejected — peers cannot reach it",
		}
	}
	if ip.IsUnspecified() {
		return &EndpointError{
			Endpoint: addr,
			Reason:   "unspecified address (0.0.0.0, ::) rejected — placeholder, not a real address",
		}
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return &EndpointError{
			Endpoint: addr,
			Reason:   "link-local address rejected — single-segment only, not LAN-wide",
		}
	}
	if ip.IsMulticast() {
		return &EndpointError{
			Endpoint: addr,
			Reason:   "multicast address rejected — never valid as a node identity",
		}
	}
	if v4 := ip.To4(); v4 != nil && v4[0] == 172 && v4[1] == 17 {
		return &EndpointError{
			Endpoint: addr,
			Reason:   "docker default bridge (172.17.0.0/16) rejected — pick the host's routable LAN NIC, not docker0",
		}
	}
	return nil
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
