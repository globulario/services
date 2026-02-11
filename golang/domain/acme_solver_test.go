package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/dnsprovider"
	"github.com/globulario/services/golang/dnsprovider/fake"
)

func TestDNS01Solver_Present(t *testing.T) {
	// Create fake provider
	provider, err := fake.NewFakeProvider(dnsprovider.Config{
		Zone:       "example.com",
		DefaultTTL: 600,
	})
	if err != nil {
		t.Fatalf("failed to create fake provider: %v", err)
	}

	fakeProvider := provider.(*fake.FakeProvider)

	// Create solver
	solver := NewDNS01Solver(provider, "example.com")
	solver.SetPropagationTimeout(5 * time.Second)
	solver.SetPropagationInterval(100 * time.Millisecond)

	// Present challenge
	domain := "test.example.com"
	keyAuth := "test-key-authorization"

	err = solver.Present(domain, "token", keyAuth)
	if err != nil {
		t.Fatalf("Present() failed: %v", err)
	}

	// Verify TXT record was created
	expectedTxtValue := solver.computeTXTValue(keyAuth)
	if !fakeProvider.HasRecord("_acme-challenge.test", "TXT", expectedTxtValue) {
		t.Error("expected TXT record not found")
	}
}

func TestDNS01Solver_CleanUp(t *testing.T) {
	// Create fake provider
	provider, err := fake.NewFakeProvider(dnsprovider.Config{
		Zone:       "example.com",
		DefaultTTL: 600,
	})
	if err != nil {
		t.Fatalf("failed to create fake provider: %v", err)
	}

	fakeProvider := provider.(*fake.FakeProvider)

	// Create solver
	solver := NewDNS01Solver(provider, "example.com")
	solver.SetPropagationTimeout(5 * time.Second)

	// First, present the challenge
	domain := "test.example.com"
	keyAuth := "test-key-authorization"

	if err := solver.Present(domain, "token", keyAuth); err != nil {
		t.Fatalf("Present() failed: %v", err)
	}

	// Verify record exists
	expectedTxtValue := solver.computeTXTValue(keyAuth)
	if !fakeProvider.HasRecord("_acme-challenge.test", "TXT", expectedTxtValue) {
		t.Fatal("TXT record should exist before cleanup")
	}

	// Clean up
	if err := solver.CleanUp(domain, "token", keyAuth); err != nil {
		t.Fatalf("CleanUp() failed: %v", err)
	}

	// Verify record is deleted
	if fakeProvider.HasRecord("_acme-challenge.test", "TXT", expectedTxtValue) {
		t.Error("TXT record should be deleted after cleanup")
	}
}

func TestDNS01Solver_ZoneMismatch(t *testing.T) {
	provider, err := fake.NewFakeProvider(dnsprovider.Config{
		Zone: "example.com",
	})
	if err != nil {
		t.Fatalf("failed to create fake provider: %v", err)
	}

	solver := NewDNS01Solver(provider, "example.com")

	// Try to present challenge for wrong zone
	err = solver.Present("test.wrongzone.com", "token", "keyauth")
	if err == nil {
		t.Fatal("expected error for zone mismatch")
	}

	if !strings.Contains(err.Error(), "not in zone") {
		t.Errorf("expected zone mismatch error, got: %v", err)
	}
}

