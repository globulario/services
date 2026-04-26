package main

import (
	"testing"

	"github.com/globulario/services/golang/config"
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

// ── buildObjectStoreDesiredStateLocked ────────────────────────────────────────

// TestBuildObjectStore_NilCredentials_PublishesDegradedContract verifies that
// buildObjectStoreDesiredStateLocked does NOT skip when credentials are nil.
// It must publish a contract with CredentialsReady=false instead of silently
// leaving /globular/objectstore/config absent.
func TestBuildObjectStore_NilCredentials_PublishesDegradedContract(t *testing.T) {
	srv := &server{
		state: &controllerState{
			MinioPoolNodes:        []string{"10.0.0.63"},
			ObjectStoreGeneration: 1,
			MinioCredentials:      nil, // not yet loaded from disk
		},
	}
	desired, skip := srv.buildObjectStoreDesiredStateLocked()
	if skip {
		t.Fatal("expected contract to be built when pool has nodes, got skip=true")
	}
	if desired == nil {
		t.Fatal("expected non-nil desired state")
	}
	if desired.CredentialsReady {
		t.Error("expected CredentialsReady=false when MinioCredentials is nil")
	}
	if desired.AccessKey != "" || desired.SecretKey != "" {
		t.Error("expected empty credentials in degraded contract")
	}
	// Endpoint should be resolved from the pool.
	if !desired.EndpointReady {
		t.Error("expected EndpointReady=true when pool contains a valid IP")
	}
	if desired.Endpoint != "10.0.0.63:9000" {
		t.Errorf("expected endpoint=10.0.0.63:9000, got %q", desired.Endpoint)
	}
}

// TestBuildObjectStore_EmptyPool_ZeroGeneration_Skips verifies the only valid
// skip condition: pool is empty AND generation is 0 (truly pre-formation).
func TestBuildObjectStore_EmptyPool_ZeroGeneration_Skips(t *testing.T) {
	srv := &server{
		state: &controllerState{
			MinioPoolNodes:        nil,
			ObjectStoreGeneration: 0,
			MinioCredentials:      &minioCredentials{RootUser: "ak", RootPassword: "sk"},
		},
	}
	_, skip := srv.buildObjectStoreDesiredStateLocked()
	if !skip {
		t.Error("expected skip=true for empty pool with zero generation")
	}
}

// TestBuildObjectStore_PoolPresent_DNSNode_EndpointUnresolved verifies that
// a DNS hostname in MinioPoolNodes (which resolveMinioEndpointLocked rejects)
// causes EndpointReady=false but the contract is still published.
func TestBuildObjectStore_PoolPresent_DNSNode_EndpointUnresolved(t *testing.T) {
	srv := &server{
		state: &controllerState{
			MinioPoolNodes:        []string{"minio.globular.internal"}, // DNS, not IP
			ObjectStoreGeneration: 2,
			MinioCredentials:      &minioCredentials{RootUser: "ak", RootPassword: "sk"},
		},
	}
	desired, skip := srv.buildObjectStoreDesiredStateLocked()
	if skip {
		t.Fatal("expected contract to be built even when endpoint is unresolved")
	}
	if desired == nil {
		t.Fatal("expected non-nil desired state")
	}
	if desired.EndpointReady {
		t.Error("expected EndpointReady=false when pool node is a DNS hostname")
	}
	if desired.Endpoint != "" {
		t.Errorf("expected empty endpoint for DNS node, got %q", desired.Endpoint)
	}
	// Credentials are present — CredentialsReady must be true.
	if !desired.CredentialsReady {
		t.Error("expected CredentialsReady=true when credentials are loaded")
	}
}

// TestBuildObjectStore_FullyReady verifies the happy path: pool has a valid IP
// and credentials are loaded → both ready flags true.
func TestBuildObjectStore_FullyReady(t *testing.T) {
	srv := &server{
		state: &controllerState{
			MinioPoolNodes:        []string{"10.0.0.63"},
			ObjectStoreGeneration: 3,
			MinioCredentials:      &minioCredentials{RootUser: "ak", RootPassword: "sk"},
		},
	}
	desired, skip := srv.buildObjectStoreDesiredStateLocked()
	if skip {
		t.Fatal("unexpected skip for fully-configured state")
	}
	if !desired.CredentialsReady {
		t.Error("expected CredentialsReady=true")
	}
	if !desired.EndpointReady {
		t.Error("expected EndpointReady=true")
	}
	if desired.Mode != config.ObjectStoreModeStandalone {
		t.Errorf("expected standalone for 1-node pool, got %v", desired.Mode)
	}
}

// TestBuildObjectStore_GenerationNonZeroEmptyPool verifies that a non-zero
// generation with an empty pool still publishes (topology was set, pool was
// cleared — a degraded but contractually present state).
func TestBuildObjectStore_GenerationNonZeroEmptyPool(t *testing.T) {
	srv := &server{
		state: &controllerState{
			MinioPoolNodes:        nil,
			ObjectStoreGeneration: 1, // generation written but pool empty
			MinioCredentials:      &minioCredentials{RootUser: "ak", RootPassword: "sk"},
		},
	}
	desired, skip := srv.buildObjectStoreDesiredStateLocked()
	if skip {
		t.Fatal("expected contract to be built when generation > 0 even with empty pool")
	}
	if desired.EndpointReady {
		t.Error("expected EndpointReady=false with empty pool")
	}
	if desired.Generation != 1 {
		t.Errorf("expected generation=1, got %d", desired.Generation)
	}
}
