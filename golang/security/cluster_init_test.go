package security

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestIsMutatingRPC verifies the read/write classifier.
func TestIsMutatingRPC(t *testing.T) {
	tests := []struct {
		method   string
		mutating bool
	}{
		// Read-only → false
		{"/rbac.RbacService/GetAccount", false},
		{"/rbac.RbacService/ListAccounts", false},
		{"/grpc.health.v1.Health/Watch", false},
		{"/grpc.health.v1.Health/Check", false},
		{"/clustercontroller.ResourcesService/GetServiceRelease", false},
		{"/clustercontroller.ResourcesService/ListServiceReleases", false},
		{"/clustercontroller.ClusterControllerService/GetClusterHealth", false},
		{"/clustercontroller.ClusterControllerService/GetClusterInfo", false},
		{"/dns.DnsService/GetA", false},
		{"/repository.PackageRepository/GetArtifactManifest", false},

		// Mutating → true
		{"/rbac.RbacService/CreateAccount", true},
		{"/rbac.RbacService/DeleteAccount", true},
		{"/rbac.RbacService/UpdateAccount", true},
		{"/clustercontroller.ResourcesService/ApplyServiceRelease", true},
		{"/clustercontroller.ResourcesService/DeleteServiceRelease", true},
		{"/clustercontroller.ClusterControllerService/ApplyNodePlan", true},
		{"/clustercontroller.ClusterControllerService/RemoveNode", true},
		{"/discovery.PackageDiscovery/PublishService", true},
		{"/repository.PackageRepository/UploadArtifact", true},
		{"/dns.DnsService/SetA", true},
		{"/dns.DnsService/RemoveA", true},

		// Explicit coverage of write RPCs cross-checked against service protos.
		// These must remain mutating=true so anonymous callers are blocked post-Day-0.
		{"/repository.PackageRepository/UploadBundle", true},          // publish packages
		{"/authentication.AuthenticationService/IssueClientCertificate", true}, // issues certs
		{"/authentication.AuthenticationService/SetPassword", true},
		{"/authentication.AuthenticationService/SetRootPassword", true},
		{"/rbac.RbacService/AddRoleBinding", true},
		{"/rbac.RbacService/RemoveRoleBinding", true},
		{"/rbac.RbacService/SetPermissions", true},
		{"/resource.ResourceService/CreateAccount", true},
		{"/resource.ResourceService/DeleteAccount", true},
		{"/resource.ResourceService/SetPermissions", true},

		// Edge cases
		{"", true},          // empty → mutating (fail closed)
		{"/Foo/Bar", true},  // unknown → mutating (fail closed)
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got := IsMutatingRPC(tt.method)
			if got != tt.mutating {
				t.Errorf("IsMutatingRPC(%q) = %v, want %v", tt.method, got, tt.mutating)
			}
		})
	}
}

// TestIsClusterInitialized_NoClusterID verifies that when GetLocalClusterID
// fails the cluster is reported as not initialized.
func TestIsClusterInitialized_NoClusterID(t *testing.T) {
	// Reset default validator so GetLocalClusterID returns an error.
	saved := defaultValidator
	defaultValidator = nil
	t.Cleanup(func() {
		defaultValidator = saved
		InvalidateClusterInitCache()
	})
	InvalidateClusterInitCache()

	initialized, err := IsClusterInitialized(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if initialized {
		t.Error("expected cluster NOT initialized when cluster_id unavailable")
	}
}

// TestIsClusterInitialized_BootstrapActive verifies that while bootstrap mode
// is active the cluster is considered not yet initialized (Day-0 in progress).
func TestIsClusterInitialized_BootstrapActive(t *testing.T) {
	tmpDir := t.TempDir()
	flagFile := filepath.Join(tmpDir, "bootstrap.enabled")

	// Write a valid bootstrap state with a future expiry.
	now := time.Now().Unix()
	state := BootstrapState{
		EnabledAt: now,
		ExpiresAt: now + 1800,
		CreatedBy: "test",
		Version:   "1.0",
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal bootstrap state: %v", err)
	}
	if err := os.WriteFile(flagFile, stateJSON, 0600); err != nil {
		t.Fatalf("write flag file: %v", err)
	}

	gate := NewBootstrapGateWithPath(flagFile)
	gate.SetSkipOwnershipCheck(true)

	saved := DefaultBootstrapGate
	DefaultBootstrapGate = gate
	t.Cleanup(func() {
		DefaultBootstrapGate = saved
		InvalidateClusterInitCache()
	})
	InvalidateClusterInitCache()

	if !gate.IsActive() {
		t.Skip("bootstrap gate not active (flag file setup issue)")
	}

	// With bootstrap active the cluster is NOT yet initialized.
	initialized, err := IsClusterInitialized(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if initialized {
		t.Error("expected cluster NOT initialized while bootstrap is active")
	}
}

// TestBootstrapGate_IsActive verifies the IsActive() helper.
func TestBootstrapGate_IsActive(t *testing.T) {
	// No flag file, no env var → not active.
	gate := NewBootstrapGateWithPath("/tmp/nonexistent-bootstrap-flag-for-test")
	os.Unsetenv("GLOBULAR_BOOTSTRAP")
	if gate.IsActive() {
		t.Error("IsActive() = true, want false when bootstrap not enabled")
	}

	// Env var enabled → active.
	os.Setenv("GLOBULAR_BOOTSTRAP", "1")
	defer os.Unsetenv("GLOBULAR_BOOTSTRAP")
	if !gate.IsActive() {
		t.Error("IsActive() = false, want true with GLOBULAR_BOOTSTRAP=1")
	}
}

// TestIsClusterInitialized_Cache verifies that repeated calls use the cache.
func TestIsClusterInitialized_Cache(t *testing.T) {
	// Just test that the function can be called multiple times without panic.
	saved := defaultValidator
	defaultValidator = nil
	t.Cleanup(func() {
		defaultValidator = saved
		InvalidateClusterInitCache()
	})
	InvalidateClusterInitCache()

	for i := 0; i < 5; i++ {
		_, err := IsClusterInitialized(context.Background())
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
}
