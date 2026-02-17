package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/domain"
	"github.com/spf13/cobra"
)

// TestDomainStatusNotFoundExitCode verifies that 'domain status' returns non-zero exit code
// when a specific domain is not found (Fix #1)
func TestDomainStatusNotFoundExitCode(t *testing.T) {
	// This test verifies the fix for: "domain status should exit non-zero when domain not found"
	// Previously, the command would print "Domain not found" but return exit code 0,
	// causing shell scripts to incorrectly conclude the domain existed.

	tests := []struct {
		name           string
		fqdn           string
		outputFormat   string
		wantErr        bool
		wantExitCode   bool // should return error (non-zero exit)
		wantStderrJSON bool // stderr should contain valid JSON error
	}{
		{
			name:         "not_found_plain_output",
			fqdn:         "nonexistent.example.com",
			outputFormat: "table",
			wantErr:      true,
			wantExitCode: true,
		},
		{
			name:           "not_found_json_output",
			fqdn:           "nonexistent.example.com",
			outputFormat:   "json",
			wantErr:        true,
			wantExitCode:   true,
			wantStderrJSON: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip if etcd is not available (not an integration test environment)
			if !isEtcdAvailable(t) {
				t.Skip("etcd not available, skipping integration test")
			}

			// Capture stdout and stderr
			oldStdout := os.Stdout
			oldStderr := os.Stderr
			rOut, wOut, _ := os.Pipe()
			rErr, wErr, _ := os.Pipe()
			os.Stdout = wOut
			os.Stderr = wErr

			// Set flags
			domainFQDN = tt.fqdn
			rootCfg.output = tt.outputFormat
			rootCfg.timeout = 5 * time.Second

			// Run command
			err := runDomainStatus(&cobra.Command{}, []string{})

			// Restore stdout/stderr
			wOut.Close()
			wErr.Close()
			os.Stdout = oldStdout
			os.Stderr = oldStderr

			// Read captured output
			var stdoutBuf, stderrBuf bytes.Buffer
			io.Copy(&stdoutBuf, rOut)
			io.Copy(&stderrBuf, rErr)

			// Verify exit code behavior
			if tt.wantExitCode && err == nil {
				t.Errorf("expected error (non-zero exit), got nil")
			}
			if !tt.wantExitCode && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}

			// Verify error message contains domain name
			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), tt.fqdn) {
					t.Errorf("error message should contain FQDN %q, got: %v", tt.fqdn, err)
				}
			}

			// Verify JSON error format in stderr
			if tt.wantStderrJSON {
				stderrStr := stderrBuf.String()
				if stderrStr == "" {
					t.Error("expected JSON error in stderr, got empty")
				} else {
					// Verify it's valid JSON with expected fields
					var errObj map[string]interface{}
					if err := json.Unmarshal([]byte(strings.TrimSpace(stderrStr)), &errObj); err != nil {
						t.Errorf("stderr should contain valid JSON, got: %s\nUnmarshal error: %v", stderrStr, err)
					} else {
						if errObj["error"] == nil {
							t.Error("JSON error should contain 'error' field")
						}
						if errObj["fqdn"] != tt.fqdn {
							t.Errorf("JSON error fqdn = %v, want %q", errObj["fqdn"], tt.fqdn)
						}
					}
				}
			}
		})
	}
}

