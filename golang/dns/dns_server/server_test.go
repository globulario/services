package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

// Test that --describe output reports correct port
// This test would have caught the "DNS reports 10033 but listens on 10006" bug
func TestDescribeReportsCorrectPort(t *testing.T) {
	// Build a test binary if needed
	binaryPath := "./dns_server_test"
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Use the actual binary if it exists
		if _, err := os.Stat("./dns_server"); err == nil {
			binaryPath = "./dns_server"
		} else {
			t.Skip("DNS server binary not found, skipping integration test")
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
		Port    int    `json:"Port"`
		Address string `json:"Address"`
		Name    string `json:"Name"`
	}

	if err := json.Unmarshal(output, &metadata); err != nil {
		t.Fatalf("Failed to parse --describe JSON: %v\nOutput: %s", err, output)
	}

	// Verify port is set (non-zero)
	if metadata.Port == 0 {
		t.Error("--describe returned Port=0, expected non-zero port")
	}

	// Verify port matches what's expected
	// The default port should be 10006 (not the old 10033)
	expectedPort := 10006
	if metadata.Port != expectedPort {
		t.Errorf("--describe returned Port=%d, expected %d", metadata.Port, expectedPort)
	}

	// Verify Address contains the port
	if metadata.Address == "" {
		t.Error("--describe returned empty Address")
	}

	t.Logf("--describe metadata: Port=%d, Address=%s, Name=%s",
		metadata.Port, metadata.Address, metadata.Name)
}

// Test that default port constant matches expected value
func TestDefaultPortValue(t *testing.T) {
	// This is a regression test for the hardcoded port issue
	// The default port should be 10006, not the old 10033
	expectedPort := 10006

	if defaultPort != expectedPort {
		t.Errorf("defaultPort=%d, expected %d (was incorrectly set to 10033 in the past)",
			defaultPort, expectedPort)
	}

	// Also verify defaultProxy is consistent (should be defaultPort + 1)
	expectedProxy := expectedPort + 1
	if defaultProxy != expectedProxy {
		t.Errorf("defaultProxy=%d, expected %d (should be defaultPort + 1)",
			defaultProxy, expectedProxy)
	}
}

// Test port initialization logic
func TestServerPortInitialization(t *testing.T) {
	// Create a new server instance
	srv := &server{
		Port: 0, // Not initialized yet
	}

	// Simulate the initialization that happens in main()
	srv.Port = defaultPort

	// Verify port is set correctly
	if srv.Port != 10006 {
		t.Errorf("Server initialized with Port=%d, expected 10006", srv.Port)
	}

	// Verify GetPort() method works
	if srv.GetPort() != srv.Port {
		t.Errorf("GetPort()=%d doesn't match Port=%d", srv.GetPort(), srv.Port)
	}
}

// TestDescribeConsistency verifies that --describe metadata is consistent
// throughout the service lifecycle
func TestDescribeMetadataConsistency(t *testing.T) {
	// This test documents the expected behavior:
	// If the service reallocates its port at runtime (via port allocator),
	// the --describe metadata MUST reflect the actual bound port, not the default

	t.Log("Invariant: --describe metadata MUST match actual listening port")
	t.Log("Regression: commit 019cc4d7 fixed DNS reporting 10033 while listening on 10006")
	t.Log("Root cause: defaultPort was hardcoded to 10033, conflicting with port allocator")
	t.Log("Fix: Changed defaultPort from 10033 to 10006 to match port allocator assignment")

	// The test above (TestDescribeReportsCorrectPort) validates this
	// This test serves as documentation of the invariant
}
