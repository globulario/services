package security

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc/peer"
)

// TestBootstrapGate_Disabled verifies that bootstrap is denied when not enabled
func TestBootstrapGate_Disabled(t *testing.T) {
	gate := NewBootstrapGate()
	gate.flagFilePath = "/tmp/nonexistent-bootstrap-flag"

	// Ensure env var is not set
	os.Unsetenv("GLOBULAR_BOOTSTRAP")

	authCtx := &AuthContext{
		GRPCMethod: "/rbac.RbacService/CreateAccount",
		IsLoopback: true,
	}

	allowed, reason := gate.ShouldAllow(authCtx)
	if allowed {
		t.Error("ShouldAllow() = true, want false when bootstrap not enabled")
	}
	if reason != "bootstrap_not_enabled" {
		t.Errorf("reason = %q, want \"bootstrap_not_enabled\"", reason)
	}
}

// TestBootstrapGate_EnvVar verifies bootstrap works with env var
func TestBootstrapGate_EnvVar(t *testing.T) {
	gate := NewBootstrapGate()
	gate.flagFilePath = "/tmp/nonexistent-bootstrap-flag"

	// Enable via env var
	os.Setenv("GLOBULAR_BOOTSTRAP", "1")
	defer os.Unsetenv("GLOBULAR_BOOTSTRAP")

	authCtx := &AuthContext{
		GRPCMethod: "/rbac.RbacService/CreateAccount",
		IsLoopback: true,
	}

	allowed, reason := gate.ShouldAllow(authCtx)
	if !allowed {
		t.Errorf("ShouldAllow() = false, want true with env var enabled (reason: %s)", reason)
	}
	if reason != "bootstrap_allowed" {
		t.Errorf("reason = %q, want \"bootstrap_allowed\"", reason)
	}
}

// TestBootstrapGate_FlagFile verifies bootstrap works with flag file
func TestBootstrapGate_FlagFile(t *testing.T) {
	// Security Fix #4: Create temp flag file with valid JSON state
	tmpDir := t.TempDir()
	flagFile := filepath.Join(tmpDir, "bootstrap.enabled")

	// Create valid bootstrap state (within time window)
	now := time.Now().Unix()
	state := BootstrapState{
		EnabledAt:  now,
		ExpiresAt:  now + 1800, // 30 minutes from now
		Nonce:      "test-nonce",
		CreatedBy:  "test",
		Version:    "1.0",
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}

	// Write with correct permissions (0600)
	if err := os.WriteFile(flagFile, stateJSON, 0600); err != nil {
		t.Fatalf("failed to create flag file: %v", err)
	}

	gate := NewBootstrapGate()
	gate.flagFilePath = flagFile

	// Ensure env var is not set
	os.Unsetenv("GLOBULAR_BOOTSTRAP")

	authCtx := &AuthContext{
		GRPCMethod: "/rbac.RbacService/CreateAccount",
		IsLoopback: true,
	}

	allowed, reason := gate.ShouldAllow(authCtx)
	if !allowed {
		t.Errorf("ShouldAllow() = false, want true with flag file (reason: %s)", reason)
	}
	if reason != "bootstrap_allowed" {
		t.Errorf("reason = %q, want \"bootstrap_allowed\"", reason)
	}
}

// TestBootstrapGate_Expired verifies time window enforcement
func TestBootstrapGate_Expired(t *testing.T) {
	// Security Fix #4: Create temp flag file with expired JSON state
	tmpDir := t.TempDir()
	flagFile := filepath.Join(tmpDir, "bootstrap.enabled")

	// Create bootstrap state that expired 1 minute ago
	now := time.Now().Unix()
	state := BootstrapState{
		EnabledAt:  now - 1860, // 31 minutes ago
		ExpiresAt:  now - 60,   // 1 minute ago (EXPIRED)
		Nonce:      "test-nonce",
		CreatedBy:  "test",
		Version:    "1.0",
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}

	// Write with correct permissions (0600)
	if err := os.WriteFile(flagFile, stateJSON, 0600); err != nil {
		t.Fatalf("failed to create flag file: %v", err)
	}

	gate := NewBootstrapGate()
	gate.flagFilePath = flagFile

	// Ensure env var is not set
	os.Unsetenv("GLOBULAR_BOOTSTRAP")

	authCtx := &AuthContext{
		GRPCMethod: "/rbac.RbacService/CreateAccount",
		IsLoopback: true,
	}

	allowed, reason := gate.ShouldAllow(authCtx)
	if allowed {
		t.Error("ShouldAllow() = true, want false for expired bootstrap")
	}
	if reason != "bootstrap_expired" {
		t.Errorf("reason = %q, want \"bootstrap_expired\"", reason)
	}
}

