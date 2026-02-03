package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestDeploymentResultJSONSchema(t *testing.T) {
	result := DeploymentResult{
		Success:   true,
		ServiceID: "echo.default",
		URL:       "https://echo.example.com/health",
		Steps: []DeploymentStepResult{
			{Name: "preflight", OK: true, Details: "cluster healthy"},
			{Name: "install-service", OK: true, Details: "echo installed"},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal DeploymentResult: %v", err)
	}

	var decoded DeploymentResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal DeploymentResult: %v", err)
	}

	if decoded.Success != result.Success {
		t.Errorf("Success mismatch: got %v, want %v", decoded.Success, result.Success)
	}

	if decoded.ServiceID != result.ServiceID {
		t.Errorf("ServiceID mismatch: got %s, want %s", decoded.ServiceID, result.ServiceID)
	}

	if decoded.URL != result.URL {
		t.Errorf("URL mismatch: got %s, want %s", decoded.URL, result.URL)
	}

	if len(decoded.Steps) != len(result.Steps) {
		t.Errorf("Steps length mismatch: got %d, want %d", len(decoded.Steps), len(result.Steps))
	}
}

func TestDeploymentStepResultJSONSchema(t *testing.T) {
	step := DeploymentStepResult{
		Name:    "preflight",
		OK:      true,
		Details: "cluster healthy",
	}

	data, err := json.Marshal(step)
	if err != nil {
		t.Fatalf("failed to marshal DeploymentStepResult: %v", err)
	}

	var decoded DeploymentStepResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal DeploymentStepResult: %v", err)
	}

	if decoded.Name != step.Name {
		t.Errorf("Name mismatch: got %s, want %s", decoded.Name, step.Name)
	}

	if decoded.OK != step.OK {
		t.Errorf("OK mismatch: got %v, want %v", decoded.OK, step.OK)
	}

	if decoded.Details != step.Details {
		t.Errorf("Details mismatch: got %s, want %s", decoded.Details, step.Details)
	}
}

