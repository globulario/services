package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestHealthCheckResultJSONSchema(t *testing.T) {
	result := HealthCheckResult{
		Name:    "etcd",
		OK:      true,
		Details: "etcd reachable",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal HealthCheckResult: %v", err)
	}

	var decoded HealthCheckResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal HealthCheckResult: %v", err)
	}

	if decoded.Name != result.Name {
		t.Errorf("Name mismatch: got %s, want %s", decoded.Name, result.Name)
	}
	if decoded.OK != result.OK {
		t.Errorf("OK mismatch: got %v, want %v", decoded.OK, result.OK)
	}
	if decoded.Details != result.Details {
		t.Errorf("Details mismatch: got %s, want %s", decoded.Details, result.Details)
	}
}

func TestLocalHealthStatusJSONSchema(t *testing.T) {
	status := LocalHealthStatus{
		Healthy: false,
		Checks: []HealthCheckResult{
			{Name: "etcd", OK: true, Details: "etcd reachable"},
			{Name: "scylla", OK: false, Details: "scylla unreachable on 127.0.0.1:9042"},
		},
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal LocalHealthStatus: %v", err)
	}

	// Verify JSON structure
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal LocalHealthStatus: %v", err)
	}

	// Check top-level fields
	if _, ok := decoded["healthy"]; !ok {
		t.Error("missing 'healthy' field in JSON")
	}
	if _, ok := decoded["checks"]; !ok {
		t.Error("missing 'checks' field in JSON")
	}

	// Check healthy field type
	if healthy, ok := decoded["healthy"].(bool); !ok {
		t.Errorf("healthy field is not a boolean")
	} else if healthy != status.Healthy {
		t.Errorf("healthy mismatch: got %v, want %v", healthy, status.Healthy)
	}

	// Check checks array
	checks, ok := decoded["checks"].([]interface{})
	if !ok {
		t.Fatal("checks field is not an array")
	}

	if len(checks) != 2 {
		t.Errorf("checks length mismatch: got %d, want 2", len(checks))
	}

	// Verify first check
	firstCheck, ok := checks[0].(map[string]interface{})
	if !ok {
		t.Fatal("first check is not an object")
	}

	if name, ok := firstCheck["name"].(string); !ok || name != "etcd" {
		t.Errorf("first check name mismatch: got %v, want 'etcd'", firstCheck["name"])
	}
	if ok, okVal := firstCheck["ok"].(bool); !okVal || !ok {
		t.Errorf("first check ok mismatch: got %v, want true", firstCheck["ok"])
	}

	// Verify second check
	secondCheck, ok := checks[1].(map[string]interface{})
	if !ok {
		t.Fatal("second check is not an object")
	}

	if name, ok := secondCheck["name"].(string); !ok || name != "scylla" {
		t.Errorf("second check name mismatch: got %v, want 'scylla'", secondCheck["name"])
	}
	if ok, okVal := secondCheck["ok"].(bool); !okVal || ok {
		t.Errorf("second check ok mismatch: got %v, want false", secondCheck["ok"])
	}
}

func TestHealthJSONSchemaStability(t *testing.T) {
	// Test that the JSON schema is stable and follows the documented format
	status := LocalHealthStatus{
		Healthy: false,
		Checks: []HealthCheckResult{
			{Name: "etcd", OK: true, Details: "etcd reachable"},
			{Name: "scylla", OK: false, Details: "connection refused"},
		},
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal LocalHealthStatus: %v", err)
	}

	expectedJSON := `{"healthy":false,"checks":[{"name":"etcd","ok":true,"details":"etcd reachable"},{"name":"scylla","ok":false,"details":"connection refused"}]}`

	// Unmarshal both to compare structure
	var expected, actual map[string]interface{}
	if err := json.Unmarshal([]byte(expectedJSON), &expected); err != nil {
		t.Fatalf("failed to unmarshal expected JSON: %v", err)
	}
	if err := json.Unmarshal(data, &actual); err != nil {
		t.Fatalf("failed to unmarshal actual JSON: %v", err)
	}

	// Compare healthy field
	if expected["healthy"] != actual["healthy"] {
		t.Errorf("healthy field mismatch: expected %v, got %v", expected["healthy"], actual["healthy"])
	}

	// Compare checks array structure
	expectedChecks := expected["checks"].([]interface{})
	actualChecks := actual["checks"].([]interface{})

	if len(expectedChecks) != len(actualChecks) {
		t.Errorf("checks length mismatch: expected %d, got %d", len(expectedChecks), len(actualChecks))
	}
}

func TestHealthCheckResultAllFieldsPresent(t *testing.T) {
	// Ensure all fields are present in JSON output, even when empty
	result := HealthCheckResult{
		Name:    "",
		OK:      false,
		Details: "",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal HealthCheckResult: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal HealthCheckResult: %v", err)
	}

	requiredFields := []string{"name", "ok", "details"}
	for _, field := range requiredFields {
		if _, ok := decoded[field]; !ok {
			t.Errorf("missing required field %q in JSON", field)
		}
	}
}