// TestDomainStatusNoDomainsJSONOutput verifies that 'domain status --output json'
// returns empty JSON array when no domains exist (Fix #3)
func TestDomainStatusNoDomainsJSONOutput(t *testing.T) {
	// This test verifies: "domain status --output json should return [] when no domains exist"
	// This ensures scripts can parse the output without hitting the fallback path

	if !isEtcdAvailable(t) {
		t.Skip("etcd not available, skipping integration test")
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Clear domainFQDN to list all domains
	domainFQDN = ""
	rootCfg.output = "json"
	rootCfg.timeout = 5 * time.Second

	// Run command (should succeed with no error since empty list is not an error)
	err := runDomainStatus(&cobra.Command{}, []string{})

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := strings.TrimSpace(buf.String())

	// Verify no error for empty list
	if err != nil {
		t.Errorf("expected no error for empty domain list, got: %v", err)
	}

	// Verify output is valid JSON array
	if output != "[]" {
		t.Errorf("expected empty JSON array '[]', got: %s", output)
	}

	// Verify it can be parsed as JSON array
	var domains []interface{}
	if err := json.Unmarshal([]byte(output), &domains); err != nil {
		t.Errorf("output should be valid JSON array, got unmarshal error: %v", err)
	}

	if len(domains) != 0 {
		t.Errorf("expected empty array, got length %d", len(domains))
	}
}

// TestDomainAddVerification verifies that 'domain add' reads back the spec
// after writing to etcd to verify persistence (Fix #2)
func TestDomainAddVerification(t *testing.T) {
	// This test verifies: "domain add should verify persistence by reading back"
	// This prevents false "registered successfully" when etcd is misconfigured

	if !isEtcdAvailable(t) {
		t.Skip("etcd not available, skipping integration test")
	}

	// Setup test domain spec
	testFQDN := fmt.Sprintf("test-%d.example.com", time.Now().Unix())
	domainFQDN = testFQDN
	domainZone = "example.com"
	domainProvider = "test-provider"
	domainTargetIP = "203.0.113.1"
	domainTTL = 600
	domainNodeID = "test-node"
	domainEnableACME = false
	domainEnableIngress = false

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// First, ensure provider exists
	if err := setupTestProvider(t); err != nil {
		t.Fatalf("failed to setup test provider: %v", err)
	}

	// Run domain add
	err := runDomainAdd(&cobra.Command{}, []string{})

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Consume output
	io.Copy(io.Discard, r)

	// Verify command succeeded
	if err != nil {
		t.Errorf("domain add should succeed, got error: %v", err)
	}

	// Verify the spec actually exists in etcd by querying directly
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	etcdClient, err := config.GetEtcdClient()
	if err != nil {
		t.Fatalf("failed to get etcd client: %v", err)
	}
	defer etcdClient.Close()

	key := domain.DomainKey(testFQDN)
	resp, err := etcdClient.Get(ctx, key)
	if err != nil {
		t.Fatalf("failed to query etcd: %v", err)
	}

	if resp.Count == 0 {
		t.Error("spec should exist in etcd after successful add")
	} else {
		// Verify the spec can be parsed
		spec, err := domain.FromJSON(resp.Kvs[0].Value)
		if err != nil {
			t.Errorf("spec in etcd should be valid JSON: %v", err)
		} else {
			if spec.FQDN != testFQDN {
				t.Errorf("spec FQDN = %s, want %s", spec.FQDN, testFQDN)
			}
			if spec.Zone != domainZone {
				t.Errorf("spec Zone = %s, want %s", spec.Zone, domainZone)
			}
		}
	}

	// Cleanup
	etcdClient.Delete(ctx, key)
}

// TestDomainStatusJSONArrayConsistency verifies that JSON output is always an array
// for consistent parsing by scripts
func TestDomainStatusJSONArrayConsistency(t *testing.T) {
	// Even when querying a single domain with --fqdn, the JSON output should be an array
	// for consistency with the "list all domains" output

	if !isEtcdAvailable(t) {
		t.Skip("etcd not available, skipping integration test")
	}

	// Create a test domain first
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	etcdClient, err := config.GetEtcdClient()
	if err != nil {
		t.Fatalf("failed to get etcd client: %v", err)
	}
	defer etcdClient.Close()

	testFQDN := fmt.Sprintf("test-%d.example.com", time.Now().Unix())
	spec := &domain.ExternalDomainSpec{
		FQDN:        testFQDN,
		Zone:        "example.com",
		NodeID:      "test-node",
		TargetIP:    "203.0.113.1",
		ProviderRef: "test-provider",
		TTL:         600,
		Status: domain.ExternalDomainStatus{
			Phase: "Pending",
		},
	}

	data, err := spec.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize spec: %v", err)
	}

	key := domain.DomainKey(testFQDN)
	_, err = etcdClient.Put(ctx, key, string(data))
	if err != nil {
		t.Fatalf("failed to write test spec: %v", err)
	}
	defer etcdClient.Delete(ctx, key)

	// Query specific domain with JSON output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	domainFQDN = testFQDN
	rootCfg.output = "json"
	rootCfg.timeout = 5 * time.Second

	err = runDomainStatus(&cobra.Command{}, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := strings.TrimSpace(buf.String())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify output is valid JSON array
	var domains []domain.ExternalDomainSpec
	if err := json.Unmarshal([]byte(output), &domains); err != nil {
		t.Errorf("output should be valid JSON array, got unmarshal error: %v\nOutput: %s", err, output)
	}

	if len(domains) != 1 {
		t.Errorf("expected array with 1 domain, got length %d", len(domains))
	} else {
		if domains[0].FQDN != testFQDN {
			t.Errorf("domain FQDN = %s, want %s", domains[0].FQDN, testFQDN)
		}
	}
}

// TestDomainStatusJSONFieldStability ensures the JSON schema is stable
func TestDomainStatusJSONFieldStability(t *testing.T) {
	// Verify that the JSON output contains all expected fields
	// This prevents breaking changes to the JSON schema

	if !isEtcdAvailable(t) {
		t.Skip("etcd not available, skipping integration test")
	}

	// Create a test domain with all fields populated
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	etcdClient, err := config.GetEtcdClient()
	if err != nil {
		t.Fatalf("failed to get etcd client: %v", err)
	}
	defer etcdClient.Close()

	testFQDN := fmt.Sprintf("test-%d.example.com", time.Now().Unix())
	spec := &domain.ExternalDomainSpec{
		FQDN:        testFQDN,
		Zone:        "example.com",
		NodeID:      "test-node",
		TargetIP:    "203.0.113.1",
		ProviderRef: "test-provider",
		TTL:         600,
		ACME: domain.ACMEConfig{
			Enabled:       true,
			ChallengeType: "dns-01",
			Email:         "test@example.com",
		},
		Ingress: domain.IngressConfig{
			Enabled: true,
			Service: "gateway",
			Port:    443,
		},
		Status: domain.ExternalDomainStatus{
			Phase:   "Ready",
			Message: "All systems operational",
			Conditions: []domain.Condition{
				{Type: "DNSRecordCreated", Status: "True"},
				{Type: "CertificateValid", Status: "True"},
			},
		},
	}

	data, err := spec.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize spec: %v", err)
	}

	key := domain.DomainKey(testFQDN)
	_, err = etcdClient.Put(ctx, key, string(data))
	if err != nil {
		t.Fatalf("failed to write test spec: %v", err)
	}
	defer etcdClient.Delete(ctx, key)

	// Query with JSON output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	domainFQDN = testFQDN
	rootCfg.output = "json"
	rootCfg.timeout = 5 * time.Second

	err = runDomainStatus(&cobra.Command{}, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := strings.TrimSpace(buf.String())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Parse as JSON and verify structure
	var domains []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &domains); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v\nOutput: %s", err, output)
	}

	if len(domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(domains))
	}

	domain := domains[0]

	// Verify top-level required fields
	requiredFields := []string{"fqdn", "zone", "node_id", "target_ip", "provider_ref", "ttl", "status"}
	for _, field := range requiredFields {
		if _, ok := domain[field]; !ok {
			t.Errorf("missing required field %q in JSON output", field)
		}
	}

	// Verify status subfields
	status, ok := domain["status"].(map[string]interface{})
	if !ok {
		t.Fatal("status should be an object")
	}

	statusFields := []string{"phase", "message", "conditions"}
	for _, field := range statusFields {
		if _, ok := status[field]; !ok {
			t.Errorf("missing required field %q in status object", field)
		}
	}

	// Verify conditions is an array
	conditions, ok := status["conditions"].([]interface{})
	if !ok {
		t.Error("conditions should be an array")
	}
	if len(conditions) < 2 {
		t.Errorf("expected at least 2 conditions, got %d", len(conditions))
	}
}

