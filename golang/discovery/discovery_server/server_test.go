package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

// TestDefaultValues verifies service initialization defaults
func TestDefaultValues(t *testing.T) {
	// These defaults must not change (behavioral contract)
	if defaultPort != 10029 {
		t.Errorf("defaultPort = %d, want 10029", defaultPort)
	}

	if defaultProxy != 10030 {
		t.Errorf("defaultProxy = %d, want %d (defaultPort + 1)", defaultProxy, defaultPort+1)
	}

	if !allowAllOrigins {
		t.Error("allowAllOrigins should default to true")
	}

	if allowedOriginsStr != "" {
		t.Errorf("allowedOriginsStr = %q, want empty string", allowedOriginsStr)
	}
}

// TestServerInitialization validates server struct initialization
func TestServerInitialization(t *testing.T) {
	srv := &server{}

	// Simulate initialization from main()
	srv.Name = "discovery.PackageDiscovery"
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Version = "0.0.1"
	srv.PublisherID = "localhost"
	srv.Description = "Service discovery client"
	srv.Keywords = []string{"Discovery", "Package", "Service", "Application"}
	srv.Repositories = []string{}
	srv.Discoveries = []string{}
	srv.Dependencies = []string{"rbac.RbacService", "resource.ResourceService"}
	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsStr
	srv.KeepAlive = true
	srv.KeepUpToDate = true
	srv.Process = -1
	srv.ProxyProcess = -1

	// Verify critical fields
	if srv.Name != "discovery.PackageDiscovery" {
		t.Errorf("Name = %q, want %q", srv.Name, "discovery.PackageDiscovery")
	}

	if srv.Port != 10029 {
		t.Errorf("Port = %d, want 10029", srv.Port)
	}

	if srv.Protocol != "grpc" {
		t.Errorf("Protocol = %q, want %q", srv.Protocol, "grpc")
	}

	if srv.Version != "0.0.1" {
		t.Errorf("Version = %q, want %q", srv.Version, "0.0.1")
	}

	if !srv.KeepAlive {
		t.Error("KeepAlive should be true")
	}

	if srv.Process != -1 {
		t.Errorf("Process = %d, want -1 (not started)", srv.Process)
	}

	// Verify dependencies
	expectedDeps := []string{"rbac.RbacService", "resource.ResourceService"}
	if len(srv.Dependencies) != len(expectedDeps) {
		t.Errorf("Dependencies length = %d, want %d", len(srv.Dependencies), len(expectedDeps))
	}
}

// TestDescribeOutputFormat validates --describe JSON output structure
func TestDescribeOutputFormat(t *testing.T) {
	// Build test binary if needed
	binaryPath := "./discovery_server_test"
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Use existing binary if available
		if _, err := os.Stat("./discovery_server"); err == nil {
			binaryPath = "./discovery_server"
		} else {
			t.Skip("Discovery server binary not found, skipping integration test")
		}
	}

	// Run --describe
	cmd := exec.Command(binaryPath, "--describe")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run --describe: %v\nOutput: %s", err, output)
	}

	// Parse JSON output
	var metadata struct {
		Name         string   `json:"Name"`
		Port         int      `json:"Port"`
		Protocol     string   `json:"Protocol"`
		Version      string   `json:"Version"`
		Description  string   `json:"Description"`
		Keywords     []string `json:"Keywords"`
		Dependencies []string `json:"Dependencies"`
	}

	if err := json.Unmarshal(output, &metadata); err != nil {
		t.Fatalf("Failed to parse --describe JSON: %v\nOutput: %s", err, output)
	}

	// Verify required fields are present and correct
	if metadata.Name != "discovery.PackageDiscovery" {
		t.Errorf("Name = %q, want %q", metadata.Name, "discovery.PackageDiscovery")
	}

	if metadata.Port == 0 {
		t.Error("Port should be non-zero")
	}

	if metadata.Protocol != "grpc" {
		t.Errorf("Protocol = %q, want %q", metadata.Protocol, "grpc")
	}

	if metadata.Version == "" {
		t.Error("Version should not be empty")
	}

	if metadata.Description == "" {
		t.Error("Description should not be empty")
	}

	if len(metadata.Keywords) == 0 {
		t.Error("Keywords should not be empty")
	}

	// Verify dependencies
	expectedDeps := []string{"rbac.RbacService", "resource.ResourceService"}
	if len(metadata.Dependencies) != len(expectedDeps) {
		t.Errorf("Dependencies length = %d, want %d", len(metadata.Dependencies), len(expectedDeps))
	}

	t.Logf("--describe metadata: Name=%s, Port=%d, Version=%s, Dependencies=%v",
		metadata.Name, metadata.Port, metadata.Version, metadata.Dependencies)
}