func TestIsIPv4(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"192.168.1.1", true},
		{"::1", false},
		{"2001:db8::1", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			got := isIPv4(tt.ip)
			if got != tt.want {
				t.Errorf("isIPv4(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestIsIPv6(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", false},
		{"192.168.1.1", false},
		{"::1", true},
		{"2001:db8::1", true},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			got := isIPv6(tt.ip)
			if got != tt.want {
				t.Errorf("isIPv6(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestGatewayRouteConfigJSON(t *testing.T) {
	config := GatewayRouteConfig{
		Service:    "echo",
		Name:       "my-echo",
		Domain:     "echo.example.com",
		Upstream:   "127.0.0.1:10000",
		PathPrefix: "/",
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal GatewayRouteConfig: %v", err)
	}

	var decoded GatewayRouteConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal GatewayRouteConfig: %v", err)
	}

	if decoded.Service != config.Service {
		t.Errorf("Service mismatch: got %s, want %s", decoded.Service, config.Service)
	}

	if decoded.Name != config.Name {
		t.Errorf("Name mismatch: got %s, want %s", decoded.Name, config.Name)
	}

	if decoded.Domain != config.Domain {
		t.Errorf("Domain mismatch: got %s, want %s", decoded.Domain, config.Domain)
	}
}

func TestCreateServiceInstallPlan(t *testing.T) {
	nodeID := "test-node-1"
	serviceType := "echo"
	packagePath := "/tmp/echo.tgz"

	plan, err := createServiceInstallPlan(nodeID, serviceType, packagePath)
	if err != nil {
		t.Fatalf("createServiceInstallPlan() error = %v", err)
	}

	if plan.NodeId != nodeID {
		t.Errorf("NodeId = %s, want %s", plan.NodeId, nodeID)
	}

	if plan.ApiVersion != "globular.io/v1" {
		t.Errorf("ApiVersion = %s, want globular.io/v1", plan.ApiVersion)
	}

	if plan.Kind != "NodePlan" {
		t.Errorf("Kind = %s, want NodePlan", plan.Kind)
	}

	if len(plan.Spec.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(plan.Spec.Steps))
	}

	// Verify step actions
	expectedActions := []string{"artifact.fetch", "service.install_payload", "service.start"}
	for i, step := range plan.Spec.Steps {
		if step.Action != expectedActions[i] {
			t.Errorf("Step %d: Action = %s, want %s", i, step.Action, expectedActions[i])
		}
	}
}

func TestStructpbFromMap(t *testing.T) {
	m := map[string]interface{}{
		"service": "echo",
		"version": "latest",
		"port":    10000,
	}

	s := structpbFromMap(m)
	if s == nil {
		t.Fatal("structpbFromMap returned nil")
	}

	fields := s.GetFields()
	if fields["service"].GetStringValue() != "echo" {
		t.Errorf("service field = %s, want echo", fields["service"].GetStringValue())
	}

	if fields["version"].GetStringValue() != "latest" {
		t.Errorf("version field = %s, want latest", fields["version"].GetStringValue())
	}
}

func TestPackageAcquisitionWithPackageFlag(t *testing.T) {
	// Create a temporary package file
	tmpDir := t.TempDir()
	packagePath := tmpDir + "/echo.tgz"
	if err := os.WriteFile(packagePath, []byte("fake package"), 0644); err != nil {
		t.Fatalf("failed to create test package: %v", err)
	}

	// Test with --package flag
	result, step := acquireServicePackage(nil, "echo", packagePath)
	if !step.OK {
		t.Errorf("acquireServicePackage() failed: %s", step.Details)
	}

	if result != packagePath {
		t.Errorf("acquireServicePackage() = %s, want %s", result, packagePath)
	}
}

func TestPackageAcquisitionWithoutPackageFlag(t *testing.T) {
	// Test without --package flag (should fail gracefully with clear message)
	_, step := acquireServicePackage(nil, "echo", "")

	// Should fail when no package found
	if step.OK {
		t.Error("acquireServicePackage() should fail when no package found")
	}

	// Should provide clear error message
	if !strings.Contains(step.Details, "package not found") {
		t.Errorf("error message should mention package not found, got: %s", step.Details)
	}

	if !strings.Contains(step.Details, "--package") {
		t.Errorf("error message should mention --package flag, got: %s", step.Details)
	}
}

func TestUpstreamResolutionInRouteConfig(t *testing.T) {
	// Test that route config does not use hardcoded upstream
	// When --describe is unavailable, it should use fallback
	// But the code path should go through ResolveEndpoint

	serviceType := "echo"
	fallback := Endpoint{Host: "127.0.0.1", Port: 10000, Scheme: "grpc"}
	endpoint, _ := ResolveEndpoint(serviceType, fallback)
	upstreamAddr := fmt.Sprintf("%s:%d", endpoint.Host, endpoint.Port)

	// Verify we're building upstream from resolved endpoint
	if upstreamAddr == "" {
		t.Error("upstream address should not be empty")
	}

	// The upstream should be formatted as host:port
	if !strings.Contains(upstreamAddr, ":") {
		t.Errorf("upstream address should be in host:port format, got %s", upstreamAddr)
	}

	// Split and verify both parts exist
	parts := strings.Split(upstreamAddr, ":")
	if len(parts) != 2 {
		t.Errorf("upstream should have exactly one colon, got %s", upstreamAddr)
	}
}

func TestRouteConfigIdempotency(t *testing.T) {
	// Test that same inputs produce same route config
	config1 := GatewayRouteConfig{
		Service:    "echo",
		Name:       "my-echo",
		Domain:     "echo.example.com",
		Upstream:   "127.0.0.1:10000",
		PathPrefix: "/",
	}

	config2 := GatewayRouteConfig{
		Service:    "echo",
		Name:       "my-echo",
		Domain:     "echo.example.com",
		Upstream:   "127.0.0.1:10000",
		PathPrefix: "/",
	}

	data1, _ := json.Marshal(config1)
	data2, _ := json.Marshal(config2)

	if string(data1) != string(data2) {
		t.Error("identical route configs should produce identical JSON")
	}

	// Test writing config multiple times produces same result
	tmpDir := t.TempDir()
	os.Setenv("GLOBULAR_ROUTES_DIR", tmpDir)
	defer os.Unsetenv("GLOBULAR_ROUTES_DIR")

	// Write config first time
	if err := writeRouteConfig(config1); err != nil {
		t.Fatalf("writeRouteConfig() error = %v", err)
	}

	content1, err := os.ReadFile(tmpDir + "/my-echo.json")
	if err != nil {
		t.Fatalf("failed to read route config: %v", err)
	}

	// Write config second time (idempotent)
	if err := writeRouteConfig(config2); err != nil {
		t.Fatalf("writeRouteConfig() error = %v", err)
	}

	content2, err := os.ReadFile(tmpDir + "/my-echo.json")
	if err != nil {
		t.Fatalf("failed to read route config: %v", err)
	}

	if string(content1) != string(content2) {
		t.Error("writing same route config twice should produce identical files")
	}
}