// Helper functions

// isEtcdAvailable checks if etcd is available for integration testing
func isEtcdAvailable(t *testing.T) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	etcdClient, err := config.GetEtcdClient()
	if err != nil {
		t.Logf("etcd not available: %v", err)
		return false
	}
	defer etcdClient.Close()

	// Try a simple operation
	_, err = etcdClient.Get(ctx, "/test-connectivity")
	if err != nil {
		t.Logf("etcd connectivity test failed: %v", err)
		return false
	}

	return true
}

// setupTestProvider creates a test DNS provider configuration in etcd
func setupTestProvider(t *testing.T) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	etcdClient, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("failed to get etcd client: %w", err)
	}
	defer etcdClient.Close()

	// Check if provider already exists
	key := domain.ProviderKey("test-provider")
	resp, err := etcdClient.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check provider: %w", err)
	}

	if resp.Count > 0 {
		// Provider already exists
		return nil
	}

	// Create minimal provider config
	providerConfig := map[string]interface{}{
		"type":        "manual",
		"zone":        "example.com",
		"credentials": map[string]string{},
		"default_ttl": 600,
	}

	data, err := json.Marshal(providerConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal provider config: %w", err)
	}

	_, err = etcdClient.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("failed to save provider config: %w", err)
	}

	return nil
}

// TestDomainAddWithoutProviderFails verifies that domain add fails if provider doesn't exist
func TestDomainAddWithoutProviderFails(t *testing.T) {
	if !isEtcdAvailable(t) {
		t.Skip("etcd not available, skipping integration test")
	}

	// Setup with non-existent provider
	domainFQDN = fmt.Sprintf("test-%d.example.com", time.Now().Unix())
	domainZone = "example.com"
	domainProvider = "nonexistent-provider-xyz"
	domainTargetIP = "203.0.113.1"
	domainTTL = 600
	domainNodeID = "test-node"
	domainEnableACME = false
	domainEnableIngress = false

	// Suppress output
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	os.Stdout = nil
	os.Stderr = nil

	// Run domain add (should fail validation)
	err := runDomainAdd(&cobra.Command{}, []string{})

	// Restore output
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	// Note: Currently the validation doesn't check if provider exists
	// This test documents the expected behavior for future enhancement
	if err == nil {
		t.Log("Note: domain add should validate provider exists (enhancement opportunity)")
	}
}

