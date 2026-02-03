package main

import (
	"encoding/json"
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

func TestResolveServiceEndpointFallback(t *testing.T) {
	// Test fallback to default ports when no config is available
	tests := []struct {
		serviceID string
		wantHost  string
		wantPort  int
	}{
		{"etcd", "127.0.0.1", 2379},
		{"scylla", "127.0.0.1", 9042},
		{"minio", "127.0.0.1", 9000},
		{"envoy-admin", "127.0.0.1", 9901},
		{"dns", "localhost", 10033},
	}

	for _, tt := range tests {
		t.Run(tt.serviceID, func(t *testing.T) {
			endpoint, err := resolveServiceEndpoint(tt.serviceID)
			if err != nil {
				t.Fatalf("resolveServiceEndpoint(%q) error = %v", tt.serviceID, err)
			}

			if endpoint.Host != tt.wantHost {
				t.Errorf("Host = %s, want %s", endpoint.Host, tt.wantHost)
			}

			if endpoint.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", endpoint.Port, tt.wantPort)
			}
		})
	}
}

func TestResolveServiceEndpointUnknown(t *testing.T) {
	// Test that unknown service returns error
	_, err := resolveServiceEndpoint("unknown-service-xyz")
	if err == nil {
		t.Error("expected error for unknown service, got nil")
	}
}
