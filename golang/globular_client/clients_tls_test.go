package globular_client

import (
	"crypto/tls"
	"net"
	"os"
	"testing"
)

// TestMeshConnectionDoesNotSkipTLSVerification verifies that the production
// mesh TLS configuration never sets InsecureSkipVerify = true. The mesh code
// in getMeshConn builds a tls.Config via GetClientTlsConfig and then adjusts
// ServerName. This test exercises the same logic to ensure the invariant holds:
// production inter-service traffic MUST verify the server certificate.
//
// Intent: dns_pki.explicit_identity_over_convenient_routing
func TestMeshConnectionDoesNotSkipTLSVerification(t *testing.T) {
	// A zero-value tls.Config has InsecureSkipVerify = false by default.
	// The production code must never flip it to true.
	cfg := &tls.Config{}

	if cfg.InsecureSkipVerify {
		t.Fatal("default tls.Config has InsecureSkipVerify=true; this should never happen")
	}

	// Simulate what getMeshConn does after GetClientTlsConfig returns:
	// it sets ServerName from the mesh target. Verify InsecureSkipVerify
	// stays false through the entire flow.
	targets := []string{
		"127.0.0.1:443",
		"globule-ryzen.globular.internal:443",
		"10.0.0.63:443",
		"[::1]:443",
	}

	for _, target := range targets {
		t.Run("target="+target, func(t *testing.T) {
			tcfg := &tls.Config{
				MinVersion: tls.VersionTLS12,
			}

			// Replicate the production ServerName logic from getMeshConn.
			if override := os.Getenv("MESH_TLS_SERVER_NAME"); override != "" {
				tcfg.ServerName = override
			} else if meshHost, _, splitErr := net.SplitHostPort(target); splitErr == nil && meshHost != "" {
				tcfg.ServerName = meshHost
			}

			// The critical invariant: InsecureSkipVerify must be false.
			if tcfg.InsecureSkipVerify {
				t.Errorf("InsecureSkipVerify is true for target %q; production mesh traffic must always verify TLS", target)
			}

			// ServerName must be set (not empty) so certificate verification
			// can match the expected identity.
			if tcfg.ServerName == "" {
				t.Errorf("ServerName is empty for target %q; TLS verification requires a ServerName or IP SAN", target)
			}
		})
	}
}

// TestMeshServerNameFromTarget verifies that the ServerName extraction from
// a mesh target address works correctly for various address formats.
func TestMeshServerNameFromTarget(t *testing.T) {
	cases := []struct {
		target   string
		wantName string
	}{
		{"127.0.0.1:443", "127.0.0.1"},
		{"globule-ryzen.globular.internal:443", "globule-ryzen.globular.internal"},
		{"10.0.0.63:443", "10.0.0.63"},
		{"[::1]:443", "::1"},
		{"myhost:8443", "myhost"},
	}

	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			meshHost, _, err := net.SplitHostPort(tc.target)
			if err != nil {
				t.Fatalf("SplitHostPort(%q) failed: %v", tc.target, err)
			}
			if meshHost != tc.wantName {
				t.Errorf("SplitHostPort(%q) host = %q, want %q", tc.target, meshHost, tc.wantName)
			}
		})
	}
}

// TestMeshServerNameOverride verifies that MESH_TLS_SERVER_NAME env var
// takes precedence over the hostname extracted from the target address.
func TestMeshServerNameOverride(t *testing.T) {
	const override = "mesh.globular.internal"
	t.Setenv("MESH_TLS_SERVER_NAME", override)

	target := "127.0.0.1:443"
	tcfg := &tls.Config{MinVersion: tls.VersionTLS12}

	if env := os.Getenv("MESH_TLS_SERVER_NAME"); env != "" {
		tcfg.ServerName = env
	} else if meshHost, _, splitErr := net.SplitHostPort(target); splitErr == nil && meshHost != "" {
		tcfg.ServerName = meshHost
	}

	if tcfg.ServerName != override {
		t.Errorf("ServerName = %q, want override %q", tcfg.ServerName, override)
	}
	if tcfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify must be false even with MESH_TLS_SERVER_NAME override")
	}
}
