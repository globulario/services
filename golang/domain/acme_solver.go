package domain

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/dnsprovider"
)

// DNS01Solver implements ACME DNS-01 challenge solving using a dnsprovider.Provider.
//
// The DNS-01 challenge requires:
// 1. Creating a TXT record at _acme-challenge.<domain> with a specific token value
// 2. Waiting for DNS propagation
// 3. ACME server validates the TXT record
// 4. Cleaning up the TXT record after validation
//
// This solver bridges the lego ACME library with our generic dnsprovider interface.
type DNS01Solver struct {
	provider dnsprovider.Provider
	zone     string
	timeout  time.Duration
	interval time.Duration
}

// NewDNS01Solver creates a new ACME DNS-01 challenge solver.
func NewDNS01Solver(provider dnsprovider.Provider, zone string) *DNS01Solver {
	return &DNS01Solver{
		provider: provider,
		zone:     zone,
		timeout:  2 * time.Minute, // DNS propagation timeout
		interval: 5 * time.Second,  // DNS check interval
	}
}

// SetPropagationTimeout configures how long to wait for DNS propagation.
func (s *DNS01Solver) SetPropagationTimeout(timeout time.Duration) {
	s.timeout = timeout
}

// SetPropagationInterval configures how often to check for DNS propagation.
func (s *DNS01Solver) SetPropagationInterval(interval time.Duration) {
	s.interval = interval
}

// Present creates the TXT record for the ACME DNS-01 challenge.
// This is called by the ACME client to prove domain ownership.
//
// Parameters:
//   - domain: The domain being validated (e.g., "globule-ryzen.globular.cloud")
//   - token: ACME challenge token (unused, kept for interface compatibility)
//   - keyAuth: Key authorization string from ACME server
//
// The TXT record is created at: _acme-challenge.<domain>
func (s *DNS01Solver) Present(domain, token, keyAuth string) error {
	// Compute DNS-01 TXT record value
	// This is: base64url(sha256(keyAuth))
	txtValue := s.computeTXTValue(keyAuth)

	// Extract zone from domain
	if !strings.HasSuffix(domain, "."+s.zone) && domain != s.zone {
		return fmt.Errorf("domain %q is not in zone %q", domain, s.zone)
	}

	// Construct challenge record name
	// For domain "test.example.com" in zone "example.com":
	// Challenge record is "_acme-challenge.test.example.com"
	// Relative name for provider is "_acme-challenge.test"
	relativeName := s.extractRelativeName(domain)
	challengeName := "_acme-challenge." + relativeName

	// Create TXT record with short TTL (DNS-01 challenges are temporary)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.provider.UpsertTXT(ctx, s.zone, challengeName, []string{txtValue}, 60); err != nil {
		return fmt.Errorf("failed to create DNS-01 challenge record: %w", err)
	}

	// Wait for DNS propagation
	if err := s.waitForPropagation(challengeName, txtValue); err != nil {
		return fmt.Errorf("DNS-01 challenge record not propagated: %w", err)
	}

	return nil
}

// CleanUp removes the TXT record created for the ACME DNS-01 challenge.
// This is called after the ACME server has validated the challenge.
func (s *DNS01Solver) CleanUp(domain, token, keyAuth string) error {
	txtValue := s.computeTXTValue(keyAuth)

	relativeName := s.extractRelativeName(domain)
	challengeName := "_acme-challenge." + relativeName

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.provider.DeleteTXT(ctx, s.zone, challengeName, []string{txtValue}); err != nil {
		// Log but don't fail - cleanup is best-effort
		// The record will expire based on TTL anyway
		return fmt.Errorf("failed to cleanup DNS-01 challenge record: %w", err)
	}

	return nil
}

// Timeout returns the DNS propagation timeout.
// This is used by lego to configure how long to wait.
func (s *DNS01Solver) Timeout() (timeout, interval time.Duration) {
	return s.timeout, s.interval
}

// Helper methods

