package main

import (
	"testing"
)

// TestPublishMinioConfigRejectsDNS verifies that resolveMinioEndpointLocked
// returns "" when MinioPoolNodes[0] is a DNS hostname rather than a bare IP.
// DNS wildcards (minio.<domain>) resolve round-robin to all cluster nodes,
// most of which have empty per-node MinIO instances, causing silent
// object-not-found errors. The function must refuse to produce such endpoints.
func TestPublishMinioConfigRejectsDNS(t *testing.T) {
	cases := []struct {
		name     string
		poolNode string
	}{
		{"wildcard hostname", "minio.globular.internal"},
		{"plain hostname", "minio-primary"},
		{"fqdn with port stripped", "node.globular.internal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := &server{
				state: &controllerState{
					MinioPoolNodes: []string{tc.poolNode},
				},
			}
			endpoint := srv.resolveMinioEndpointLocked()
			if endpoint != "" {
				t.Fatalf("resolveMinioEndpointLocked(%q): expected empty string for DNS name, got %q — DNS endpoints must be rejected", tc.poolNode, endpoint)
			}
		})
	}
}

// TestPublishMinioConfigUsesPoolNodeIP verifies that resolveMinioEndpointLocked
// returns "IP:9000" when MinioPoolNodes[0] is a valid routable IP address.
// Index 0 is the founding node — the stable MinIO primary for all consumer connections.
func TestPublishMinioConfigUsesPoolNodeIP(t *testing.T) {
	cases := []struct {
		name     string
		poolNode string
		want     string
	}{
		{"IPv4 address", "10.0.0.63", "10.0.0.63:9000"},
		{"IPv4 VIP", "10.0.0.100", "10.0.0.100:9000"},
		{"IPv6 address", "fd00::1", "[fd00::1]:9000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := &server{
				state: &controllerState{
					MinioPoolNodes: []string{tc.poolNode},
				},
			}
			endpoint := srv.resolveMinioEndpointLocked()
			if endpoint != tc.want {
				t.Fatalf("resolveMinioEndpointLocked(%q): expected %q, got %q", tc.poolNode, tc.want, endpoint)
			}
		})
	}
}

// TestResolveMinioEndpointEmptyPool verifies that resolveMinioEndpointLocked
// returns "" when the pool has no nodes yet (pre-bootstrap state).
// publishMinioConfigLocked must not publish an empty endpoint.
func TestResolveMinioEndpointEmptyPool(t *testing.T) {
	srv := &server{
		state: &controllerState{
			MinioPoolNodes: nil,
		},
	}
	endpoint := srv.resolveMinioEndpointLocked()
	if endpoint != "" {
		t.Fatalf("expected empty endpoint for empty pool, got %q", endpoint)
	}
}