func TestDNS01Solver_ExtractRelativeName(t *testing.T) {
	solver := &DNS01Solver{zone: "example.com"}

	tests := []struct {
		domain   string
		expected string
	}{
		{"test.example.com", "test"},
		{"sub.test.example.com", "sub.test"},
		{"example.com", "@"},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			result := solver.extractRelativeName(tt.domain)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDNS01Solver_ComputeTXTValue(t *testing.T) {
	solver := &DNS01Solver{}

	keyAuth := "test-key-authorization"
	txtValue := solver.computeTXTValue(keyAuth)

	// Verify it's a valid base64url string
	if txtValue == "" {
		t.Error("expected non-empty TXT value")
	}

	// Verify it's deterministic
	txtValue2 := solver.computeTXTValue(keyAuth)
	if txtValue != txtValue2 {
		t.Error("TXT value should be deterministic")
	}

	// Verify different keyAuth gives different value
	txtValue3 := solver.computeTXTValue("different-key-auth")
	if txtValue == txtValue3 {
		t.Error("different keyAuth should produce different TXT value")
	}
}

func TestComputeACMEDNS01Value(t *testing.T) {
	keyAuth := "test-key-authorization"
	value := ComputeACMEDNS01Value(keyAuth)

	if value == "" {
		t.Error("expected non-empty value")
	}

	// Verify it matches solver computation
	solver := &DNS01Solver{}
	if value != solver.computeTXTValue(keyAuth) {
		t.Error("ComputeACMEDNS01Value should match solver.computeTXTValue")
	}
}

func TestLegoProvider(t *testing.T) {
	provider, err := fake.NewFakeProvider(dnsprovider.Config{
		Zone: "example.com",
	})
	if err != nil {
		t.Fatalf("failed to create fake provider: %v", err)
	}

	legoProvider := NewLegoProvider(provider, "example.com")

	// Test Present
	domain := "test.example.com"
	keyAuth := "test-key-auth"

	if err := legoProvider.Present(domain, "token", keyAuth); err != nil {
		t.Fatalf("Present() failed: %v", err)
	}

	// Test CleanUp
	if err := legoProvider.CleanUp(domain, "token", keyAuth); err != nil {
		t.Fatalf("CleanUp() failed: %v", err)
	}

	// Test Timeout
	timeout, interval := legoProvider.Timeout()
	if timeout == 0 || interval == 0 {
		t.Error("expected non-zero timeout and interval")
	}
}

func TestValidateACMEDNS01(t *testing.T) {
	provider, err := fake.NewFakeProvider(dnsprovider.Config{
		Zone: "example.com",
	})
	if err != nil {
		t.Fatalf("failed to create fake provider: %v", err)
	}

	// Should validate successfully with fake provider
	err = ValidateACMEDNS01(provider, "example.com", "test.example.com")
	if err != nil {
		t.Errorf("validation should succeed, got: %v", err)
	}

	// Test with failing provider
	fakeProvider := provider.(*fake.FakeProvider)
	fakeProvider.SetFailure("UpsertTXT", true)

	err = ValidateACMEDNS01(provider, "example.com", "test2.example.com")
	if err == nil {
		t.Error("expected validation to fail with failing provider")
	}
}

func TestDNS01Solver_PropagationTimeout(t *testing.T) {
	provider, err := fake.NewFakeProvider(dnsprovider.Config{
		Zone: "example.com",
	})
	if err != nil {
		t.Fatalf("failed to create fake provider: %v", err)
	}

	solver := NewDNS01Solver(provider, "example.com")
	solver.SetPropagationTimeout(100 * time.Millisecond) // Very short timeout
	solver.SetPropagationInterval(50 * time.Millisecond)

	// Configure provider to fail GetRecords (simulating propagation delay)
	fakeProvider := provider.(*fake.FakeProvider)
	fakeProvider.SetFailure("GetRecords", true)

	// Present should timeout waiting for propagation
	err = solver.Present("test.example.com", "token", "keyauth")
	if err == nil {
		t.Fatal("expected timeout error")
	}

	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "propagat") {
		t.Errorf("expected propagation/timeout error, got: %v", err)
	}
}

func TestDNS01Solver_ApexDomain(t *testing.T) {
	provider, err := fake.NewFakeProvider(dnsprovider.Config{
		Zone: "example.com",
	})
	if err != nil {
		t.Fatalf("failed to create fake provider: %v", err)
	}

	fakeProvider := provider.(*fake.FakeProvider)

	solver := NewDNS01Solver(provider, "example.com")
	solver.SetPropagationTimeout(5 * time.Second)

	// Present challenge for apex domain
	err = solver.Present("example.com", "token", "keyauth")
	if err != nil {
		t.Fatalf("Present() for apex domain failed: %v", err)
	}

	// Verify TXT record created with @ name
	expectedTxtValue := solver.computeTXTValue("keyauth")
	if !fakeProvider.HasRecord("_acme-challenge.@", "TXT", expectedTxtValue) {
		t.Error("expected TXT record for apex domain not found")
	}
}