// computeTXTValue computes the DNS-01 TXT record value from keyAuth.
// This implements the DNS-01 challenge specification:
// TXT value = base64url(sha256(keyAuth))
func (s *DNS01Solver) computeTXTValue(keyAuth string) string {
	// Compute manually: base64url(sha256(keyAuth))
	h := sha256.Sum256([]byte(keyAuth))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// extractRelativeName extracts the relative DNS name from a FQDN.
// Example: "test.example.com" in zone "example.com" → "test"
//          "example.com" in zone "example.com" → "@"
func (s *DNS01Solver) extractRelativeName(domain string) string {
	if domain == s.zone {
		return "@"
	}
	return strings.TrimSuffix(domain, "."+s.zone)
}

// waitForPropagation waits for the DNS TXT record to propagate.
// This is necessary because DNS updates are not instantaneous.
func (s *DNS01Solver) waitForPropagation(name, expectedValue string) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for DNS propagation")

		case <-ticker.C:
			// Query the provider for the TXT record
			records, err := s.provider.GetRecords(context.Background(), s.zone, name, "TXT")
			if err != nil {
				// Continue waiting on transient errors
				continue
			}

			// Check if expected value is present
			for _, rec := range records {
				if rec.Value == expectedValue {
					// Found it! DNS has propagated
					return nil
				}
			}
		}
	}
}

// LegoProvider wraps DNS01Solver to implement lego's challenge.Provider interface.
// This allows using our solver directly with lego's ACME client.
type LegoProvider struct {
	solver *DNS01Solver
}

// NewLegoProvider creates a lego-compatible DNS provider from our solver.
func NewLegoProvider(provider dnsprovider.Provider, zone string) *LegoProvider {
	return &LegoProvider{
		solver: NewDNS01Solver(provider, zone),
	}
}

// Present implements lego's challenge.Provider interface.
func (p *LegoProvider) Present(domain, token, keyAuth string) error {
	return p.solver.Present(domain, token, keyAuth)
}

// CleanUp implements lego's challenge.Provider interface.
func (p *LegoProvider) CleanUp(domain, token, keyAuth string) error {
	return p.solver.CleanUp(domain, token, keyAuth)
}

// Timeout implements lego's challenge.ProviderTimeout interface (optional).
func (p *LegoProvider) Timeout() (timeout, interval time.Duration) {
	return p.solver.Timeout()
}

// ValidateACMEDNS01 validates that a DNS provider can perform DNS-01 challenges.
// This is useful for testing provider configurations before using them.
func ValidateACMEDNS01(provider dnsprovider.Provider, zone, testDomain string) error {
	solver := NewDNS01Solver(provider, zone)

	// Generate a test key auth
	testKeyAuth := "test-key-auth-" + fmt.Sprintf("%x", sha256.Sum256([]byte(testDomain)))
	testTxtValue := solver.computeTXTValue(testKeyAuth)

	// Try to create a test TXT record
	relativeName := solver.extractRelativeName(testDomain)
	testName := "_acme-challenge-test." + relativeName

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := provider.UpsertTXT(ctx, zone, testName, []string{testTxtValue}, 60); err != nil {
		return fmt.Errorf("DNS provider cannot create TXT records: %w", err)
	}

	// Verify we can read it back
	records, err := provider.GetRecords(ctx, zone, testName, "TXT")
	if err != nil {
		return fmt.Errorf("DNS provider cannot query TXT records: %w", err)
	}

	found := false
	for _, rec := range records {
		if rec.Value == testTxtValue {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("DNS provider created TXT record but cannot retrieve it")
	}

	// Clean up test record
	if err := provider.DeleteTXT(ctx, zone, testName, []string{testTxtValue}); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to cleanup test record: %v\n", err)
	}

	return nil
}

// Helper function for computing ACME DNS-01 values (exposed for testing)
func ComputeACMEDNS01Value(keyAuth string) string {
	// base64url(sha256(keyAuth))
	h := sha256.Sum256([]byte(keyAuth))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