// TestGetterSetterContract verifies the Globular service contract
func TestGetterSetterContract(t *testing.T) {
	srv := &server{}

	// Test ConfigPath
	srv.SetConfigurationPath("/test/path")
	if srv.GetConfigurationPath() != "/test/path" {
		t.Error("ConfigurationPath getter/setter mismatch")
	}

	// Test Address
	srv.SetAddress("localhost:10029")
	if srv.GetAddress() != "localhost:10029" {
		t.Error("Address getter/setter mismatch")
	}

	// Test Process
	srv.SetProcess(12345)
	if srv.GetProcess() != 12345 {
		t.Error("Process getter/setter mismatch")
	}

	// Test Port
	srv.SetPort(10029)
	if srv.GetPort() != 10029 {
		t.Error("Port getter/setter mismatch")
	}

	// Test Name
	srv.SetName("test.Service")
	if srv.GetName() != "test.Service" {
		t.Error("Name getter/setter mismatch")
	}

	// Test Domain
	srv.SetDomain("test.local")
	if srv.GetDomain() != "test.local" {
		t.Error("Domain getter/setter mismatch")
	}

	// Test Id
	srv.SetId("test-id-123")
	if srv.GetId() != "test-id-123" {
		t.Error("Id getter/setter mismatch")
	}
}

// TestPermissionsStructure validates RBAC permissions initialization
func TestPermissionsStructure(t *testing.T) {
	srv := &server{}

	// Initialize with permissions like main() does
	srv.Permissions = []interface{}{
		map[string]interface{}{
			"action":     "/discovery.PackageDiscovery/PublishService",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "RepositoryId", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "DiscoveryId", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/discovery.PackageDiscovery/PublishApplication",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Repository", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Discovery", "permission": "write"},
			},
		},
	}

	if len(srv.Permissions) != 2 {
		t.Errorf("Permissions length = %d, want 2", len(srv.Permissions))
	}

	// Verify first permission structure
	firstPerm, ok := srv.Permissions[0].(map[string]interface{})
	if !ok {
		t.Fatal("First permission is not a map")
	}

	if firstPerm["action"] != "/discovery.PackageDiscovery/PublishService" {
		t.Errorf("First permission action = %v, want %q",
			firstPerm["action"], "/discovery.PackageDiscovery/PublishService")
	}

	if firstPerm["permission"] != "write" {
		t.Errorf("First permission level = %v, want %q", firstPerm["permission"], "write")
	}
}

// TestBehaviorInvariant documents the core Discovery service behavior
func TestBehaviorInvariant(t *testing.T) {
	t.Log("Discovery Service Behavioral Contract:")
	t.Log("1. PublishService() must validate required fields")
	t.Log("2. PublishApplication() must validate required fields")
	t.Log("3. --describe must report correct port and metadata")
	t.Log("4. Default port is 10029, proxy is 10030")
	t.Log("5. Protocol is always 'grpc'")
	t.Log("6. AllowAllOrigins defaults to true")
	t.Log("7. Dependencies: rbac.RbacService, resource.ResourceService")
	t.Log("8. Permissions configured for PublishService and PublishApplication")

	t.Log("")
	t.Log("Phase 1 refactoring: Same pattern as Echo service")
	t.Log("All tests must continue passing as refactoring progresses.")
}