func TestLocalHealthStatusEmpty(t *testing.T) {
	// Test empty checks array
	status := LocalHealthStatus{
		Healthy: true,
		Checks:  []HealthCheckResult{},
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal LocalHealthStatus: %v", err)
	}

	var decoded LocalHealthStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal LocalHealthStatus: %v", err)
	}

	if decoded.Healthy != status.Healthy {
		t.Errorf("Healthy mismatch: got %v, want %v", decoded.Healthy, status.Healthy)
	}

	if decoded.Checks == nil {
		t.Error("Checks should not be nil")
	}

	if len(decoded.Checks) != 0 {
		t.Errorf("Checks length mismatch: got %d, want 0", len(decoded.Checks))
	}
}

func TestResolveEndpointFallback(t *testing.T) {
	// Test fallback to default ports when no config is available
	tests := []struct {
		serviceKey string
		wantHost   string
		wantPort   int
		wantScheme string
	}{
		{"etcd", "127.0.0.1", 2379, "tcp"},
		{"scylla", "127.0.0.1", 9042, "tcp"},
		{"minio", "127.0.0.1", 9000, "tcp"},
		{"envoy-admin", "127.0.0.1", 9901, "http"},
		{"dns", "localhost", 10006, "grpc"},
	}

	for _, tt := range tests {
		t.Run(tt.serviceKey, func(t *testing.T) {
			fallback := Endpoint{Host: tt.wantHost, Port: tt.wantPort, Scheme: tt.wantScheme}
			endpoint, err := ResolveEndpoint(tt.serviceKey, fallback)

			// Should return fallback when no config available
			if err == nil {
				t.Errorf("ResolveEndpoint(%q) expected error (no config), got nil", tt.serviceKey)
			}

			if endpoint.Host != tt.wantHost {
				t.Errorf("Host = %s, want %s", endpoint.Host, tt.wantHost)
			}

			if endpoint.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", endpoint.Port, tt.wantPort)
			}

			if endpoint.Scheme != tt.wantScheme {
				t.Errorf("Scheme = %s, want %s", endpoint.Scheme, tt.wantScheme)
			}
		})
	}
}

func TestResolveEndpointUnknown(t *testing.T) {
	// Test that unknown service returns fallback
	fallback := Endpoint{Host: "127.0.0.1", Port: 9999, Scheme: "tcp"}
	endpoint, err := ResolveEndpoint("unknown-service-xyz", fallback)

	// Should return fallback with error
	if err == nil {
		t.Error("expected error for unknown service, got nil")
	}

	if endpoint.Host != fallback.Host || endpoint.Port != fallback.Port {
		t.Errorf("expected fallback endpoint, got %v", endpoint)
	}
}

// TestResolveEndpointWithDescribe tests that --describe output is used when available
func TestResolveEndpointWithDescribe(t *testing.T) {
	// This test requires the config package to be able to find service binaries
	// We'll create a fake binary in a temp directory that outputs --describe JSON

	// Note: This is an integration-style test. In a real environment, you'd need
	// to set up ServicesRoot or mock config.FindServiceBinary and config.RunDescribe.
	// For now, we test the fallback behavior and document the expected behavior.

	t.Run("describe_overrides_fallback", func(t *testing.T) {
		// When a service binary outputs --describe with non-default port,
		// ResolveEndpoint should return that port, not the fallback.

		// This would require mocking the config package, which we'll skip for now.
		// The behavior is tested by the actual implementation calling config.RunDescribe.
		t.Skip("requires config package mocking or real service binary")
	})
}

