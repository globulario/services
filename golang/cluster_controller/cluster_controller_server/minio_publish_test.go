package main

import (
	"testing"
	"time"

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

// TestBuildObjectStore_EmptyPool_ZeroGeneration_PublishesDegradedContract
// verifies Day-0 behavior: even pre-formation state publishes a degraded
// contract so /globular/objectstore/config is present with EndpointReady=false.
func TestBuildObjectStore_EmptyPool_ZeroGeneration_PublishesDegradedContract(t *testing.T) {
	srv := &server{
		state: &controllerState{
			MinioPoolNodes:        nil,
			ObjectStoreGeneration: 0,
			MinioCredentials:      &minioCredentials{RootUser: "ak", RootPassword: "sk"},
		},
	}
	desired, skip := srv.buildObjectStoreDesiredStateLocked()
	if skip {
		t.Fatal("expected skip=false for empty pool Day-0 baseline contract")
	}
	if desired == nil {
		t.Fatal("expected non-nil desired state")
	}
	if desired.EndpointReady {
		t.Error("expected EndpointReady=false for empty pool")
	}
	if desired.Endpoint != "" {
		t.Errorf("expected empty endpoint for empty pool, got %q", desired.Endpoint)
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

// TestBuildObjectStore_FiltersStalePoolIPs verifies that desired-state publish
// excludes stale pool entries that no longer belong to eligible cluster nodes.
func TestBuildObjectStore_FiltersStalePoolIPs(t *testing.T) {
	srv := &server{
		state: &controllerState{
			MinioPoolNodes:        []string{"10.0.0.63", "10.0.0.9", "10.0.0.20"},
			ObjectStoreGeneration: 7,
			MinioCredentials:      &minioCredentials{RootUser: "ak", RootPassword: "sk"},
			Nodes: map[string]*nodeState{
				"ryzen": {NodeID: "ryzen", Status: "healthy", Identity: storedIdentity{Ips: []string{"10.0.0.63"}}},
				"dell":  {NodeID: "dell", Status: "active", Identity: storedIdentity{Ips: []string{"10.0.0.20"}}},
			},
		},
	}
	now := time.Now()
	srv.state.Nodes["ryzen"].LastSeen = now
	srv.state.Nodes["dell"].LastSeen = now
	desired, skip := srv.buildObjectStoreDesiredStateLocked()
	if skip {
		t.Fatal("expected contract to be built")
	}
	if got, want := len(desired.Nodes), 2; got != want {
		t.Fatalf("expected %d pool nodes after filtering, got %d: %v", want, got, desired.Nodes)
	}
	if desired.Nodes[0] != "10.0.0.63" || desired.Nodes[1] != "10.0.0.20" {
		t.Fatalf("unexpected filtered pool order/content: %v", desired.Nodes)
	}
	if desired.Endpoint != "10.0.0.63:9000" {
		t.Fatalf("expected endpoint to use first remaining pool node, got %q", desired.Endpoint)
	}
}

// TestMigratePoolNodeHostnames verifies that stale FQDN/hostname entries in
// MinioPoolNodes are replaced with the routable IP of the matching node.
// This covers clusters where an older code version wrote hostnames instead of IPs,
// which caused resolveMinioEndpointLocked to permanently reject the endpoint.
func TestMigratePoolNodeHostnames(t *testing.T) {
	cases := []struct {
		name        string
		poolNodes   []string
		nodes       map[string]*nodeState
		wantPool    []string
		wantLogged  bool // whether a replacement should have occurred
	}{
		{
			name:      "FQDN replaced with IP via advertise_fqdn",
			poolNodes: []string{"globule-ryzen.globular.internal"},
			nodes: map[string]*nodeState{
				"ryzen": {
					NodeID:        "ryzen",
					Identity:      storedIdentity{Hostname: "globule-ryzen", Ips: []string{"10.0.0.63"}},
					AdvertiseFqdn: "globule-ryzen.globular.internal",
				},
			},
			wantPool:   []string{"10.0.0.63"},
			wantLogged: true,
		},
		{
			name:      "short hostname replaced via identity hostname",
			poolNodes: []string{"globule-ryzen"},
			nodes: map[string]*nodeState{
				"ryzen": {
					NodeID:   "ryzen",
					Identity: storedIdentity{Hostname: "globule-ryzen", Ips: []string{"10.0.0.63"}},
				},
			},
			wantPool:   []string{"10.0.0.63"},
			wantLogged: true,
		},
		{
			name:      "valid IP left unchanged",
			poolNodes: []string{"10.0.0.63"},
			nodes: map[string]*nodeState{
				"ryzen": {
					NodeID:   "ryzen",
					Identity: storedIdentity{Hostname: "globule-ryzen", Ips: []string{"10.0.0.63"}},
				},
			},
			wantPool:   []string{"10.0.0.63"},
			wantLogged: false,
		},
		{
			name:      "mixed pool: FQDN and existing IP",
			poolNodes: []string{"globule-ryzen.globular.internal", "10.0.0.8"},
			nodes: map[string]*nodeState{
				"ryzen": {
					NodeID:        "ryzen",
					Identity:      storedIdentity{Hostname: "globule-ryzen", Ips: []string{"10.0.0.63"}},
					AdvertiseFqdn: "globule-ryzen.globular.internal",
				},
				"nuc": {
					NodeID:   "nuc",
					Identity: storedIdentity{Hostname: "globule-nuc", Ips: []string{"10.0.0.8"}},
				},
			},
			wantPool:   []string{"10.0.0.63", "10.0.0.8"},
			wantLogged: true,
		},
		{
			name:      "unresolvable hostname left as-is",
			poolNodes: []string{"unknown-host.globular.internal"},
			nodes:     map[string]*nodeState{},
			wantPool:  []string{"unknown-host.globular.internal"},
			wantLogged: false, // just a warning, no change
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := &controllerState{
				MinioPoolNodes: append([]string(nil), tc.poolNodes...),
				Nodes:          tc.nodes,
			}
			migratePoolNodeHostnames(state)
			if len(state.MinioPoolNodes) != len(tc.wantPool) {
				t.Fatalf("pool length: got %d want %d: %v", len(state.MinioPoolNodes), len(tc.wantPool), state.MinioPoolNodes)
			}
			for i, want := range tc.wantPool {
				if state.MinioPoolNodes[i] != want {
					t.Errorf("pool[%d]: got %q want %q", i, state.MinioPoolNodes[i], want)
				}
			}
		})
	}
}

func TestBuildObjectStore_ExcludesStaleAndNonMemberPoolIPs(t *testing.T) {
	now := time.Now()
	srv := &server{
		state: &controllerState{
			MinioPoolNodes:        []string{"10.0.0.63", "10.0.0.9", "10.0.0.102", "10.0.0.20"},
			ObjectStoreGeneration: 11,
			MinioCredentials:      &minioCredentials{RootUser: "ak", RootPassword: "sk"},
			Nodes: map[string]*nodeState{
				"ryzen": {
					NodeID:         "ryzen",
					Status:         "healthy",
					LastSeen:       now,
					MinioJoinPhase: MinioJoinVerified,
					Identity:       storedIdentity{Ips: []string{"10.0.0.63"}},
				},
				"dell": {
					NodeID:         "dell",
					Status:         "active",
					LastSeen:       now,
					MinioJoinPhase: MinioJoinVerified,
					Identity:       storedIdentity{Ips: []string{"10.0.0.20"}},
				},
				"old-non-member": {
					NodeID:         "old-non-member",
					Status:         "active",
					LastSeen:       now,
					MinioJoinPhase: MinioJoinNonMember,
					Identity:       storedIdentity{Ips: []string{"10.0.0.9"}},
				},
				"old-stale": {
					NodeID:         "old-stale",
					Status:         "active",
					LastSeen:       now.Add(-(heartbeatStaleThreshold + time.Minute)),
					MinioJoinPhase: MinioJoinVerified,
					Identity:       storedIdentity{Ips: []string{"10.0.0.102"}},
				},
			},
		},
	}

	desired, skip := srv.buildObjectStoreDesiredStateLocked()
	if skip {
		t.Fatal("expected contract to be built")
	}
	if got, want := len(desired.Nodes), 2; got != want {
		t.Fatalf("expected %d pool nodes after filtering, got %d: %v", want, got, desired.Nodes)
	}
	if desired.Nodes[0] != "10.0.0.63" || desired.Nodes[1] != "10.0.0.20" {
		t.Fatalf("unexpected filtered pool order/content: %v", desired.Nodes)
	}
}