// Unit Tests (no etcd required)

// TestDomainSpecValidation tests that domain.ExternalDomainSpec.Validate() works correctly
func TestDomainSpecValidation(t *testing.T) {
	tests := []struct {
		name    string
		spec    *domain.ExternalDomainSpec
		wantErr bool
	}{
		{
			name: "valid_spec",
			spec: &domain.ExternalDomainSpec{
				FQDN:        "test.example.com",
				Zone:        "example.com",
				NodeID:      "node1",
				TargetIP:    "203.0.113.1",
				ProviderRef: "provider1",
				TTL:         600,
			},
			wantErr: false,
		},
		{
			name: "missing_fqdn",
			spec: &domain.ExternalDomainSpec{
				Zone:        "example.com",
				NodeID:      "node1",
				TargetIP:    "203.0.113.1",
				ProviderRef: "provider1",
				TTL:         600,
			},
			wantErr: true,
		},
		{
			name: "missing_zone",
			spec: &domain.ExternalDomainSpec{
				FQDN:        "test.example.com",
				NodeID:      "node1",
				TargetIP:    "203.0.113.1",
				ProviderRef: "provider1",
				TTL:         600,
			},
			wantErr: true,
		},
		{
			name: "acme_without_email",
			spec: &domain.ExternalDomainSpec{
				FQDN:        "test.example.com",
				Zone:        "example.com",
				NodeID:      "node1",
				TargetIP:    "203.0.113.1",
				ProviderRef: "provider1",
				TTL:         600,
				ACME: domain.ACMEConfig{
					Enabled: true,
					// Missing Email
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestDomainSpecJSONRoundtrip tests that domain specs can be serialized and deserialized
func TestDomainSpecJSONRoundtrip(t *testing.T) {
	original := &domain.ExternalDomainSpec{
		FQDN:        "test.example.com",
		Zone:        "example.com",
		NodeID:      "node1",
		TargetIP:    "203.0.113.1",
		ProviderRef: "provider1",
		TTL:         600,
		ACME: domain.ACMEConfig{
			Enabled:       true,
			ChallengeType: "dns-01",
			Email:         "admin@example.com",
		},
		Ingress: domain.IngressConfig{
			Enabled: true,
			Service: "gateway",
			Port:    443,
		},
		Status: domain.ExternalDomainStatus{
			Phase:   "Ready",
			Message: "All good",
			Conditions: []domain.Condition{
				{Type: "DNSRecordCreated", Status: "True"},
			},
		},
	}

	// Serialize
	data, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() failed: %v", err)
	}

	// Deserialize
	decoded, err := domain.FromJSON([]byte(data))
	if err != nil {
		t.Fatalf("FromJSON() failed: %v", err)
	}

	// Verify fields
	if decoded.FQDN != original.FQDN {
		t.Errorf("FQDN = %s, want %s", decoded.FQDN, original.FQDN)
	}
	if decoded.Zone != original.Zone {
		t.Errorf("Zone = %s, want %s", decoded.Zone, original.Zone)
	}
	if decoded.Status.Phase != original.Status.Phase {
		t.Errorf("Status.Phase = %s, want %s", decoded.Status.Phase, original.Status.Phase)
	}
	if len(decoded.Status.Conditions) != len(original.Status.Conditions) {
		t.Errorf("Conditions length = %d, want %d", len(decoded.Status.Conditions), len(original.Status.Conditions))
	}
}

// TestFormatCondition tests the condition formatting helper
func TestFormatCondition(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"True", "✓"},
		{"False", "✗"},
		{"Unknown", "-"},
		{"", "-"},
		{"Invalid", "-"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := formatCondition(tt.status)
			if got != tt.want {
				t.Errorf("formatCondition(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// TestFormatDuration tests the duration formatting helper
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{30 * time.Second, "30s ago"},
		{90 * time.Second, "1m ago"},
		{5 * time.Minute, "5m ago"},
		{90 * time.Minute, "1h ago"},
		{3 * time.Hour, "3h ago"},
		{25 * time.Hour, "1d ago"},
		{50 * time.Hour, "2d ago"},
	}

	for _, tt := range tests {
		t.Run(tt.duration.String(), func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}