// TestBootstrapGate_NonLoopback verifies loopback-only enforcement
func TestBootstrapGate_NonLoopback(t *testing.T) {
	gate := NewBootstrapGate()

	// Enable via env var
	os.Setenv("GLOBULAR_BOOTSTRAP", "1")
	defer os.Unsetenv("GLOBULAR_BOOTSTRAP")

	authCtx := &AuthContext{
		GRPCMethod: "/rbac.RbacService/CreateAccount",
		IsLoopback: false, // Remote request
	}

	allowed, reason := gate.ShouldAllow(authCtx)
	if allowed {
		t.Error("ShouldAllow() = true, want false for non-loopback request")
	}
	if reason != "bootstrap_remote" {
		t.Errorf("reason = %q, want \"bootstrap_remote\"", reason)
	}
}

// TestBootstrapGate_MethodNotAllowed verifies method allowlist enforcement
func TestBootstrapGate_MethodNotAllowed(t *testing.T) {
	gate := NewBootstrapGate()

	// Enable via env var
	os.Setenv("GLOBULAR_BOOTSTRAP", "1")
	defer os.Unsetenv("GLOBULAR_BOOTSTRAP")

	// Try to access a non-allowlisted method
	authCtx := &AuthContext{
		GRPCMethod: "/file.FileService/DeleteFile", // NOT in allowlist
		IsLoopback: true,
	}

	allowed, reason := gate.ShouldAllow(authCtx)
	if allowed {
		t.Error("ShouldAllow() = true, want false for non-allowlisted method")
	}
	if reason != "bootstrap_method_blocked" {
		t.Errorf("reason = %q, want \"bootstrap_method_blocked\"", reason)
	}
}

// TestBootstrapGate_AllowedMethods verifies that all allowlisted methods work
func TestBootstrapGate_AllowedMethods(t *testing.T) {
	gate := NewBootstrapGate()

	// Enable via env var
	os.Setenv("GLOBULAR_BOOTSTRAP", "1")
	defer os.Unsetenv("GLOBULAR_BOOTSTRAP")

	allowedMethods := []string{
		"/grpc.health.v1.Health/Check",
		"/rbac.RbacService/CreateAccount",
		"/rbac.RbacService/CreateRole",
		"/authentication.AuthenticationService/Authenticate",
		"/resource.ResourceService/CreatePeer",
		"/dns.DnsService/CreateZone",
	}

	for _, method := range allowedMethods {
		t.Run(method, func(t *testing.T) {
			authCtx := &AuthContext{
				GRPCMethod: method,
				IsLoopback: true,
			}

			allowed, reason := gate.ShouldAllow(authCtx)
			if !allowed {
				t.Errorf("ShouldAllow() = false for allowed method %q (reason: %s)", method, reason)
			}
		})
	}
}

