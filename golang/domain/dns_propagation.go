package domain

import (
	"context"
	"fmt"
	"net"
	"time"
)

// DNSPropagator checks DNS propagation to public resolvers.
// This is critical for ACME DNS-01 challenges - the provider API may report
// a record exists, but Let's Encrypt validation will fail if public resolvers
// haven't seen it yet.
type DNSPropagator interface {
	// WaitForTXT waits for a TXT record to propagate to public DNS resolvers.
	// Returns nil when the expected value is found, or error on timeout.
	WaitForTXT(ctx context.Context, fqdn string, wantValue string, timeout time.Duration, interval time.Duration) error
}

// PublicResolverPropagator queries public DNS resolvers (Cloudflare, Google)
// to verify TXT record propagation before ACME validation.
type PublicResolverPropagator struct {
	// Resolvers is the list of public DNS servers to query (IP:port format).
	// Default: Cloudflare (1.1.1.1:53) and Google (8.8.8.8:53)
	Resolvers []string
}

// NewPublicResolverPropagator creates a propagator with default public resolvers.
func NewPublicResolverPropagator() *PublicResolverPropagator {
	return &PublicResolverPropagator{
		Resolvers: []string{
			"1.1.1.1:53",   // Cloudflare
			"8.8.8.8:53",   // Google
			"1.0.0.1:53",   // Cloudflare secondary
			"8.8.4.4:53",   // Google secondary
		},
	}
}

// WaitForTXT polls public DNS resolvers until the expected TXT record value appears.
// This ensures ACME DNS-01 validation will succeed.
func (p *PublicResolverPropagator) WaitForTXT(
	ctx context.Context,
	fqdn string,
	wantValue string,
	timeout time.Duration,
	interval time.Duration,
) error {
	if !hasDot(fqdn) {
		fqdn = fqdn + "."
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Try immediately first
	if err := p.checkTXT(ctx, fqdn, wantValue); err == nil {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())

		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for TXT record %s to propagate (expected value: %s)", fqdn, wantValue)
			}

			if err := p.checkTXT(ctx, fqdn, wantValue); err == nil {
				return nil // Propagated!
			}
			// Continue polling
		}
	}
}

// checkTXT queries all configured resolvers and returns nil if the expected value
// is found on at least one resolver. Returns error if value not found on any resolver.
func (p *PublicResolverPropagator) checkTXT(ctx context.Context, fqdn string, wantValue string) error {
	// Query each resolver
	for _, resolver := range p.Resolvers {
		records, err := p.lookupTXT(ctx, fqdn, resolver)
		if err != nil {
			// Resolver failed - try next one
			continue
		}

		// Check if expected value is present
		for _, record := range records {
			if record == wantValue {
				return nil // Found!
			}
		}
	}

	return fmt.Errorf("TXT record not found on any public resolver")
}

// lookupTXT queries a specific DNS resolver for TXT records.
func (p *PublicResolverPropagator) lookupTXT(ctx context.Context, fqdn string, resolver string) ([]string, error) {
	// Create resolver with custom nameserver
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 10 * time.Second,
			}
			return d.DialContext(ctx, network, resolver)
		},
	}

	records, err := r.LookupTXT(ctx, fqdn)
	if err != nil {
		return nil, fmt.Errorf("lookup TXT %s via %s failed: %w", fqdn, resolver, err)
	}

	return records, nil
}

// hasDot checks if a FQDN ends with a dot.
func hasDot(fqdn string) bool {
	return len(fqdn) > 0 && fqdn[len(fqdn)-1] == '.'
}
