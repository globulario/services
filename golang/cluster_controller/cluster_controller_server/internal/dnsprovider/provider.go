// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.dnsprovider.interface
// @awareness file_role=external_dns_provider_interface_and_shared_safety_helpers
// @awareness enforces=globular.platform:invariant.dnsprovider.private_ips_must_not_publish_to_public_dns
// @awareness risk=high
package dnsprovider

// provider.go — the abstraction every external DNS publisher
// (cloudflare, rfc2136, noop, future route53) implements. Three
// shared helpers carry the safety contract:
//
//  1. IsPrivateIP recognizes the RFC1918 / RFC4193 / loopback ranges.
//     Used by FilterPublicIPs to drop addresses that must not appear
//     in public DNS.
//
//  2. FilterPublicIPs is the single chokepoint: every provider
//     implementation calls it before any DNS write. Publishing a
//     10.0.0.0/8 address to api.cloudflare.com would leak internal
//     topology to the public internet and expose the cluster to
//     reconnaissance. The AllowPrivateIPs escape hatch exists for
//     operators with deliberately split-horizon setups; default is
//     filter-out.
//
//  3. ValidateName rejects malformed DNS names before they reach
//     the provider API — protects against injection-shaped inputs
//     and catches operator typos before they become broken records.
//
// Adding a new provider means adding a constructor here AND
// re-using these helpers; do not reinvent them inside the provider.

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
)

// Provider defines the interface for external DNS providers (PR8)
type Provider interface {
	// UpsertA creates or updates an A record
	UpsertA(ctx context.Context, name string, ips []net.IP, ttl int) error

	// UpsertAAAA creates or updates an AAAA record
	UpsertAAAA(ctx context.Context, name string, ips []net.IP, ttl int) error

	// Delete removes a DNS record (all types for the given name)
	Delete(ctx context.Context, name string) error

	// Close cleans up provider resources
	Close() error
}

// Config holds provider configuration (PR8)
type Config struct {
	Provider         string            // "rfc2136", "cloudflare", "route53"
	Domain           string            // Public domain (e.g., "example.com")
	TTL              int               // Default TTL in seconds
	AllowPrivateIPs  bool              // Allow publishing RFC1918 IPs
	ProviderConfig   map[string]string // Provider-specific config
}

// New creates a new DNS provider based on the configuration (PR8)
func New(cfg Config) (Provider, error) {
	if cfg.Domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 300 // Default 5 minutes
	}

	switch cfg.Provider {
	case "rfc2136":
		return NewRFC2136Provider(cfg)
	case "cloudflare":
		return NewCloudflareProvider(cfg)
	case "noop", "":
		return NewNoopProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
}

// IsPrivateIP checks if an IP is in a private range (RFC1918, RFC4193, loopback) (PR8)
func IsPrivateIP(ip net.IP) bool {
	// IPv4 private ranges
	private4 := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
	}

	// IPv6 private ranges
	private6 := []string{
		"fc00::/7",  // Unique local addresses (ULA)
		"fe80::/10", // Link-local
		"::1/128",   // Loopback
	}

	for _, cidr := range private4 {
		_, network, _ := net.ParseCIDR(cidr)
		if network != nil && network.Contains(ip) {
			return true
		}
	}

	for _, cidr := range private6 {
		_, network, _ := net.ParseCIDR(cidr)
		if network != nil && network.Contains(ip) {
			return true
		}
	}

	return false
}

// FilterPublicIPs filters out private IPs unless AllowPrivateIPs is true (PR8)
func FilterPublicIPs(ips []net.IP, allowPrivate bool) []net.IP {
	if allowPrivate {
		return ips
	}

	public := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		if !IsPrivateIP(ip) {
			public = append(public, ip)
		}
	}
	return public
}

// ValidateName validates a DNS name (PR8)
// Accepts wildcard names of the form "*.example.com".
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	// Strip leading wildcard label before validating the rest.
	check := name
	if strings.HasPrefix(check, "*.") {
		check = check[2:]
	}

	// DNS name validation (simplified)
	validName := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)*$`)
	if !validName.MatchString(check) {
		return fmt.Errorf("invalid DNS name: %s", name)
	}

	return nil
}