// TestBootstrapGate_FourGatesOrdered verifies gates are checked in order
func TestBootstrapGate_FourGatesOrdered(t *testing.T) {
	tests := []struct {
		name           string
		setupGate      func(*BootstrapGate)
		setupEnv       func()
		cleanupEnv     func()
		authCtx        *AuthContext
		wantAllowed    bool
		wantReason     string
		description    string
	}{
		{
			name: "Gate 1 fails - not enabled",
			setupGate: func(g *BootstrapGate) {
				g.flagFilePath = "/tmp/nonexistent"
			},
			setupEnv:   func() { os.Unsetenv("GLOBULAR_BOOTSTRAP") },
			cleanupEnv: func() {},
			authCtx: &AuthContext{
				GRPCMethod: "/rbac.RbacService/CreateAccount",
				IsLoopback: true,
			},
			wantAllowed: false,
			wantReason:  "bootstrap_not_enabled",
			description: "Should fail at gate 1 (enablement)",
		},
		{
			name: "Gate 2 fails - expired",
			setupGate: func(g *BootstrapGate) {
				// Create expired flag file
				tmpDir := os.TempDir()
				flagFile := filepath.Join(tmpDir, "test-bootstrap-expired")
				os.WriteFile(flagFile, []byte(""), 0644)
				oldTime := time.Now().Add(-31 * time.Minute)
				os.Chtimes(flagFile, oldTime, oldTime)
				g.flagFilePath = flagFile
			},
			setupEnv:   func() { os.Unsetenv("GLOBULAR_BOOTSTRAP") },
			cleanupEnv: func() {},
			authCtx: &AuthContext{
				GRPCMethod: "/rbac.RbacService/CreateAccount",
				IsLoopback: true,
			},
			wantAllowed: false,
			wantReason:  "bootstrap_expired",
			description: "Should fail at gate 2 (time window)",
		},
		{
			name:      "Gate 3 fails - remote",
			setupGate: func(g *BootstrapGate) {},
			setupEnv: func() {
				os.Setenv("GLOBULAR_BOOTSTRAP", "1")
			},
			cleanupEnv: func() {
				os.Unsetenv("GLOBULAR_BOOTSTRAP")
			},
			authCtx: &AuthContext{
				GRPCMethod: "/rbac.RbacService/CreateAccount",
				IsLoopback: false, // Remote!
			},
			wantAllowed: false,
			wantReason:  "bootstrap_remote",
			description: "Should fail at gate 3 (loopback-only)",
		},
		{
			name:      "Gate 4 fails - method blocked",
			setupGate: func(g *BootstrapGate) {},
			setupEnv: func() {
				os.Setenv("GLOBULAR_BOOTSTRAP", "1")
			},
			cleanupEnv: func() {
				os.Unsetenv("GLOBULAR_BOOTSTRAP")
			},
			authCtx: &AuthContext{
				GRPCMethod: "/file.FileService/DeleteFile", // Not allowed!
				IsLoopback: true,
			},
			wantAllowed: false,
			wantReason:  "bootstrap_method_blocked",
			description: "Should fail at gate 4 (method allowlist)",
		},
		{
			name:      "All gates pass",
			setupGate: func(g *BootstrapGate) {},
			setupEnv: func() {
				os.Setenv("GLOBULAR_BOOTSTRAP", "1")
			},
			cleanupEnv: func() {
				os.Unsetenv("GLOBULAR_BOOTSTRAP")
			},
			authCtx: &AuthContext{
				GRPCMethod: "/rbac.RbacService/CreateAccount",
				IsLoopback: true,
			},
			wantAllowed: true,
			wantReason:  "bootstrap_allowed",
			description: "Should pass all 4 gates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gate := NewBootstrapGate()
			tt.setupGate(gate)
			tt.setupEnv()
			defer tt.cleanupEnv()

			allowed, reason := gate.ShouldAllow(tt.authCtx)

			if allowed != tt.wantAllowed {
				t.Errorf("%s: allowed = %v, want %v", tt.description, allowed, tt.wantAllowed)
			}
			if reason != tt.wantReason {
				t.Errorf("%s: reason = %q, want %q", tt.description, reason, tt.wantReason)
			}
		})
	}
}

// TestBootstrapGate_GetBootstrapStatus verifies status reporting
func TestBootstrapGate_GetBootstrapStatus(t *testing.T) {
	gate := NewBootstrapGate()
	gate.flagFilePath = "/tmp/nonexistent"

	// Disabled
	os.Unsetenv("GLOBULAR_BOOTSTRAP")
	status := gate.GetBootstrapStatus()
	if status != "disabled" {
		t.Errorf("GetBootstrapStatus() = %q, want \"disabled\"", status)
	}

	// Enabled via env var
	os.Setenv("GLOBULAR_BOOTSTRAP", "1")
	defer os.Unsetenv("GLOBULAR_BOOTSTRAP")
	status = gate.GetBootstrapStatus()
	if status != "enabled (env_var)" {
		t.Errorf("GetBootstrapStatus() = %q, want \"enabled (env_var)\"", status)
	}
}

// TestBootstrapGate_IntegrationWithAuthContext verifies end-to-end flow
func TestBootstrapGate_IntegrationWithAuthContext(t *testing.T) {
	// Enable bootstrap mode
	os.Setenv("GLOBULAR_BOOTSTRAP", "1")
	defer os.Unsetenv("GLOBULAR_BOOTSTRAP")

	// Create loopback peer context
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:12345")
	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: addr})

	// Create AuthContext
	authCtx, err := NewAuthContext(ctx, "/rbac.RbacService/CreateAccount")
	if err != nil {
		t.Fatalf("NewAuthContext() error = %v", err)
	}

	// Verify loopback detected
	if !authCtx.IsLoopback {
		t.Error("AuthContext.IsLoopback = false, want true for 127.0.0.1")
	}

	// Check bootstrap gate
	gate := DefaultBootstrapGate
	allowed, reason := gate.ShouldAllow(authCtx)
	if !allowed {
		t.Errorf("ShouldAllow() = false, want true (reason: %s)", reason)
	}
	if reason != "bootstrap_allowed" {
		t.Errorf("reason = %q, want \"bootstrap_allowed\"", reason)
	}
}
