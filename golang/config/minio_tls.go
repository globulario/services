// @awareness namespace=globular.platform
// @awareness component=platform_config
// @awareness file_role=minio_tls_config
// @awareness implements=globular.platform:intent.dns_pki.explicit_identity_over_convenient_routing
// @awareness risk=high
package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"
)

// MinIOTLSConfig builds a TLS config suitable for connecting to MinIO.
//
// Security model:
//   - Local addresses (loopback or any IP belonging to this host's network
//     interfaces) skip verification — traffic never leaves the machine.
//   - All other addresses load the cluster CA and verify the server.
//   - If the CA is unavailable for a non-local address, returns an error
//     rather than silently disabling verification.
func MinIOTLSConfig(endpoint string) (*tls.Config, error) {
	host := extractHost(endpoint)

	// Local addresses skip TLS verification — traffic never leaves the machine.
	// This covers loopback (127.0.0.1, ::1, localhost) and non-loopback IPs
	// that belong to this host's network interfaces (co-located MinIO).
	if isLoopback(host) || IsLocalIP(host) {
		return &tls.Config{InsecureSkipVerify: true}, nil //nolint:gosec // local only
	}

	// Non-loopback: load cluster CA for verification.
	caPath := GetLocalCACertificate()
	if caPath == "" {
		// Try canonical path (respects GLOBULAR_STATE_DIR override).
		caPath = GetCACertificatePath()
	}
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("MinIO TLS: CA unavailable at %s for non-loopback endpoint %s: %w", caPath, endpoint, err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)

	return &tls.Config{
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12,
	}, nil
}

func isLoopback(host string) bool {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// IsLocalIP reports whether ip belongs to one of this machine's network
// interfaces (including non-loopback addresses such as 10.0.0.63).  This is
// used to detect co-located MinIO instances whose traffic never leaves the
// host even though the address is not a loopback address.
func IsLocalIP(ip string) bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, a := range addrs {
		if ipNet, ok := a.(*net.IPNet); ok && ipNet.IP.String() == ip {
			return true
		}
	}
	return false
}

func extractHost(endpoint string) string {
	// Strip scheme if present.
	s := endpoint
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	// Handle bare IPv6 without brackets (e.g., "::1:9000" → "::1").
	if ip := net.ParseIP(s); ip != nil {
		return s // already a plain IP, no port
	}
	// Strip port.
	host, _, err := net.SplitHostPort(s)
	if err != nil {
		// SplitHostPort fails on bare IPv6 like "::1:9000".
		// Try parsing as IPv6 by stripping the last :port segment.
		if lastColon := strings.LastIndex(s, ":"); lastColon > 0 {
			candidate := s[:lastColon]
			if ip := net.ParseIP(candidate); ip != nil {
				return candidate
			}
		}
		return s
	}
	return host
}
