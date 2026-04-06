package config

import (
	"testing"
)

func TestValidateServiceEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		mode     ClusterMode
		epName   string
		ep       string
		wantErr  bool
	}{
		// Single-node mode allows anything
		{"single-node-localhost", ModeSingleNode, "controller", "localhost:12000", false},
		{"single-node-loopback", ModeSingleNode, "controller", "127.0.0.1:12000", false},
		{"single-node-remote", ModeSingleNode, "controller", "10.0.0.63:12000", false},
		{"single-node-empty", ModeSingleNode, "controller", "", false},

		// Cluster mode rejects loopback
		{"cluster-localhost", ModeCluster, "controller", "localhost:12000", true},
		{"cluster-loopback-ipv4", ModeCluster, "controller", "127.0.0.1:12000", true},
		{"cluster-loopback-ipv6", ModeCluster, "controller", "[::1]:12000", true},

		// Cluster mode allows routable addresses
		{"cluster-remote-ip", ModeCluster, "controller", "10.0.0.63:12000", false},
		{"cluster-dns", ModeCluster, "controller", "controller.globular.internal:12000", false},
		{"cluster-empty", ModeCluster, "controller", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateServiceEndpoint(tt.mode, tt.epName, tt.ep)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateServiceEndpoint(%v, %q, %q) error = %v, wantErr = %v",
					tt.mode, tt.epName, tt.ep, err, tt.wantErr)
			}
		})
	}
}

func TestValidateServiceEndpoints(t *testing.T) {
	// Cluster mode with one bad endpoint should fail
	err := ValidateServiceEndpoints(ModeCluster, map[string]string{
		"event":    "10.0.0.63:10002",
		"workflow": "localhost:10220", // bad
	})
	if err == nil {
		t.Error("expected error for loopback endpoint in cluster mode")
	}

	// All routable should pass
	err = ValidateServiceEndpoints(ModeCluster, map[string]string{
		"event":    "10.0.0.63:10002",
		"workflow": "workflow.globular.internal:10220",
	})
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}