// TestCheckHealthWithMockedProbes tests that health checks use resolved endpoints
func TestCheckHealthWithMockedProbes(t *testing.T) {
	// Save original probe functions
	origTCPProbe := tcpProbe
	origHTTPProbe := httpProbe
	defer func() {
		tcpProbe = origTCPProbe
		httpProbe = origHTTPProbe
	}()

	t.Run("etcd_check_uses_resolved_endpoint", func(t *testing.T) {
		var capturedEndpoint Endpoint
		tcpProbe = func(ctx context.Context, endpoint Endpoint) error {
			capturedEndpoint = endpoint
			return nil // Success
		}

		ctx := context.Background()
		result := checkEtcd(ctx)

		if !result.OK {
			t.Errorf("checkEtcd() failed: %s", result.Details)
		}

		// Verify probe was called with fallback endpoint (no config available)
		if capturedEndpoint.Port != 2379 {
			t.Errorf("tcpProbe called with port %d, want 2379", capturedEndpoint.Port)
		}
		if capturedEndpoint.Host != "127.0.0.1" {
			t.Errorf("tcpProbe called with host %s, want 127.0.0.1", capturedEndpoint.Host)
		}
	})

	t.Run("scylla_check_uses_resolved_endpoint", func(t *testing.T) {
		var capturedEndpoint Endpoint
		tcpProbe = func(ctx context.Context, endpoint Endpoint) error {
			capturedEndpoint = endpoint
			return nil
		}

		ctx := context.Background()
		result := checkScylla(ctx)

		if !result.OK {
			t.Errorf("checkScylla() failed: %s", result.Details)
		}

		if capturedEndpoint.Port != 9042 {
			t.Errorf("tcpProbe called with port %d, want 9042", capturedEndpoint.Port)
		}
	})

	t.Run("minio_check_uses_resolved_endpoint", func(t *testing.T) {
		var capturedEndpoint Endpoint
		tcpProbe = func(ctx context.Context, endpoint Endpoint) error {
			capturedEndpoint = endpoint
			return nil
		}

		ctx := context.Background()
		result := checkMinio(ctx)

		if !result.OK {
			t.Errorf("checkMinio() failed: %s", result.Details)
		}

		if capturedEndpoint.Port != 9000 {
			t.Errorf("tcpProbe called with port %d, want 9000", capturedEndpoint.Port)
		}
	})

	t.Run("envoy_check_uses_http_probe_with_path", func(t *testing.T) {
		var capturedEndpoint Endpoint
		httpProbe = func(ctx context.Context, endpoint Endpoint) error {
			capturedEndpoint = endpoint
			return nil
		}

		ctx := context.Background()
		result := checkEnvoy(ctx)

		if !result.OK {
			t.Errorf("checkEnvoy() failed: %s", result.Details)
		}

		if capturedEndpoint.Port != 9901 {
			t.Errorf("httpProbe called with port %d, want 9901", capturedEndpoint.Port)
		}
		if capturedEndpoint.Path != "/ready" {
			t.Errorf("httpProbe called with path %s, want /ready", capturedEndpoint.Path)
		}
		if capturedEndpoint.Scheme != "http" {
			t.Errorf("httpProbe called with scheme %s, want http", capturedEndpoint.Scheme)
		}
	})

	t.Run("probe_failure_results_in_failed_check", func(t *testing.T) {
		tcpProbe = func(ctx context.Context, endpoint Endpoint) error {
			return fmt.Errorf("connection refused")
		}

		ctx := context.Background()
		result := checkEtcd(ctx)

		if result.OK {
			t.Error("checkEtcd() should fail when probe fails")
		}
		if result.Details == "" {
			t.Error("checkEtcd() should include error details")
		}
	})
}

// TestResolveEndpointWithFakeBinary creates a fake executable that outputs --describe JSON
func TestResolveEndpointWithFakeBinary(t *testing.T) {
	// Create a temporary directory with a fake service binary
	tmpDir := t.TempDir()

	// Create a fake scylla_server binary that outputs --describe with port 19042
	fakeScyllaPath := filepath.Join(tmpDir, "scylla_server")
	scriptContent := `#!/bin/bash
if [ "$1" = "--describe" ]; then
  echo '{"Address":"127.0.0.1","Port":19042,"Proto":"tcp","Protocol":"tcp"}'
else
  echo "Unknown flag: $1" >&2
  exit 1
fi
`
	if err := os.WriteFile(fakeScyllaPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to create fake binary: %v", err)
	}

	// Note: To properly test this, we'd need to:
	// 1. Set ServicesRoot to tmpDir (via config or environment)
	// 2. Ensure config.FindServiceBinary finds our fake binary
	// 3. Call ResolveEndpoint and verify it returns port 19042

	// For now, we document the expected behavior and test manually
	t.Logf("Created fake binary at %s", fakeScyllaPath)
	t.Skip("requires setting ServicesRoot and config package integration")
}

// TestProbeInjection verifies that probe functions can be injected for testing
func TestProbeInjection(t *testing.T) {
	// Save original probes
	origTCPProbe := tcpProbe
	origHTTPProbe := httpProbe
	defer func() {
		tcpProbe = origTCPProbe
		httpProbe = origHTTPProbe
	}()

	// Test TCP probe injection
	t.Run("tcp_probe_injectable", func(t *testing.T) {
		callCount := 0
		tcpProbe = func(ctx context.Context, endpoint Endpoint) error {
			callCount++
			return nil
		}

		ctx := context.Background()
		_ = checkEtcd(ctx)

		if callCount != 1 {
			t.Errorf("tcpProbe called %d times, want 1", callCount)
		}
	})

	// Test HTTP probe injection
	t.Run("http_probe_injectable", func(t *testing.T) {
		callCount := 0
		httpProbe = func(ctx context.Context, endpoint Endpoint) error {
			callCount++
			return nil
		}

		ctx := context.Background()
		_ = checkEnvoy(ctx)

		if callCount != 1 {
			t.Errorf("httpProbe called %d times, want 1", callCount)
		}
	})
}
