package domain

import (
	"context"
	"testing"
	"time"
)

// TestHasDot verifies FQDN dot checking.
func TestHasDot(t *testing.T) {
	tests := []struct {
		fqdn string
		want bool
	}{
		{"example.com.", true},
		{"example.com", false},
		{"", false},
		{".", true},
	}

	for _, tt := range tests {
		got := hasDot(tt.fqdn)
		if got != tt.want {
			t.Errorf("hasDot(%q) = %v, want %v", tt.fqdn, got, tt.want)
		}
	}
}

// TestPublicResolverPropagator_DefaultResolvers verifies default resolver list.
func TestPublicResolverPropagator_DefaultResolvers(t *testing.T) {
	p := NewPublicResolverPropagator()

	if len(p.Resolvers) == 0 {
		t.Fatal("expected default resolvers, got empty list")
	}

	// Should include Cloudflare and Google
	hasCloudflare := false
	hasGoogle := false

	for _, r := range p.Resolvers {
		if r == "1.1.1.1:53" || r == "1.0.0.1:53" {
			hasCloudflare = true
		}
		if r == "8.8.8.8:53" || r == "8.8.4.4:53" {
			hasGoogle = true
		}
	}

	if !hasCloudflare {
		t.Error("expected Cloudflare resolver (1.1.1.1:53) in default list")
	}
	if !hasGoogle {
		t.Error("expected Google resolver (8.8.8.8:53) in default list")
	}
}

// TestPublicResolverPropagator_LookupTXT_RealDNS tests actual DNS lookup.
// This is a real network test that queries public resolvers.
func TestPublicResolverPropagator_LookupTXT_RealDNS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	p := NewPublicResolverPropagator()
	ctx := context.Background()

	// Query a known TXT record (Google's public DNS has a TXT record)
	// Note: This test may be flaky if network is unavailable
	records, err := p.lookupTXT(ctx, "google.com", "8.8.8.8:53")
	if err != nil {
		t.Logf("DNS lookup failed (network may be unavailable): %v", err)
		t.Skip("skipping real DNS test - network unavailable")
	}

	// google.com should have at least one TXT record (SPF)
	if len(records) == 0 {
		t.Error("expected at least one TXT record for google.com")
	}

	t.Logf("Found %d TXT records for google.com", len(records))
}

// TestPublicResolverPropagator_CheckTXT_NotFound verifies error when record not found.
func TestPublicResolverPropagator_CheckTXT_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	p := NewPublicResolverPropagator()
	ctx := context.Background()

	// Try to find a non-existent value
	err := p.checkTXT(ctx, "google.com.", "this-value-does-not-exist-xyz123")
	if err == nil {
		t.Error("expected error for non-existent TXT value, got nil")
	}
}

// TestPublicResolverPropagator_WaitForTXT_Timeout verifies timeout behavior.
func TestPublicResolverPropagator_WaitForTXT_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	p := NewPublicResolverPropagator()
	ctx := context.Background()

	// Wait for a non-existent value with short timeout
	start := time.Now()
	err := p.WaitForTXT(ctx, "google.com", "non-existent-value-xyz", 2*time.Second, 500*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error, got nil")
	}

	// Should have waited approximately the timeout duration
	if elapsed < 1*time.Second {
		t.Errorf("timeout happened too quickly: %v", elapsed)
	}
	if elapsed > 5*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}

	t.Logf("Timeout behavior correct: waited %v", elapsed)
}

// TestPublicResolverPropagator_WaitForTXT_ContextCancellation verifies context handling.
func TestPublicResolverPropagator_WaitForTXT_ContextCancellation(t *testing.T) {
	p := NewPublicResolverPropagator()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	err := p.WaitForTXT(ctx, "google.com", "non-existent", 10*time.Second, 1*time.Second)
	if err == nil {
		t.Error("expected context cancellation error, got nil")
	}

	if ctx.Err() == nil {
		t.Error("expected context to be cancelled")
	}
}

// NOTE: Real propagation tests require:
// 1. A DNS provider account with API access
// 2. A test domain under your control
// 3. Ability to create/delete TXT records
//
// These should be integration tests with build tags:
//
// //go:build integration
// func TestDNSPropagation_EndToEnd(t *testing.T) {
//     // 1. Create TXT record via provider API
//     // 2. Wait for propagation using PublicResolverPropagator
//     // 3. Verify record appears on public resolvers
//     // 4. Delete TXT record
// }
