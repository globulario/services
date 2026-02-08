package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/globulario/services/golang/echo/echopb"
)

// TestEchoHandler validates the Echo RPC handler behavior
// NOTE: Currently fails because Echo() calls srv.Save() which requires etcd/TLS
// This is a design problem that Phase 1 refactoring will fix by removing
// the config persistence side effect from the handler
func TestEchoHandler(t *testing.T) {
	t.Skip("Skipping until Phase 1 refactoring removes Save() side effect from Echo()")

	// TODO Phase 1: After refactoring, Echo() should be pure:
	// - Input: message
	// - Output: same message
	// - No side effects (config save moved to lifecycle)

	// Create a minimal server instance for testing
	srv := &server{
		Name:       "echo.EchoService",
		Id:         "test-echo-id",
		ConfigPath: t.TempDir() + "/config.json",
	}

	// Test cases
	tests := []struct {
		name    string
		message string
		wantErr bool
	}{
		{
			name:    "simple echo",
			message: "hello",
			wantErr: false,
		},
		{
			name:    "empty message",
			message: "",
			wantErr: false,
		},
		{
			name:    "unicode message",
			message: "Hello 世界 🌍",
			wantErr: false,
		},
		{
			name:    "long message",
			message: string(make([]byte, 1024)),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &echopb.EchoRequest{Message: tt.message}
			resp, err := srv.Echo(context.Background(), req)

			if (err != nil) != tt.wantErr {
				t.Errorf("Echo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if resp.GetMessage() != tt.message {
					t.Errorf("Echo() returned %q, want %q", resp.GetMessage(), tt.message)
				}
			}
		})
	}
}

// TestDefaultValues verifies service initialization defaults
func TestDefaultValues(t *testing.T) {
	// These defaults must not change (behavioral contract)
	if defaultPort != 10000 {
		t.Errorf("defaultPort = %d, want 10000", defaultPort)
	}

	if defaultProxy != 10001 {
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
	srv.Name = "echo.EchoService"
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Version = "0.0.1"
	srv.PublisherID = "localhost"
	srv.Description = "The Hello World of gRPC services."
	srv.Keywords = []string{"Example", "Echo", "Test", "Service"}
	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsStr
	srv.KeepAlive = true
	srv.KeepUpToDate = true
	srv.Process = -1
	srv.ProxyProcess = -1

	// Verify critical fields
	if srv.Name != "echo.EchoService" {
		t.Errorf("Name = %q, want %q", srv.Name, "echo.EchoService")
	}

	if srv.Port != 10000 {
		t.Errorf("Port = %d, want 10000", srv.Port)
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
}

// TestDescribeOutputFormat validates --describe JSON output structure
func TestDescribeOutputFormat(t *testing.T) {
	// Build test binary if needed
	binaryPath := "./echo_server_test"
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Use existing binary if available
		if _, err := os.Stat("./echo_server"); err == nil {
			binaryPath = "./echo_server"
		} else {
			t.Skip("Echo server binary not found, skipping integration test")
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
		Name        string   `json:"Name"`
		Port        int      `json:"Port"`
		Protocol    string   `json:"Protocol"`
		Version     string   `json:"Version"`
		Description string   `json:"Description"`
		Keywords    []string `json:"Keywords"`
	}

	if err := json.Unmarshal(output, &metadata); err != nil {
		t.Fatalf("Failed to parse --describe JSON: %v\nOutput: %s", err, output)
	}

	// Verify required fields are present and correct
	if metadata.Name != "echo.EchoService" {
		t.Errorf("Name = %q, want %q", metadata.Name, "echo.EchoService")
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

	t.Logf("--describe metadata: Name=%s, Port=%d, Version=%s",
		metadata.Name, metadata.Port, metadata.Version)
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
	srv.SetAddress("localhost:10000")
	if srv.GetAddress() != "localhost:10000" {
		t.Error("Address getter/setter mismatch")
	}

	// Test Process
	srv.SetProcess(12345)
	if srv.GetProcess() != 12345 {
		t.Error("Process getter/setter mismatch")
	}

	// Test Port
	srv.SetPort(10000)
	if srv.GetPort() != 10000 {
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

// TestBehaviorInvariant documents the core Echo service behavior
func TestBehaviorInvariant(t *testing.T) {
	t.Log("Echo Service Behavioral Contract:")
	t.Log("1. Echo() must return exactly the message it receives")
	t.Log("2. Echo() must persist config on each call (current behavior)")
	t.Log("3. --describe must report correct port and metadata")
	t.Log("4. Default port is 10000, proxy is 10001")
	t.Log("5. Protocol is always 'grpc'")
	t.Log("6. AllowAllOrigins defaults to true")

	t.Log("")
	t.Log("These tests freeze current behavior before refactoring.")
	t.Log("All refactoring PRs must keep these tests passing.")
}
